# Algorand BA* Consensus Dashboard

This dashboard visualizes the live state of the Algorand BA* consensus engine integrated into Hyperledger Fabric.

## Requirements
- Node.js (v18+)
- Fabric test network running with the `hyperledger/fabric-orderer:algorand` image.

## Setup
1. Ensure the network is running (`../fabric/scripts/run-network.sh`).
2. Run `npm install` in this directory.

## Running the Dashboard
Start the backend server:
```bash
node server.js
```

Then open your browser to [http://localhost:3000](http://localhost:3000).

## Running the Demo
In a separate terminal window, ensure you have the dashboard open in a browser, then run:
```bash
./demo.sh
```
This script will narrate a sequence of transactions while taking one orderer offline to demonstrate Byzantine Fault Tolerance.
