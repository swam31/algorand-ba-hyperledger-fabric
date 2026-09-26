package telemetry

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// RoundStats captures the outcome of a single BA* round.
type RoundStats struct {
	Round      uint64 `json:"round"`
	Steps      uint64 `json:"steps"`
	Outcome    string `json:"outcome"`
	TxCount    int    `json:"txCount"`
	DurationMs int64  `json:"durationMs"`
	Timestamp  int64  `json:"timestamp"` // Unix millis
}

type clientChan chan RoundStats

// VoteStats captures a single vote message sent or received.
type VoteStats struct {
	Timestamp int64  `json:"ts"`
	NodeID    string `json:"nodeId"`
	Dir       string `json:"dir"`
	Peer      string `json:"peer"` // for recv: who it came from; for sent: unused or destination
	Round     uint64 `json:"round"`
	Step      string `json:"step"`
	Value     string `json:"value"` // first 4 bytes of the voted hash
	Seats     uint64 `json:"seats"`
}

type voteChan chan VoteStats

// Server holds the telemetry state and serves HTTP requests.
type Server struct {
	// Static config
	nodeId          string
	channelId       string
	n               int
	w               uint64
	tau             uint64
	threshold       uint64
	tFinal          float64
	h               float64
	p               float64
	toleratedFaults int
	peersConfigured int
	startTime       time.Time

	// Dynamic state using atomics
	ledgerHeight    atomic.Uint64
	lastFinalRound  atomic.Uint64
	roundsTotal     atomic.Uint64
	roundsFinal     atomic.Uint64
	roundsTentative atomic.Uint64
	roundsFailed    atomic.Uint64
	roundsEmpty     atomic.Uint64
	lastBlockHash   atomic.Value // string

	// Mutex protects only lastRound, seatsLastRound, and pubsub clients
	mu sync.RWMutex
	lastRound      *RoundStats
	seatsLastRound uint64
	clients        map[clientChan]struct{}
	history        []RoundStats
	voteClients    map[voteChan]struct{}
	voteHistory    []VoteStats

	// Function to get active connections dynamically
	getConnectedCount func() int
}

// NewServer initializes a telemetry server.
func NewServer(
	nodeId, channelId string,
	n int, w, tau, threshold uint64,
	tFinal, h, p float64,
	toleratedFaults, peersConfigured int,
	getConnectedCount func() int,
) *Server {
	return &Server{
		nodeId:          nodeId,
		channelId:       channelId,
		n:               n,
		w:               w,
		tau:             tau,
		threshold:       threshold,
		tFinal:          tFinal,
		h:               h,
		p:               p,
		toleratedFaults: toleratedFaults,
		peersConfigured: peersConfigured,
		startTime:       time.Now(),
		clients:         make(map[clientChan]struct{}),
		history:         make([]RoundStats, 0, 50),
		voteClients:     make(map[voteChan]struct{}),
		voteHistory:     make([]VoteStats, 0, 2000),
		getConnectedCount: getConnectedCount,
	}
}

// Start launches the HTTP server on the given address.
func (s *Server) Start(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		fmt.Fprint(w, "ok")
	})
	mux.HandleFunc("/status", s.handleStatus)
	mux.HandleFunc("/events", s.handleEvents)
	mux.HandleFunc("/votes", s.handleVotes)

	go func() {
		http.ListenAndServe(addr, mux)
	}()
	return nil
}

// RecordRound updates the telemetry with the results of a completed round.
func (s *Server) RecordRound(round uint64, steps uint64, outcome string, txCount int, durationMs int64, seats uint64) {
	rs := RoundStats{
		Round:      round,
		Steps:      steps,
		Outcome:    outcome,
		TxCount:    txCount,
		DurationMs: durationMs,
		Timestamp:  time.Now().UnixNano() / int64(time.Millisecond),
	}

	s.roundsTotal.Add(1)
	
	switch outcome {
	case "FINAL":
		s.roundsFinal.Add(1)
		for {
			current := s.lastFinalRound.Load()
			if round <= current {
				break
			}
			if s.lastFinalRound.CompareAndSwap(current, round) {
				break
			}
		}
	case "TENTATIVE":
		s.roundsTentative.Add(1)
	case "EMPTY":
		s.roundsEmpty.Add(1)
	default:
		s.roundsFailed.Add(1)
	}

	s.mu.Lock()
	s.lastRound = &rs
	s.seatsLastRound = seats
	
	// Add to history
	if len(s.history) >= 50 {
		s.history = s.history[1:]
	}
	s.history = append(s.history, rs)

	// Broadcast to all clients
	for ch := range s.clients {
		select {
		case ch <- rs:
		default: // drop if channel is full to prevent blocking
		}
	}
	s.mu.Unlock()
}

// SetLastBlockHash records the header hash of the newest block written.
func (s *Server) SetLastBlockHash(h string) {
	s.lastBlockHash.Store(h)
}

// SetLedgerHeight updates the known ledger height.
func (s *Server) SetLedgerHeight(height uint64) {
	s.ledgerHeight.Store(height)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	var lastRound *RoundStats
	if s.lastRound != nil {
		copyRound := *s.lastRound
		lastRound = &copyRound
	}
	seats := s.seatsLastRound
	s.mu.RUnlock()

	connected := 0
	if s.getConnectedCount != nil {
		connected = s.getConnectedCount()
	}

	lastHash, _ := s.lastBlockHash.Load().(string)
	status := map[string]interface{}{
		"nodeId":          s.nodeId,
		"channelId":       s.channelId,
		"n":               s.n,
		"w":               s.w,
		"tau":             s.tau,
		"threshold":       s.threshold,
		"tFinal":          s.tFinal,
		"h":               s.h,
		"p":               s.p,
		"toleratedFaults": s.toleratedFaults,
		"ledgerHeight":    s.ledgerHeight.Load(),
		"lastBlockHash":   lastHash,
		"lastFinalRound":  s.lastFinalRound.Load(),
		"peersConfigured": s.peersConfigured,
		"peersConnected":  connected,
		"rounds": map[string]uint64{
			"total":     s.roundsTotal.Load(),
			"final":     s.roundsFinal.Load(),
			"tentative": s.roundsTentative.Load(),
			"failed":    s.roundsFailed.Load(),
			"empty":     s.roundsEmpty.Load(),
		},
		"lastRound":      lastRound,
		"seatsLastRound": seats,
		"uptimeSeconds":  int(time.Since(s.startTime).Seconds()),
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(status)
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	ch := make(clientChan, 100)
	
	s.mu.Lock()
	s.clients[ch] = struct{}{}
	history := make([]RoundStats, len(s.history))
	copy(history, s.history)
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.clients, ch)
		s.mu.Unlock()
	}()

	// Replay history
	for _, event := range history {
		data, _ := json.Marshal(event)
		fmt.Fprintf(w, "data: %s\n\n", data)
	}
	flusher.Flush()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-ch:
			data, _ := json.Marshal(event)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

func (s *Server) RecordVote(dir string, peer string, round uint64, step string, value string, seats uint64) {
	vs := VoteStats{
		Timestamp: time.Now().UnixNano() / int64(time.Millisecond),
		NodeID:    s.nodeId,
		Dir:       dir,
		Peer:      peer,
		Round:     round,
		Step:      step,
		Value:     value,
		Seats:     seats,
	}

	s.mu.Lock()
	if len(s.voteHistory) >= 2000 {
		s.voteHistory = s.voteHistory[1:]
	}
	s.voteHistory = append(s.voteHistory, vs)

	for ch := range s.voteClients {
		select {
		case ch <- vs:
		default:
		}
	}
	s.mu.Unlock()
}

func (s *Server) handleVotes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	ch := make(voteChan, 2000)
	
	s.mu.Lock()
	s.voteClients[ch] = struct{}{}
	history := make([]VoteStats, len(s.voteHistory))
	copy(history, s.voteHistory)
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.voteClients, ch)
		s.mu.Unlock()
	}()

	for _, event := range history {
		data, _ := json.Marshal(event)
		fmt.Fprintf(w, "data: %s\n\n", data)
	}
	flusher.Flush()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-ch:
			data, _ := json.Marshal(event)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}
