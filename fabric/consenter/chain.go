package consenter

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric/orderer/consensus"
	"github.com/hyperledger/fabric/protoutil"
	"github.com/swayamchavan/algorand-fabric/engine/ba"
	"github.com/swayamchavan/algorand-fabric/engine/vrf"
	"github.com/swayamchavan/algorand-fabric/fabric/config"
	"github.com/swayamchavan/algorand-fabric/fabric/telemetry"
	"github.com/swayamchavan/algorand-fabric/fabric/transport"
)

// Chain implements the consensus.Chain interface for the Algorand consenter.
// It wires the Fabric ordering service to the BA* engine.
//
// One BA* round runs per block attempt, numbered height+failedRounds. Every
// round either writes exactly one block (height++) or agrees on the empty block
// (failedRounds++), so each attempt gets a fresh round number, seed and
// proposer lottery, and every honest node walks through the same sequence.
type Chain struct {
	support   consensus.ConsenterSupport
	node      *ba.Node
	inbox     *ba.Inbox
	registry  *ba.Registry
	transport *transport.Transport
	telemetry *telemetry.Server

	nodeID    string
	privKey   ed25519.PrivateKey
	seedChain *config.SeedChain
	channelID string
	params    ba.Params
	tolerated int // f: Byzantine orderers tolerated

	// Channels for managing the main consensus loop
	haltChan  chan struct{}
	errorChan chan struct{}

	txChan     chan *common.Envelope
	configChan chan *common.Envelope
	notify     chan struct{} // capacity 1: a proposal, wakeup or peer round arrived

	sync.Mutex
	// height is our own count of written blocks. Fabric's Height() reads the
	// ledger, which WriteBlock updates asynchronously, so reading it straight
	// after a write can return the old value and repeat a round number.
	height       uint64
	failedRounds uint64

	// poolMu guards everything the network goroutine hands to the main loop.
	// Nothing here is dropped just because it is for a round we have not
	// reached yet: a node that falls behind needs exactly those messages to
	// catch up. Entries are pruned only once their round is decided.
	poolMu     sync.Mutex
	proposals  map[uint64][]*ProposalMsg       // round -> as received, unverified
	candidates map[uint64]map[string]candidate // round -> block hash -> block
	wanted     map[uint64][]byte               // round -> hash we are fetching
	wakeups    map[uint64]bool
	peerRounds map[string]uint64 // member ID -> highest round it has signed a vote in

	// txpool holds every orderer's pending transactions (see config.TxPool).
	txpool *config.TxPool

	// lastHeard (guarded by poolMu) is when each other orderer last sent us a
	// signed heartbeat or vote. quorumPeers is how many others we must hear
	// from to be part of a quorum (n - f - 1).
	lastHeard   map[string]time.Time
	quorumPeers int

	// voteArchive (guarded by poolMu) keeps the encoded votes of recently
	// decided rounds so peers that missed them can catch up (see
	// VotesRequestMsg). archiveOrder evicts the oldest first.
	voteArchive  map[uint64][][]byte
	archiveOrder []uint64
	lastAsked    map[uint64]time.Time

	wg sync.WaitGroup
}

// Order accepts normal transactions from Fabric.
//
// The main loop gossips each one to the other orderers, so every orderer's
// pool holds everyone's pending transactions and the winning proposer's block
// includes them all. The block written in a round is the one carried by the
// winning proposal (paper section 6), fetched from peers when a node agreed on
// it without having received it.
func (c *Chain) Order(env *common.Envelope, configSeq uint64) error {
	if err := c.checkQuorum(); err != nil {
		return err
	}
	select {
	case c.txChan <- env:
		return nil
	case <-c.haltChan:
		return nil // shutdown
	}
}

// Configure accepts config transactions
func (c *Chain) Configure(configEnv *common.Envelope, configSeq uint64) error {
	if err := c.checkQuorum(); err != nil {
		return err
	}
	select {
	case c.configChan <- configEnv:
		return nil
	case <-c.haltChan:
		return nil
	}
}

// WaitReady blocks until the chain is ready to accept messages
func (c *Chain) WaitReady() error {
	return nil
}

// Errored returns a channel which will close when an error occurs
func (c *Chain) Errored() <-chan struct{} {
	return c.errorChan
}

// Start boots the algorithm
func (c *Chain) Start() {
	if c.transport != nil {
		if err := c.transport.Start(); err != nil {
			fmt.Printf(">>> [ALGORAND BA*] Failed to start transport: %v\n", err)
		}
		c.wg.Add(1)
		go c.networkLoop()
		c.wg.Add(1)
		go c.heartbeatLoop()
	}

	c.wg.Add(1)
	go c.mainLoop()
}

// Halt shuts down the chain
func (c *Chain) Halt() {
	select {
	case <-c.haltChan:
		// already halted
	default:
		close(c.haltChan)
	}
	if c.transport != nil {
		c.transport.Close()
	}
	c.wg.Wait()
}

// --- network side ---------------------------------------------------------

func (c *Chain) networkLoop() {
	defer c.wg.Done()
	for {
		select {
		case <-c.haltChan:
			return
		case msg, ok := <-c.transport.Incoming():
			if !ok {
				return
			}
			if len(msg) < 1 {
				continue
			}
			c.handleMessage(msg[0], msg[1:])
		}
	}
}

func (c *Chain) handleMessage(msgType byte, payload []byte) {
	switch msgType {
	case MsgTypeVote:
		vote, err := ba.DecodeVote(payload)
		if err != nil {
			return
		}
		c.inbox.Add(vote)
		c.notePeerRound(vote)
		c.recordVote("recv", vote)

	case MsgTypeProposal:
		var prop ProposalMsg
		if err := json.Unmarshal(payload, &prop); err != nil {
			fmt.Printf(">>> [ALGORAND BA*] Dropping malformed proposal: %v\n", err)
			return
		}
		c.storeProposal(&prop)

	case MsgTypeEnvelope:
		// A transaction another orderer's client submitted. The mesh is a
		// full graph, so it is not forwarded again.
		if c.txpool.Add(append([]byte(nil), payload...)) {
			c.poke()
		}

	case MsgTypeWakeup:
		if len(payload) != 8 {
			return
		}
		round := binary.BigEndian.Uint64(payload)
		c.poolMu.Lock()
		c.wakeups[round] = true
		c.poolMu.Unlock()
		c.poke()

	case MsgTypeBlockRequest:
		var req BlockRequestMsg
		if err := json.Unmarshal(payload, &req); err == nil {
			c.answerBlockRequest(&req)
		}

	case MsgTypeBlockReply:
		var rep BlockReplyMsg
		if err := json.Unmarshal(payload, &rep); err == nil {
			c.storeFetchedBlock(&rep)
		}

	case MsgTypeHeartbeat:
		var hb HeartbeatMsg
		if err := json.Unmarshal(payload, &hb); err == nil {
			c.noteHeartbeat(&hb)
		}

	case MsgTypeTxRequest:
		var req TxRequestMsg
		if err := json.Unmarshal(payload, &req); err != nil || len(req.Hashes)%32 != 0 {
			return
		}
		var rep TxReplyMsg
		for i := 0; i < len(req.Hashes); i += 32 {
			var h [32]byte
			copy(h[:], req.Hashes[i:])
			if raw, ok := c.txpool.Get(h); ok {
				rep.Txs = append(rep.Txs, raw)
			}
		}
		if len(rep.Txs) > 0 {
			raw, _ := json.Marshal(&rep)
			c.send(MsgTypeTxReply, raw)
		}

	case MsgTypeVotesRequest:
		var req VotesRequestMsg
		if err := json.Unmarshal(payload, &req); err != nil {
			return
		}
		if votes := c.votesFor(req.Round); len(votes) > 0 {
			raw, _ := json.Marshal(&VotesReplyMsg{Round: req.Round, Votes: votes})
			c.send(MsgTypeVotesReply, raw)
		}

	case MsgTypeVotesReply:
		var rep VotesReplyMsg
		if err := json.Unmarshal(payload, &rep); err != nil || rep.Round < c.currentRound() {
			return
		}
		for _, raw := range rep.Votes {
			if v, err := ba.DecodeVote(raw); err == nil && v.Round == rep.Round {
				c.inbox.Add(v)
				c.notePeerRound(v)
			}
		}

	case MsgTypeTxReply:
		var rep TxReplyMsg
		if err := json.Unmarshal(payload, &rep); err == nil {
			for _, tx := range rep.Txs {
				c.txpool.Add(tx)
			}
			c.poke()
		}

	default:
		fmt.Printf(">>> [ALGORAND BA*] Unknown message type: %d\n", msgType)
	}
}

// notePeerRound remembers the latest round each orderer has signed a vote in.
// Only signed votes count: they are the one message type a peer cannot forge
// on another's behalf.
func (c *Chain) notePeerRound(v *ba.Vote) {
	m, ok := c.registry.Lookup(v.Voter)
	if !ok || !v.VerifySignature() {
		return
	}
	c.poolMu.Lock()
	advanced := v.Round > c.peerRounds[m.ID]
	if advanced {
		c.peerRounds[m.ID] = v.Round
	}
	c.lastHeard[m.ID] = time.Now()
	c.poolMu.Unlock()
	if advanced {
		c.poke()
	}
}

func (c *Chain) storeProposal(p *ProposalMsg) {
	if p.Round < c.currentRound() {
		return // already decided
	}
	c.poolMu.Lock()
	for _, q := range c.proposals[p.Round] {
		if bytes.Equal(q.PubKey, p.PubKey) {
			c.poolMu.Unlock()
			return // one proposal per proposer per round; the first one counts
		}
	}
	c.proposals[p.Round] = append(c.proposals[p.Round], p)
	c.poolMu.Unlock()
	c.poke()
}

// answerBlockRequest serves block data to a peer that agreed on a block it
// never received. Undecided rounds are served from the candidate pool, decided
// ones from the ledger.
func (c *Chain) answerBlockRequest(req *BlockRequestMsg) {
	data := c.candidate(req.Round, req.Hash)
	if data == nil {
		data = c.committedData(req.Round, req.Hash)
	}
	if data == nil {
		return
	}
	raw, err := proto.Marshal(data)
	if err != nil {
		return
	}
	rep, _ := json.Marshal(&BlockReplyMsg{Round: req.Round, BlockData: raw})
	c.send(MsgTypeBlockReply, rep)
}

// committedData looks a decided round up in the ledger via the round number
// stored in each block's consenter metadata.
func (c *Chain) committedData(round uint64, hash []byte) *common.BlockData {
	c.Lock()
	h := c.height
	c.Unlock()
	for n, scanned := h, 0; n > 0 && scanned < 1024; n, scanned = n-1, scanned+1 {
		blk := c.support.Block(n - 1)
		if blk == nil {
			continue // written but not yet committed; the requester will ask again
		}
		m, err := protoutil.GetConsenterMetadataFromBlock(blk)
		if err != nil {
			continue
		}
		r, ok := config.ParseRoundMetadata(string(m.Value))
		if !ok {
			continue
		}
		if r < round {
			return nil
		}
		if r == round && bytes.Equal(dataHash(blk.Data), hash) {
			return blk.Data
		}
	}
	return nil
}

func (c *Chain) storeFetchedBlock(rep *BlockReplyMsg) {
	data := &common.BlockData{}
	if err := proto.Unmarshal(rep.BlockData, data); err != nil {
		return
	}
	hash := dataHash(data)
	c.poolMu.Lock()
	want := c.wanted[rep.Round]
	c.poolMu.Unlock()
	// Only data that hashes to the value BA* agreed on is accepted, which is
	// why it does not matter which peer answered.
	if want != nil && bytes.Equal(hash, want) {
		c.addCandidate(rep.Round, hash, data, "")
	}
}

// --- connectivity -------------------------------------------------------------

const heartbeatEvery = time.Second

// heardWithin is how recent a heartbeat must be for its sender to count as
// reachable: three missed heartbeats.
const heardWithin = 3 * heartbeatEvery

func (c *Chain) heartbeatLoop() {
	defer c.wg.Done()
	pk := c.privKey.Public().(ed25519.PublicKey)
	t := time.NewTicker(heartbeatEvery)
	defer t.Stop()
	for {
		select {
		case <-c.haltChan:
			return
		case <-t.C:
			ms := time.Now().UnixMilli()
			hb := &HeartbeatMsg{PubKey: pk, UnixMs: ms, Sig: ed25519.Sign(c.privKey, heartbeatSigningBytes(ms))}
			raw, _ := json.Marshal(hb)
			c.send(MsgTypeHeartbeat, raw)
		}
	}
}

func heartbeatSigningBytes(ms int64) []byte {
	return binary.BigEndian.AppendUint64([]byte("algorand-fabric-heartbeat"), uint64(ms))
}

func (c *Chain) noteHeartbeat(hb *HeartbeatMsg) {
	m, ok := c.registry.Lookup(ed25519.PublicKey(hb.PubKey))
	if !ok || !ed25519.Verify(m.PK, heartbeatSigningBytes(hb.UnixMs), hb.Sig) {
		return
	}
	// A replayed old heartbeat must not make a dead link look alive.
	if age := time.Since(time.UnixMilli(hb.UnixMs)); age > heardWithin || age < -heardWithin {
		return
	}
	c.poolMu.Lock()
	c.lastHeard[m.ID] = time.Now()
	c.poolMu.Unlock()
}

// checkQuorum refuses new transactions while this orderer cannot hear enough
// other orderers to take part in agreement. Fabric answers the client with
// SERVICE_UNAVAILABLE, and the gateway tries another orderer. Accepting them
// instead would strand them here, unsent, until the link came back.
func (c *Chain) checkQuorum() error {
	if c.transport == nil {
		return nil
	}
	c.poolMu.Lock()
	heard := 0
	for _, t := range c.lastHeard {
		if time.Since(t) <= heardWithin {
			heard++
		}
	}
	c.poolMu.Unlock()
	if heard < c.quorumPeers {
		return fmt.Errorf("this orderer can reach only %d other orderer(s), %d are needed for a quorum; submit to another orderer", heard, c.quorumPeers)
	}
	return nil
}

// --- catching up on missed votes ------------------------------------------------

const archiveRounds = 4096

// roundSteps lists every step a round's votes can belong to.
func (c *Chain) roundSteps() []uint64 {
	steps := []uint64{ba.StepReductionOne, ba.StepReductionTwo}
	for k := uint64(1); k <= uint64(c.params.MaxSteps)+3; k++ {
		steps = append(steps, ba.StepBinaryBase+k)
	}
	return append(steps, ba.StepFinal)
}

func (c *Chain) collectVotes(round uint64) [][]byte {
	var out [][]byte
	for _, step := range c.roundSteps() {
		for _, v := range c.inbox.All(round, step) {
			out = append(out, v.Encode())
		}
	}
	return out
}

func (c *Chain) archiveVotes(round uint64) {
	votes := c.collectVotes(round)
	c.poolMu.Lock()
	defer c.poolMu.Unlock()
	if _, ok := c.voteArchive[round]; !ok {
		c.archiveOrder = append(c.archiveOrder, round)
	}
	c.voteArchive[round] = votes
	for len(c.archiveOrder) > archiveRounds {
		delete(c.voteArchive, c.archiveOrder[0])
		c.archiveOrder = c.archiveOrder[1:]
	}
	for r := range c.lastAsked {
		if r <= round {
			delete(c.lastAsked, r)
		}
	}
}

// votesFor serves a decided round from the archive, or the round in progress
// from the live inbox.
func (c *Chain) votesFor(round uint64) [][]byte {
	c.poolMu.Lock()
	votes, ok := c.voteArchive[round]
	c.poolMu.Unlock()
	if ok {
		return votes
	}
	return c.collectVotes(round)
}

func (c *Chain) requestVotes(round uint64) {
	c.poolMu.Lock()
	if time.Since(c.lastAsked[round]) < time.Second {
		c.poolMu.Unlock()
		return
	}
	c.lastAsked[round] = time.Now()
	c.poolMu.Unlock()
	raw, _ := json.Marshal(&VotesRequestMsg{Round: round})
	c.send(MsgTypeVotesRequest, raw)
}

// --- pool helpers -----------------------------------------------------------

func (c *Chain) poke() {
	select {
	case c.notify <- struct{}{}:
	default:
	}
}

func (c *Chain) currentRound() uint64 {
	c.Lock()
	defer c.Unlock()
	return c.height + c.failedRounds
}

func (c *Chain) isBehind(round uint64) bool {
	c.poolMu.Lock()
	defer c.poolMu.Unlock()
	return config.IsBehind(c.peerRounds, round, c.tolerated)
}

// invited reports whether another orderer has started this round, so an
// orderer with nothing to propose should join it.
func (c *Chain) invited(round uint64) bool {
	c.poolMu.Lock()
	defer c.poolMu.Unlock()
	return c.wakeups[round] || len(c.proposals[round]) > 0
}

func (c *Chain) prunePool(round uint64) {
	c.poolMu.Lock()
	defer c.poolMu.Unlock()
	for r := range c.proposals {
		if r < round {
			delete(c.proposals, r)
		}
	}
	for r := range c.candidates {
		if r < round {
			delete(c.candidates, r)
		}
	}
	for r := range c.wanted {
		if r < round {
			delete(c.wanted, r)
		}
	}
	for r := range c.wakeups {
		if r < round {
			delete(c.wakeups, r)
		}
	}
}

// candidate is a block that could be agreed on this round, with the orderer
// that proposed it ("" when it arrived through a block fetch).
type candidate struct {
	data     *common.BlockData
	proposer string
}

func (c *Chain) addCandidate(round uint64, hash []byte, data *common.BlockData, proposer string) {
	c.poolMu.Lock()
	defer c.poolMu.Unlock()
	if c.candidates[round] == nil {
		c.candidates[round] = make(map[string]candidate)
	}
	if _, known := c.candidates[round][string(hash)]; !known || proposer != "" {
		c.candidates[round][string(hash)] = candidate{data: data, proposer: proposer}
	}
}

func (c *Chain) candidate(round uint64, hash []byte) *common.BlockData {
	c.poolMu.Lock()
	defer c.poolMu.Unlock()
	return c.candidates[round][string(hash)].data
}

func (c *Chain) proposerOf(round uint64, hash []byte) string {
	c.poolMu.Lock()
	defer c.poolMu.Unlock()
	return c.candidates[round][string(hash)].proposer
}

func (c *Chain) send(msgType byte, payload []byte) {
	if c.transport != nil {
		c.transport.Broadcast(append([]byte{msgType}, payload...))
	}
}

// --- main loop ------------------------------------------------------------

// ready is always selectable; it stands in for "run a round now".
var ready = func() <-chan time.Time {
	ch := make(chan time.Time)
	close(ch)
	return ch
}()

// mainLoop starts a round as soon as there is anything to propose - a pending
// transaction from any orderer, or a config update - or when another orderer
// has started one, or when we are behind. Fabric's BatchTimeout is not used:
// the round's own lambda_priority wait already gathers what arrives, and
// waiting for the timer only added up to 2 s to every transaction.
func (c *Chain) mainLoop() {
	defer c.wg.Done()

	// Config updates are rare; they travel alone in their own block and are
	// proposed only by the orderer that received them.
	var configQ [][]byte

	onConfig := func(env *common.Envelope) {
		raw, err := proto.Marshal(env)
		if err != nil {
			fmt.Printf(">>> [ALGORAND BA*] Dropping unmarshalable config envelope: %v\n", err)
			return
		}
		configQ = append(configQ, raw)
	}
	onTx := func(env *common.Envelope) {
		raw, err := proto.Marshal(env)
		if err != nil {
			return
		}
		if c.txpool.Add(raw) && c.transport != nil {
			c.transport.BroadcastLossy(append([]byte{MsgTypeEnvelope}, raw...))
		}
	}

	for {
		// Take in everything already queued before deciding what to do, so
		// a round proposes all of it.
	drain:
		for {
			select {
			case env := <-c.configChan:
				onConfig(env)
			case env := <-c.txChan:
				onTx(env)
			default:
				break drain
			}
		}

		round := c.currentRound()
		c.prunePool(round)

		var runNow <-chan time.Time
		if len(configQ) > 0 || c.txpool.Len() > 0 || c.invited(round) || c.isBehind(round) {
			runNow = ready
		}

		select {
		case <-c.haltChan:
			return
		case env := <-c.configChan:
			onConfig(env)
		case env := <-c.txChan:
			onTx(env)
		case <-c.notify:
			// Something arrived; loop round and re-evaluate.
		case <-runNow:
			var proposal [][]byte
			if len(configQ) > 0 {
				proposal = configQ[:1]
			} else {
				limits := c.support.SharedConfig().BatchSize()
				proposal = c.txpool.Take(int(limits.MaxMessageCount), int(limits.PreferredMaxBytes))
			}
			if err := c.runRound(proposal); err != nil {
				fmt.Printf(">>> [ALGORAND BA*] BA* failed: %v\n", err)
				// The same round is retried; don't spin if it fails fast.
				select {
				case <-time.After(time.Second):
				case <-c.haltChan:
					return
				}
			}
			if len(configQ) > 0 && c.txpool.Committed(configQ[0]) {
				configQ = configQ[1:]
			}
		}
	}
}

// runRound executes one full round: propose, wait for the highest-priority
// proposal, run BA*, apply the decision.
func (c *Chain) runRound(proposal [][]byte) error {
	c.Lock()
	height, round := c.height, c.height+c.failedRounds
	c.Unlock()

	tip := c.support.CreateNextBlock(nil)
	if tip.Header.Number != height {
		return fmt.Errorf("height bookkeeping out of step: Fabric's next block is %d but ours is %d", tip.Header.Number, height)
	}
	prevHash := tip.Header.PreviousHash
	seed := c.seedChain.Get(round, c.channelID)
	behind := c.isBehind(round)
	start := time.Now()
	if behind {
		c.requestVotes(round)
	}

	// 1. Propose (paper section 6). Only an orderer holding transactions has a
	//    block to offer; the others still vote.
	if len(proposal) > 0 {
		c.broadcastWakeup(round)
		mine := protoutil.NewBlock(height, prevHash)
		mine.Data = &common.BlockData{Data: proposal}
		mine.Header.DataHash = protoutil.BlockDataHash(mine.Data)
		myHash := ba.HashBlock(mine.Header.DataHash)
		c.addCandidate(round, myHash, mine.Data, c.selfID())
		c.propose(round, seed, myHash, mine)
	}

	// 2. Wait lambda_priority for competing proposals. If f+1 peers have
	//    already moved past this round it is decided, and waiting only slows
	//    down catching up.
	if !behind {
		select {
		case <-time.After(c.params.LambdaPriority):
		case <-c.haltChan:
			return nil
		}
	}
	startHash, from := c.bestProposal(round, seed)
	if startHash == nil {
		startHash = ba.EmptyBlockHash(round, prevHash)
		fmt.Printf(">>> [ALGORAND BA*] Round %d: no valid proposal after %v, starting from the empty block\n", round, time.Since(start).Round(time.Millisecond))
	} else {
		fmt.Printf(">>> [ALGORAND BA*] Round %d: highest-priority proposal %x from %s (after %v, behind=%v)\n", round, startHash[:4], from, time.Since(start).Round(time.Millisecond), behind)
	}

	// 3. Agree.
	ctx := &ba.Context{
		Round:    round,
		PrevHash: prevHash,
		Params:   c.params,
		Registry: c.registry,
		Seed:     seed,
	}
	// If we are behind, the votes that decide this round may never have
	// reached us; keep asking until BA* finishes.
	done := make(chan struct{})
	if behind {
		go func() {
			t := time.NewTicker(2 * time.Second)
			defer t.Stop()
			for {
				select {
				case <-done:
					return
				case <-c.haltChan:
					return
				case <-t.C:
					c.requestVotes(round)
				}
			}
		}()
	}
	res, err := c.node.Run(ctx, startHash)
	close(done)
	if err != nil {
		return fmt.Errorf("round %d: %w", round, err)
	}

	// 4. Apply the decision. The block written is looked up by the hash BA*
	//    agreed on - never "whatever we proposed or preferred", which may be
	//    a different block.
	outcome := res.Outcome.String()
	txCount := 0
	if res.IsEmpty {
		outcome = "EMPTY"
		c.Lock()
		c.failedRounds++
		c.Unlock()
		fmt.Printf(">>> [ALGORAND BA*] Agreed on EMPTY block for round %d. Incrementing failedRounds.\n", round)
	} else {
		data := c.candidate(round, res.BlockHash)
		if data == nil {
			data = c.fetchBlock(round, res.BlockHash)
			if data == nil {
				return nil // halted while fetching
			}
		}
		block := protoutil.NewBlock(height, prevHash)
		block.Data = data
		block.Header.DataHash = protoutil.BlockDataHash(data)
		meta := []byte(config.FormatRoundMetadata(round, res.Steps, res.Outcome.String(), c.proposerOf(round, res.BlockHash), time.Now().UnixMilli()))
		if isConfigBlock(block) {
			c.support.WriteConfigBlock(block, meta)
		} else {
			c.support.WriteBlock(block, meta)
		}
		c.Lock()
		c.height++
		c.Unlock()
		if c.telemetry != nil {
			// Blocks are hash-chained, so equal height and equal last hash
			// on every orderer means identical ledgers: no fork.
			c.telemetry.SetLastBlockHash(hex.EncodeToString(protoutil.BlockHeaderHash(block.Header)))
		}

		txCount = len(data.Data)
		// Every orderer drops the block's transactions from its pool,
		// whoever proposed them.
		c.txpool.MarkCommitted(data.Data)
		if res.Outcome == ba.Final {
			fmt.Printf(">>> [ALGORAND BA*] SUCCESS! Reached FINAL consensus on block %d (round %d, %d txs)! Writing to ledger...\n", height, round, txCount)
		} else {
			// Paper section 7.4: a TENTATIVE block is appended too. It is
			// confirmed once a later block reaches FINAL, because that block
			// extends it. Refusing to write it would leave this node on an
			// older round than every peer that did, and wedge it.
			fmt.Printf(">>> [ALGORAND BA*] TENTATIVE consensus on block %d (round %d, %d txs). Appending; a later FINAL block confirms it.\n", height, round, txCount)
		}
	}

	if c.telemetry != nil {
		c.Lock()
		h := c.height
		c.Unlock()
		c.telemetry.SetLedgerHeight(h)
		c.telemetry.RecordRound(round, res.Steps, outcome, txCount, int64(time.Since(start)/time.Millisecond), res.SeatsWon)
	}

	c.archiveVotes(round)
	c.inbox.Prune(round)
	return nil
}

// selfID is this orderer's name in the member registry.
func (c *Chain) selfID() string {
	if m, ok := c.registry.Lookup(c.privKey.Public().(ed25519.PublicKey)); ok {
		return m.ID
	}
	return c.nodeID
}

func (c *Chain) broadcastWakeup(round uint64) {
	payload := make([]byte, 8)
	binary.BigEndian.PutUint64(payload, round)
	c.send(MsgTypeWakeup, payload)
}

// propose runs proposer sortition and, if selected, gossips a signed proposal.
func (c *Chain) propose(round uint64, seed, hash []byte, block *common.Block) {
	pk := c.privKey.Public().(ed25519.PublicKey)
	m, ok := c.registry.Lookup(pk)
	if !ok {
		return
	}
	role := vrf.Role{Name: "proposer", Round: round, Step: 0}
	sortHash, proof, j := vrf.Sortition(c.privKey, seed, c.params.TauProposer, role, m.Stake, c.registry.TotalStake())
	if j == 0 {
		fmt.Printf(">>> [ALGORAND BA*] Round %d: not selected as a proposer this round\n", round)
		return
	}
	p := &ProposalMsg{
		Round:    round,
		PubKey:   pk,
		Priority: vrf.Priority(sortHash, j),
		SortHash: sortHash,
		Proof:    proof,
		Sig:      ed25519.Sign(c.privKey, proposalSigningBytes(round, hash)),
	}
	if hashes, ok := c.compactForm(block.Data.Data); ok {
		p.TxHashes = hashes
	} else {
		blockBytes, err := proto.Marshal(block)
		if err != nil {
			return
		}
		p.BlockBytes = blockBytes
	}
	c.storeProposal(p)
	raw, err := json.Marshal(p)
	if err != nil {
		return
	}
	c.send(MsgTypeProposal, raw)
	fmt.Printf(">>> [ALGORAND BA*] Round %d: proposed %x (%d sub-users, %d txs)\n", round, hash[:4], j, len(block.Data.Data))
}

// compactForm lists the transactions' hashes if every one of them is in the
// pool (and so, through gossip, in every other orderer's pool too).
func (c *Chain) compactForm(txs [][]byte) ([]byte, bool) {
	out := make([]byte, 0, 32*len(txs))
	for _, raw := range txs {
		h := sha256.Sum256(raw)
		if _, ok := c.txpool.Get(h); !ok {
			return nil, false
		}
		out = append(out, h[:]...)
	}
	return out, true
}

// missingHashes lists which of a compact proposal's transactions we lack.
func (c *Chain) missingHashes(hashes []byte) []byte {
	var out []byte
	for i := 0; i+32 <= len(hashes); i += 32 {
		var h [32]byte
		copy(h[:], hashes[i:])
		if _, ok := c.txpool.Get(h); !ok {
			out = append(out, h[:]...)
		}
	}
	return out
}

// rebuild turns a compact proposal back into block data from our pool, or
// returns nil if some transaction has not reached us yet.
func (c *Chain) rebuild(hashes []byte) *common.BlockData {
	data := make([][]byte, len(hashes)/32)
	for i := range data {
		var h [32]byte
		copy(h[:], hashes[32*i:])
		raw, ok := c.txpool.Get(h)
		if !ok {
			return nil
		}
		data[i] = raw
	}
	return &common.BlockData{Data: data}
}

type proposalStatus int

const (
	propValid      proposalStatus = iota
	propIncomplete                // compact, and some transaction has not arrived yet
	propInvalid
)

// bestProposal verifies every proposal received for the round, files each
// valid block as a candidate, and returns the hash of the highest-priority one.
func (c *Chain) bestProposal(round uint64, seed []byte) ([]byte, string) {
	c.poolMu.Lock()
	props := append([]*ProposalMsg(nil), c.proposals[round]...)
	c.poolMu.Unlock()

	var bestPri, bestHash []byte
	var from string
	consider := func(p *ProposalMsg) proposalStatus {
		hash, data, who, st := c.verifyProposal(p, round, seed)
		if st == propValid {
			c.addCandidate(round, hash, data, who)
			if bestPri == nil || bytes.Compare(p.Priority, bestPri) > 0 {
				bestPri, bestHash, from = p.Priority, hash, who
			}
		}
		return st
	}
	var waiting []*ProposalMsg
	for _, p := range props {
		if consider(p) == propIncomplete {
			waiting = append(waiting, p)
		}
	}
	// A transaction's gossip normally arrives before any proposal naming it
	// (same TCP stream, sent first), but one relayed via a third orderer can
	// lag - on a slow link by more than a second. Ask for what is missing from
	// any proposal that could still win, and give the replies a moment.
	var missing []byte
	for _, p := range waiting {
		if bestPri == nil || bytes.Compare(p.Priority, bestPri) > 0 {
			missing = append(missing, c.missingHashes(p.TxHashes)...)
		}
	}
	if len(missing) > 0 {
		req, _ := json.Marshal(&TxRequestMsg{Hashes: missing})
		c.send(MsgTypeTxRequest, req)
	}
	deadline := time.Now().Add(500 * time.Millisecond)
	for len(waiting) > 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
		var still []*ProposalMsg
		for _, p := range waiting {
			if bestPri != nil && bytes.Compare(p.Priority, bestPri) <= 0 {
				continue // could not win anyway
			}
			if consider(p) == propIncomplete {
				still = append(still, p)
			}
		}
		waiting = still
	}
	if len(waiting) > 0 {
		fmt.Printf(">>> [ALGORAND BA*] Round %d: %d compact proposal(s) still missing transactions; ignoring them\n", round, len(waiting))
	}
	return bestHash, from
}

func (c *Chain) verifyProposal(p *ProposalMsg, round uint64, seed []byte) ([]byte, *common.BlockData, string, proposalStatus) {
	if p.Round != round || p.Step != 0 {
		return nil, nil, "", propInvalid
	}
	m, ok := c.registry.Lookup(ed25519.PublicKey(p.PubKey))
	if !ok {
		fmt.Printf(">>> [ALGORAND BA*] Rejected proposal for round %d: unknown proposer\n", round)
		return nil, nil, "", propInvalid
	}
	role := vrf.Role{Name: "proposer", Round: round, Step: 0}
	j := vrf.VerifySort(m.PK, p.SortHash, p.Proof, seed, c.params.TauProposer, role, m.Stake, c.registry.TotalStake())
	if j == 0 || !bytes.Equal(vrf.Priority(p.SortHash, j), p.Priority) {
		fmt.Printf(">>> [ALGORAND BA*] Rejected proposal for round %d from %s: bad sortition proof or priority\n", round, m.ID)
		return nil, nil, "", propInvalid
	}
	var data *common.BlockData
	switch {
	case len(p.BlockBytes) > 0:
		block := &common.Block{}
		if err := proto.Unmarshal(p.BlockBytes, block); err != nil || block.Data == nil {
			fmt.Printf(">>> [ALGORAND BA*] Rejected proposal for round %d from %s: undecodable block\n", round, m.ID)
			return nil, nil, "", propInvalid
		}
		data = block.Data
	case len(p.TxHashes) > 0 && len(p.TxHashes)%32 == 0:
		if data = c.rebuild(p.TxHashes); data == nil {
			return nil, nil, m.ID, propIncomplete
		}
	}
	if data == nil || len(data.Data) == 0 {
		fmt.Printf(">>> [ALGORAND BA*] Rejected proposal for round %d from %s: empty block\n", round, m.ID)
		return nil, nil, "", propInvalid
	}
	hash := dataHash(data)
	if !ed25519.Verify(m.PK, proposalSigningBytes(round, hash), p.Sig) {
		fmt.Printf(">>> [ALGORAND BA*] Rejected proposal for round %d from %s: signature does not cover this block\n", round, m.ID)
		return nil, nil, "", propInvalid
	}
	return hash, data, m.ID, propValid
}

// fetchBlock asks peers for the data behind an agreed hash until one answers.
// Honest orderers vote for a block only after receiving it, so a quorum
// agreeing on a hash means some honest peer holds the data.
func (c *Chain) fetchBlock(round uint64, hash []byte) *common.BlockData {
	fmt.Printf(">>> [ALGORAND BA*] Round %d agreed on block %x, which we never received; requesting it from peers\n", round, hash[:4])
	c.poolMu.Lock()
	c.wanted[round] = hash
	c.poolMu.Unlock()
	req, _ := json.Marshal(&BlockRequestMsg{Round: round, Hash: hash})
	for attempt := 0; ; attempt++ {
		if data := c.candidate(round, hash); data != nil {
			fmt.Printf(">>> [ALGORAND BA*] Round %d: fetched block %x from a peer after %d request(s)\n", round, hash[:4], attempt)
			return data
		}
		c.send(MsgTypeBlockRequest, req)
		select {
		case <-time.After(500 * time.Millisecond):
		case <-c.haltChan:
			return nil
		}
	}
}

// proposalSigningBytes is what a proposer signs: the round and the block hash.
// The VRF proof alone binds only the round, so without this signature anyone
// who saw a winning proof could reattach it to a block of their own.
func proposalSigningBytes(round uint64, hash []byte) []byte {
	buf := make([]byte, 0, 32+8+len(hash))
	buf = append(buf, "algorand-fabric-proposal"...)
	buf = binary.BigEndian.AppendUint64(buf, round)
	return append(buf, hash...)
}

// dataHash is the value BA* votes on for a block with this data.
func dataHash(d *common.BlockData) []byte {
	return ba.HashBlock(protoutil.BlockDataHash(d))
}

func isConfigBlock(block *common.Block) bool {
	if len(block.Data.Data) == 0 {
		return false
	}
	env := &common.Envelope{}
	if err := proto.Unmarshal(block.Data.Data[0], env); err != nil || env.Payload == nil {
		return false
	}
	payload := &common.Payload{}
	if err := proto.Unmarshal(env.Payload, payload); err != nil || payload.Header == nil || payload.Header.ChannelHeader == nil {
		return false
	}
	ch := &common.ChannelHeader{}
	if err := proto.Unmarshal(payload.Header.ChannelHeader, ch); err != nil {
		return false
	}
	return ch.Type == int32(common.HeaderType_CONFIG) || ch.Type == int32(common.HeaderType_CONFIG_UPDATE)
}

// Broadcast implements the ba.Network interface
func (c *Chain) Broadcast(msg []byte) error {
	// Loopback our own votes locally
	if vote, err := ba.DecodeVote(msg); err == nil {
		c.inbox.Add(vote)
		c.recordVote("sent", vote)
	}
	if c.transport != nil {
		return c.transport.Broadcast(append([]byte{MsgTypeVote}, msg...))
	}
	return nil
}

// recordVote feeds the dashboard's per-vote stream, including how many
// committee seats (sub-users) the vote carries.
func (c *Chain) recordVote(dir string, v *ba.Vote) {
	if c.telemetry == nil {
		return
	}
	peer := ""
	var seats uint64
	if m, ok := c.registry.Lookup(v.Voter); ok {
		if dir == "recv" {
			peer = m.ID
		}
		tau := c.params.TauStep
		if v.Step == ba.StepFinal {
			tau = c.params.TauFinal
		}
		seed := c.seedChain.Get(v.Round, c.channelID)
		seats = vrf.VerifySort(v.Voter, v.SortHash, v.SortProof, seed, tau, v.Role(), m.Stake, c.registry.TotalStake())
	}
	// The empty block's hash is derivable from the vote itself, so the
	// dashboard can say "empty" instead of showing an anonymous hash.
	val := ""
	if bytes.Equal(v.Value, ba.EmptyBlockHash(v.Round, v.PrevHash)) {
		val = "empty"
	} else if len(v.Value) >= 4 {
		val = fmt.Sprintf("%x", v.Value[:4])
	}
	c.telemetry.RecordVote(dir, peer, v.Round, ba.StepName(v.Step), val, seats)
}
