package main

import (
	"math"
	"testing"
)

func logBinomPMF(k, n uint64, p float64) float64 {
	lgN, _ := math.Lgamma(float64(n) + 1)
	lgK, _ := math.Lgamma(float64(k) + 1)
	lgNK, _ := math.Lgamma(float64(n-k) + 1)
	return lgN - lgK - lgNK + float64(k)*math.Log(p) + float64(n-k)*math.Log1p(-p)
}

func BenchmarkOrig(b *testing.B) {
	w := uint64(100000)
	p := 0.5
	for i := 0; i < b.N; i++ {
		logBinomPMF(50000, w, p)
	}
}

func BenchmarkRecurse(b *testing.B) {
	w := uint64(100000)
	p := 0.5
	logPRatio := math.Log(p) - math.Log1p(-p)
	for i := 0; i < b.N; i++ {
		_ = math.Log(float64(w-50000)) - math.Log(float64(50000+1)) + logPRatio
	}
}
