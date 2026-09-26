package vrf

import (
	"crypto/ed25519"
	"encoding/binary"
	"fmt"
	"math"
)

// Role identifies which lottery is being run. Changing any field yields a
// completely independent selection, which is how Algorand draws a fresh
// committee for every single step at zero coordination cost (paper section 5.1).
type Role struct {
	Name  string // "proposer", "committee"
	Round uint64
	Step  uint64 // step within BA*; ignored for "proposer"
}

// Bytes is the canonical wire encoding of a role, concatenated after the seed
// as the VRF input.
func (r Role) Bytes() []byte {
	b := make([]byte, 0, len(r.Name)+17)
	b = append(b, r.Name...)
	b = append(b, '|')
	b = binary.BigEndian.AppendUint64(b, r.Round)
	b = append(b, '|')
	return binary.BigEndian.AppendUint64(b, r.Step)
}

func (r Role) String() string {
	return fmt.Sprintf("%s/r%d/s%d", r.Name, r.Round, r.Step)
}

// input builds the VRF message: seed || role.
func input(seed []byte, role Role) []byte {
	m := make([]byte, 0, len(seed)+32)
	m = append(m, seed...)
	return append(m, role.Bytes()...)
}

// Sortition implements Algorithm 1 of the paper.
//
// It answers, privately and offline: "how many committee seats did I win?"
//
//	sk    - this node's secret key
//	seed  - the public random seed for this round
//	tau   - expected total number of seats to hand out across ALL nodes
//	role  - which lottery (see Role)
//	w     - this node's weight (stake)
//	W     - total weight of every node in the system
//
// Returns the VRF hash, its proof, and j = the number of seats won. j == 0
// means "not selected, stay silent".
func Sortition(sk ed25519.PrivateKey, seed []byte, tau float64, role Role, w, W uint64) (hash, proof []byte, j uint64) {
	hash, proof = Default.Prove(sk, input(seed, role))
	return hash, proof, SubUsers(hash, tau, w, W)
}

// VerifySort implements Algorithm 2: given someone else's claimed hash and
// proof, recompute how many seats they legitimately won. Returns 0 if the proof
// does not check out, which is also the "not selected" answer - a malformed
// vote and a losing vote are treated identically, as the paper does.
func VerifySort(pk ed25519.PublicKey, hash, proof, seed []byte, tau float64, role Role, w, W uint64) uint64 {
	if !Default.Verify(pk, input(seed, role), hash, proof) {
		return 0
	}
	return SubUsers(hash, tau, w, W)
}

// SubUsers is the mathematical core of sortition: it maps a uniformly random
// hash to a number of selected "sub-users", weighted by stake.
//
// THE SYBIL-RESISTANCE ARGUMENT. A node with weight w is treated as w
// independent lottery tickets, each winning with probability p = tau/W. The
// number of winners therefore follows the binomial distribution B(k; w, p).
// Because
//
//	B(k1; n1, p) + B(k2; n2, p) = B(k1+k2; n1+n2, p)
//
// splitting a stake of w across any number of fake identities yields exactly
// the same distribution of total seats. Creating Sybils is arithmetically
// pointless. This is the whole reason the binomial appears here rather than a
// simple per-node coin flip.
//
// We normalise hash into x in [0,1) and walk the binomial CDF, returning the
// first j whose cumulative probability exceeds x.
//
// DEVIATION FROM THE PAPER'S PSEUDOCODE. Algorithm 1 as printed defines the
// interval I_j = [ sum_{k=0..j} B(k), sum_{k=0..j+1} B(k) ) and loops while
// x is not in I_j. Taken literally that never terminates for x < B(0), since
// every interval starts at or above B(0). The intended (and standard)
// semantics, implemented here, is:
//
//	select j such that  CDF(j-1) <= x < CDF(j),  with CDF(-1) = 0
//
// which gives P[j = 0] = B(0) as required.
func SubUsers(hash []byte, tau float64, w, W uint64) uint64 {
	if w == 0 || W == 0 || tau <= 0 {
		return 0
	}
	p := tau / float64(W)
	if p <= 0 {
		return 0
	}
	if p >= 1 {
		return w // tau >= W: everyone is selected for every ticket
	}

	x := ratio(hash)
	cum := 0.0
	
	logPMF := float64(w) * math.Log1p(-p)
	logPRatio := math.Log(p) - math.Log1p(-p)
	pRatio := p / (1 - p)
	
	var pmf float64
	useLog := true
	for j := uint64(0); j <= w; j++ {
		if useLog {
			if logPMF > -700 {
				pmf = math.Exp(logPMF)
				useLog = false
			}
		}
		if !useLog {
			cum += pmf
		}
		if x < cum {
			return j
		}
		if j < w {
			if useLog {
				logPMF += math.Log(float64(w-j)/float64(j+1)) + logPRatio
			} else {
				pmf = pmf * float64(w-j) / float64(j+1) * pRatio
			}
		}
	}
	// Floating point drift can leave cum a hair under 1.0; treat as all-selected.
	return w
}

// logBinomPMF returns log B(k; n, p), computed via log-gamma so that large
// weights do not overflow or underflow. Direct evaluation of C(n,k) p^k q^(n-k)
// loses all precision once n is in the thousands, which is exactly the regime
// Algorand runs in (W is the entire money supply).
func logBinomPMF(k, n uint64, p float64) float64 {
	if k > n {
		return math.Inf(-1)
	}
	lgN, _ := math.Lgamma(float64(n) + 1)
	lgK, _ := math.Lgamma(float64(k) + 1)
	lgNK, _ := math.Lgamma(float64(n-k) + 1)
	return lgN - lgK - lgNK + float64(k)*math.Log(p) + float64(n-k)*math.Log1p(-p)
}

// ratio maps a hash to a float in [0,1), i.e. the paper's hash / 2^hashlen.
//
// We take the leading 64 bits and keep the top 53, which is exactly the mantissa
// width of a float64. Using more bytes would add no precision, and the naive
// float64(u)/2^64 can round up to exactly 1.0, which would break the CDF walk.
func ratio(hash []byte) float64 {
	var u uint64
	if len(hash) >= 8 {
		u = binary.BigEndian.Uint64(hash[:8])
	} else {
		buf := make([]byte, 8)
		copy(buf, hash)
		u = binary.BigEndian.Uint64(buf)
	}
	return float64(u>>11) / float64(uint64(1)<<53)
}

// Priority implements the block-proposal priority of paper section 6.
//
// A proposer that won j seats hashes its sortition output once per sub-user and
// keeps the best (lowest) result. Lowest hash wins, so a node with more stake
// gets more draws and is more likely to hold the top priority - proportional
// influence, without ever letting a node pick its own number.
func Priority(hash []byte, j uint64) []byte {
	if j == 0 {
		return nil
	}
	var best []byte
	for i := uint64(1); i <= j; i++ {
		h := subUserHash(hash, i)
		if best == nil || less(h, best) {
			best = h
		}
	}
	return best
}
