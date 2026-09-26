package config

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sync"
)

// SeedChain implements a deterministic seed chain for BA* as defined in paper section 5.2.
// This is a simplified fallback for when a proposed block carries no valid VRF seed.
type SeedChain struct {
	mu    sync.RWMutex
	seeds map[string]map[uint64][]byte
}

// NewSeedChain creates a new SeedChain.
func NewSeedChain() *SeedChain {
	return &SeedChain{
		seeds: make(map[string]map[uint64][]byte),
	}
}

// Get derives or retrieves the seed for a given round iteratively to avoid lock re-entrancy.
func (s *SeedChain) Get(round uint64, channelID string) []byte {
	s.mu.RLock()
	channelSeeds, ok := s.seeds[channelID]
	if ok {
		seed, ok2 := channelSeeds[round]
		s.mu.RUnlock()
		if ok2 {
			return seed
		}
	} else {
		s.mu.RUnlock()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	channelSeeds, ok = s.seeds[channelID]
	if !ok {
		channelSeeds = make(map[uint64][]byte)
		s.seeds[channelID] = channelSeeds
	}

	// Check again in case it was created while waiting for lock
	if seed, ok2 := channelSeeds[round]; ok2 {
		return seed
	}

	return s.getUnlocked(round, channelID, channelSeeds)
}

func (s *SeedChain) getUnlocked(round uint64, channelID string, channelSeeds map[uint64][]byte) []byte {
	if seed, ok := channelSeeds[round]; ok {
		return seed
	}

	var baseRound uint64 = 0
	var prevSeed []byte

	// Find the highest cached round below `round`
	for r := round; r > 0; r-- {
		if seed, ok := channelSeeds[r-1]; ok {
			baseRound = r - 1
			prevSeed = seed
			break
		}
	}

	// Compute base round if not found
	if prevSeed == nil {
		h := sha256.Sum256([]byte(fmt.Sprintf("algorand-fabric-genesis%s", channelID)))
		prevSeed = h[:]
		channelSeeds[0] = prevSeed
		baseRound = 0
	}

	// Iterate upwards to compute all missing seeds
	for r := baseRound + 1; r <= round; r++ {
		buf := make([]byte, len(prevSeed)+8)
		copy(buf, prevSeed)
		binary.BigEndian.PutUint64(buf[len(prevSeed):], r)
		h := sha256.Sum256(buf)
		newSeed := h[:]
		channelSeeds[r] = newSeed
		prevSeed = newSeed
	}

	return channelSeeds[round]
}
