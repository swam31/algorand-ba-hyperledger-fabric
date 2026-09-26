package config

import "testing"

func TestRoundMetadataRoundTrip(t *testing.T) {
	s := FormatRoundMetadata(286, 1, "FINAL", "orderer3", 1790328813417)
	if s != "round=286,steps=1,outcome=FINAL,proposer=orderer3,time=1790328813417" {
		t.Fatalf("unexpected metadata %q", s)
	}
	r, ok := ParseRoundMetadata(s)
	if !ok || r != 286 {
		t.Fatalf("ParseRoundMetadata(%q) = %d, %v; want 286, true", s, r, ok)
	}
	if r, ok := ParseRoundMetadata("round=12,steps=3"); !ok || r != 12 {
		t.Fatalf("old format: got %d, %v", r, ok)
	}
	for _, bad := range []string{"", "steps=3", "round=x", "round="} {
		if _, ok := ParseRoundMetadata(bad); ok {
			t.Errorf("ParseRoundMetadata(%q) accepted malformed input", bad)
		}
	}
}

func TestRoundsFromMetadata(t *testing.T) {
	// Block 106 (height 107) was produced in round 286: resume at round 287.
	if got := RoundsFromMetadata(107, "round=286,steps=1"); got != 180 {
		t.Fatalf("got failedRounds=%d, want 180", got)
	}
	if got := RoundsFromMetadata(107, ""); got != 0 {
		t.Fatalf("no metadata: got %d, want 0", got)
	}
	// A round lower than the block number is impossible; never underflow.
	if got := RoundsFromMetadata(107, "round=5"); got != 0 {
		t.Fatalf("inconsistent metadata: got %d, want 0", got)
	}
}

func TestIsBehind(t *testing.T) {
	peers := map[string]uint64{"o2": 30, "o3": 11, "o4": 11}
	if IsBehind(peers, 11, 1) {
		t.Fatal("one peer ahead (possibly Byzantine) must not count as proof we are behind")
	}
	peers["o4"] = 29
	if !IsBehind(peers, 11, 1) {
		t.Fatal("two peers ahead with f=1 must count as behind")
	}
	if IsBehind(peers, 30, 1) {
		t.Fatal("nobody is past round 30")
	}
}
