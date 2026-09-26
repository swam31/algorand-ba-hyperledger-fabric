package vrf

import (
	"crypto/rand"
	"math"
	"testing"
)

// oldSubUsers is the original implementation using Lgamma for equivalence testing
func oldSubUsers(hash []byte, tau float64, w, W uint64) uint64 {
	if w == 0 || W == 0 || tau <= 0 {
		return 0
	}
	p := tau / float64(W)
	if p <= 0 {
		return 0
	}
	if p >= 1 {
		return w
	}
	x := ratio(hash)
	cum := 0.0
	for j := uint64(0); j <= w; j++ {
		cum += math.Exp(logBinomPMF(j, w, p))
		if x < cum {
			return j
		}
	}
	return w
}

func TestSubUsersEquivalence(t *testing.T) {
	ws := []uint64{1, 100, 10000, 100000}
	trials := 10000
	W := uint64(400000)

	for i := 0; i < trials; i++ {
		hash := make([]byte, 32)
		rand.Read(hash)

		w := ws[i%len(ws)]
		
		// Random p in [0.01, 0.99]
		// tau = p * W
		tau := (float64(i%99) + 1) / 100.0 * float64(W)

		got := SubUsers(hash, tau, w, W)
		expected := oldSubUsers(hash, tau, w, W)

		if got != expected {
			t.Fatalf("Mismatch at w=%d, tau=%f, hash=%x: got %d, expected %d", w, tau, hash[:8], got, expected)
		}
	}
}

func BenchmarkSortition_Old(b *testing.B) {
	hash := make([]byte, 32)
	rand.Read(hash)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		oldSubUsers(hash, 50000, 100000, 400000)
	}
}

func BenchmarkSortition_New(b *testing.B) {
	hash := make([]byte, 32)
	rand.Read(hash)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SubUsers(hash, 50000, 100000, 400000)
	}
}
