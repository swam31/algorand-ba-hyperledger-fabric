#!/bin/bash
set -e

# This script spins up a local test network using our custom Orderer image.

echo ">>> Launching Private Hyperledger Fabric Network with Algorand Consensus"

# Ensure the fabric-src directory exists (meaning build.sh was run)
if [ ! -d "fabric-src" ]; then
    echo "Error: fabric-src not found. Please run build.sh first."
    exit 1
fi

if [ ! -d "fabric-samples" ]; then
    echo ">>> Downloading official Hyperledger Fabric Test Network (fabric-samples)..."
    curl -vsS https://raw.githubusercontent.com/hyperledger/fabric/master/scripts/bootstrap.sh | bash -s -- 2.5.16 1.5.8 0.4.24
fi

cd fabric-samples/test-network

echo ">>> Bringing down any existing network..."
./network.sh down

echo ">>> Bringing up new network using hijacked etcdraft (Algorand)..."
export ORDERER_IMAGE_TAG=latest  # Use the image we just built
export BFT_IMAGE_TAG=latest
# Fabric's test-network defaults to etcdraft if we don't specify consensus.
# Since we overwrote etcdraft in main.go, this will actually launch Algorand!

./network.sh up createChannel -c mychannel

echo ">>> Network is UP and running!"
echo ">>> Orderer nodes are now utilizing Algorand's BA* for consensus."
