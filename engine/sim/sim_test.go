package sim

import (
	"bytes"
	"crypto/ed25519"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/swayamchavan/algorand-fabric/engine/ba"
	"github.com/swayamchavan/algorand-fabric/engine/vrf"
)

// runAdversarial simulates a network where some nodes might be Byzantine or silent.
func runAdversarial(t *testing.T, params ba.Params, n int, f int,
	silentCount int,
	byzantineIndex int,
	byzantineStrategy string,
	partition bool,
	messageLossRate float64,
	randomInitialValues bool) ([]*ba.Result, []error, []*SimNode) {

	sim := NewSimulator()
	// Set params explicitly as asked
	params.StakePerNode = 100000
	params.TauStep = float64(uint64(n)*params.StakePerNode) * 0.5 // tau = 0.5 * W
	params.TauFinal = params.TauStep
	params.StakePerNode = 100000
	params.TauStep = float64(uint64(n)*params.StakePerNode) * 0.5 // tau = 0.5 * W
	params.TauFinal = params.TauStep

	members := make([]ba.Member, n)
	nodes := make([]*SimNode, n)
	keys := make([]ed25519.PrivateKey, n)
	for i := 0; i < n; i++ {
		pub, priv, err := ed25519.GenerateKey(nil)
		if err != nil {
			t.Fatal(err)
		}
		keys[i] = priv
		members[i] = ba.Member{
			ID:    fmt.Sprintf("node-%d", i),
			PK:    pub,
			Stake: params.StakePerNode,
		}
		nodes[i] = sim.AddNode(members[i].ID, priv, nil)
	}

	for i := 0; i < silentCount; i++ {
		nodes[i].IsSilent = true
	}

	sim.Tamper = func(src, dst int, v *ba.Vote) (*ba.Vote, bool) {
		// Equivocation strategy
		if src == byzantineIndex && byzantineStrategy == "equivocate" {
			// Send one value to half the network, another to the rest
			if v.Step == ba.StepReductionOne || v.Step == ba.StepReductionTwo {
				// Must build a NEW properly signed vote for the different value
				newVal := ba.HashBlock([]byte("malicious-A"))
				if dst%2 != 0 {
					newVal = ba.HashBlock([]byte("malicious-B"))
				}
				
				// Create a new Vote struct with the same metadata but different Value
				badVote := new(ba.Vote)
				*badVote = *v
				badVote.Value = newVal
				
				// Re-sign it with the adversary's own private key
				badVote.Sign(keys[src])
				return badVote, false
			}
		}

		// Partition strategy
		if partition {
			// nodes 0,1 cannot talk to nodes 2,3
			if (src < 2 && dst >= 2) || (src >= 2 && dst < 2) {
				// Let's say partition heals after step 10
				if v.Step < ba.StepBinaryBase+5 {
					return v, true
				}
			}
		}

		if messageLossRate > 0 {
			if rand.Float64() < messageLossRate {
				return v, true
			}
		}

		return v, false
	}

	registry := ba.NewRegistry(members)
	if err := params.Validate(registry.HonestFraction(f)); err != nil {
		t.Fatal(err)
	}

	seed := []byte("test-seed")
	prevHash := []byte("genesis")
	blockA := ba.HashBlock([]byte("blockA"))
	blockB := ba.HashBlock([]byte("blockB"))

	var wg sync.WaitGroup
	results := make([]*ba.Result, n)
	errs := make([]error, n)

	for i := 0; i < n; i++ {
		if nodes[i].IsSilent {
			continue
		}
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
			
			proposedBlock := blockA
			if randomInitialValues {
				if rand.Float32() < 0.5 {
					proposedBlock = blockB
				}
			}
			if idx == byzantineIndex && byzantineStrategy == "wrong_value" {
				proposedBlock = ba.HashBlock([]byte("malicious"))
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
	case <-time.After(60 * time.Second): // Ensure it terminates (increased for -race overhead)
	}
	return results, errs, nodes
}

// assertSafety validates the fundamental safety invariant of BA*.
// Under weak synchrony, BA* guarantees that if ANY node reaches FINAL on value V, 
// then NO honest node will decide (FINAL or TENTATIVE) on anything != V. 
// However, if NO node reaches FINAL, differing TENTATIVE values are entirely LEGAL 
// (which is exactly what TENTATIVE means).
//
// A real safety violation is impossible at n=4 with f=1 by quorum intersection:
// - any decision needs >68.5% of seats, i.e. >=3 of 4 nodes
// - two different decisions need two 3-node sets out of 4, which overlap in >=2 nodes
// - both overlapping nodes would have to have equivocated
// - since only 1 Byzantine node exists, this is a contradiction.
func assertSafety(results []*ba.Result, errs []error) (finalCount, stallCount int, err error) {
	var finalValue []byte
	var tentativeValues [][]byte

	for _, res := range results {
		if res == nil {
			stallCount++
			continue
		}
		if res.Outcome == ba.Final {
			finalCount++
			if finalValue == nil {
				finalValue = res.BlockHash
			} else if !bytes.Equal(finalValue, res.BlockHash) {
				panic(fmt.Sprintf("SAFETY VIOLATION! Multiple distinct FINAL values decided: %x and %x", finalValue, res.BlockHash))
			}
		} else if res.Outcome == ba.Tentative {
			tentativeValues = append(tentativeValues, res.BlockHash)
		}
	}

	if finalValue != nil {
		// If there is a FINAL decision, no TENTATIVE decision can contradict it.
		for _, tv := range tentativeValues {
			if !bytes.Equal(finalValue, tv) {
				panic(fmt.Sprintf("SAFETY VIOLATION! A node reached FINAL on %x, but another reached TENTATIVE on %x", finalValue, tv))
			}
		}
	}

	return finalCount, stallCount, nil
}

func TestSim_AllHonest(t *testing.T) {
	results, errs, _ := runAdversarial(t, ba.DefaultParams(), 4, 1, 0, -1, "", false, 0, false)
	finals, _, err := assertSafety(results, errs)
	if err != nil {
		t.Fatal(err)
	}
	if finals != 4 {
		t.Fatalf("Expected 4 FINAL, got %d", finals)
	}
}

func TestSim_OneSilent(t *testing.T) {
	results, errs, _ := runAdversarial(t, ba.DefaultParams(), 4, 1, 1, -1, "", false, 0, false)
	finals, _, err := assertSafety(results, errs)
	if err != nil {
		t.Fatal(err)
	}
	if finals != 3 {
		t.Fatalf("Expected 3 FINAL, got %d", finals)
	}
}

func TestSim_OneByzantineWrongValue(t *testing.T) {
	results, errs, _ := runAdversarial(t, ba.DefaultParams(), 4, 1, 0, 3, "wrong_value", false, 0, false)
	finals, _, err := assertSafety(results, errs)
	if err != nil {
		t.Fatal(err)
	}
	if finals < 3 {
		t.Fatalf("Expected at least 3 FINAL, got %d", finals)
	}
}

func TestSim_OneByzantineEquivocate(t *testing.T) {
	results, errs, nodes := runAdversarial(t, ba.DefaultParams(), 4, 1, 0, 3, "equivocate", false, 0, false)
	_, _, err := assertSafety(results, errs)
	if err != nil {
		t.Fatal(err)
	}

	// 4. Add an assertion that the adversary's votes were actually COUNTED (non-zero seats).
	// We check node 0's inbox to see if node 3 (the adversary) had a vote in Step 1 or Step 2.
	// We verify its seats > 0 using the VRF.
	counted := false
	seed := []byte("test-seed")
	advPK := nodes[3].Node.PublicKey()
	for step := uint64(1); step <= 2; step++ {
		votes := nodes[0].Node.Inbox().All(1, step)
		for _, v := range votes {
			if bytes.Equal(v.Voter, advPK) {
				seats := vrf.VerifySort(
					v.Voter, v.SortHash, v.SortProof,
					seed, ba.DefaultParams().TauStep,
					vrf.Role{Name: "committee", Round: v.Round, Step: v.Step},
					ba.DefaultParams().StakePerNode, ba.DefaultParams().StakePerNode*4,
				)
				if seats > 0 {
					counted = true
				}
			}
		}
	}
	if !counted {
		t.Fatalf("Adversary's equivocated votes were not counted (0 seats or invalid) by honest node 0")
	}
}

func FastParams() ba.Params {
	p := ba.DefaultParams()
	// With -race and concurrency=4, 250ms ensures stable scheduling and prevents false timeouts 
	// while keeping the run within the 45m limit.
	p.LambdaStep = 250 * time.Millisecond
	p.LambdaBlock = 250 * time.Millisecond
	p.LambdaPriority = 100 * time.Millisecond
	p.LambdaStepVar = 100 * time.Millisecond
	return p
}

func TestSim_PartitionHealing(t *testing.T) {
	results, errs, _ := runAdversarial(t, FastParams(), 4, 1, 0, -1, "", true, 0, false)
	finals, stalls, err := assertSafety(results, errs)
	if err != nil {
		t.Fatal(err)
	}
	// Because the partition heals after reduction() and early binaryBA steps, 
	// nodes decide on the empty block at a later step. Therefore, they DO NOT
	// cast StepFinal votes (which only happens for step=1 decisions).
	// So they must all reach TENTATIVE (finals=0, stalls=0) rather than FINAL.
	if stalls > 0 {
		t.Fatalf("Expected 0 stalls after partition heals, got %d", stalls)
	}
	if finals != 0 {
		t.Fatalf("Expected 0 FINAL (only TENTATIVE possible), got %d", finals)
	}
}

func TestAdversarialSoak(t *testing.T) {
	trials := 100
	
	runRegime := func(lossRate float64, livenessThreshold float64) {
		var totalFinals, totalStalls atomic.Int32
		var successTrials atomic.Int32

		var wg sync.WaitGroup
		sem := make(chan struct{}, 4) // Concurrency 4 to prevent total CPU starvation under -race
		errChan := make(chan error, trials)

		for i := 0; i < trials; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				byzantineIdx := -1
				strategy := ""
				if rand.Float32() < 0.2 {
					byzantineIdx = rand.Intn(4)
					strategy = []string{"wrong_value", "equivocate"}[rand.Intn(2)]
				}

				results, errs, _ := runAdversarial(t, FastParams(), 4, 1, 0, byzantineIdx, strategy, false, lossRate, true)
				finals, stalls, err := assertSafety(results, errs)
				if err != nil {
					errChan <- fmt.Errorf("Trial %d: %v", idx, err)
				}
				totalFinals.Add(int32(finals))
				totalStalls.Add(int32(stalls))
				if finals >= 3 {
					successTrials.Add(1)
				}
			}(i)
		}

		wg.Wait()
		close(errChan)
		
		var violations []error
		for err := range errChan {
			violations = append(violations, err)
		}
		
		t.Logf("Regime (%.0f%% loss): trials=%d, trials with >=3 FINAL=%d, total nodes FINAL=%d, total stalls=%d, safety violations=%d",
			lossRate*100, trials, successTrials.Load(), totalFinals.Load(), totalStalls.Load(), len(violations))
			
		for _, err := range violations {
			t.Errorf("Safety violation: %v", err)
		}
		
		if float64(successTrials.Load())/float64(trials) < livenessThreshold {
			t.Errorf("Liveness failure: only %d/%d trials reached FINAL (needed %.0f%%)", successTrials.Load(), trials, livenessThreshold*100)
		}
	}

	t.Log("Running 5% loss regime...")
	runRegime(0.05, LivenessThreshold)
	
	t.Log("Running 30% loss regime...")
	runRegime(0.30, 0.0) // No liveness requirement, just safety
}
