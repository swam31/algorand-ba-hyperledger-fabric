package transport

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"testing"
	"time"
)

type testLogger struct {
	t *testing.T
}

func (l testLogger) Printf(format string, v ...interface{}) {
	l.t.Logf(format, v...)
}

func getEphemeralPort() string {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	addr := l.Addr().String()
	l.Close()
	return addr
}

func TestTransport_BroadcastDelivery(t *testing.T) {
	log := testLogger{t}

	addr1 := getEphemeralPort()
	addr2 := getEphemeralPort()
	addr3 := getEphemeralPort()
	peers := []string{addr1, addr2, addr3}

	node1 := New("node1", addr1, peers, log)
	node2 := New("node2", addr2, peers, log)
	node3 := New("node3", addr3, peers, log)

	if err := node1.Start(); err != nil {
		t.Fatal(err)
	}
	defer node1.Close()

	if err := node2.Start(); err != nil {
		t.Fatal(err)
	}
	defer node2.Close()

	if err := node3.Start(); err != nil {
		t.Fatal(err)
	}
	defer node3.Close()

	// Wait for connections to establish
	time.Sleep(500 * time.Millisecond)

	msg1 := []byte("hello from 1")
	msg2 := []byte("hello from 2")
	msg3 := []byte("hello from 3")

	node1.Broadcast(msg1)
	node2.Broadcast(msg2)
	node3.Broadcast(msg3)

	checkReceived := func(node *Transport, expectedMessages [][]byte) {
		received := make(map[string]bool)
		timeout := time.After(2 * time.Second)
		for len(received) < len(expectedMessages) {
			select {
			case msg := <-node.Incoming():
				received[string(msg)] = true
			case <-timeout:
				t.Fatalf("%s failed to receive all messages. Got: %v", node.nodeID, received)
			}
		}
		for _, msg := range expectedMessages {
			if !received[string(msg)] {
				t.Errorf("%s missing message %s", node.nodeID, msg)
			}
		}
	}

	checkReceived(node1, [][]byte{msg2, msg3})
	checkReceived(node2, [][]byte{msg1, msg3})
	checkReceived(node3, [][]byte{msg1, msg2})
}

func TestTransport_Reconnect(t *testing.T) {
	log := testLogger{t}

	addr1 := getEphemeralPort()
	addr2 := getEphemeralPort()
	peers := []string{addr1, addr2}

	node1 := New("node1", addr1, peers, log)
	node2 := New("node2", addr2, peers, log)

	node1.Start()
	defer node1.Close()

	node2.Start()

	// Wait for connection
	time.Sleep(500 * time.Millisecond)

	msg1 := []byte("before stop")
	node1.Broadcast(msg1)

	select {
	case m := <-node2.Incoming():
		if !bytes.Equal(m, msg1) {
			t.Errorf("expected %s, got %s", msg1, m)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for before stop msg")
	}

	// Stop node2
	node2.Close()
	time.Sleep(500 * time.Millisecond)

	// Restart node2 on the SAME address
	node2 = New("node2", addr2, peers, log)
	node2.Start()
	defer node2.Close()

	// Wait for reconnect
	time.Sleep(2 * time.Second) // allow node1 dial backoff to hit

	msg2 := []byte("after restart")
	node1.Broadcast(msg2)

	select {
	case m := <-node2.Incoming():
		if !bytes.Equal(m, msg2) {
			t.Errorf("expected %s, got %s", msg2, m)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for after restart msg")
	}
}

func TestTransport_OversizeFrame(t *testing.T) {
	log := testLogger{t}

	addr1 := getEphemeralPort()
	addr2 := getEphemeralPort()
	peers := []string{addr1, addr2}

	node1 := New("node1", addr1, peers, log)
	node2 := New("node2", addr2, peers, log)

	node1.Start()
	defer node1.Close()
	node2.Start()
	defer node2.Close()

	time.Sleep(500 * time.Millisecond)

	// Inject a malicious frame manually into node2's listener
	conn, err := net.Dial("tcp", addr2)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// Oversize length
	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, maxFrameSize+10) // > 1 MiB

	// Write header and the full oversized payload
	conn.Write(header)
	conn.Write(make([]byte, maxFrameSize+10))

	// Give it time to process and discard
	time.Sleep(200 * time.Millisecond)

	// Send a valid frame on the SAME connection
	validMsg := []byte("valid msg after oversize")
	binary.BigEndian.PutUint32(header, uint32(len(validMsg)))
	conn.Write(header)
	conn.Write(validMsg)

	select {
	case m := <-node2.Incoming():
		if !bytes.Equal(m, validMsg) {
			t.Errorf("expected %s, got %s", validMsg, m)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for valid msg after oversized frame was discarded")
	}
}

func TestTransport_SelfDialAvoidance(t *testing.T) {
	log := testLogger{t}

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()

	listenAddr := fmt.Sprintf("0.0.0.0:%d", port)
	peerAddr := fmt.Sprintf("127.0.0.1:%d", port)

	node := New("node1", listenAddr, []string{peerAddr}, log)
	if len(node.peerAddrs) != 0 {
		t.Fatalf("expected 0 peerAddrs, got %d: %v", len(node.peerAddrs), node.peerAddrs)
	}

	if err := node.Start(); err != nil {
		t.Fatal(err)
	}
	defer node.Close()

	msg := []byte("self broadcast")
	node.Broadcast(msg)

	select {
	case <-node.Incoming():
		t.Fatal("received own broadcast")
	case <-time.After(500 * time.Millisecond):
		// success
	}
}

// With a peer unreachable, gossip piles up and is shed, but consensus messages
// must still be queued for delivery when the peer comes back.
func TestTransport_GossipCannotCrowdOutConsensus(t *testing.T) {
	tr := New("node1", getEphemeralPort(), []string{getEphemeralPort()}, testLogger{t})
	if err := tr.Start(); err != nil {
		t.Fatal(err)
	}
	defer tr.Close()

	for i := 0; i < gossipQueueSize+500; i++ {
		tr.BroadcastLossy([]byte("tx"))
	}
	tr.Broadcast([]byte("vote"))

	tr.peersMutex.RLock()
	defer tr.peersMutex.RUnlock()
	for _, pc := range tr.peers {
		if len(pc.gossipQ) != gossipQueueSize {
			t.Fatalf("gossip queue holds %d, want it capped at %d", len(pc.gossipQ), gossipQueueSize)
		}
		if len(pc.sendQ) != 1 {
			t.Fatalf("the consensus message was not queued behind a full gossip queue")
		}
	}
}
