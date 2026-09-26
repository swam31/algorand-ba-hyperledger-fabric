package consenter

import (
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric/orderer/consensus"
	"github.com/hyperledger/fabric/protoutil"
	"github.com/swayamchavan/algorand-fabric/engine/ba"
	"github.com/swayamchavan/algorand-fabric/fabric/config"
	"github.com/swayamchavan/algorand-fabric/fabric/identity"
	"github.com/swayamchavan/algorand-fabric/fabric/telemetry"
	"github.com/swayamchavan/algorand-fabric/fabric/transport"
)

type simpleLogger struct{}

func (l simpleLogger) Printf(format string, v ...interface{}) {
	fmt.Printf(format+"\n", v...)
}

// Consenter implements the consensus.Consenter factory interface.
// It allows Fabric to create a new Algorand Chain instance for each channel.
type Consenter struct {
	// dependencies like communication channels, metrics, etc. could go here
}

// New creates a new Algorand Consenter.
// This is the function called in orderer/common/server/main.go.
func New() *Consenter {
	return &Consenter{}
}

// IsChannelMember implements the consensus.ClusterConsenter interface required by Fabric 2.5
// when hijacking etcdraft for osnadmin channel joins.
func (c *Consenter) IsChannelMember(joinBlock *common.Block) (bool, error) {
	return true, nil // Always allow joining for this proof-of-concept
}

// RemoveInactiveChainRegistry implements the consensus.ClusterConsenter interface.
func (c *Consenter) RemoveInactiveChainRegistry() {
	// No-op
}

// HandleChain is called by Fabric when a new channel is created or when the orderer starts.
// It initializes a new Chain instance to manage consensus for that specific channel.
func (c *Consenter) HandleChain(support consensus.ConsenterSupport, metadata *common.Metadata) (consensus.Chain, error) {
	// 1. Validate the metadata to ensure it contains Algorand-specific configuration.
	// (Mocked for now)

	nodeID := os.Getenv("ALGORAND_NODE_ID")
	listenAddr := os.Getenv("ALGORAND_LISTEN")
	peerStr := os.Getenv("ALGORAND_PEERS")
	keyFile := os.Getenv("ALGORAND_KEY_FILE")
	membersStr := os.Getenv("ALGORAND_MEMBERS")

	// 1. Identity & Registry Validation
	pub, priv, err := identity.LoadOrGenerateKey(keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load identity: %w", err)
	}

	members, err := config.ParseMembers(membersStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ALGORAND_MEMBERS: %w", err)
	}

	stakePerNode := members[0].Stake
	params := ba.ParamsForNetwork(len(members), stakePerNode)
	// Optional override for experiments: how long to wait for competing
	// proposals before starting BA* (default 1s).
	if v := os.Getenv("ALGORAND_LAMBDA_PRIORITY"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d <= 0 {
			return nil, fmt.Errorf("invalid ALGORAND_LAMBDA_PRIORITY %q", v)
		}
		params.LambdaPriority = d
		fmt.Printf(">>> [ALGORAND BA*] lambda_priority overridden to %v\n", d)
	}
	totalStake, h, err := config.ValidateMembers(members, pub, params)
	if err != nil {
		return nil, fmt.Errorf("registry validation failed: %w", err)
	}

	// Calculate and print sigma margin
	p := params.TauStep / float64(totalStake)
	honestStake := float64(totalStake) * h
	expectedHonestSeats := honestStake * p
	stdDev := math.Sqrt(honestStake * p * (1 - p))
	threshold := params.FinalThreshold()
	sigma := (expectedHonestSeats - threshold) / stdDev

	fmt.Printf(">>> [ALGORAND BA*] Registry validated: n=%d, W=%d, tau=%.0f, threshold=%.0f, h=%f, p=%f\n", len(members), totalStake, params.TauStep, params.StepThreshold(), h, p)
	fmt.Printf(">>> [ALGORAND BA*] Sigma margin (3-of-4 against TFinal): %.2f sigma\n", sigma)

	registry := ba.NewRegistry(members)
	tolerated := len(members) - int(float64(len(members))*h+0.5)

	// Resume at the right round after a restart. Height alone does not say
	// which round we are in, because empty rounds advance the round without
	// adding a block; the last block's metadata records the round that wrote it.
	height := support.Height()
	var failedRounds uint64
	if height > 0 {
		if last := support.Block(height - 1); last != nil {
			if m, err := protoutil.GetConsenterMetadataFromBlock(last); err == nil {
				failedRounds = config.RoundsFromMetadata(height, string(m.Value))
			}
		}
	}
	fmt.Printf(">>> [ALGORAND BA*] Starting at height %d, round %d\n", height, height+failedRounds)

	var t *transport.Transport
	if listenAddr == "" {
		fmt.Println(">>> [ALGORAND BA*] WARNING: ALGORAND_LISTEN not set. Consensus running as single-node (no network transport).")
	} else {
		peers := config.ParseCommaList(peerStr)
		t = transport.New(nodeID, listenAddr, peers, simpleLogger{})
	}

	inbox := ba.NewInbox()

	var getConnected func() int
	if t != nil {
		getConnected = t.ConnectedCount
	}

	var telemetrySrv *telemetry.Server
	telemetryListen := os.Getenv("ALGORAND_TELEMETRY_LISTEN")
	if telemetryListen != "" {
		telemetrySrv = telemetry.NewServer(
			nodeID, support.ChannelID(),
			len(members), totalStake, uint64(params.TauStep), uint64(params.StepThreshold()),
			params.TFinal, h, p,
			tolerated, len(members)-1,
			getConnected,
		)
		telemetrySrv.Start(telemetryListen)
		if h := support.Height(); h > 0 {
			if last := support.Block(h - 1); last != nil {
				telemetrySrv.SetLastBlockHash(hex.EncodeToString(protoutil.BlockHeaderHash(last.Header)))
			}
		}
		fmt.Printf(">>> [ALGORAND BA*] Telemetry server started on %s\n", telemetryListen)
	}

	// 2. Initialize the Chain structure.
	chain := &Chain{
		support:        support,
		inbox:          inbox,
		registry:       registry,
		haltChan:       make(chan struct{}),
		errorChan:      make(chan struct{}),
		txChan:         make(chan *common.Envelope, 1000), // Buffer for incoming txs
		configChan:     make(chan *common.Envelope, 100),
		notify:         make(chan struct{}, 1),
		transport:      t,
		telemetry:      telemetrySrv,
		nodeID:         nodeID,
		privKey:        priv,
		seedChain:      config.NewSeedChain(),
		channelID:      support.ChannelID(),
		params:         params,
		tolerated:      tolerated,
		height:         height,
		failedRounds:   failedRounds,
		proposals:      make(map[uint64][]*ProposalMsg),
		candidates:     make(map[uint64]map[string]candidate),
		wanted:         make(map[uint64][]byte),
		wakeups:        make(map[uint64]bool),
		peerRounds:     make(map[string]uint64),
		txpool:         config.NewTxPool(1_000_000, 200_000),
		lastHeard:      make(map[string]time.Time),
		quorumPeers:    len(members) - tolerated - 1,
		voteArchive:    make(map[uint64][][]byte),
		lastAsked:      make(map[uint64]time.Time),
	}

	chain.node = ba.NewNode(nodeID, priv, chain, inbox, nil)

	return chain, nil
}
