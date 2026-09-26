# Project Explanation: Algorand BA* Consensus for Hyperledger Fabric

This document provides an extreme-detail explanation of the custom Byzantine Fault Tolerant (BFT) consensus mechanism we implemented for Hyperledger Fabric, based on the Algorand BA* protocol.

## 1. Overview and Problem Statement
Hyperledger Fabric is a permissioned blockchain that traditionally relies on crash fault-tolerant (CFT) consensus algorithms like Raft. Raft tolerates node crashes but not malicious behavior (Byzantine faults). 

Our goal was to replace Fabric's default Raft consenter with a true BFT consensus engine based on **Algorand's BA*** (Byzantine Agreement). BA* uses Cryptographic Sortition and a Verifiable Random Function (VRF) to randomly select a committee of nodes proportional to their stake for every step of consensus.

## 2. Architecture
The codebase is divided into two primary areas:
1. **`engine/`**: The standalone Algorand BA* consensus implementation. It contains the state machine, VRF logic, and network simulation.
2. **`fabric/`**: The adapter that wires our engine into the Hyperledger Fabric `orderer` binary.

### 2.1 The Algorand Engine (`engine/ba`)
The engine implements the exact steps of the Algorand paper:
- **Propose (Step 1):** Nodes selected as proposers broadcast their blocks.
- **Filter (Step 2):** Nodes filter proposals and vote on the lowest hash.
- **Certify (Step 3):** Nodes vote to certify a block hash.
- **Recover (Steps 4+):** If consensus isn't reached, nodes fall back to empty blocks until agreement is forced.

A crucial component is the **VRF (Verifiable Random Function)** in `engine/vrf`. The VRF allows any node to locally, secretly determine if it is part of the committee for a given round/step. If selected, it computes a cryptographic proof and broadcasts its vote. The number of votes a node gets is proportional to its stake (simulated via `Tau`, `W`, and `p`).

### 2.2 The Fabric Adapter (`fabric/consenter`)
To use the engine in Fabric, we wrote a custom consenter plugin that implements Fabric's `consensus.Consenter` and `consensus.Chain` interfaces.

When Fabric wants to propose a block, it calls `Order()`. Our `Chain` batches the transactions, creates a block, and feeds its hash into the BA* engine (`node.Run()`). 

If BA* returns `FINAL`, the `Chain` writes the block to the ledger (`WriteBlock()`).

## 3. The Implementation Journey

### Phase 1: The TCP Transport (`fabric/transport`)
Fabric's internal gRPC dialing mechanism is tightly coupled to Raft and TLS. We built a custom TCP mesh network in `fabric/transport`.
- **Mesh Topology:** Every node dials every other node based on the `ALGORAND_PEERS` environment variable.
- **Self-Dialing Fix:** We implemented a robust `isSelf()` check that compares ports and local interface IPs to prevent nodes from dialing themselves.
- **Reliability:** The transport queues messages and continuously retries disconnected peers in the background.

### Phase 2: Resolving Deadlocks and Sorting Out VRFs
We encountered a critical deadlock where `Get()` in the `SeedChain` called itself recursively, acquiring an `RLock` while already holding a `Lock`. We refactored it to be iterative.

We also fixed the **Cryptographic Sortition** tests. Initially, the test used static keys, meaning the VRF output never varied. We rewrote the test to iterate through different rounds/steps and assert that the statistical distribution of selected seats matched the expected binomial distribution mathematically required for BFT safety.

### Phase 3: BFT Fault Tolerance Math
Byzantine Fault Tolerance requires a specific ratio of honest nodes. We implemented the formula: `f < n * (1 - TFinal)`.
If a network is too small to tolerate faults at the given threshold, it refuses to start. For our 4-node network, $n=4, f=1$, meaning we can tolerate 1 malicious or offline node and still safely reach consensus.

### Phase 4: Telemetry and Dashboard Visualization
Because BA* requires nodes to stay perfectly synchronized, a restarted node (which missed previous rounds) cannot immediately participate in consensus—it falls behind in ledger height.

To surface this limitation (and monitor consensus in real-time), we built:
1. **Telemetry Server (`fabric/telemetry`)**: A non-blocking SSE (Server-Sent Events) HTTP server embedded directly in the Fabric orderer. It records the outcome, duration, and transaction count of every consensus round.
2. **Node.js Dashboard (`ui/`)**: A frontend application that connects to the Fabric Gateway to listen for committed blocks, and connects to each orderer's telemetry endpoint to visualize network health and live consensus rounds.

## 4. The Final Result
We successfully built a 4-node Fabric network powered by Algorand BA*. 
- **Safety:** Transactions are grouped into blocks and agreed upon using cryptographic sortition.
- **Liveness:** The network continues to commit blocks even if 1 of the 4 nodes is forcefully stopped.
- **Transparency:** The live dashboard proves that consensus is occurring in real-time, displaying `FINAL` outcomes and sub-second consensus durations.

The project proves that it is entirely possible to decouple Fabric from Raft and integrate a state-of-the-art public blockchain consensus algorithm into an enterprise permissioned setting.
