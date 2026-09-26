package ba

import "time"

// Params holds every tunable constant of BA*. The defaults below were chosen
// numerically (see docs/02-parameters.md), not copied from the paper: the paper
// targets millions of stakeholders, we target a handful of Fabric orderers.
type Params struct {
	// --- committee sizing (paper section 7.5) ---

	// StakePerNode is how many lottery tickets each orderer holds. This must be
	// LARGE even though the node count is small. If each orderer held a single
	// ticket, p = Tau/W would be ~1 and sortition would select everyone every
	// time - a degenerate committee that defeats the entire mechanism. With
	// 10,000 tickets each, a node contributes a randomly VARYING number of
	// votes, which is what makes the threshold arithmetic meaningful.
	StakePerNode uint64

	// TauStep is the expected total number of committee seats per BA* step.
	TauStep float64
	// TStep is the fraction of TauStep needed to decide. MUST be < h, the
	// honest stake fraction: liveness requires honest nodes ALONE to cross
	// T*Tau, and they only hold h*Tau seats in expectation. T >= h makes
	// progress mathematically impossible.
	TStep float64

	// TauFinal / TFinal govern the extra confirmation step that upgrades a
	// decision from TENTATIVE to FINAL. Stricter than the per-step values
	// because this is what guarantees "no other block can ever be agreed".
	TauFinal float64
	TFinal   float64

	// MaxSteps bounds BinaryBA* before giving up and handing over to recovery.
	MaxSteps int

	// --- timeouts ---

	LambdaStep     time.Duration // wait for a step's votes
	LambdaBlock    time.Duration // wait for a proposed block
	LambdaPriority time.Duration // wait to learn the top proposer
	LambdaStepVar  time.Duration // slack for step-completion skew

	// TauProposer is the expected number of block proposers per round.
	TauProposer float64
}

// DefaultParams returns parameters for a 10-orderer Fabric network tolerating
// 2 Byzantine or crashed orderers.
//
// WHY 10 AND NOT 7. Both are SAFE, but at n=7 with 2 orderers down the online
// honest stake is 5/7 = 71.4%, which is below TFinal = 0.74. FINAL then becomes
// mathematically unreachable: the chain keeps agreeing, but every decision stays
// TENTATIVE, so no transaction is ever confirmed. Simulation confirmed this
// exactly - 0 of 5 surviving nodes reached FINAL at n=7, versus 8 of 8 at n=10.
//
// n=10, f=2 gives h = 80%, which is precisely the honest-stake fraction the
// paper assumes. The governing rule, for BOTH thresholds, is T < h.
//
//	W          = 10 * 10,000 = 100,000 tickets
//	TauStep    = 50,000  -> p = 0.50, each orderer contributes ~5,000 votes
//	threshold  = 0.685 * 50,000 = 34,250 votes
//	final thr  = 0.740 * 50,000 = 37,000 votes
//
// Validated: 0 safety violations across 1,500 randomised adversarial trials
// with up to 2 Byzantine nodes and up to 45% message loss.
//
// Timeouts are far tighter than the paper's because we run on a LAN or a single
// Docker host, not the open internet.
func DefaultParams() Params {
	return Params{
		StakePerNode: 10_000,

		TauStep: 50_000,
		TStep:   0.685, // unchanged from the paper: the 2/3+ safety bar

		TauFinal: 50_000,
		TFinal:   0.74, // unchanged from the paper

		MaxSteps: 30, // paper uses 150; fail fast in a lab

		LambdaStep:     2 * time.Second,
		LambdaBlock:    5 * time.Second,
		LambdaPriority: 1 * time.Second,
		LambdaStepVar:  1 * time.Second,

		// Paper figure 4 value. What matters is P(an orderer holding
		// transactions wins the proposer lottery) = 1 - exp(-tau*w/W). With
		// four equal orderers that is 39% at tau=2 - so ~60% of rounds had
		// no proposer and agreed on the empty block - and 99.85% at tau=26.
		// Extra winners cost nothing: only the highest priority is used.
		TauProposer: 26,
	}
}

// TotalStake returns W for a network of n equally-staked nodes.
func (p Params) TotalStake(n int) uint64 { return p.StakePerNode * uint64(n) }

// StepThreshold returns T*Tau: the vote count a value must EXCEED to be decided.
func (p Params) StepThreshold() float64  { return p.TStep * p.TauStep }
func (p Params) FinalThreshold() float64 { return p.TFinal * p.TauFinal }

// Validate catches parameter sets that cannot possibly work, given h.
//
// Two conditions from paper section 7.5, with g = honest seats, b = malicious:
//
//	LIVENESS: g > T*Tau         -> needs T < h
//	SAFETY:   g/2 + b <= T*Tau  -> needs T >= (1 - h/2), i.e. T >= 1 - h/2
//
// Combining: 1 - h/2 <= T < h, which has a solution only when h > 2/3.
// That inequality IS the Byzantine 2/3 bound, falling out of the arithmetic.
func (p Params) Validate(h float64) error {
	if h <= 2.0/3.0 {
		return errParam("honest stake fraction h=%.3f must exceed 2/3; no threshold T can satisfy both safety and liveness below that", h)
	}
	if p.TStep >= h {
		return errParam("TStep=%.3f must be < h=%.3f, else honest nodes alone can never reach the threshold and the chain cannot make progress", p.TStep, h)
	}
	if p.TStep < 1-h/2 {
		return errParam("TStep=%.3f must be >= 1-h/2=%.3f, else a split honest vote plus all malicious votes could cross the threshold on two different values", p.TStep, 1-h/2)
	}
	if p.TStep <= 2.0/3.0 {
		return errParam("TStep=%.3f must exceed 2/3", p.TStep)
	}
	// The final step obeys the same T < h rule. Violating it does not break
	// safety - it silently destroys usefulness, because every decision stays
	// TENTATIVE forever and no transaction is ever confirmed. Catch it loudly
	// at startup rather than discovering it as a mysteriously stalled ledger.
	if p.TFinal >= h {
		return errParam("TFinal=%.3f must be < h=%.3f, else FINAL consensus is unreachable and every block stays TENTATIVE (no transaction can ever be confirmed); either add orderers or lower TFinal", p.TFinal, h)
	}
	return nil
}

// ParamsForNetwork computes parameters based on the actual total stake in the network,
// replacing the hardcoded TauStep and TauFinal to ensure proper sortition properties.
func ParamsForNetwork(n int, stakePerNode uint64) Params {
	// Start from defaults for the unchanged thresholds
	p := DefaultParams()
	
	// Set the per-node stake
	p.StakePerNode = stakePerNode
	
	// Total stake in the network
	W := uint64(n) * stakePerNode
	
	// We want p = Tau/W = 0.5, so that sortition genuinely samples (p < 1)
	// and the binomial distribution of seats has adequate variance.
	p.TauStep = 0.5 * float64(W)
	p.TauFinal = p.TauStep
	
	// TStep (0.685) and TFinal (0.74) remain unchanged from the paper.
	return p
}
