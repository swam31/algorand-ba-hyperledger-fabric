package config

import (
	"fmt"
	"strconv"
	"strings"
)

// These helpers live here rather than in fabric/consenter because that package
// imports Fabric and cannot be unit-tested on its own.

// FormatRoundMetadata is the consenter metadata written into every block. It
// records which BA* round produced the block, which is what lets a restarted
// orderer recover its round number (height alone does not determine it, because
// empty rounds advance the round without adding a block). The proposer and
// time make the ledger itself a complete round history: the gap between two
// blocks' rounds is the number of empty rounds between them.
func FormatRoundMetadata(round, steps uint64, outcome, proposer string, unixMs int64) string {
	if proposer == "" {
		proposer = "unknown"
	}
	return fmt.Sprintf("round=%d,steps=%d,outcome=%s,proposer=%s,time=%d", round, steps, outcome, proposer, unixMs)
}

// ParseRoundMetadata extracts the round from FormatRoundMetadata output. It
// accepts the older "round=R,steps=S" form too.
func ParseRoundMetadata(s string) (uint64, bool) {
	for _, field := range strings.Split(s, ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(field), "=")
		if !ok || k != "round" {
			continue
		}
		r, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return 0, false
		}
		return r, true
	}
	return 0, false
}

// RoundsFromMetadata turns the last block's metadata into the failedRounds
// counter a restarting orderer should resume with, so that height+failedRounds
// is the round after the one that produced that block.
func RoundsFromMetadata(height uint64, meta string) uint64 {
	r, ok := ParseRoundMetadata(meta)
	if !ok || r+1 < height {
		return 0
	}
	return r + 1 - height
}

// IsBehind reports whether at least f+1 other orderers have voted in a round
// later than ours. With at most f Byzantine orderers, f+1 witnesses include at
// least one honest node, so a single liar cannot make us skip ahead.
func IsBehind(peerRounds map[string]uint64, round uint64, f int) bool {
	ahead := 0
	for _, r := range peerRounds {
		if r > round {
			ahead++
		}
	}
	return ahead >= f+1
}
