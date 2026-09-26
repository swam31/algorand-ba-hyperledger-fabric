// Command scalesim runs Algorand BA* rounds with thousands of in-process
// nodes, using the same engine code (engine/ba, engine/vrf) as the Fabric
// orderers.
//
// Every node starts with $1,000, and its dollars ARE its stake: the weight it
// carries in every sortition lottery (paper section 5). Each round, proposer
// sortition picks who builds the block; the winner packs signed random payments
// into it, refusing any that would overdraw an account; then every online node
// runs BA* on it, and the agreed payments move money for the next round.
//
//	go run ./cmd/scalesim -nodes 1000 -rounds 10 -txs 500
//
// What is and is not realistic. Signatures, sortition, committee selection,
// vote counting and the BA* state machine are the real thing. The network is
// not: all nodes share one lossless in-memory inbox, so this measures how the
// protocol's work scales with the number of nodes (committee sizes, votes,
// bytes, verification cost per node), not wide-area latency. Wall-clock time
// is the cost of simulating every node on one machine.
package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/swayamchavan/algorand-fabric/engine/ba"
	"github.com/swayamchavan/algorand-fabric/engine/vrf"
)

type account struct {
	id      string
	sk      ed25519.PrivateKey
	pk      ed25519.PublicKey
	balance uint64
	online  bool
	node    *ba.Node
}

// Payment is a signed transfer between two nodes' accounts.
type Payment struct {
	ID     string `json:"id"`
	From   string `json:"from"`
	To     string `json:"to"`
	Amount uint64 `json:"amount"`
	Kind   string `json:"kind"` // normal, overdraft, forged, double-spend
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
	from   *account
	to     *account
	sig    []byte
}

func (p *Payment) signingBytes() []byte {
	b := make([]byte, 0, 96)
	b = append(b, "sim-payment:"...)
	b = append(b, p.ID...)
	b = append(b, p.from.pk...)
	b = append(b, p.to.pk...)
	return binary.BigEndian.AppendUint64(b, p.Amount)
}

type StepStat struct {
	Step   string `json:"step"`
	Voters int    `json:"voters"` // distinct nodes that won a seat and voted
	Seats  uint64 `json:"seats"`  // total committee seats among them
}

type RoundRecord struct {
	Round          uint64     `json:"round"`
	Proposer       string     `json:"proposer"`
	Proposers      int        `json:"proposers"` // nodes that won proposer sortition
	Outcome        string     `json:"outcome"`   // FINAL, TENTATIVE or EMPTY
	Agreed         bool       `json:"agreed"`    // every online node decided the same block
	FinalNodes     int        `json:"finalNodes"`
	TentativeNodes int        `json:"tentativeNodes"`
	Steps          uint64     `json:"steps"`
	Committee      []StepStat `json:"committee"`
	Votes          int        `json:"votes"`
	VoteBytes      int64      `json:"voteBytes"`
	BlockHash      string     `json:"blockHash"`
	Payments       []*Payment `json:"payments"`
	Rejected       []*Payment `json:"rejected"`
	DurationMs     int64      `json:"durationMs"`
}

type Summary struct {
	Nodes             int       `json:"nodes"`
	Online            int       `json:"online"`
	Rounds            int       `json:"rounds"`
	TxsPerRound       int       `json:"txsPerRound"`
	StartBalance      uint64    `json:"startBalance"`
	W                 uint64    `json:"w"`
	TauProposer       float64   `json:"tauProposer"`
	TauStep           float64   `json:"tauStep"`
	TauFinal          float64   `json:"tauFinal"`
	FinalRounds       int       `json:"finalRounds"`
	TentativeRounds   int       `json:"tentativeRounds"`
	EmptyRounds       int       `json:"emptyRounds"`
	AllAgreed         bool      `json:"allAgreed"`
	AvgProposers      float64   `json:"avgProposers"`
	AvgVotersPerStep  float64   `json:"avgVotersPerStep"`
	AvgSeatsPerStep   float64   `json:"avgSeatsPerStep"`
	VotesPerRound     float64   `json:"votesPerRound"`
	SentPerNode       float64   `json:"sentPerNodePerRound"`
	KBPerNodePerRound float64   `json:"kbReceivedPerNodePerRound"`
	VerifyMicros      float64   `json:"verifyMicrosPerVote"`
	NodeVerifyMs      float64   `json:"nodeVerifyMsPerRound"`
	PaymentsCommitted int       `json:"paymentsCommitted"`
	PaymentsRejected  int       `json:"paymentsRejected"`
	// Per payment kind: how many were submitted, committed and refused. An
	// invalid-looking payment can legitimately commit if the sender was paid
	// earlier in the same block; the audit below is the real check.
	Kinds       map[string]*KindCount `json:"kinds"`
	AuditPassed bool                  `json:"auditPassed"`
	AuditNote   string                `json:"auditNote"`
	SupplyStart       uint64    `json:"supplyStart"`
	SupplyEnd         uint64    `json:"supplyEnd"`
	SupplyConserved   bool      `json:"supplyConserved"`
	RichestEnd        uint64    `json:"richestEnd"`
	PoorestEnd        uint64    `json:"poorestEnd"`
	SetupSeconds      float64   `json:"setupSeconds"`
	SimSecondsRound   float64   `json:"simSecondsPerRound"`
	Timestamp         time.Time `json:"timestamp"`
}

// sharedNet stands in for gossip. ba.Node already files each vote in the
// (shared) inbox before broadcasting, so every node sees it; here we only
// count what a real gossip network would carry.
type sharedNet struct {
	msgs  atomic.Int64
	bytes atomic.Int64
}

func (s *sharedNet) Broadcast(msg []byte) error {
	s.msgs.Add(1)
	s.bytes.Add(int64(len(msg)))
	return nil
}

func main() {
	nodes := flag.Int("nodes", 1000, "number of nodes")
	rounds := flag.Int("rounds", 10, "BA* rounds to run")
	txsPerRound := flag.Int("txs", 500, "random payments submitted per round")
	offline := flag.Float64("offline", 0, "fraction of nodes (and their stake) that never vote")
	startBalance := flag.Uint64("balance", 1000, "starting dollars per node; dollars are stake")
	outDir := flag.String("out", "ui/bench", "directory for scale_results.json and scale_history_<nodes>.json")
	seed := flag.Int64("seed", 1, "seed for payment generation")
	bad := flag.Float64("bad", 0.06, "fraction of payments that are deliberately invalid (overdraft, forged signature, double-spend)")
	flag.Parse()

	setupStart := time.Now()
	rng := rand.New(rand.NewSource(*seed))
	n := *nodes
	W := uint64(n) * *startBalance

	// Paper figure 4 values, capped at W/2 so a tiny network (4 nodes, $4,000)
	// still samples (p = tau/W must stay below 1).
	params := ba.DefaultParams()
	params.StakePerNode = *startBalance
	params.TauProposer = math.Min(26, float64(W)/2)
	params.TauStep = math.Min(2000, float64(W)/2)
	params.TauFinal = math.Min(10000, float64(W)/2)
	// With every vote already in memory a threshold is crossed as soon as
	// enough votes are counted, so this only needs to outlast a slow machine
	// scheduling thousands of goroutines. It is not a network timeout.
	params.LambdaStep = 60 * time.Second

	inbox := ba.NewInbox()
	net := &sharedNet{}
	offlineCount := int(float64(n) * *offline)
	accts := make([]*account, n)
	for i := range accts {
		pk, sk, err := ed25519.GenerateKey(nil)
		if err != nil {
			panic(err)
		}
		a := &account{id: fmt.Sprintf("node-%05d", i), sk: sk, pk: pk, balance: *startBalance, online: i >= offlineCount}
		a.node = ba.NewNode(a.id, sk, net, inbox, nil)
		accts[i] = a
	}
	var online []*account
	for _, a := range accts {
		if a.online {
			online = append(online, a)
		}
	}
	supplyStart := supply(accts)
	verifyMicros := measureVerifyCost(accts[0], params, W)

	fmt.Printf("scalesim: %d nodes (%d online), $%d each, W=%d, tau proposer/step/final = %.0f/%.0f/%.0f, %d payments/round\n",
		n, len(online), *startBalance, W, params.TauProposer, params.TauStep, params.TauFinal, *txsPerRound)
	fmt.Printf("  one vote costs %.0f us to verify on this machine\n", verifyMicros)
	setupSeconds := time.Since(setupStart).Seconds()

	prevHash := sha256.Sum256([]byte("scalesim-genesis"))
	prev := prevHash[:]
	var records []*RoundRecord
	var pool []*Payment
	nextTx := 0
	simStart := time.Now()

	for r := uint64(1); r <= uint64(*rounds); r++ {
		start := time.Now()
		for k := 0; k < *txsPerRound; k++ {
			pool = append(pool, makePayments(rng, accts, *bad, &nextTx)...)
		}

		members := make([]ba.Member, n)
		for i, a := range accts {
			members[i] = ba.Member{ID: a.id, PK: a.pk, Stake: a.balance}
		}
		registry := ba.NewRegistry(members)
		roundSeed := roundSeed(r)

		proposer, nProposers := electProposer(online, registry, roundSeed, params, r)
		rec := &RoundRecord{Round: r, Proposers: nProposers}

		var blockHash []byte
		if proposer == nil {
			// Nobody won the proposer lottery: BA* starts from the empty block
			// and the payments wait for the next round.
			blockHash = ba.EmptyBlockHash(r, prev)
			rec.Proposer = "(none)"
		} else {
			rec.Proposer = proposer.id
			rec.Payments, rec.Rejected, pool = buildBlock(pool)
			blockHash = hashBlock(r, prev, proposer.pk, rec.Payments)
		}

		results := runBA(online, &ba.Context{Round: r, Seed: roundSeed, PrevHash: prev, Registry: registry, Params: params, Cache: &ba.VerifyCache{}}, blockHash)

		agreed := results[0].BlockHash
		rec.Agreed = true
		for _, res := range results {
			if string(res.BlockHash) != string(agreed) {
				rec.Agreed = false
			}
			if res.Outcome == ba.Final {
				rec.FinalNodes++
			} else {
				rec.TentativeNodes++
			}
			if res.Steps > rec.Steps {
				rec.Steps = res.Steps
			}
		}
		rec.BlockHash = hex.EncodeToString(agreed[:8])
		empty := string(agreed) == string(ba.EmptyBlockHash(r, prev))
		switch {
		case empty:
			rec.Outcome = "EMPTY"
			// Block proposed but not agreed: its payments go back in the pool.
			for _, p := range rec.Payments {
				p.Status = "pending"
			}
			pool = append(rec.Payments, pool...)
			rec.Payments = nil
		case rec.FinalNodes > rec.TentativeNodes:
			rec.Outcome = "FINAL"
		default:
			rec.Outcome = "TENTATIVE"
		}
		if !empty {
			for _, p := range rec.Payments {
				p.from.balance -= p.Amount
				p.to.balance += p.Amount
				p.Status = "committed"
			}
			prev = agreed
		}

		rec.Committee, rec.Votes, rec.VoteBytes = committeeStats(inbox, r, roundSeed, registry, params, W)
		inbox.Prune(r + 1)
		rec.DurationMs = time.Since(start).Milliseconds()
		records = append(records, rec)
		fmt.Printf("  round %d: %s by %s (%d proposers), %d payments (+%d refused), %d votes, agreed=%v, %.1fs\n",
			r, rec.Outcome, rec.Proposer, rec.Proposers, len(rec.Payments), len(rec.Rejected), rec.Votes, rec.Agreed, float64(rec.DurationMs)/1000)
	}

	sum := summarise(records, accts, len(online), params, W, *startBalance, *txsPerRound, supplyStart, verifyMicros)
	sum.AuditPassed, sum.AuditNote = audit(records, accts, *startBalance)
	sum.SetupSeconds = setupSeconds
	sum.SimSecondsRound = time.Since(simStart).Seconds() / float64(*rounds)
	fmt.Printf("summary: %d FINAL, %d TENTATIVE, %d EMPTY; all agreed=%v; supply $%d -> $%d (conserved=%v)\n",
		sum.FinalRounds, sum.TentativeRounds, sum.EmptyRounds, sum.AllAgreed, sum.SupplyStart, sum.SupplyEnd, sum.SupplyConserved)
	fmt.Printf("  audit passed=%v: %s\n", sum.AuditPassed, sum.AuditNote)
	for k, c := range sum.Kinds {
		fmt.Printf("  %-12s committed %5d  refused %5d\n", k, c.Committed, c.Refused)
	}
	fmt.Printf("  per round: %.0f proposers, %.0f voters/step (%.0f seats), %.0f votes; each node receives %.1f KB and spends ~%.0f ms verifying votes\n",
		sum.AvgProposers, sum.AvgVotersPerStep, sum.AvgSeatsPerStep, sum.VotesPerRound, sum.KBPerNodePerRound, sum.NodeVerifyMs)

	if err := writeOutputs(*outDir, sum, records, *offline); err != nil {
		fmt.Fprintln(os.Stderr, "writing results:", err)
		os.Exit(1)
	}
}

// makePayments returns one random payment, or a deliberately invalid one that
// the proposer must refuse: an overdraft, a forged signature, or a
// double-spend (two payments each affordable alone but not together).
func makePayments(rng *rand.Rand, accts []*account, bad float64, next *int) []*Payment {
	n := len(accts)
	pick := func(not *account) *account {
		for {
			if a := accts[rng.Intn(n)]; a != not {
				return a
			}
		}
	}
	mk := func(from, to *account, amount uint64, kind string) *Payment {
		p := &Payment{ID: fmt.Sprintf("tx-%07d", *next), From: from.id, To: to.id, Amount: amount, Kind: kind, from: from, to: to}
		*next++
		p.sig = ed25519.Sign(from.sk, p.signingBytes())
		return p
	}
	from := pick(nil)
	to := pick(from)
	if rng.Float64() >= bad || from.balance < 2 {
		return []*Payment{mk(from, to, uint64(1+rng.Intn(100)), "normal")}
	}
	switch rng.Intn(3) {
	case 0:
		// Far beyond what the sender could plausibly be paid within one block.
		return []*Payment{mk(from, to, from.balance+1000+uint64(rng.Intn(500)), "overdraft")}
	case 1:
		p := mk(from, to, uint64(1+rng.Intn(100)), "forged")
		p.sig = ed25519.Sign(to.sk, p.signingBytes()) // signed by the recipient, not the owner
		return []*Payment{p}
	default:
		half := from.balance/2 + 1 // each fits the balance; both together do not
		return []*Payment{mk(from, to, half, "double-spend"), mk(from, pick(from), half, "double-spend")}
	}
}

func supply(accts []*account) uint64 {
	var s uint64
	for _, a := range accts {
		s += a.balance
	}
	return s
}

func roundSeed(r uint64) []byte {
	b := binary.BigEndian.AppendUint64([]byte("scalesim-seed"), r)
	h := sha256.Sum256(b)
	return h[:]
}

// electProposer runs proposer sortition for every online node in parallel and
// returns the highest-priority winner (paper section 6).
func electProposer(online []*account, reg *ba.Registry, seed []byte, params ba.Params, r uint64) (*account, int) {
	type win struct {
		a        *account
		priority []byte
	}
	role := vrf.Role{Name: "proposer", Round: r}
	wins := make([]*win, len(online))
	parallel(len(online), func(i int) {
		a := online[i]
		m, _ := reg.Lookup(a.pk)
		hash, proof, j := vrf.Sortition(a.sk, seed, params.TauProposer, role, m.Stake, reg.TotalStake())
		if j == 0 {
			return
		}
		// Every node checks the winner's proof; do it once here.
		if vrf.VerifySort(a.pk, hash, proof, seed, params.TauProposer, role, m.Stake, reg.TotalStake()) != j {
			panic("proposer sortition failed verification")
		}
		wins[i] = &win{a, vrf.Priority(hash, j)}
	})
	var best *win
	count := 0
	for _, w := range wins {
		if w == nil {
			continue
		}
		count++
		if best == nil || string(w.priority) > string(best.priority) {
			best = w
		}
	}
	if best == nil {
		return nil, 0
	}
	return best.a, count
}

// buildBlock is the proposer's job: include every pending payment whose
// signature checks out and whose sender can afford it, applying them in order
// against a scratch copy of the balances. Unaffordable payments are refused
// outright rather than left to clog the pool.
func buildBlock(pool []*Payment) (included, rejected, remaining []*Payment) {
	scratch := make(map[*account]uint64)
	bal := func(a *account) uint64 {
		if v, ok := scratch[a]; ok {
			return v
		}
		return a.balance
	}
	for _, p := range pool {
		if !ed25519.Verify(p.from.pk, p.signingBytes(), p.sig) {
			p.Status, p.Reason = "rejected", "bad signature"
			rejected = append(rejected, p)
			continue
		}
		if bal(p.from) < p.Amount {
			p.Status, p.Reason = "rejected", fmt.Sprintf("insufficient funds: has $%d", bal(p.from))
			rejected = append(rejected, p)
			continue
		}
		scratch[p.from] = bal(p.from) - p.Amount
		scratch[p.to] = bal(p.to) + p.Amount
		p.Status = "proposed"
		included = append(included, p)
	}
	return included, rejected, nil
}

func hashBlock(r uint64, prev, proposer []byte, pays []*Payment) []byte {
	h := sha256.New()
	h.Write(binary.BigEndian.AppendUint64(nil, r))
	h.Write(prev)
	h.Write(proposer)
	for _, p := range pays {
		h.Write(p.signingBytes())
		h.Write(p.sig)
	}
	return h.Sum(nil)
}

// runBA runs one BA* round on every online node concurrently, each with its
// own ba.Node and Context, all reading the same inbox.
func runBA(online []*account, template *ba.Context, blockHash []byte) []*ba.Result {
	results := make([]*ba.Result, len(online))
	var wg sync.WaitGroup
	for i, a := range online {
		wg.Add(1)
		go func(i int, a *account) {
			defer wg.Done()
			ctx := *template
			res, err := a.node.Run(&ctx, blockHash)
			if err != nil {
				panic(fmt.Sprintf("%s: %v", a.id, err))
			}
			results[i] = res
		}(i, a)
	}
	wg.Wait()
	return results
}

// committeeStats reports, for each step of the round, how many distinct
// nodes voted and how many committee seats they held.
func committeeStats(inbox *ba.Inbox, r uint64, seed []byte, reg *ba.Registry, params ba.Params, W uint64) ([]StepStat, int, int64) {
	steps := []uint64{ba.StepReductionOne, ba.StepReductionTwo}
	for k := uint64(1); k <= uint64(params.MaxSteps); k++ {
		steps = append(steps, ba.StepBinaryBase+k)
	}
	steps = append(steps, ba.StepFinal)

	var stats []StepStat
	votes := 0
	var bytes int64
	for _, step := range steps {
		vs := inbox.All(r, step)
		if len(vs) == 0 {
			continue
		}
		tau := params.TauStep
		if step == ba.StepFinal {
			tau = params.TauFinal
		}
		seatsBy := make([]uint64, len(vs))
		parallel(len(vs), func(i int) {
			v := vs[i]
			m, _ := reg.Lookup(v.Voter)
			seatsBy[i] = vrf.VerifySort(v.Voter, v.SortHash, v.SortProof, seed, tau, v.Role(), m.Stake, W)
		})
		st := StepStat{Step: ba.StepName(step), Voters: len(vs)}
		for i, v := range vs {
			st.Seats += seatsBy[i]
			bytes += int64(len(v.Encode()))
		}
		votes += len(vs)
		stats = append(stats, st)
	}
	return stats, votes, bytes
}

// measureVerifyCost times what every node pays to check one incoming vote:
// its signature plus its sortition proof.
func measureVerifyCost(a *account, params ba.Params, W uint64) float64 {
	seed := roundSeed(0)
	var v *ba.Vote
	for step := uint64(1); v == nil; step++ {
		role := vrf.Role{Name: "committee", Round: 0, Step: step}
		hash, proof, j := vrf.Sortition(a.sk, seed, params.TauStep, role, a.balance, W)
		if j > 0 {
			v = &ba.Vote{Voter: a.pk, Round: 0, Step: step, SortHash: hash, SortProof: proof, PrevHash: seed, Value: seed}
			v.Sign(a.sk)
		}
	}
	const reps = 300
	start := time.Now()
	for i := 0; i < reps; i++ {
		if !v.VerifySignature() || vrf.VerifySort(a.pk, v.SortHash, v.SortProof, seed, params.TauStep, v.Role(), a.balance, W) == 0 {
			panic("sample vote failed verification")
		}
	}
	return float64(time.Since(start).Microseconds()) / reps
}

func summarise(records []*RoundRecord, accts []*account, online int, params ba.Params, W, start uint64, txs int, supplyStart uint64, verifyMicros float64) *Summary {
	s := &Summary{
		Nodes: len(accts), Online: online, Rounds: len(records), TxsPerRound: txs, StartBalance: start, W: W,
		TauProposer: params.TauProposer, TauStep: params.TauStep, TauFinal: params.TauFinal,
		AllAgreed: true, SupplyStart: supplyStart, VerifyMicros: verifyMicros, Timestamp: time.Now(),
	}
	var steps int
	var voters, seats, votes, bytes, proposers float64
	for _, r := range records {
		switch r.Outcome {
		case "FINAL":
			s.FinalRounds++
		case "TENTATIVE":
			s.TentativeRounds++
		default:
			s.EmptyRounds++
		}
		s.AllAgreed = s.AllAgreed && r.Agreed
		proposers += float64(r.Proposers)
		votes += float64(r.Votes)
		bytes += float64(r.VoteBytes)
		for _, st := range r.Committee {
			if st.Step == "FINAL" {
				continue
			}
			steps++
			voters += float64(st.Voters)
			seats += float64(st.Seats)
		}
		s.PaymentsCommitted += len(r.Payments)
		s.PaymentsRejected += len(r.Rejected)
		for _, p := range r.Payments {
			s.kind(p.Kind).Committed++
		}
		for _, p := range r.Rejected {
			s.kind(p.Kind).Refused++
		}
	}
	nr := float64(len(records))
	s.AvgProposers = proposers / nr
	s.AvgVotersPerStep = voters / float64(steps)
	s.AvgSeatsPerStep = seats / float64(steps)
	s.VotesPerRound = votes / nr
	s.SentPerNode = votes / nr / float64(online)
	s.KBPerNodePerRound = bytes / nr / 1024
	s.NodeVerifyMs = s.VotesPerRound * verifyMicros / 1000
	s.SupplyEnd = supply(accts)
	s.SupplyConserved = s.SupplyEnd == s.SupplyStart
	s.PoorestEnd = math.MaxUint64
	for _, a := range accts {
		if a.balance > s.RichestEnd {
			s.RichestEnd = a.balance
		}
		if a.balance < s.PoorestEnd {
			s.PoorestEnd = a.balance
		}
	}
	return s
}

type KindCount struct {
	Committed int `json:"committed"`
	Refused   int `json:"refused"`
}

func (s *Summary) kind(k string) *KindCount {
	if s.Kinds == nil {
		s.Kinds = make(map[string]*KindCount)
	}
	if s.Kinds[k] == nil {
		s.Kinds[k] = &KindCount{}
	}
	return s.Kinds[k]
}

// audit replays every committed payment, in block order, from the starting
// balances - independently of the proposer's own checks - and confirms each
// was correctly signed by its sender, never overdrew an account, and that the
// replay ends at exactly the balances the simulation holds.
func audit(records []*RoundRecord, accts []*account, start uint64) (bool, string) {
	bal := make(map[*account]uint64, len(accts))
	for _, a := range accts {
		bal[a] = start
	}
	checked := 0
	for _, r := range records {
		for _, p := range r.Payments {
			if !ed25519.Verify(p.from.pk, p.signingBytes(), p.sig) {
				return false, fmt.Sprintf("round %d: %s committed with an invalid signature", r.Round, p.ID)
			}
			if bal[p.from] < p.Amount {
				return false, fmt.Sprintf("round %d: %s overdrew %s ($%d < $%d)", r.Round, p.ID, p.From, bal[p.from], p.Amount)
			}
			bal[p.from] -= p.Amount
			bal[p.to] += p.Amount
			checked++
		}
	}
	for _, a := range accts {
		if bal[a] != a.balance {
			return false, fmt.Sprintf("%s: replay gives $%d, simulation holds $%d", a.id, bal[a], a.balance)
		}
	}
	return true, fmt.Sprintf("replayed %d committed payments: every signature valid, no account ever negative, all %d balances match", checked, len(accts))
}

func writeOutputs(dir string, sum *Summary, records []*RoundRecord, offline float64) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	name := fmt.Sprintf("scale_history_%d", sum.Nodes)
	if offline > 0 {
		name += fmt.Sprintf("_offline%.0f", offline*100)
	}
	hist, err := json.Marshal(struct {
		Summary *Summary       `json:"summary"`
		Rounds  []*RoundRecord `json:"rounds"`
	}{sum, records})
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, name+".json"), hist, 0o644); err != nil {
		return err
	}

	// One row per (nodes, online) configuration; a rerun replaces its row.
	resPath := filepath.Join(dir, "scale_results.json")
	var rows []*Summary
	if raw, err := os.ReadFile(resPath); err == nil {
		_ = json.Unmarshal(raw, &rows)
	}
	kept := rows[:0]
	for _, r := range rows {
		if r.Nodes != sum.Nodes || r.Online != sum.Online {
			kept = append(kept, r)
		}
	}
	kept = append(kept, sum)
	out, err := json.MarshalIndent(kept, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(resPath, out, 0o644)
}

func parallel(n int, fn func(i int)) {
	workers := runtime.GOMAXPROCS(0)
	var next atomic.Int64
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				i := int(next.Add(1) - 1)
				if i >= n {
					return
				}
				fn(i)
			}
		}()
	}
	wg.Wait()
}
