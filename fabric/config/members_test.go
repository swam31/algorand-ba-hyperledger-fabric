package config

import (
	"crypto/ed25519"
	"fmt"
	"math"
	"reflect"
	"testing"

	"github.com/swayamchavan/algorand-fabric/engine/ba"
	"github.com/swayamchavan/algorand-fabric/engine/vrf"
)

func TestParseCommaList(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"a:1, b:2,", []string{"a:1", "b:2"}},
		{"  orderer1:7060  ,  orderer2:7060", []string{"orderer1:7060", "orderer2:7060"}},
		{",,,", nil},
		{"", nil},
		{"a,b,c", []string{"a", "b", "c"}},
	}

	for _, tc := range tests {
		actual := ParseCommaList(tc.input)
		if len(actual) == 0 && len(tc.expected) == 0 {
			continue // Both empty/nil
		}
		if !reflect.DeepEqual(actual, tc.expected) {
			t.Errorf("ParseCommaList(%q) = %v, expected %v", tc.input, actual, tc.expected)
		}
	}
}

func TestValidateMembers_FaultTolerance(t *testing.T) {
	selfPK := []byte("dummy-self-pk")
	params := ba.DefaultParams() // TFinal = 0.74

	tests := []struct {
		n           int
		expectError bool
		expectedF   uint64 // expected chosen f if not error
	}{
		// For TFinal=0.74
		// n=4: fSafety=1, fFinal: h(4,1)=3/4=0.75 > 0.74 => fFinal=1. min=1
		{4, false, 1},
		// n=5: fSafety=1, fFinal: h(5,1)=4/5=0.80 > 0.74 => fFinal=1. min=1
		{5, false, 1},
		// n=6: fSafety=1, fFinal: h(6,1)=5/6=0.83 > 0.74 => fFinal=1. min=1
		{6, false, 1},
		// n=7: fSafety=2, fFinal: h(7,2)=5/7=0.714 < 0.74 => fFinal=1. min=1
		{7, false, 1},
		// n=8: fSafety=2, fFinal: h(8,2)=6/8=0.75 > 0.74 => fFinal=2. min=2
		{8, false, 2},
		// n=9: fSafety=2, fFinal: h(9,2)=7/9=0.77 > 0.74 => fFinal=2. min=2
		{9, false, 2},
		// n=10: fSafety=3, fFinal: h(10,3)=7/10=0.70 < 0.74 => h(10,2)=8/10=0.80 => fFinal=2. min=2
		{10, false, 2},
		// n=11: fSafety=3, fFinal: h(11,3)=8/11=0.727 < 0.74 => h(11,2)=9/11=0.81 => fFinal=2. min=2
		{11, false, 2},
		// n=12: fSafety=3, fFinal: h(12,3)=9/12=0.75 > 0.74 => fFinal=3. min=3
		{12, false, 3},
		// n=13: fSafety=4, fFinal: h(13,4)=9/13=0.69 < 0.74 => h(13,3)=10/13=0.76 => fFinal=3. min=3
		{13, false, 3},
	}

	for _, tc := range tests {
		var members []ba.Member
		for i := 0; i < tc.n; i++ {
			pk := []byte("other-pk")
			if i == 0 {
				pk = selfPK
			}
			members = append(members, ba.Member{
				ID:    fmt.Sprintf("node%d", i),
				PK:    pk,
				Stake: 10000,
			})
		}

		_, h, err := ValidateMembers(members, selfPK, params)
		if tc.expectError {
			if err == nil {
				t.Errorf("n=%d: expected error, got nil", tc.n)
			}
		} else {
			if err != nil {
				t.Errorf("n=%d: unexpected error: %v", tc.n, err)
			}
			fActual := tc.n - int(float64(tc.n)*h+0.5) // h = (n-f)/n => f = n - n*h
			if fActual != int(tc.expectedF) {
				t.Errorf("n=%d: expected f=%d, got f=%d (h=%v)", tc.n, tc.expectedF, fActual, h)
			}
		}
	}
}

func hashOfInt(i int) []byte {
	return []byte(fmt.Sprintf("vrf-hash-%d", i))
}

func TestValidateMembers_SortitionProperties(t *testing.T) {
	n := 4
	stakePerNode := uint64(100000)
	params := ba.ParamsForNetwork(n, stakePerNode)

	W := params.TotalStake(n)
	tau := params.TauStep

	p := tau / float64(W)
	if p >= 1 {
		t.Fatalf("p >= 1 (tau=%v, W=%v), sortition is degenerate", tau, W)
	}

	trials := 100

	// Generate keys for the nodes
	var keys []ed25519.PrivateKey
	for i := 0; i < n; i++ {
		_, priv, err := ed25519.GenerateKey(nil)
		if err != nil {
			t.Fatalf("Failed to generate key: %v", err)
		}
		keys = append(keys, priv)
	}

	// Stats for a single node over all trials
	minSeats := uint64(1<<63 - 1)
	maxSeats := uint64(0)
	sumSeats := uint64(0)
	var observedSeats []uint64

	// Liveness: 3 honest nodes can exceed TStep*Tau
	// Final Reachability: 3 honest nodes can exceed TFinal*Tau
	// Safety: 2 nodes cannot exceed TStep*Tau
	
	livenessSuccess := false
	finalSuccess := false
	safetyViolations := 0

	seed := []byte("test-seed-for-sortition")

	for i := 0; i < trials; i++ {
		role := vrf.Role{Name: "committee", Round: uint64(i), Step: 1}
		
		seats2 := uint64(0)
		seats3 := uint64(0)
		
		// Simulate each node evaluating the VRF
		for nodeID := 0; nodeID < n; nodeID++ {
			_, _, seats := vrf.Sortition(keys[nodeID], seed, tau, role, stakePerNode, W)
			
			if nodeID == 0 {
				if seats < minSeats { minSeats = seats }
				if seats > maxSeats { maxSeats = seats }
				sumSeats += seats
				observedSeats = append(observedSeats, seats)
			}
			
			if nodeID < 2 {
				seats2 += seats
			}
			
			if nodeID < 3 {
				seats3 += seats
			}
		}
		
		if float64(seats3) > params.StepThreshold() {
			livenessSuccess = true
		}
		if float64(seats3) > params.FinalThreshold() {
			finalSuccess = true
		}
		if float64(seats2) > params.StepThreshold() {
			safetyViolations++
		}
	}
	
	meanSeats := float64(sumSeats) / float64(trials)
	
	// Calculate variance and sd
	var sumSqDiff float64
	for _, seats := range observedSeats {
		diff := float64(seats) - meanSeats
		sumSqDiff += diff * diff
	}
	sd := math.Sqrt(sumSqDiff / float64(trials))
	
	expectedMean := float64(stakePerNode) * p
	expectedSD := math.Sqrt(float64(stakePerNode) * p * (1 - p))
	
	t.Logf("Observed seats per node: min=%d, mean=%.2f, max=%d, sd=%.2f", minSeats, meanSeats, maxSeats, sd)
	t.Logf("Expected: mean=%.2f, sd=%.2f", expectedMean, expectedSD)
	
	if minSeats == maxSeats {
		t.Errorf("min == max (%d), VRF is not varying output", minSeats)
	}
	
	// mean within 1% of expected (note: 100 trials is small for 1% bounds, so we use a loose bound if needed, but 1% of 50000 is 500. It might fail on 100 trials)
	// Actually, wait, standard error of mean is SD / sqrt(trials) = 158 / 10 = 15.8. 
	// So 99% CI is 50000 +/- 45. 500 is very safe (many sigmas).
	if math.Abs(meanSeats-expectedMean) > expectedMean*0.01 {
		t.Errorf("Mean %.2f is not within 1%% of expected %.2f", meanSeats, expectedMean)
	}
	
	if math.Abs(sd-expectedSD) > expectedSD*0.20 {
		t.Errorf("SD %.2f is not within 20%% of expected %.2f", sd, expectedSD)
	}
	
	if !livenessSuccess {
		t.Errorf("Liveness failure: 3 honest nodes never crossed TStep*Tau (%.0f) in %d trials", params.StepThreshold(), trials)
	}
	if !finalSuccess {
		t.Errorf("FINAL reachability failure: 3 honest nodes never crossed TFinal*Tau (%.0f) in %d trials", params.FinalThreshold(), trials)
	}
	if safetyViolations > 0 {
		t.Errorf("Safety violation: 2 adversary nodes crossed TStep*Tau (%.0f) in %d out of %d trials", params.StepThreshold(), safetyViolations, trials)
	}
}
