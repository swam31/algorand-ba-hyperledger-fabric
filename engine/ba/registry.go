package ba

import "crypto/ed25519"

// Member is one consensus participant and its stake (its number of lottery
// tickets). In Fabric this is one orderer node; its stake comes from channel
// configuration.
type Member struct {
	ID    string // human-readable, for logs only
	PK    ed25519.PublicKey
	Stake uint64
}

// Registry is the set of participants and the total stake W. It is the
// "ctx.weight[] / ctx.W" of the paper's pseudocode.
//
// In Fabric this is derived from the channel config, so every orderer computes
// an identical Registry - which is essential, since sortition verification
// depends on agreeing about W.
type Registry struct {
	byKey map[string]Member
	W     uint64
}

func NewRegistry(members []Member) *Registry {
	r := &Registry{byKey: make(map[string]Member, len(members))}
	for _, m := range members {
		r.byKey[string(m.PK)] = m
		r.W += m.Stake
	}
	return r
}

// Lookup returns the member for a public key, and whether it is known.
// Votes from unknown keys are worth zero - this is what makes the network
// permissioned, and is the reason a Fabric deployment does not need Algorand's
// full open-membership Sybil defence at the network layer.
func (r *Registry) Lookup(pk ed25519.PublicKey) (Member, bool) {
	m, ok := r.byKey[string(pk)]
	return m, ok
}

func (r *Registry) TotalStake() uint64 { return r.W }
func (r *Registry) Size() int          { return len(r.byKey) }

// HonestFraction is h for a network where f members are assumed Byzantine,
// used only to validate parameters at startup.
func (r *Registry) HonestFraction(f int) float64 {
	if r.Size() == 0 {
		return 0
	}
	return float64(r.Size()-f) / float64(r.Size())
}
