package ba

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/swayamchavan/algorand-fabric/engine/vrf"
)

// Context is the paper's ctx: everything a node needs to interpret votes for
// the current round. Every honest node must hold an identical Context, or
// sortition verification will disagree.
type Context struct {
	Round    uint64
	Seed     []byte // the round's public sortition seed
	PrevHash []byte // hash of the last agreed block
	Registry *Registry
	Params   Params

	// Cache is for simulation only and is nil on a real orderer. When many
	// simulated nodes share one inbox (and so the very same *Vote pointers),
	// each vote's verification is identical for all of them; the cache lets a
	// single laptop simulate thousands of nodes by doing that work once.
	// Real nodes each verify every vote themselves.
	Cache *VerifyCache
}

// VerifyCache memoises processMsg results per vote. See Context.Cache.
type VerifyCache struct {
	m      sync.Map // *Vote -> verified
	Misses atomic.Int64
}

type verified struct {
	seats           uint64
	value, sortHash []byte
}

// processMsg implements Algorithm 6.
//
// Returns how many committee seats this vote carries (0 = does not count), the
// value voted for, and the sortition hash (needed later by CommonCoin).
//
// Note that an invalid vote and a losing vote are both reported as zero seats.
// The paper makes no distinction, and neither do we: there is nothing useful to
// do with a bad vote except ignore it.
func (c *Context) processMsg(tau float64, v *Vote) (seats uint64, value, sortHash []byte) {
	if c.Cache == nil {
		return c.verifyMsg(tau, v)
	}
	if r, ok := c.Cache.m.Load(v); ok {
		rv := r.(verified)
		return rv.seats, rv.value, rv.sortHash
	}
	seats, value, sortHash = c.verifyMsg(tau, v)
	c.Cache.m.Store(v, verified{seats, value, sortHash})
	c.Cache.Misses.Add(1)
	return seats, value, sortHash
}

func (c *Context) verifyMsg(tau float64, v *Vote) (seats uint64, value, sortHash []byte) {
	// 1. Is it really from who it claims to be from?
	if !v.VerifySignature() {
		return 0, nil, nil
	}

	// 2. Is the voter a known participant, and what is its stake?
	m, ok := c.Registry.Lookup(v.Voter)
	if !ok {
		return 0, nil, nil
	}

	// 3. Does this vote build on the same chain we are on?
	//
	// This check is what makes forks self-isolating: a node on a different
	// branch has a different PrevHash, so we discard its votes entirely and it
	// discards ours. Safety is preserved - but note this is also precisely why
	// a fork STALLS the chain (neither side can reach the threshold alone), and
	// why the paper needs the separate recovery protocol of section 8.2.
	if !bytesEqual(v.PrevHash, c.PrevHash) {
		return 0, nil, nil
	}

	// 4. Was the voter genuinely selected for this step's committee, and for
	//    how many seats? This is the only thing standing between us and a node
	//    inventing unlimited voting power.
	seats = vrf.VerifySort(
		v.Voter, v.SortHash, v.SortProof,
		c.Seed, tau,
		vrf.Role{Name: "committee", Round: v.Round, Step: v.Step},
		m.Stake, c.Registry.TotalStake(),
	)
	if seats == 0 {
		return 0, nil, nil
	}
	return seats, v.Value, v.SortHash
}

// countVotes implements Algorithm 5.
//
// It streams votes for (round, step) out of the inbox, tallies seats per value,
// and returns the first value to EXCEED threshold seats. If lambda elapses
// first it reports a timeout.
//
// THE KEY PROPERTY, and the foundation of everything else: because threshold is
// set above 2/3 of the expected committee, two different values can never both
// exceed it. So if one honest node returns value V here, every other honest
// node returns either V or timeout - never a different value. (Paper Lemma 1.)
func (c *Context) countVotes(in *Inbox, step uint64, threshold, tau float64, lambda time.Duration) (value []byte, decided bool) {
	deadline := time.After(lambda)

	counts := make(map[string]float64) // value -> total seats
	voted := make(map[string]bool)     // voter ID -> already counted
	seen := 0

	for {
		batch, more := in.Since(c.Round, step, seen)
		seen += len(batch)

		for _, v := range batch {
			// One vote per participant per step. A malicious node that
			// equivocates - sending value A to some peers and value B to
			// others - gets only its FIRST message counted here. Different
			// honest nodes may therefore count different halves of that
			// equivocation, which is exactly the situation BA* is built to
			// survive, not something we can prevent at this layer.
			id := v.VoterID()
			if voted[id] {
				continue
			}

			seats, val, _ := c.processMsg(tau, v)
			if seats == 0 {
				continue
			}
			voted[id] = true

			key := string(val)
			counts[key] += float64(seats)
			if counts[key] > threshold {
				return val, true
			}
		}

		select {
		case <-more:
			// more votes arrived; loop and process them
		case <-deadline:
			return nil, false
		}
	}
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
