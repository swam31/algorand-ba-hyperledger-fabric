package sim

import (
	"crypto/ed25519"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/swayamchavan/algorand-fabric/engine/ba"
)

func runSimulation(t *testing.T, n int, f int, silentCount int) {
	sim := NewSimulator()
	params := ba.DefaultParams()

	members := make([]ba.Member, n)
	nodes := make([]*SimNode, n)
	for i := 0; i < n; i++ {
		pub, priv, err := ed25519.GenerateKey(nil)
		if err != nil {
			t.Fatal(err)
		}
		members[i] = ba.Member{
			ID:    fmt.Sprintf("node-%d", i),
			PK:    pub,
			Stake: params.StakePerNode,
		}
		nodes[i] = sim.AddNode(members[i].ID, priv, nil)
	}

	// Make some nodes silent (crashed)
	for i := n - silentCount; i < n; i++ {
		nodes[i].IsSilent = true
	}

	registry := ba.NewRegistry(members)
	if err := params.Validate(registry.HonestFraction(f)); err != nil {
		t.Fatal(err)
	}

	seed := []byte("test-seed-for-sortition")
	prevHash := []byte("genesis-hash")
	proposedBlock := ba.HashBlock([]byte("transaction-data"))

	var wg sync.WaitGroup
	results := make([]*ba.Result, n)
	errs := make([]error, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ctx := &ba.Context{
				Round:    1,
				Seed:     seed,
				PrevHash: prevHash,
				Registry: registry,
				Params:   params,
			}
			res, err := nodes[idx].Node.Run(ctx, proposedBlock)
			results[idx] = res
			errs[idx] = err
		}(i)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("test timed out")
	}

	for i := 0; i < n; i++ {
		if errs[i] != nil {
			t.Errorf("node %d failed: %v", i, errs[i])
			continue
		}
		if results[i].Outcome != ba.Final {
			t.Errorf("node %d returned outcome %s, expected FINAL", i, results[i].Outcome)
		}
		if string(results[i].BlockHash) != string(proposedBlock) {
			t.Errorf("node %d returned block %x, expected %x", i, results[i].BlockHash, proposedBlock)
		}
	}
}

func TestBA_AllHonestSameBlock(t *testing.T) {
	// 10 orderers, 0 Byzantine, 0 silent
	runSimulation(t, 10, 2, 0)
}

func TestBA_SilentNodes(t *testing.T) {
	// 10 orderers, f=2 assumption, but 2 nodes are actually crashed/silent
	runSimulation(t, 10, 2, 2)
}
