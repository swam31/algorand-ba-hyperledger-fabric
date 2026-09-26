package ba

import (
	"crypto/ed25519"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/swayamchavan/algorand-fabric/engine/vrf"
)

// Reserved step numbers. Every distinct step value is a distinct sortition
// lottery (see vrf.Role), so these must never collide.
const (
	StepPropose      uint64 = 0 // block proposal
	StepReductionOne uint64 = 1 // Reduction, first step  (Algorithm 7)
	StepReductionTwo uint64 = 2 // Reduction, second step
	StepBinaryBase   uint64 = 3 // BinaryBA* step k is encoded as StepBinaryBase+k

	// StepFinal is the special confirmation step that upgrades a decision from
	// TENTATIVE to FINAL (paper section 7.4). A sentinel so it can never be
	// confused with a BinaryBA* step.
	StepFinal uint64 = ^uint64(0)
)

// StepName renders a step number for logs.
func StepName(s uint64) string {
	switch s {
	case StepPropose:
		return "propose"
	case StepReductionOne:
		return "reduction-1"
	case StepReductionTwo:
		return "reduction-2"
	case StepFinal:
		return "FINAL"
	default:
		return fmt.Sprintf("binary-%d", s-StepBinaryBase)
	}
}

// Vote is one committee member's message for one step (paper Algorithm 4).
//
// It carries its own proof of committee membership, which is what makes the
// protocol stateless: a receiver needs nothing but the vote itself and the
// public seed to decide whether it counts, and for how many seats.
type Vote struct {
	Voter     ed25519.PublicKey // who
	Round     uint64            // which block height
	Step      uint64            // which step of BA*
	SortHash  []byte            // VRF output proving committee selection
	SortProof []byte            // VRF proof
	PrevHash  []byte            // hash of the block this vote builds on
	Value     []byte            // the block hash being voted for
	Sig       []byte            // signature over all of the above
}

// signingBytes is the canonical byte string that Sig covers.
//
// Encoding is length-prefixed and fixed-order so it is unambiguous: two
// different Votes can never produce the same bytes, which would otherwise let
// an attacker replay a signature from one context into another.
func (v *Vote) signingBytes() []byte {
	b := make([]byte, 0, 256)
	put := func(p []byte) {
		b = binary.BigEndian.AppendUint32(b, uint32(len(p)))
		b = append(b, p...)
	}
	put(v.Voter)
	b = binary.BigEndian.AppendUint64(b, v.Round)
	b = binary.BigEndian.AppendUint64(b, v.Step)
	put(v.SortHash)
	put(v.SortProof)
	put(v.PrevHash)
	put(v.Value)
	return b
}

// Sign fills in Sig using the voter's secret key.
func (v *Vote) Sign(sk ed25519.PrivateKey) {
	v.Sig = ed25519.Sign(sk, v.signingBytes())
}

// VerifySignature checks Sig against the claimed Voter key.
func (v *Vote) VerifySignature() bool {
	if len(v.Voter) != ed25519.PublicKeySize || len(v.Sig) != ed25519.SignatureSize {
		return false
	}
	return ed25519.Verify(v.Voter, v.signingBytes(), v.Sig)
}

// VoterID is the map key identifying a voter (hex of the public key).
func (v *Vote) VoterID() string { return string(v.Voter) }

// Role reconstructs the sortition role this vote claims membership under.
func (v *Vote) Role() vrf.Role {
	return vrf.Role{Name: "committee", Round: v.Round, Step: v.Step}
}

// --- wire encoding ---------------------------------------------------------

// Encode serialises a Vote for the network. Same length-prefixed scheme as
// signingBytes, plus the signature.
func (v *Vote) Encode() []byte {
	b := v.signingBytes()
	b = binary.BigEndian.AppendUint32(b, uint32(len(v.Sig)))
	return append(b, v.Sig...)
}

var errTruncated = errors.New("ba: truncated vote")

// DecodeVote parses bytes produced by Encode.
func DecodeVote(p []byte) (*Vote, error) {
	off := 0
	readBytes := func() ([]byte, error) {
		if off+4 > len(p) {
			return nil, errTruncated
		}
		n := int(binary.BigEndian.Uint32(p[off:]))
		off += 4
		if n < 0 || off+n > len(p) {
			return nil, errTruncated
		}
		out := p[off : off+n]
		off += n
		return out, nil
	}
	readU64 := func() (uint64, error) {
		if off+8 > len(p) {
			return 0, errTruncated
		}
		x := binary.BigEndian.Uint64(p[off:])
		off += 8
		return x, nil
	}

	v := &Vote{}
	var err error
	var raw []byte
	if raw, err = readBytes(); err != nil {
		return nil, err
	}
	v.Voter = ed25519.PublicKey(raw)
	if v.Round, err = readU64(); err != nil {
		return nil, err
	}
	if v.Step, err = readU64(); err != nil {
		return nil, err
	}
	if v.SortHash, err = readBytes(); err != nil {
		return nil, err
	}
	if v.SortProof, err = readBytes(); err != nil {
		return nil, err
	}
	if v.PrevHash, err = readBytes(); err != nil {
		return nil, err
	}
	if v.Value, err = readBytes(); err != nil {
		return nil, err
	}
	if v.Sig, err = readBytes(); err != nil {
		return nil, err
	}
	if off != len(p) {
		return nil, errors.New("ba: trailing bytes after vote")
	}
	return v, nil
}
