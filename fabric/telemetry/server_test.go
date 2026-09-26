package telemetry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTelemetryRace(t *testing.T) {
	s := NewServer("test-node", "test-channel", 4, 1000, 500, 300, 0.75, 0.75, 0.5, 1, 4, func() int { return 3 })

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/status" {
			s.handleStatus(w, r)
		}
	}))
	defer ts.Close()

	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var rounds atomic.Uint64

	// Simulate consensus loop
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			default:
				r := rounds.Add(1)
				s.RecordRound(r, 1, "FINAL", 1, 100, 50000)
				time.Sleep(1 * time.Millisecond) // Don't tight-loop too hard
			}
		}
	}()

	// Simulate concurrent /status requests
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					resp, err := http.Get(ts.URL + "/status")
					if err == nil {
						resp.Body.Close()
					}
				}
			}
		}()
	}

	wg.Wait()

	if rounds.Load() < 10 {
		t.Fatalf("Consensus loop made virtually no progress: %d rounds", rounds.Load())
	}
}

func TestVoteEmitterNonBlocking(t *testing.T) {
	s := NewServer("test-node", "test-channel", 4, 1000, 500, 300, 0.75, 0.75, 0.5, 1, 4, func() int { return 3 })

	// Push 5000 votes while no one is subscribed. 
	// The ring buffer is 2000. It should overwrite and NEVER block.
	done := make(chan struct{})
	go func() {
		for i := 0; i < 5000; i++ {
			s.RecordVote("sent", "peer", uint64(i), "reduction-1", "abcd", 100)
		}
		close(done)
	}()

	select {
	case <-done:
		// Passed, did not block
	case <-time.After(1 * time.Second):
		t.Fatal("RecordVote blocked when no clients were subscribed")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.voteHistory) != 2000 {
		t.Fatalf("Expected vote history to be capped at 2000, got %d", len(s.voteHistory))
	}
}
