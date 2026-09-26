# Byzantine-Fault-Tolerant Ordering for Hyperledger Fabric with Algorand's BA*

*Course project report.*

## Abstract

Hyperledger Fabric 2.5 orders transactions with Raft, which keeps working if
orderers crash but fails if any orderer lies. We replaced Raft with the
consensus protocol from the Algorand paper (Gilad et al., SOSP 2017):
verifiable-random-function (VRF) sortition picks, for every step, a committee
weighted by stake. A block proposer is chosen by lottery, and the BA* protocol
(Reduction, BinaryBA*, FINAL) agrees on one block per round. The new ordering
service runs inside Fabric's own orderer process as a consenter plug-in.
Clients, peers, chaincode and the ledger are unchanged.

On a four-orderer network that tolerates one malicious orderer, the
implementation:

- commits every transaction in runs of 1,000, 10,000 and 100,000;
- confirms transactions 2.5× faster than stock Raft under light load (median
  0.86 s vs 2.19 s), and matches it within run-to-run noise under heavy load;
- loses no transactions and pauses 1.1 s when an orderer crashes, where Raft
  pauses 12.5 s and loses every in-flight transaction;
- never forked in 1,000 simulated trials with message loss and Byzantine
  nodes, or in eight network-fault scenarios on the live network.

A simulator running the same engine code shows the paper's central property:
the committee stays at about 2,000 seats as the network grows from 4 to 10,000
nodes.

---

## 1. Background

### 1.1 Hyperledger Fabric

Fabric is a permissioned blockchain: every participant has a certificate from a
known organisation. A transaction passes through three stages:

1. **Execute.** A client sends a proposal to peers, which run the smart contract
   (*chaincode*) and sign the result (*endorse*) without changing anything yet.
2. **Order.** The client sends the endorsed transaction to the ordering service,
   which puts transactions into blocks in one agreed order.
3. **Validate.** Every peer checks each transaction in a block (enough
   endorsements, and no data it read has changed since, the *MVCC* check), then
   updates its ledger.

Only stage 2 needs consensus. Fabric 2.5 ships one ordering algorithm, Raft
(Solo and Kafka are deprecated). Raft elects a leader that orders everything,
and it is *crash* fault tolerant: it assumes no orderer ever lies.

### 1.2 Algorand's consensus

The Algorand paper targets a public network where some participants may be
malicious. Its ideas, which we implemented:

- **Stake-weighted sortition (§5).** Each participant has stake (money). For
  every protocol step, each one privately runs a lottery with a VRF, a function
  whose output is random-looking but provably computed from the participant's
  key. A participant with stake *w* gets `j ~ Binomial(w, τ/W)` committee seats.
  Anyone can verify the proof, nobody can predict or forge it, and the expected
  committee size is τ however many participants exist.
- **Block proposal (§6).** The same lottery with τ_proposer = 26 picks a few
  proposers; the one with the highest VRF priority wins.
- **BA\* (§7).** *Reduction* turns "many possible blocks" into "this block or the
  empty block". *BinaryBA\** then agrees on one of the two, with a common coin to
  break ties. A decision in BinaryBA*'s first step, confirmed by a second
  committee's FINAL votes above 74%, is **FINAL**. Otherwise it is **TENTATIVE**.
  If no proposal arrives in time, nodes agree on the **empty** block and move on.

## 2. Design

### 2.1 Where the module plugs in

Fabric's orderer keeps a table of consensus implementations indexed by type.
`fabric/scripts/build.sh` adds our consenter to that table (in place of
`etcdraft` and `BFT`), vendors our Go module into Fabric 2.5.16's source, and
cross-compiles the orderer into the Docker image
`hyperledger/fabric-orderer:algorand`. Fabric then calls our code through its
standard `Consenter`/`Chain` interfaces:

- `Order(tx)` and `Configure(tx)` hand us transactions;
- `WriteBlock`/`WriteConfigBlock` is where we hand back ordered blocks.

Everything else in Fabric is untouched.

The four orderers talk to each other over our own TCP mesh on port 7060
(`fabric/transport`), not Fabric's cluster gRPC, because BA* needs broadcast
gossip of small signed messages.

### 2.2 Parameters and why they are safe

| Parameter | Value | Reason |
|---|---|---|
| Orderers *n* | 4 | Smallest network tolerating one malicious orderer |
| Stake per orderer | 100,000 (W = 400,000) | Equal weight |
| Committee size τ_step = τ_final | 200,000 (p = τ/W = 0.5) | Real sampling (p < 1). Each orderer's seats vary around 50,000 (measured σ 164 vs 158 predicted) |
| Step threshold T_step | 0.685 → 137,000 seats | From the paper |
| Final threshold T_final | 0.74 → 148,000 seats | From the paper |
| Honest fraction *h* | 0.75 (3 of 4) | |
| τ_proposer | 26 | Paper value. The orderer with transactions wins a proposer seat 99.85% of the time |
| λ_priority | 250 ms | How long to wait for proposals; tuned for one host (§4.7) |
| λ_step, MaxSteps | 2 s, 30 | Per-step timeout; the paper uses 150 steps |

The safety conditions from the paper's analysis reduce to `1 − h/2 ≤ T < h`,
which only has a solution when *h* > 2/3. That is the Byzantine two-thirds bound.
`fabric/config` checks these conditions at start-up and refuses to run with
parameters that could fork or never finalise. With *h* = 0.75, three honest
orderers give an expected 150,000 seats: 7.3 standard deviations above the FINAL
threshold.

**Quorum intersection.** A decision needs more than 68.5% of seats, which takes 3
of the 4 orderers. Any two groups of 3 share at least 2 orderers. For two
conflicting decisions, both shared orderers would have to vote twice, but at
most one orderer is malicious. So two different blocks can never both be
decided.

### 2.3 One round, step by step

```
client ─tx─▶ any orderer ──gossip──▶ every orderer's transaction pool
                                              │
round r begins as soon as any pool is non-empty
  1. proposer lottery (VRF, τ=26); winners broadcast a COMPACT proposal:
     the 32-byte hashes of the transactions, signed, plus the VRF proof
  2. wait λ_priority (250 ms); take the highest-priority valid proposal,
     rebuilding its block from the local pool (fetch any missing transactions)
  3. BA*: Reduction (2 steps) → BinaryBA* → FINAL vote count
  4. agreed a block  → write it through Fabric (FINAL or TENTATIVE),
                       record "round, steps, outcome, proposer, time" in its
                       metadata, drop its transactions from every pool
     agreed "empty"  → no block; the round number still advances
```

The round number is `height + empty rounds so far`, so every orderer walks
through the same sequence. The block written is always the one whose hash BA*
agreed on, never simply the node's own proposal. If an orderer agreed on a block
it never received, it fetches it from a peer; the data must hash to the agreed
value, so any peer may answer.

### 2.4 Staying correct when things go wrong

| Situation | Mechanism |
|---|---|
| An orderer falls behind (slow, partitioned, restarted) | It notices when f+1 = 2 peers vote in later rounds. It then asks for those rounds' signed votes, which every orderer archives for its last 4,096 rounds, and replays them. Votes carry their own signature and sortition proof, so any peer may answer. |
| An orderer is cut off from the others | Orderers exchange signed heartbeats every second. One that cannot hear a quorum refuses new transactions (SERVICE_UNAVAILABLE), and Fabric's gateway sends them to another orderer. |
| Network congestion | Votes and proposals use a deep priority send queue; transaction gossip uses a separate queue that may drop, which is safe because blocks can be fetched. |
| A link dies silently | TCP_USER_TIMEOUT (20 s) drops it, and it is redialled within 5 s. |
| A restart | The last block's metadata records its round, so the orderer resumes at the right round. |

## 3. Implementation

About 8,700 lines of Go and JavaScript, plus 34 unit and simulation tests.

| Component | Lines | Role |
|---|---|---|
| `engine/ba` | 1,020 | BA*: committee voting, vote counting (Alg. 5–6), Reduction (Alg. 7), BinaryBA* (Alg. 8), CommonCoin (Alg. 9) |
| `engine/vrf` | 380 | VRF (proof = deterministic Ed25519 signature; output = SHA-512 of the proof) and binomial sortition |
| `engine/sim` | 590 | In-process network with message loss, partitions and equivocating attackers; safety assertion |
| `fabric/consenter` | 1,430 | The Fabric plug-in: rounds, proposals, pool, fetch, catch-up, heartbeats |
| `fabric/transport` | 730 | TCP mesh, length-prefixed frames, priority and gossip queues |
| `fabric/config` | 840 | Parameter validation, seed chain, transaction pool, round metadata |
| `fabric/telemetry` | 450 | HTTP status and live event streams for the dashboard |
| `cmd/scalesim` | 690 | Many-node simulation with payments and an audit |
| `chaincode/payments` | 110 | $1,000 accounts with owner checks |
| `ui/` | 2,400 | Dashboard server, ledger-history decoder, pages, benchmark tools |

**Deployment.** Everything runs in Docker on one machine: four orderers (our
image), two peers (Org1 and Org2), and chaincode containers. The network is
Fabric's `test-network` with four files changed (compose file, two
channel-configuration files, channel script); `network/` holds them and an
installer. `run-network.sh` generates orderer keys, starts the containers and
creates the channel. `deploy-cc.sh` installs the chaincode.

**Dashboard.** The dashboard reads each orderer's telemetry and streams the
ledger through Fabric's Gateway from block 0. It shows:

- live votes between orderers, round by round;
- every past round with its transactions and votes;
- account balances rebuilt by replaying the ledger;
- all benchmark results.

## 4. Evaluation

Setup: one Apple-silicon Mac, Docker Desktop with 8 CPUs and 3.8 GB of memory,
Fabric 2.5.16, blocks of up to 500 transactions. Every number is measured.
Blocks and rounds come from orderer telemetry, and every run ends by checking
that all four orderers hold the same chain. Repeated runs of the same build
vary by about ±15%.

### 4.1 Correctness

- **Adversarial simulation:** `TestAdversarialSoak` runs 200 randomised
  four-node trials: 100 with 5% message loss and 100 with 30%. In 20% of trials
  one node is Byzantine, either voting for a wrong value or equivocating (voting
  for two values at once). Five runs gave 1,000 trials and 0 safety violations.
  With 5% loss, 43–51% of trials reached FINAL on at least three nodes. With 30%
  loss almost none did: rounds stalled, which is safe. Separate tests cover one
  silent node, one wrong-value node, one equivocating node, and a partition that
  heals. Under the paper's weak-synchrony conditions, different TENTATIVE
  decisions are allowed. A FINAL decision can never conflict with anything.
- **Live network:** in every benchmark and fault test, all four orderers ended
  with the same height and the same last block hash. Blocks are hash-chained, so
  that means identical ledgers.
- **Payments:** after every run, all 1,000 balances matched a replay of the
  committed transfers, and the total supply stayed exactly $1,000,000.

### 4.2 Throughput and latency (final build)

| Transactions | In flight | Time | Throughput | Median / p95 latency | Empty rounds |
|---|---|---|---|---|---|
| 1,000 | 25 | 33.8 s | 29.6 tps | 0.86 s / 1.16 s | 0 / 105 |
| 1,000 | 500 | 10.5 s | 95.5 tps | 4.5 s / 7.9 s | 0 / 17 |
| 10,000 | 500 | 80.7 s | 123.9 tps | 3.6 s / 8.4 s | 1 / 163 |
| 100,000 | 500 | 1,016 s | 98.5 tps | 4.4 s / 9.5 s | 12 / 1,750 |

Every transaction committed in every run.

### 4.3 How latency came down

The first version waited for Fabric's 2 s batch timer and gave a 4 s median
latency. Each change below was measured, and two of them first made things
worse:

| Change | Light load (tps · p50) | Heavy load (tps · p50) |
|---|---|---|
| Baseline: rounds wait for the 2 s timer | 6.5 · 4.1 s | 115 · 4.0 s |
| Start a round as soon as a transaction arrives | 4.6 · 4.3 s (no help) | not measured |
| Orderers share pending transactions | 7.2 · 3.2 s | not measured |
| Proposal window 1 s → 250 ms | 26.7 · 0.89 s | 94 · 4.4 s (worse) |
| Compact proposals (hashes, not whole blocks) | 33.2 · 0.83 s | 130 · 3.2 s |
| Robustness fixes (§2.4), final | 29.6 · 0.86 s | 124 · 3.6 s |

- **The timer was not the problem.** Each round wrote only the winning
  orderer's transactions. Clients spread transactions over four orderers, so a
  transaction waited for its orderer to win: about four rounds.
- **The proposal window came next.** A stage-by-stage probe showed the 1 s wait
  was the biggest fixed cost left.
- **The heavy-load regression came from proposal size.** Full-block proposals
  (~2 MB) missed the 250 ms window, so orderers started from different blocks
  and rounds came up empty. Compact proposals are 16 KB.

### 4.4 Comparison with stock Raft

`build-stock.sh` compiles the same Fabric 2.5.16 source without our module, and
`RAFT=1 run-network.sh` starts the same four orderers as a Raft cluster. Both
were measured back to back.

| Test | Algorand BA* | Stock Raft |
|---|---|---|
| 1,000 tx, 25 in flight | **29.6 tps, median 0.86 s** | 11.3 tps, median 2.19 s |
| 1,000 tx, 500 in flight | 95.5 tps | **116.4 tps** |
| 10,000 tx, 500 in flight | **123.9 tps** | 110.7 tps |
| 100,000 tx, 500 in flight | 98.5 tps | **109.1 tps** |
| Payments, 10,000 transfers | 32.5 tps | **54.6 tps** |
| One orderer crashes (Raft: the leader) | **1.1 s pause, 0 lost** | 12.5 s pause, 20 lost |

- **Light load: ours confirms 2.5× faster.** Raft waits for its batch timer.
- **Heavy load: a tie.** The peers (JavaScript chaincode, validation) are the
  limit for both.
- **Contended payments: Raft wins.** We write a block about every 0.5 s. Fabric
  needs both endorsing peers to agree on state, and with blocks this frequent they
  are often one block apart, so more endorsements mismatch.
- **Crashes: ours hardly notices.** Raft must elect a new leader (6.3 s), and the
  transactions the old leader held are lost. Ours has no leader, and every
  pending transaction is in every orderer's pool.
- **Malicious orderers: only ours copes.** A malicious Raft leader can censor,
  reorder or fork. Ours tolerates one of four.

### 4.5 Payments and attacks

`paygen.js` opens 1,000 accounts of $1,000, sends random transfers, and makes 6%
of them attacks:

| Attack | Stopped by | Result (100,000 transfers) |
|---|---|---|
| Overdraft ($1,000,000) | chaincode | 1,432 of 1,432 refused |
| Spending another organisation's account | chaincode owner check | 1,539 of 1,539 refused |
| Signature that does not match the certificate | peer, before the chaincode runs | 1,469 of 1,469 refused |
| Double spend (two concurrent transfers) | Fabric's MVCC check at commit | 0 pairs with both halves committed |

Concurrent transfers touching the same account conflict at commit. A client that
resubmits lost races (up to 5 times) gets 94–99.7% of honest transfers through.

### 4.6 Scaling to 10,000 nodes (simulation)

1,000 real orderers would need about 50 GB of memory, so `cmd/scalesim` runs the
same engine with thousands of in-process nodes. Each node holds $1,000 as stake,
and the paper's τ values are used. The shared in-memory network measures work,
not latency.

| Nodes | Voters / step | Committee seats / step | Votes per round | Verify time per node per round |
|---|---|---|---|---|
| 4 | 4 | 2,003 | 28 | 4 ms |
| 100 | 97 | 1,997 | 682 | 81 ms |
| 1,000 | 852 | 1,994 | 6,110 | 0.85 s |
| 10,000 | 1,822 | 2,010 | 17,286 | 2.8 s |

- **The committee stays at about 2,000 seats.**
- **Voters per step follow N(1 − e^(−τ/N)).** The formula predicts 865 and 1,813
  for 1,000 and 10,000 nodes, and levels off near 2,000, so each node's work is
  bounded however many join.
- **Every round was FINAL,** including with 10% of nodes offline.
- **Invalid payments never committed:** no forged signature ever did, and an
  audit found that no committed payment ever overdrew an account.

### 4.7 Bad networks (weak synchrony, paper §7.4)

`netem.sh` adds delay or loss to consensus traffic only (port 7060). Each
scenario runs 90 s of load; afterwards we check that all four chains are
identical.

| Scenario | 250 ms window | 1 s window | Chains identical afterwards |
|---|---|---|---|
| Healthy | 4,459 committed | 1,219 | ✓ |
| 100 ms delay | 982 · 3–4 empty rounds | 971 · 0 empty | ✓ |
| 500 ms delay | 0 | 151 | ✓ |
| 10% packet loss | 572 (1 TENTATIVE) | 975 | ✓ |
| 1.5 s delay everywhere | 0 | 0 | ✓ |
| One orderer delayed 1.5 s | 312 | – | ✓ |
| One orderer cut off | 2,710 (the other three continue) | – | ✓ (39 s after reconnecting) |
| Two orderers cut off for 45 s | chain pauses, then resumes | – | ✓ |

- **Safety held everywhere.** When messages are too slow, rounds end EMPTY (or
  TENTATIVE), never in conflicting blocks.
- **The proposal window trades speed for tolerance of slow networks.** Like the
  paper's λ values, it must match the real network delay.

## 5. Problems found along the way

Most of the engineering effort went into problems that only showed up on the
live network:

- **An earlier version silently dropped transactions.** When another orderer's
  block won, the losing orderer treated its own batch as committed.
- **It could also write the wrong block.** An orderer wrote its preferred
  proposal instead of the agreed one, which could fork.
- **Lagging orderers wedged.** They discarded proposals for future rounds.
- **Proposer τ = 2 left 60% of rounds empty.** The paper's τ = 26 fixes it.
- **Fabric writes blocks asynchronously.** Its reported height lagged, which
  could reuse a round number.
- **Large proposals were dropped** by the transport's 1 MB message limit.
- **Cut-off orderers stranded clients,** proposals named transactions some
  orderers had not received, dead links recovered slowly, and orderers that
  missed votes could never catch up. These four came from the bad-network tests
  (§2.4 lists the fixes).
- **A 5 s TCP timeout dropped healthy links under load.** It is now 20 s.

Each was found by measuring, fixed, and re-measured. The lesson that recurred:
a plausible explanation is not a result until the live network confirms it.

## 6. Limitations and future work

- **VRF:** built from Ed25519 signatures. It is unique only for honest keys; a
  production system should use ECVRF (RFC 9381).
- **Scale:** four real orderers on one machine. The large-network results are
  simulated, so wide-area latency is not measured.
- **Proposal window:** 250 ms suits one host. It should be tuned per deployment,
  or adapt to measured delay.
- **Contended workloads:** these suffer from frequent blocks. A minimum block
  interval would trade latency for fewer endorsement mismatches.
- **Catch-up window:** covers the last 4,096 rounds. An orderer offline longer
  than that needs Fabric's block replication.
- **Stake:** equal and fixed. The paper's stake changes with account balances.

## 7. Reproducing

See [README.md](../README.md): build the image, install the network, run it,
deploy, start the dashboard. `ui/bench/run_benchmark.sh` repeats the benchmark
set. Raw results are in `ui/bench/*.json` and run logs in `ui/bench/logs/`.

## References

1. Y. Gilad, R. Hemo, S. Micali, G. Vlachos, N. Zeldovich. *Algorand: Scaling
   Byzantine Agreements for Cryptocurrencies.* SOSP 2017 (IACR ePrint 2017/454).
2. Hyperledger Fabric 2.5 documentation, https://hyperledger-fabric.readthedocs.io/en/release-2.5/
3. D. Ongaro, J. Ousterhout. *In Search of an Understandable Consensus Algorithm (Raft).* USENIX ATC 2014.
