# Algorand BA* as a Hyperledger Fabric Consensus Module

## 1. Where consensus lives in Fabric

Fabric splits a transaction's life into three phases (the "execute-order-validate"
model, as opposed to Bitcoin/Ethereum's "order-execute"):

```
   CLIENT                PEERS                  ORDERERS               PEERS
     |                     |                        |                    |
  1. propose ----------->  |                        |                    |
     |            EXECUTE chaincode,                |                    |
     |            return endorsement                |                    |
     |  <------------------|                        |                    |
  2. collect endorsements                           |                    |
     |                                              |                    |
  3. submit tx ------------------------------------>|                    |
     |                                      *** ORDER ***                |
     |                                      consensus happens            |
     |                                      HERE and ONLY here           |
     |                                              |                    |
     |                                        cut a block                |
     |                                              |---- deliver ------>|
     |                                              |              VALIDATE
     |                                              |              commit to ledger
```

Peers execute smart contracts and hold the ledger. **Orderers do exactly one job:
decide the order of transactions and package them into blocks.** They never
execute chaincode.

So the consensus algorithm is the ordering service, and nothing else. Replacing
consensus = replacing the ordering service implementation.

## 2. The contract we must implement

`orderer/consensus/consensus.go` (Fabric release-2.5):

```go
// A Consenter is a factory: given a channel, produce a Chain for it.
type Consenter interface {
    HandleChain(support ConsenterSupport, metadata *cb.Metadata) (Chain, error)
}

// A Chain is one running consensus instance, for one channel.
type Chain interface {
    Order(env *cb.Envelope, configSeq uint64) error        // normal tx arrived
    Configure(config *cb.Envelope, configSeq uint64) error // config tx arrived
    WaitReady() error                                      // backpressure
    Errored() <-chan struct{}                              // closed when broken
    Start()                                                // boot the algorithm
    Halt()                                                 // shut down
}
```

`ConsenterSupport` is the toolbox Fabric hands us. The four methods that matter:

| Method | What we use it for |
|---|---|
| `BlockCutter()` | batches incoming txs; tells us when a block is "full" (BatchSize / BatchTimeout) |
| `CreateNextBlock(msgs)` | builds a candidate `*cb.Block` with the right prev-hash and number |
| `WriteBlock(block, meta)` | **commits** the agreed block to the ledger |
| `Height()`, `Block(n)` | read our current chain state |

**The shape of our implementation:**

```
Order(tx) ──> BlockCutter batches it
                    |
             batch is ready
                    |
                    v
          CreateNextBlock(batch)  = our PROPOSED block for round r
                    |
                    v
      ┌─────────────────────────────────┐
      │   *** ALGORAND BA* ***          │   <-- our module
      │   sortition -> proposal ->      │
      │   Reduction -> BinaryBA* ->     │
      │   final vote                    │
      └─────────────────────────────────┘
                    |
             agreed block hash
                    v
             WriteBlock(block)   = committed, FINAL
```

Registration, in `orderer/common/server/main.go` (~line 806):

```go
consenters := map[string]consensus.Consenter{}
consenters["etcdraft"]  = etcdraft.New(...)
consenters["algorand"]  = algorand.New(...)   // <-- our one line
```

Then a channel selects it via `OrdererType: algorand` in configtx.yaml.

## 3. Mapping the paper onto Fabric

The paper assumes an open, permissionless network of millions of users. Fabric
is permissioned with a handful of orderers. Every difference below is a
deliberate, documented adaptation -- not a shortcut.

| Algorand paper | Fabric equivalent | Note |
|---|---|---|
| Millions of users | 4-7 orderer nodes | committee params must scale down |
| Weight = money held | Weight = configured stake per orderer org | no native currency in Fabric |
| `W` = total currency | sum of configured stakes | read from channel config |
| Gossip network (4-8 peers) | orderer-to-orderer gRPC cluster | Fabric already has this (`cluster` service) |
| Block proposal by ~26 proposers | sortition among orderers, tau_proposer ~ 2 | |
| Committee tau_step = 2000 | tau_step ~ 3 of 4 orderers | see params table below |
| Users' public keys | orderer MSP identities / TLS certs | Fabric MSP gives us identity for free |
| seed_r from VRF chain | same, stored in block metadata | goes in `cb.Metadata` |
| FINAL vs TENTATIVE | FINAL -> WriteBlock; TENTATIVE -> hold | Fabric has no "unconfirmed block" concept |

### Parameter scaling

The paper's committee sizes assume millions of stakeholders. Ours are derived
numerically for a small orderer set (`tools/pick_params.py`), not guessed.

**Two findings drove the final choice, both discovered by simulation:**

1. **Orderers must hold many stake units, not one vote each.** With one ticket
   per orderer, `p = tau/W = 1.0` and sortition selects everybody every step -
   a degenerate committee. Giving each orderer 10,000 tickets makes each
   contribute a randomly *varying* number of votes, which is what makes the
   threshold arithmetic mean anything.

2. **`T < h` is a hard requirement, for BOTH thresholds.** Liveness needs
   honest nodes ALONE to cross `T*tau`, and they hold only `h*tau` seats. At
   n=7 with 2 orderers down, h = 71.4% < TFinal = 0.74, so FINAL becomes
   unreachable: the chain agrees but never confirms anything. Simulation showed
   0 of 5 survivors reaching FINAL at n=7, versus 8 of 8 at n=10.

| Param | Paper | Ours (n=10) | Reason |
|---|---|---|---|
| n orderers | millions of users | 10 | f=2 tolerated -> h = 80% |
| stake each | user's balance | 10,000 tickets | keeps p < 1 so sortition is non-degenerate |
| W | total currency | 100,000 | |
| tau_proposer | 26 | 2 | expect >=1 proposer per round |
| tau_step | 2,000 | 50,000 | p = 0.5; each orderer contributes ~5,000 votes |
| T_step | 0.685 | 0.685 | **unchanged** - the 2/3+ safety bar |
| tau_final | 10,000 | 50,000 | |
| T_final | 0.74 | 0.74 | **unchanged**; requires h > 0.74, hence n=10 |
| MaxSteps | 150 | 30 | fail fast in a lab |
| lambda_step | 20s | 2s | LAN, not internet |
| lambda_block | 60s | 5s | |
| lambda_priority | 5s | 1s | |

Derivation of the feasible range for T, in expectation (g = h*tau honest seats,
b = (1-h)*tau malicious seats), from paper section 7.5:

```
  LIVENESS   g > T*tau          ->  T <  h
  SAFETY     g/2 + b <= T*tau   ->  T >= 1 - h/2
  feasible iff  1 - h/2 < h     ->  h > 2/3
```

The Byzantine 2/3 bound is not an extra assumption - it is exactly the condition
for those two inequalities to have a common solution. At h = 2/3 the interval
collapses to the single point T = 0.667.

### Validation performed before writing any Fabric code

`tools/validate.py` reimplements the BinaryBA* state machine as a generator
whose control flow matches `engine/ba/ba.go` line for line, and drives it
through an adversarial network simulator.

| Scenario | Result |
|---|---|
| all honest, same block | FINAL in 1 binary step, all 10 nodes |
| 2 orderers crashed (silent) | FINAL, 8 of 8 survivors |
| 2 orderers Byzantine, voting EMPTY | FINAL, 8 of 8 honest |
| proposer equivocated ~50/50 | agreed, TENTATIVE (correct - not decided at step 1) |
| split attack, adversary feeds one group | agreed via common coin |
| partition healing mid-round | agreed |
| **1,500 randomised trials**, <=2 Byzantine, <=45% loss | **0 safety violations** |

IMPORTANT - the invariant being checked. It is NOT "BinaryBA* always returns the
same value everywhere". The paper explicitly says (section 7.4) that under weak
synchrony BinaryBA* CAN return different values to different nodes. The real
safety property, and the one verified above, is:

> If any honest node declares FINAL(V), then no honest node decided anything
> other than V.

An early version of the test asserted the wrong invariant and duly reported
"violations" that were simply TENTATIVE-level divergence - which is the behaviour
the FINAL step exists to render harmless.

## 4. Build strategy: engine first, then adapter

We do NOT start by hacking on the Fabric source tree. Two reasons: Fabric is a
large codebase with a slow build/test loop, and a bug in BA* would be
indistinguishable from a bug in the Fabric plumbing.

Instead:

```
  Stage 1   engine/     A standalone, dependency-free Go implementation of the
                        paper: VRF sortition, Reduction, BinaryBA*, CommonCoin,
                        final/tentative. Driven by an in-process simulator with
                        injectable faults (silent nodes, equivocating nodes,
                        network partitions). Unit-testable in milliseconds.

  Stage 2   fabric/consenter/   A thin adapter implementing consensus.Consenter
                        and consensus.Chain that drives the Stage 1 engine and
                        wires its I/O to Fabric's BlockCutter/WriteBlock and the
                        orderer cluster gRPC transport.

  Stage 3   fabric/scripts/     Fork Fabric release-2.5, drop the package in,
                        add the registration line, rebuild the orderer image,
                        launch a private network with OrdererType: algorand.
```

Stage 1 is a complete, demonstrable deliverable on its own. Stage 2 and 3 turn
it into a real private blockchain.

## 5. Interface between engine and Fabric

The engine must not import any Fabric package. It talks through two small
interfaces that the adapter implements:

```go
// engine/ba: what BA* needs from the outside world
type Network interface {
    Broadcast(msg []byte) error   // gossip a signed vote
    Incoming() <-chan []byte      // votes from other nodes
}

type Ledger interface {
    LastBlockHash() []byte
    Round() uint64
    ProposeBlock() (payload []byte, err error)  // "give me a candidate block"
    Commit(payload []byte) error                 // "this one won, write it"
}
```

In the simulator these are backed by Go channels and a map.
In Fabric they are backed by the orderer cluster gRPC service and
`ConsenterSupport.CreateNextBlock` / `WriteBlock`.
