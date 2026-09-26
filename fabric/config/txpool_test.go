package config

import (
	"crypto/sha256"
	"fmt"
	"testing"
)

func tx(i int) []byte { return []byte(fmt.Sprintf("envelope-%06d-padding-padding", i)) }

func TestTxPoolDedupAndOrder(t *testing.T) {
	p := NewTxPool(100, 100)
	if !p.Add(tx(1)) || !p.Add(tx(2)) || !p.Add(tx(3)) {
		t.Fatal("fresh transactions must be accepted")
	}
	if p.Add(tx(2)) {
		t.Fatal("a transaction gossiped twice must be accepted once")
	}
	got := p.Take(2, 1<<20)
	if len(got) != 2 || string(got[0]) != string(tx(1)) || string(got[1]) != string(tx(2)) {
		t.Fatalf("Take must return the oldest first, got %q", got)
	}
	if p.Len() != 3 {
		t.Fatal("Take must not remove transactions")
	}
}

func TestTxPoolByteLimit(t *testing.T) {
	p := NewTxPool(100, 100)
	for i := 0; i < 10; i++ {
		p.Add(tx(i))
	}
	one := len(tx(0))
	if got := p.Take(100, 3*one); len(got) != 3 {
		t.Fatalf("byte limit: got %d transactions, want 3", len(got))
	}
	if got := p.Take(100, 1); len(got) != 1 {
		t.Fatal("an oversized transaction must still be proposable on its own")
	}
}

func TestTxPoolCommitRemovesAndBlocksLateGossip(t *testing.T) {
	p := NewTxPool(100, 100)
	p.Add(tx(1))
	p.Add(tx(2))
	if n := p.MarkCommitted([][]byte{tx(1), tx(9)}); n != 1 {
		t.Fatalf("removed %d, want 1 (tx 9 was never pending here)", n)
	}
	if p.Len() != 1 || !p.Committed(tx(1)) || !p.Committed(tx(9)) {
		t.Fatal("committed transactions must leave the pool and be remembered")
	}
	if p.Add(tx(1)) || p.Add(tx(9)) {
		t.Fatal("gossip arriving after the block was written must not re-queue the transaction")
	}
}

func TestTxPoolForgetsOldestCommitted(t *testing.T) {
	p := NewTxPool(100, 3)
	for i := 0; i < 5; i++ {
		p.MarkCommitted([][]byte{tx(i)})
	}
	if p.Committed(tx(0)) || p.Committed(tx(1)) || !p.Committed(tx(4)) {
		t.Fatal("the committed set must keep only the most recent entries")
	}
	if !p.Add(tx(0)) {
		t.Fatal("a forgotten transaction can be queued again")
	}
}

func TestTxPoolCapacityAndCompaction(t *testing.T) {
	p := NewTxPool(2000, 10)
	for i := 0; i < 2000; i++ {
		p.Add(tx(i))
	}
	if p.Add(tx(5000)) {
		t.Fatal("a full pool must refuse new transactions")
	}
	var all [][]byte
	for i := 0; i < 1990; i++ {
		all = append(all, tx(i))
	}
	p.MarkCommitted(all)
	got := p.Take(100, 1<<20)
	if len(got) != 10 || string(got[0]) != string(tx(1990)) {
		t.Fatalf("after compaction Take must see the 10 remaining, oldest first; got %d", len(got))
	}
}

func TestTxPoolGetByHash(t *testing.T) {
	p := NewTxPool(10, 10)
	p.Add(tx(7))
	if raw, ok := p.Get(sha256.Sum256(tx(7))); !ok || string(raw) != string(tx(7)) {
		t.Fatal("a pending transaction must be retrievable by its hash")
	}
	p.MarkCommitted([][]byte{tx(7)})
	if _, ok := p.Get(sha256.Sum256(tx(7))); ok {
		t.Fatal("a committed transaction is no longer pending")
	}
}
