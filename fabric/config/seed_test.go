package config

import (
	"bytes"
	"sync"
	"testing"
)

func TestSeedChain_Consistency(t *testing.T) {
	sc := NewSeedChain()
	s0 := sc.Get(0, "test-channel")
	s1 := sc.Get(1, "test-channel")
	s50 := sc.Get(50, "test-channel")

	if len(s0) != 32 || len(s1) != 32 || len(s50) != 32 {
		t.Fatalf("Expected 32 byte seeds")
	}

	// Determinism
	s50_again := sc.Get(50, "test-channel")
	if !bytes.Equal(s50, s50_again) {
		t.Fatalf("Seeds for same round must match")
	}

	// Different channels
	s0_other := sc.Get(0, "other-channel")
	if bytes.Equal(s0, s0_other) {
		t.Fatalf("Seeds for different channels must not match")
	}
}

func TestSeedChain_Concurrency(t *testing.T) {
	sc := NewSeedChain()
	var wg sync.WaitGroup
	
	// Spin up 50 goroutines
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for r := 0; r <= 100; r++ {
				// We call Get with a mix of rounds to test locking under contention
				// Some hit cache, some compute
				_ = sc.Get(uint64(r), "chan1")
				_ = sc.Get(uint64(100-r), "chan1") 
			}
		}(i)
	}
	wg.Wait()
}
