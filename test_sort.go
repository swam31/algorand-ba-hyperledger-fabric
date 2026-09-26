package main

import (
	"fmt"
	"math"
)

func logBinomPMF(k, n uint64, p float64) float64 {
	lgN, _ := math.Lgamma(float64(n) + 1)
	lgK, _ := math.Lgamma(float64(k) + 1)
	lgNK, _ := math.Lgamma(float64(n-k) + 1)
	return lgN - lgK - lgNK + float64(k)*math.Log(p) + float64(n-k)*math.Log1p(-p)
}

func main() {
    w := uint64(10000)
    p := 0.5
    // user's exact code
    pmf := math.Exp(float64(w) * math.Log1p(-p))
    fmt.Printf("pmf: %v\n", pmf)
}
