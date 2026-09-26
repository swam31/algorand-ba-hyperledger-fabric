package config

import (
	"crypto/sha256"
	"sync"
)

// TxPool holds transactions waiting to be ordered.
//
// Every orderer gossips the transactions its clients submit to the other
// orderers, so all pools converge on the same pending set and whichever
// orderer wins the proposer lottery includes everyone's transactions. Without
// this, each round could only write the winner's own clients' transactions and
// every other transaction waited for its orderer's turn - about four rounds on
// average with four orderers.
//
// Entries are raw marshalled envelopes, keyed by their SHA-256. Once a block is
// written every orderer removes its transactions, and remembers them for a
// while so gossip that arrives late cannot put them back.
type TxPool struct {
	mu         sync.Mutex
	order      [][32]byte // arrival order; may hold already-removed hashes
	pending    map[[32]byte][]byte
	committed  map[[32]byte]struct{}
	ring       [][32]byte // committed hashes, oldest overwritten first
	ringNext   int
	maxPending int
}

func NewTxPool(maxPending, rememberCommitted int) *TxPool {
	return &TxPool{
		pending:    make(map[[32]byte][]byte),
		committed:  make(map[[32]byte]struct{}),
		ring:       make([][32]byte, 0, rememberCommitted),
		maxPending: maxPending,
	}
}

// Add queues a transaction. It returns false if the transaction is already
// pending, was recently committed, or the pool is full.
func (p *TxPool) Add(raw []byte) bool {
	h := sha256.Sum256(raw)
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.pending[h]; ok {
		return false
	}
	if _, ok := p.committed[h]; ok {
		return false
	}
	if len(p.pending) >= p.maxPending {
		return false
	}
	p.pending[h] = raw
	p.order = append(p.order, h)
	return true
}

func (p *TxPool) Len() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.pending)
}

// Take returns the oldest pending transactions, up to maxCount and maxBytes
// (at least one if any are pending), without removing them: they leave the
// pool only when a block containing them is written.
func (p *TxPool) Take(maxCount, maxBytes int) [][]byte {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.compact()
	var out [][]byte
	size := 0
	for _, h := range p.order {
		raw, ok := p.pending[h]
		if !ok {
			continue
		}
		if len(out) > 0 && (len(out) >= maxCount || size+len(raw) > maxBytes) {
			break
		}
		out = append(out, raw)
		size += len(raw)
	}
	return out
}

// Get returns a pending transaction by its SHA-256, which is how compact
// proposals name the transactions in a block.
func (p *TxPool) Get(h [32]byte) ([]byte, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	raw, ok := p.pending[h]
	return raw, ok
}

// MarkCommitted removes a written block's transactions and remembers them.
// It reports how many were still pending here.
func (p *TxPool) MarkCommitted(raws [][]byte) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	removed := 0
	for _, raw := range raws {
		h := sha256.Sum256(raw)
		if _, ok := p.pending[h]; ok {
			delete(p.pending, h)
			removed++
		}
		if _, ok := p.committed[h]; ok {
			continue
		}
		if cap(p.ring) == 0 {
			continue
		}
		if len(p.ring) < cap(p.ring) {
			p.ring = append(p.ring, h)
		} else {
			delete(p.committed, p.ring[p.ringNext])
			p.ring[p.ringNext] = h
			p.ringNext = (p.ringNext + 1) % cap(p.ring)
		}
		p.committed[h] = struct{}{}
	}
	return removed
}

// Committed reports whether the transaction is in the recently-written set.
func (p *TxPool) Committed(raw []byte) bool {
	h := sha256.Sum256(raw)
	p.mu.Lock()
	defer p.mu.Unlock()
	_, ok := p.committed[h]
	return ok
}

// compact drops removed hashes from the arrival list once they dominate it.
func (p *TxPool) compact() {
	if len(p.order) < 2*len(p.pending)+1024 {
		return
	}
	kept := p.order[:0]
	for _, h := range p.order {
		if _, ok := p.pending[h]; ok {
			kept = append(kept, h)
		}
	}
	p.order = kept
}
