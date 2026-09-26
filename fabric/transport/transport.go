package transport

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

const (
	// A proposal carries a whole block. With MaxMessageCount 500 that is ~2 MB
	// of JSON, which the old 1 MiB cap silently discarded. Match Fabric's own
	// 100 MiB gRPC message limit (AbsoluteMaxBytes is 99 MB).
	maxFrameSize = 100 << 20
	// Consensus messages (votes, proposals, block requests) must survive an
	// outage: an orderer that misses votes cannot replay those rounds and stays
	// behind. They get a deep queue that is always written first. Transaction
	// gossip is bulky and can be dropped safely - a block naming a transaction
	// we never got is fetched whole instead - so it gets a shallow one.
	sendQueueSize   = 65536
	gossipQueueSize = 4096
	minBackoff      = time.Second
	maxBackoff      = 5 * time.Second // retry often, so a healed link reconnects fast
	// userTimeoutMs bounds how long sent data may go unacknowledged before
	// the kernel drops the connection (see setUserTimeout). 5 s proved too
	// short: under heavy load a busy receiver can stall a healthy link that
	// long, and each needless drop loses the votes in flight. 20 s still ends
	// a dead link well before TCP's own back-off would.
	userTimeoutMs = 20000
)

// Logger is a simple interface for logging to avoid external dependencies.
type Logger interface {
	Printf(format string, v ...interface{})
}

// Transport provides a simple TCP peer-to-peer network for consensus messages.
type Transport struct {
	nodeID     string
	listenAddr string
	peerAddrs  []string
	log        Logger

	listener net.Listener
	incoming chan []byte

	peersMutex sync.RWMutex
	peers      map[string]*peerConn // peerAddr -> peerConn

	wg       sync.WaitGroup
	quitChan chan struct{}

	connsMutex sync.Mutex
	conns      []net.Conn
}

// peerConn represents a connection to a specific remote peer.
type peerConn struct {
	addr    string
	sendQ   chan []byte
	gossipQ chan []byte
	conn    net.Conn
	mu      sync.Mutex
}

// New creates a new TCP Transport. It deduplicates self (listenAddr) from peerAddrs.
func New(nodeID, listenAddr string, peerAddrs []string, log Logger) *Transport {
	uniquePeers := make(map[string]bool)
	var filteredPeers []string

	_, listenPort, err := net.SplitHostPort(listenAddr)
	if err != nil {
		listenPort = ""
	}

	localIPs := make(map[string]bool)
	ifaces, err := net.InterfaceAddrs()
	if err == nil {
		for _, i := range ifaces {
			if ipnet, ok := i.(*net.IPNet); ok {
				localIPs[ipnet.IP.String()] = true
			}
		}
	}
	localIPs["127.0.0.1"] = true
	localIPs["::1"] = true
	localIPs["0.0.0.0"] = true

	for _, addr := range peerAddrs {
		if addr == listenAddr || uniquePeers[addr] {
			continue
		}

		host, port, err := net.SplitHostPort(addr)
		if err == nil && port == listenPort {
			isLocal := false
			if host == nodeID || localIPs[host] {
				isLocal = true
			} else {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
				cancel()
				if err == nil {
					for _, ip := range ips {
						if localIPs[ip.String()] {
							isLocal = true
							break
						}
					}
				} else {
					log.Printf("[Transport] DNS lookup for %s timed out or failed (%v). Keeping peer tentatively.", host, err)
				}
			}
			if isLocal {
				continue
			}
		}

		uniquePeers[addr] = true
		filteredPeers = append(filteredPeers, addr)
	}

	return &Transport{
		nodeID:     nodeID,
		listenAddr: listenAddr,
		peerAddrs:  filteredPeers,
		log:        log,
		incoming:   make(chan []byte, 4096),
		peers:      make(map[string]*peerConn),
		quitChan:   make(chan struct{}),
	}
}

// Start begins listening for incoming connections and dialing remote peers.
func (t *Transport) Start() error {
	l, err := net.Listen("tcp", t.listenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", t.listenAddr, err)
	}
	t.listener = l

	t.log.Printf("[Transport] %s listening on %s", t.nodeID, t.listenAddr)

	t.wg.Add(1)
	go t.acceptLoop()

	for _, addr := range t.peerAddrs {
		pc := &peerConn{
			addr:    addr,
			sendQ:   make(chan []byte, sendQueueSize),
			gossipQ: make(chan []byte, gossipQueueSize),
		}
		t.peersMutex.Lock()
		t.peers[addr] = pc
		t.peersMutex.Unlock()

		t.wg.Add(1)
		go t.dialLoop(pc)
	}

	return nil
}

// Incoming returns a channel emitting messages received from all peers.
func (t *Transport) Incoming() <-chan []byte {
	return t.incoming
}

// Broadcast sends a consensus message to all peers without blocking. It is
// queued ahead of any gossip and dropped only if a peer is so far behind that
// 65,536 messages are already waiting for it.
func (t *Transport) Broadcast(msg []byte) error {
	return t.enqueue(msg, false)
}

// BroadcastLossy sends bulk gossip (transactions). It waits behind consensus
// traffic and is dropped first when a peer cannot keep up.
func (t *Transport) BroadcastLossy(msg []byte) error {
	return t.enqueue(msg, true)
}

func (t *Transport) enqueue(msg []byte, lossy bool) error {
	t.peersMutex.RLock()
	defer t.peersMutex.RUnlock()

	for _, pc := range t.peers {
		q := pc.sendQ
		if lossy {
			q = pc.gossipQ
		}
		select {
		case q <- msg:
		default:
			if !lossy {
				t.log.Printf("[Transport] Warning: consensus send queue for peer %s is full, dropping message", pc.addr)
			}
		}
	}
	return nil
}

// ConnectedCount returns the number of currently active peer connections.
func (t *Transport) ConnectedCount() int {
	t.peersMutex.RLock()
	defer t.peersMutex.RUnlock()
	count := 0
	for _, pc := range t.peers {
		pc.mu.Lock()
		if pc.conn != nil {
			count++
		}
		pc.mu.Unlock()
	}
	return count
}

// Close gracefully shuts down the transport, halting all connections and loops.
func (t *Transport) Close() error {
	select {
	case <-t.quitChan:
		return nil // already closed
	default:
		close(t.quitChan)
	}

	if t.listener != nil {
		t.listener.Close()
	}

	t.connsMutex.Lock()
	for _, c := range t.conns {
		c.Close()
	}
	t.connsMutex.Unlock()

	t.wg.Wait()
	close(t.incoming)
	return nil
}

// trackConn adds a connection to the list of active connections to be closed on shutdown
func (t *Transport) trackConn(c net.Conn) {
	t.connsMutex.Lock()
	t.conns = append(t.conns, c)
	t.connsMutex.Unlock()
}

// acceptLoop accepts incoming connections and spawns read loops.
func (t *Transport) acceptLoop() {
	defer t.wg.Done()
	for {
		conn, err := t.listener.Accept()
		if err != nil {
			select {
			case <-t.quitChan:
				return
			default:
				t.log.Printf("[Transport] Accept error: %v", err)
				continue
			}
		}

		t.trackConn(conn)
		setUserTimeout(conn, userTimeoutMs)

		t.wg.Add(1)
		go t.readLoop(conn)
	}
}

// dialLoop maintains a persistent outbound connection to a specific peer with exponential backoff.
func (t *Transport) dialLoop(pc *peerConn) {
	defer t.wg.Done()

	backoff := minBackoff

	for {
		select {
		case <-t.quitChan:
			return
		default:
		}

		conn, err := net.DialTimeout("tcp", pc.addr, 5*time.Second)
		if err != nil {
			t.log.Printf("[Transport] Failed to connect to %s, retrying in %s: %v", pc.addr, backoff, err)

			select {
			case <-time.After(backoff):
			case <-t.quitChan:
				return
			}

			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		// Lazy self filtering
		remoteAddr, ok := conn.RemoteAddr().(*net.TCPAddr)
		_, listenPort, _ := net.SplitHostPort(t.listenAddr)
		if ok && fmt.Sprintf("%d", remoteAddr.Port) == listenPort {
			isLocal := false
			if remoteAddr.IP.IsLoopback() || remoteAddr.IP.IsUnspecified() {
				isLocal = true
			} else {
				ifaces, _ := net.InterfaceAddrs()
				for _, i := range ifaces {
					if ipnet, ok := i.(*net.IPNet); ok {
						if ipnet.IP.Equal(remoteAddr.IP) {
							isLocal = true
							break
						}
					}
				}
			}
			if isLocal {
				t.log.Printf("[Transport] Connected to local interface %s. Dropping self-peer.", pc.addr)
				conn.Close()
				return
			}
		}

		t.log.Printf("[Transport] Connected to %s", pc.addr)
		backoff = minBackoff // reset backoff on success
		setUserTimeout(conn, userTimeoutMs)

		t.trackConn(conn)

		pc.mu.Lock()
		pc.conn = conn
		pc.mu.Unlock()

		// Launch a writer specifically for this connection
		writerDone := make(chan struct{})
		t.wg.Add(1)
		go t.writeLoop(pc, conn, writerDone)

		// Read loop to detect disconnects (and technically process bidirectional if desired,
		// but we treat connections as full duplex here).
		t.wg.Add(1)
		t.readLoop(conn)

		// readLoop exited, connection is dead. Clean up.
		pc.mu.Lock()
		pc.conn = nil
		pc.mu.Unlock()
		close(writerDone)

		t.log.Printf("[Transport] Disconnected from %s", pc.addr)
	}
}

// writeLoop processes the send queue and writes frames to the network.
func (t *Transport) writeLoop(pc *peerConn, conn net.Conn, done <-chan struct{}) {
	defer t.wg.Done()

	header := make([]byte, 4)

	write := func(msg []byte) bool {
		binary.BigEndian.PutUint32(header, uint32(len(msg)))
		if _, err := conn.Write(header); err != nil {
			conn.Close() // force readLoop to exit
			return false
		}
		if _, err := conn.Write(msg); err != nil {
			conn.Close()
			return false
		}
		return true
	}

	for {
		// Consensus traffic first; gossip only when none is waiting.
		select {
		case msg := <-pc.sendQ:
			if !write(msg) {
				return
			}
			continue
		default:
		}
		select {
		case <-t.quitChan:
			return
		case <-done:
			return
		case msg := <-pc.sendQ:
			if !write(msg) {
				return
			}
		case msg := <-pc.gossipQ:
			if !write(msg) {
				return
			}
		}
	}
}

// readLoop reads length-prefixed frames from a connection until it drops.
func (t *Transport) readLoop(conn net.Conn) {
	defer t.wg.Done()
	defer conn.Close()

	header := make([]byte, 4)

	for {
		if _, err := io.ReadFull(conn, header); err != nil {
			return
		}

		msgLen := binary.BigEndian.Uint32(header)
		if msgLen > maxFrameSize {
			t.log.Printf("[Transport] Frame too large (%d > %d), discarding", msgLen, maxFrameSize)
			if _, err := io.CopyN(io.Discard, conn, int64(msgLen)); err != nil {
				return
			}
			continue
		}

		msg := make([]byte, msgLen)
		if _, err := io.ReadFull(conn, msg); err != nil {
			return
		}

		select {
		case t.incoming <- msg:
		case <-t.quitChan:
			return
		}
	}
}
