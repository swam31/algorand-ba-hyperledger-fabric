package consenter

const (
	MsgTypeVote         byte = 1
	MsgTypeEnvelope     byte = 2
	MsgTypeProposal     byte = 3
	MsgTypeWakeup       byte = 4
	MsgTypeBlockRequest byte = 5
	MsgTypeBlockReply   byte = 6
	MsgTypeHeartbeat    byte = 7
	MsgTypeTxRequest    byte = 8
	MsgTypeTxReply      byte = 9
	MsgTypeVotesRequest byte = 10
	MsgTypeVotesReply   byte = 11
)

// VotesRequestMsg asks peers for every vote they hold for a round. An orderer
// that fell behind while cut off missed those votes, and without them it can
// never replay the round the others already decided.
type VotesRequestMsg struct {
	Round uint64
}

// VotesReplyMsg carries encoded votes. Each vote carries its own signature and
// sortition proof, so it does not matter which peer answers.
type VotesReplyMsg struct {
	Round uint64
	Votes [][]byte
}

// HeartbeatMsg is sent every second so an orderer can tell whether it can
// still reach a quorum even when no transactions are flowing. It is signed so
// that nobody can make a cut-off orderer believe it is connected.
type HeartbeatMsg struct {
	PubKey []byte
	UnixMs int64
	Sig    []byte
}

// TxRequestMsg asks for transactions a compact proposal names but this orderer
// has not received yet (their gossip was slower to reach us than to the
// proposer). Hashes are concatenated 32-byte SHA-256 values.
type TxRequestMsg struct {
	Hashes []byte
}

// TxReplyMsg answers a TxRequestMsg with the raw transactions the sender holds.
type TxReplyMsg struct {
	Txs [][]byte
}

// ProposalMsg is a block proposal (paper section 6). The VRF fields prove the
// sender won proposer sortition for this round; Sig binds that win to this
// particular block, so nobody can staple a winning proof onto a different block.
//
// A proposal is normally compact: TxHashes lists the SHA-256 of each
// transaction, which every orderer already holds through transaction gossip,
// so a 500-transaction proposal is 16 KB instead of ~2 MB. BlockBytes carries
// the full block only when it contains something that was not gossiped (a
// config update).
type ProposalMsg struct {
	Round      uint64
	Step       uint64 // always 0: the round number alone makes each lottery fresh
	PubKey     []byte
	Priority   []byte
	SortHash   []byte
	Proof      []byte
	BlockBytes []byte `json:",omitempty"`
	TxHashes   []byte `json:",omitempty"` // concatenated 32-byte hashes
	Sig        []byte
}

// BlockRequestMsg asks peers for the block data behind an agreed hash. A node
// that agreed on a block it never received (it was behind, or the proposal was
// lost) uses this instead of wedging.
type BlockRequestMsg struct {
	Round uint64
	Hash  []byte
}

// BlockReplyMsg carries marshalled common.BlockData. The receiver trusts it only
// if it hashes to the value BA* agreed on, so any peer may answer.
type BlockReplyMsg struct {
	Round     uint64
	BlockData []byte
}
