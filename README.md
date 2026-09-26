# Algorand-Fabric

This repository implements Algorand's BA* Byzantine agreement protocol as a custom ordering service plugin for Hyperledger Fabric 2.5. 

## Project Overview

Hyperledger Fabric traditionally uses Crash Fault Tolerant (CFT) ordering services like Raft or Kafka. In this project, we replace the default CFT consensus with a true Byzantine Fault Tolerant (BFT) protocol based on the Algorand blockchain's core consensus algorithm (BA*).

This allows the Fabric ordering network to safely tolerate malicious or compromised nodes, as long as the total fraction of Byzantine nodes `f` stays below the theoretical safety bound `f ≤ (n-1)/3`.

## Architecture & Components

The implementation is broken down into several modular components:

### 1. The BA* Engine (`engine/ba`)
The core consensus logic that implements the algorithms defined in the Algorand whitepaper:
- **Verifiable Random Functions (VRF)**: Cryptographically secure and private random sortition that elects committee members for each consensus step using `ed25519` keys.
- **Algorithm 7 (Reduction)**: Narrows down the proposed Fabric blocks to a binary choice (a specific block hash or the empty block).
- **Algorithm 8 (Binary BA)**: A highly efficient multi-round binary Byzantine agreement protocol that drives the network to a `FINAL` or `TENTATIVE` consensus on the block.
- **Vote & Threshold Management**: Nodes cast votes weighted by their stake, and the engine correctly counts and verifies votes against cryptographic thresholds.

### 2. P2P Transport Layer (`fabric/transport`)
Fabric's internal gRPC dialing mechanism is tightly coupled to its Raft implementation. To give the BA* engine a dedicated, unhindered peer-to-peer message channel:
- We built a custom TCP mesh transport layer.
- Orderers discover each other via `ALGORAND_PEERS` and maintain persistent TCP connections.
- The transport layer multiplexes Fabric `Envelope` payloads (transactions) and Algorand `Vote` payloads seamlessly.

### 3. Consenter Adapter (`fabric/consenter`)
The bridge between Fabric's ordering service interfaces (`consensus.Consenter`, `consensus.Chain`) and our BA* engine:
- **Transaction Forwarding**: When an orderer receives a transaction via the `Broadcast` gRPC API, it immediately forwards the transaction Envelope to all other orderers over the custom P2P transport.
- **Synchronous Block Cutting**: By distributing the transactions before cutting blocks, all orderers deterministically cut the same block locally.
- **Execution Hook**: After cutting a block, the adapter passes the block hash to the BA* engine. Once the engine reaches `FINAL` consensus, the adapter writes the block to the ledger.
- **Seed Derivation**: We implemented an iterative VRF seed derivation chain to prevent deadlocks and ensure unpredictable, cryptographically secure sortition sequences.

### 4. Configuration and Identity (`fabric/config`, `fabric/identity`)
- **Rigorous Fault Tolerance**: Computes the exact number of tolerated faults by verifying both the safety bound (`(n-1)/3`) and Algorand's `FINAL` reachability bound (`TFinal`).
- **Identity Parsing**: Hooks into Fabric's standard MSP identity structures to extract the ed25519 public/private key pairs used for signing VRF tickets and BA* votes.

## How to Run

1. Build the custom Docker orderer image:
   ```bash
   cd fabric/scripts
   ./build.sh
   ```
   This will compile the orderer binary with the BA* plugin and package it into `hyperledger/fabric-orderer:algorand`.

2. Launch the 4-node test network:
   ```bash
   ./run-network.sh
   ```
   This script will:
   - Generate all necessary cryptographic identities.
   - Launch four orderers running the BA* consensus engine.
   - Create a channel (`mychannel`).

3. Deploy and Test Chaincode:
   ```bash
   ./deploy-cc.sh
   ```
   This deploys the `asset-transfer-basic` smart contract to the network. During deployment, you will see the BA* consensus engine seamlessly ordering the initialization transactions and logging `SUCCESS! Reached FINAL consensus on block <N>! Writing to ledger...`.

## Conclusion

This project successfully proves that a highly advanced, mathematically proven BFT algorithm (Algorand's BA*) can be effectively retrofitted into enterprise blockchain platforms like Hyperledger Fabric, significantly enhancing their security against malicious actors.
