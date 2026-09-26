package config

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/swayamchavan/algorand-fabric/engine/ba"
)

// ParseCommaList splits a string by commas, trims whitespace, and drops empty strings.
func ParseCommaList(s string) []string {
	var res []string
	parts := strings.Split(s, ",")
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			res = append(res, trimmed)
		}
	}
	return res
}

// ParseMembers decodes the nodeID:pubkeyHex:stake format into ba.Member structs.
func ParseMembers(membersStr string) ([]ba.Member, error) {
	var members []ba.Member
	parts := ParseCommaList(membersStr)
	for _, p := range parts {
		fields := strings.Split(p, ":")
		if len(fields) != 3 {
			return nil, fmt.Errorf("invalid member format (expected ID:pubkeyHex:stake): %s", p)
		}

		id := strings.TrimSpace(fields[0])
		pkHex := strings.TrimSpace(fields[1])
		stakeStr := strings.TrimSpace(fields[2])

		pk, err := hex.DecodeString(pkHex)
		if err != nil {
			return nil, fmt.Errorf("invalid pubkey hex for member %s: %w", id, err)
		}

		stake, err := strconv.ParseUint(stakeStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid stake for member %s: %w", id, err)
		}

		members = append(members, ba.Member{
			ID:    id,
			PK:    pk,
			Stake: stake,
		})
	}
	return members, nil
}

// ValidateMembers ensures the registry configuration meets all BA* requirements.
func ValidateMembers(members []ba.Member, selfPK []byte, params ba.Params) (uint64, float64, error) {
	if len(members) < 4 {
		return 0, 0, fmt.Errorf("BA* requires at least 4 members, got %d", len(members))
	}

	var totalStake uint64
	hasSelfPK := false
	for _, m := range members {
		if m.Stake == 0 {
			return 0, 0, fmt.Errorf("member %s has 0 stake", m.ID)
		}
		totalStake += m.Stake
		if bytes.Equal(m.PK, selfPK) {
			hasSelfPK = true
		}
	}

	if !hasSelfPK {
		return 0, 0, fmt.Errorf("local node public key is absent from the members list")
	}

	n := uint64(len(members))
	fSafety := (n - 1) / 3

	fFinal := uint64(0)
	for i := fSafety; ; i-- {
		h := float64(n-i) / float64(n)
		if h > params.TFinal {
			fFinal = i
			break
		}
		if i == 0 {
			break
		}
	}

	f := fSafety
	if fFinal < f {
		f = fFinal
	}

	h := float64(n-f) / float64(n)

	if f < 1 {
		return 0, 0, fmt.Errorf("network is too small to tolerate any fault at this TFinal")
	}

	fmt.Printf(">>> [ALGORAND BA*] n=%d tolerating f=%d Byzantine orderers (h=%.3f); safety bound allows %d, FINAL reachability allows %d, using the lower\n",
		n, f, h, fSafety, fFinal)

	if err := params.Validate(h); err != nil {
		return 0, 0, fmt.Errorf("BA* parameters validation failed for h=%f: %w", h, err)
	}

	return totalStake, h, nil
}
