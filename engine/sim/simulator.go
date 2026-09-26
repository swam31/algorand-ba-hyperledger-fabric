package sim

import (
	"crypto/ed25519"

	"github.com/swayamchavan/algorand-fabric/engine/ba"
)

// SimNode wraps a ba.Node with simulator-specific state.
type SimNode struct {
	Node     *ba.Node
	IsSilent bool // if true, drops all outgoing messages
}

// Simulator coordinates an in-process network of ba.Nodes.
type Simulator struct {
	nodes []*SimNode
	
	// Tamper is a hook to simulate network unreliability and Byzantine behavior.
	// It is called for every (sender, receiver) pair on every broadcast.
	// If it returns true, the message is dropped. Otherwise, it returns the
	// (possibly modified) vote to deliver.
	Tamper func(src, dst int, v *ba.Vote) (*ba.Vote, bool)
}

// NewSimulator creates a new simulation environment.
func NewSimulator() *Simulator {
	return &Simulator{
		nodes:  make([]*SimNode, 0),
		Tamper: func(src, dst int, v *ba.Vote) (*ba.Vote, bool) { return v, false },
	}
}

// AddNode creates and adds a new honest node to the simulation.
func (s *Simulator) AddNode(id string, sk ed25519.PrivateKey, log ba.Logger) *SimNode {
	nIdx := len(s.nodes)
	net := &simNetwork{sim: s, srcIndex: nIdx}
	inbox := ba.NewInbox()
	node := ba.NewNode(id, sk, net, inbox, log)
	sn := &SimNode{Node: node}
	s.nodes = append(s.nodes, sn)
	return sn
}

// Nodes returns all nodes in the simulation.
func (s *Simulator) Nodes() []*SimNode {
	return s.nodes
}

// simNetwork is the ba.Network implementation handed to each node.
type simNetwork struct {
	sim      *Simulator
	srcIndex int
}

// Broadcast delivers a message to all other nodes in the simulation, subject to faults.
func (sn *simNetwork) Broadcast(msg []byte) error {
	src := sn.sim.nodes[sn.srcIndex]
	if src.IsSilent {
		return nil // Drop at source
	}

	// Deliver to all other nodes
	for dstIndex, dst := range sn.sim.nodes {
		if sn.srcIndex == dstIndex {
			continue // Already delivered to self in ba.Node.committeeVote
		}

		// Decode the message for EACH recipient so they don't share the same pointer.
		// This allows an equivocating adversary to mutate the vote without corrupting
		// the memory of earlier recipients.
		v, err := ba.DecodeVote(msg)
		if err != nil {
			continue
		}

		if sn.sim.Tamper != nil {
			var drop bool
			v, drop = sn.sim.Tamper(sn.srcIndex, dstIndex, v)
			if drop {
				continue
			}
		}

		dst.Node.Inbox().Add(v)
	}

	return nil
}

// InjectMessage allows an adversary to inject a custom or forged message into the network.
func (s *Simulator) InjectMessage(srcIndex int, v *ba.Vote) {
	for dstIndex, dst := range s.nodes {
		if srcIndex == dstIndex {
			continue
		}
		
		deliverVote := v
		if s.Tamper != nil {
			var drop bool
			deliverVote, drop = s.Tamper(srcIndex, dstIndex, v)
			if drop {
				continue
			}
		}
		dst.Node.Inbox().Add(deliverVote)
	}
}
