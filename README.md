# Algorand BA* ordering for Hyperledger Fabric

A course project that implements the consensus protocol from the Algorand paper
(Gilad, Hemo, Micali, Vlachos, Zeldovich, *Algorand: Scaling Byzantine
Agreements for Cryptocurrencies*, SOSP 2017) and plugs it into **Hyperledger
Fabric 2.5** in place of Fabric's standard ordering service (Raft).

Fabric's built-in Raft assumes every orderer is honest. With this module,
four orderers agree on every block using Algorand's BA* protocol, so the
network keeps working correctly even if one of them is malicious.

For the full write-up with every measurement, see [docs/REPORT.md](docs/REPORT.md).

---

## How it fits together

```
 client apps (loadgen.js, paygen.js)          dashboard (ui/, http://localhost:3001)
        │ submit transactions                        ▲ live votes, rounds, blocks
        ▼                                            │
 ┌────────────── Fabric peers (Org1, Org2) ──────────┴───────────────┐
 │ run chaincode (basic, payments), endorse, validate, keep ledger   │
 └────────────────────────┬──────────────────────────────────────────┘
                          │ endorsed transactions        ▲ ordered blocks
                          ▼                              │
 ┌──────────── 4 orderers: hyperledger/fabric-orderer:algorand ──────┐
 │  Fabric orderer process                                           │
 │   └─ our consenter (fabric/consenter)                             │
 │        ├─ shares transactions with the other orderers             │
 │        ├─ proposer lottery (VRF sortition)          engine/vrf    │
 │        ├─ BA* agreement: Reduction → BinaryBA* → FINAL  engine/ba │
 │        └─ writes the agreed block into Fabric's ledger            │
 │  TCP mesh between orderers on port 7060        fabric/transport   │
 │  telemetry HTTP on ports 7070-7073             fabric/telemetry   │
 └───────────────────────────────────────────────────────────────────┘
          all of it runs as Docker containers on one machine
```

Fabric works in three stages: *execute* (peers run the chaincode and sign the
result), *order* (orderers put transactions into blocks), *validate* (peers
check and commit each block). This project replaces only the middle stage.

## Folder map

| Path | What is in it |
|---|---|
| `engine/ba` | BA* itself: Reduction, BinaryBA*, CommonCoin, vote counting (paper §7) |
| `engine/vrf` | Sortition with a verifiable random function: who votes, with how many seats (paper §5) |
| `engine/sim` | Adversarial simulator and tests: 1,000 attack trials, 0 safety violations |
| `fabric/consenter` | The Fabric plug-in: rounds, proposals, transaction pool, catch-up, heartbeats |
| `fabric/transport` | TCP mesh between orderers (priority queue for votes, lossy queue for gossip) |
| `fabric/config` | Parameter checks, seed chain, transaction pool, round metadata (unit-tested) |
| `fabric/telemetry` | `/status`, `/events`, `/votes` endpoints the dashboard reads |
| `fabric/identity` | Orderer Ed25519 keys |
| `fabric/scripts` | Build, run, deploy, fault and network-fault scripts |
| `network/` | Our four changes to Fabric's test network, plus an installer |
| `chaincode/payments` | $1,000-account payments chaincode (owner checks, overdraft refusal) |
| `cmd/scalesim` | Runs the real BA* engine with 4 to 10,000 in-process nodes |
| `ui/` | Dashboard server and pages; `ui/bench` has every benchmark script and result |
| `docs/REPORT.md` | The project report |
| `study/`, `tools/` | Study PDFs and the Python scripts that checked the maths |

## Prerequisites

Tested on an Apple-silicon Mac with:

- Docker Desktop 29 (at least 4 CPUs and about 4 GB of memory)
- Go 1.25
- Node.js 24
- Python 3 (only for `tools/` and some script output)
- A clone of [fabric-samples](https://github.com/hyperledger/fabric-samples) with its `bin/`
  (Fabric 2.5.16 binaries: `peer`, `configtxgen`, `osnadmin`). This project
  expects it at `../teluskohf/fabric-samples`, next to this folder.

## Run it

```bash
# 1. Build our orderer image (fetches Fabric 2.5 source once, ~5 min first time)
cd fabric/scripts && ./build.sh

# 2. Create test-network-algorand inside fabric-samples (once)
cd ../../network && ./install.sh

# 3. Start 4 orderers + 2 peers, create the channel (500-transaction blocks)
cd ../fabric/scripts && BATCH500=1 ./run-network.sh

# 4. Deploy both chaincodes (after EVERY run-network.sh - the ledger is new)
./deploy-cc.sh

# 5. Start the dashboard, then open http://localhost:3001
cd ../../ui && npm install && node server.js
```

To send some traffic, run these from `ui/bench`:

```bash
node loadgen.js 1000 25            # 1,000 asset transactions, 25 at a time
node paygen.js 2000 100 1000 0.06  # 2,000 transfers between 1,000 $1,000 accounts, 6% attacks
```

## The dashboard (http://localhost:3001)

| Panel | Shows |
|---|---|
| Consensus Flow | The current round live: votes travelling between the 4 orderers, delivered/sent per link, and a per-step table of who voted for which block, with seats. PAUSE and use the slider to replay earlier rounds. |
| Round History | Every round rebuilt from the ledger: outcome, proposer, block, transactions. Click a round for its transactions and votes. Search by round, transaction ID or account. |
| Accounts & Payments | Balances recomputed by replaying the ledger, each account's history, and attacks stopped before reaching the ledger |
| Benchmarks | Every run's throughput, latency, blocks and BA* rounds |
| Algorand vs Raft | Side-by-side results, crash test, and what each latency change bought |
| Bad Network | Delay, loss and partition experiments and whether the chains stayed identical |
| Scale Simulation | 4 to 10,000 simulated nodes; committee size stays ~2,000 |
| Orderer Health / Live Rounds / Committed Blocks | Each orderer's status, round outcomes, recent blocks |

## Experiments

All of these are run from `ui/bench` unless noted. Results are saved as JSON next
to the scripts and appear on the dashboard.

| Command | What it does |
|---|---|
| `./run_benchmark.sh` | Full set: crash test, 1k/10k/100k CreateAsset, 1k/10k payments (~35 min; `quick` skips 100k) |
| `node loadgen.js <n> <in-flight>` | CreateAsset transactions; records blocks, rounds, latency |
| `node paygen.js <n> <in-flight> <accounts> <bad%>` | Transfers with overdraft, theft, forged and double-spend attacks, then a full balance audit |
| `node crash-test.js` | Stops one orderer (the leader on Raft) under load and measures the pause |
| `node badnet.js [scenario...]` | Network delay, loss and partitions via netem; checks for forks afterwards |
| `../../fabric/scripts/lag-test.sh` | Freezes an orderer mid-load and checks it catches up |
| `go run ./cmd/scalesim -nodes 10000 -rounds 5` | (from the repo root) Simulates 10,000 nodes |

**Comparing with stock Raft:** run `fabric/scripts/build-stock.sh` once. Then start
the network with `RAFT=1 BATCH500=1 ./run-network.sh`, deploy, and run the same
benchmarks. That orderer is compiled from the same Fabric source without our module.

## Settings

| Setting | Default | Meaning |
|---|---|---|
| `BATCH500=1` (run-network) | off | Blocks of up to 500 transactions instead of 10 |
| `RAFT=1` (run-network) | off | Use the stock Raft orderer image instead of ours |
| `ALGORAND_LAMBDA_PRIORITY` | `250ms` | How long each round waits for proposals. Raise it (e.g. `1s`) on slower networks. |

## Pitfalls we hit

- **Always rebuild with `build.sh`**, never plain `go build`. It runs `go mod vendor`, and without that the image silently contains old code.
- **Deploy both chaincodes after every network restart** (`deploy-cc.sh`); the ledger starts empty.
- **zsh does not split `$args` into words**, so `node loadgen.js $args` in a loop passes one argument. Write the numbers out.
- **Don't build or run heavy `docker logs | grep` during a benchmark**; it takes CPU from the measurement.
- **After many hours of load, Docker Desktop itself can use 1-2 CPU cores while idle.** Restart it before measuring, and compare systems back to back.

## Known limits

- The VRF is built from Ed25519 signatures, not the standard ECVRF (RFC 9381).
- Four orderers on one machine; the 1,000- and 10,000-node results are simulated.
- The 250 ms proposal window suits a single-host network only.
- Payments with heavy key contention are slower than on Raft (more frequent blocks mean more peer-state mismatches).
