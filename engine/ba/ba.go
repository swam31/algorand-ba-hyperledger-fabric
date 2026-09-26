package ba

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"errors"

	"github.com/swayamchavan/algorand-fabric/engine/vrf"
)

// Outcome distinguishes the two kinds of agreement BA* can reach.
type Outcome int

const (
	// Tentative: agreement was reached, but we could not PROVE that no other
	// value was also agreed elsewhere. Do not confirm transactions on it yet.
	Tentative Outcome = iota

	// Final: mathematically guaranteed that no other block can ever be agreed
	// for this round. Safe to commit and confirm.
	Final
)

func (o Outcome) String() string {
	if o == Final {
		return "FINAL"
	}
	return "TENTATIVE"
}

// ErrMaxSteps means BinaryBA* ran past MaxSteps without deciding. The paper
// treats this as evidence of a network problem and hands over to the recovery
// protocol (section 8.2) rather than looping forever.
var ErrMaxSteps = errors.New("ba: exceeded MaxSteps without consensus")

// Network is the gossip transport BA* needs. The engine deliberately knows
// nothing about how messages actually travel - in the simulator this is Go
// channels, in Fabric it is the orderer cluster's gRPC service.
type Network interface {
	Broadcast(msg []byte) error
}

// Logger is optional structured logging; nil is fine.
type Logger interface {
	Debugf(format string, args ...any)
}

type nopLogger struct{}

func (nopLogger) Debugf(string, ...any) {}

// Node is one BA* participant.
//
// Beyond its private key it holds NO secret state. That is a deliberate
// property of the paper's design, not an accident: it means a node can be
// replaced on any step boundary, and it means any observer can follow the
// protocol and reach the same conclusion without being a participant.
type Node struct {
	ID    string
	sk    ed25519.PrivateKey
	pk    ed25519.PublicKey
	net   Network
	inbox *Inbox
	log   Logger
}

func NewNode(id string, sk ed25519.PrivateKey, net Network, inbox *Inbox, log Logger) *Node {
	if log == nil {
		log = nopLogger{}
	}
	return &Node{
		ID:    id,
		sk:    sk,
		pk:    sk.Public().(ed25519.PublicKey),
		net:   net,
		inbox: inbox,
		log:   log,
	}
}

func (n *Node) PublicKey() ed25519.PublicKey { return n.pk }
func (n *Node) Inbox() *Inbox                { return n.inbox }

// --- Algorithm 4: CommitteeVote ------------------------------------------

// committeeVote runs sortition for this step and, only if selected, gossips a
// signed vote carrying its own proof of selection.
//
// A node that loses the lottery says NOTHING. That silence is not a failure
// mode - it is the mechanism. Only a small random subset speaks each step,
// which is what keeps the message cost independent of the network size.
func (n *Node) committeeVote(c *Context, step uint64, tau float64, value []byte) uint64 {
	m, ok := c.Registry.Lookup(n.pk)
	if !ok {
		return 0 // we are not a participant on this channel
	}

	role := vrf.Role{Name: "committee", Round: c.Round, Step: step}
	hash, proof, seats := vrf.Sortition(n.sk, c.Seed, tau, role, m.Stake, c.Registry.TotalStake())
	if seats == 0 {
		return 0 // lost the lottery: stay silent
	}

	v := &Vote{
		Voter:     n.pk,
		Round:     c.Round,
		Step:      step,
		SortHash:  hash,
		SortProof: proof,
		PrevHash:  c.PrevHash,
		Value:     value,
	}
	v.Sign(n.sk)

	n.log.Debugf("%s vote round=%d step=%s seats=%d value=%x", n.ID, c.Round, StepName(step), seats, trunc(value))

	// Deliver to ourselves too: we are a member of our own committee and our
	// seats must count toward the threshold, exactly as any other node's do.
	n.inbox.Add(v)
	_ = n.net.Broadcast(v.Encode())
	return seats
}

// --- Algorithm 7: Reduction ----------------------------------------------

// reduction narrows the problem from "agree on one of many proposed blocks" to
// "agree on one of exactly two values: this block, or the empty block".
//
// Why bother? Binary agreement is a well-understood problem with a clean
// termination argument. Multi-valued agreement is not. Two cheap steps here buy
// a much simpler and provably-terminating second phase.
func (n *Node) reduction(c *Context, hblock []byte) ([]byte, uint64) {
	p := c.Params
	emptyHash := EmptyBlockHash(c.Round, c.PrevHash)

	// Step 1: tell everyone what block hash we are holding.
	seats := n.committeeVote(c, StepReductionOne, p.TauStep, hblock)

	// Longer timeout here than elsewhere: other nodes may still be waiting for
	// the block itself to arrive over gossip, so we allow LambdaBlock on top.
	hblock1, ok := c.countVotes(n.inbox, StepReductionOne, p.StepThreshold(), p.TauStep, p.LambdaBlock+p.LambdaStep)

	// Step 2: re-broadcast whatever was popular in step 1.
	if !ok {
		n.committeeVote(c, StepReductionTwo, p.TauStep, emptyHash)
	} else {
		n.committeeVote(c, StepReductionTwo, p.TauStep, hblock1)
	}

	hblock2, ok := c.countVotes(n.inbox, StepReductionTwo, p.StepThreshold(), p.TauStep, p.LambdaStep)
	if !ok {
		return emptyHash, seats
	}
	return hblock2, seats
}

// --- Algorithm 8: BinaryBA* ----------------------------------------------

// binStep maps a BinaryBA* step index to a global step number, keeping it
// distinct from the reduction and final steps so each gets its own lottery.
func binStep(k uint64) uint64 { return StepBinaryBase + k }

// binaryBA decides between blockHash and the empty block.
//
// The loop body has THREE sub-steps, and their asymmetry is the core of the
// design:
//
//	A - can only ever terminate on the REAL block
//	B - can only ever terminate on the EMPTY block
//	C - breaks a deadlock using a common coin
//
// Two separate exits that cannot contradict each other, so honest nodes never
// terminate on different values in the same pass.
func (n *Node) binaryBA(c *Context, blockHash []byte) ([]byte, uint64, error) {
	p := c.Params
	emptyHash := EmptyBlockHash(c.Round, c.PrevHash)
	r := blockHash
	step := uint64(1)

	// commit finishes the protocol: before returning we keep voting for our
	// decided value in the next three steps.
	//
	// This is not politeness. A node that decides and immediately goes silent
	// removes its seats from the tally that slower nodes are still trying to
	// push over the threshold. If enough deciders go quiet at once, the
	// remaining nodes can never reach T*tau and the round stalls. Echoing for
	// three steps drags everyone else across the line.
	commit := func(val []byte, at uint64) {
		for s := at + 1; s <= at+3; s++ {
			n.committeeVote(c, binStep(s), p.TauStep, val)
		}
	}

	for step < uint64(p.MaxSteps) {
		// ---- sub-step A: commits only to the real block ----
		n.committeeVote(c, binStep(step), p.TauStep, r)
		v, ok := c.countVotes(n.inbox, binStep(step), p.StepThreshold(), p.TauStep, p.LambdaStep)
		switch {
		case !ok:
			r = blockHash // optimistic default on timeout
		case !bytesEqual(v, emptyHash):
			r = v
			commit(r, step)
			if step == 1 {
				// Decided on the very FIRST step. Only such a decision is
				// eligible to be FINAL, because only then can we be confident
				// no competing value was ever in play. Cast the final vote.
				n.committeeVote(c, StepFinal, p.TauFinal, r)
			}
			n.log.Debugf("%s binaryBA decided BLOCK at step %d", n.ID, step)
			return r, step, nil
		default:
			r = v // saw empty; carry it forward
		}
		step++

		// ---- sub-step B: commits only to the empty block ----
		n.committeeVote(c, binStep(step), p.TauStep, r)
		v, ok = c.countVotes(n.inbox, binStep(step), p.StepThreshold(), p.TauStep, p.LambdaStep)
		switch {
		case !ok:
			r = emptyHash // pessimistic default on timeout
		case bytesEqual(v, emptyHash):
			r = v
			commit(r, step)
			n.log.Debugf("%s binaryBA decided EMPTY at step %d", n.ID, step)
			return r, step, nil
		default:
			r = v
		}
		step++

		// ---- sub-step C: common-coin tie-break ----
		n.committeeVote(c, binStep(step), p.TauStep, r)
		v, ok = c.countVotes(n.inbox, binStep(step), p.StepThreshold(), p.TauStep, p.LambdaStep)
		if !ok {
			// Deadlocked. The fallback MUST be unpredictable, or an adversary
			// who knows what we will do on timeout can steer us apart forever
			// by selectively delivering its own votes. See commonCoin.
			if n.commonCoin(c, binStep(step)) == 0 {
				r = blockHash
			} else {
				r = emptyHash
			}
			n.log.Debugf("%s coin flip at step %d -> %x", n.ID, step, trunc(r))
		} else {
			r = v
		}
		step++
	}

	return nil, step, ErrMaxSteps
}

// --- Algorithm 9: CommonCoin ---------------------------------------------

// commonCoin produces a bit that is USUALLY the same for every node, and that
// nobody could predict before the step began.
//
// Every node takes the lowest sub-user hash it saw this step and returns its
// least significant bit. Because those hashes are VRF outputs, the bit is
// pseudorandom and was unknowable in advance.
//
// It does NOT need to be unanimous. The lowest hash belongs to an honest node
// with probability h > 2/3; when it does, everyone who received that node's
// message sees the same bit, and an adversary's guess is wrong half the time.
// So each pass of the BinaryBA* loop escapes with probability at least h/2 > 1/3,
// and the chance of still being stuck decays geometrically.
//
// DEVIATION FROM THE PAPER. Algorithm 9 prints "for 1 <= j < votes", which
// contributes no hash at all for a node holding a single seat. We read that as
// a typo and iterate j = 1..votes inclusive, so every seat contributes exactly
// one draw. With the printed bound a single-seat node would be silently
// excluded from the coin, weakening it for no reason.
func (n *Node) commonCoin(c *Context, step uint64) int {
	var minHash []byte
	for _, v := range n.inbox.All(c.Round, step) {
		seats, _, sortHash := c.processMsg(c.Params.TauStep, v)
		if seats == 0 {
			continue
		}
		for j := uint64(1); j <= seats; j++ {
			h := vrf.SubUserHash(sortHash, j)
			if minHash == nil || vrf.Less(h, minHash) {
				minHash = h
			}
		}
	}
	if minHash == nil {
		return 0 // saw nothing; any deterministic choice is as good as another
	}
	return int(minHash[len(minHash)-1] & 1)
}

// --- Algorithm 3: BA* -----------------------------------------------------

// Result is what one round of BA* produces.
type Result struct {
	Outcome   Outcome
	BlockHash []byte // the agreed hash
	IsEmpty   bool   // true if the round agreed on the empty block
	Steps     uint64 // BinaryBA* steps taken, for measurement
	SeatsWon  uint64
}

// Run executes one full round of BA* on a proposed block.
//
// blockHash is the hash of the highest-priority block this node received during
// proposal, or the empty-block hash if none arrived in time.
func (n *Node) Run(c *Context, blockHash []byte) (*Result, error) {
	p := c.Params
	emptyHash := EmptyBlockHash(c.Round, c.PrevHash)

	// Phase 1: many candidate blocks -> two options.
	h, seats := n.reduction(c, blockHash)

	// Phase 2: pick one of the two.
	agreed, steps, err := n.binaryBA(c, h)
	if err != nil {
		return nil, err
	}

	// Honest nodes cast FINAL votes only for a real block decided on the
	// first BinaryBA* step, so for the empty block the FINAL count below could
	// only time out and return TENTATIVE. Skip that wait: same outcome, one
	// LambdaStep sooner on every empty round.
	if bytesEqual(agreed, emptyHash) {
		return &Result{BlockHash: agreed, IsEmpty: true, Steps: steps, Outcome: Tentative, SeatsWon: seats}, nil
	}

	// Phase 3: was that decision provably unique?
	//
	// A FINAL vote is cast only by nodes that decided on BinaryBA* step 1. If
	// enough of those votes exist to cross the (stricter) final threshold, then
	// a large, verified majority already committed to this value and cannot
	// vote for anything else this round - so no other block can ever be agreed.
	// Anything less is TENTATIVE: probably right, but not provable.
	final, ok := c.countVotes(n.inbox, StepFinal, p.FinalThreshold(), p.TauFinal, p.LambdaStep)

	res := &Result{
		BlockHash: agreed,
		IsEmpty:   bytesEqual(agreed, emptyHash),
		Steps:     steps,
		Outcome:   Tentative,
		SeatsWon:  seats,
	}
	if ok && bytesEqual(agreed, final) {
		res.Outcome = Final
	}
	return res, nil
}

// --- helpers --------------------------------------------------------------

// EmptyBlockHash is the canonical hash of the empty block for a round.
//
// It must be derivable identically by every node without communication - it is
// the fallback all of them converge on when a proposer misbehaves or the
// network drops the real block.
func EmptyBlockHash(round uint64, prevHash []byte) []byte {
	h := sha256.New()
	h.Write([]byte("algorand-empty-block"))
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], round)
	h.Write(b[:])
	h.Write(prevHash)
	return h.Sum(nil)
}

// HashBlock is the block hash function used for voting. BA* votes on 32-byte
// hashes rather than whole blocks, which is why its bandwidth cost is
// independent of block size (paper section 7.1).
func HashBlock(payload []byte) []byte {
	s := sha256.Sum256(payload)
	return s[:]
}

func trunc(b []byte) []byte {
	if len(b) > 4 {
		return b[:4]
	}
	return b
}
