#!/bin/bash
set -e

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
ALG_DIR="$DIR/../.."
NET_DIR="$ALG_DIR/../teluskohf/fabric-samples/test-network-algorand"

echo ">>> Tearing down network..."
cd "$NET_DIR"
./network.sh down

echo ">>> Generating new keys..."
cd "$DIR"
./gen-identities.sh

echo ">>> Exporting members..."
source "$DIR/keys/members.env"
export ALGORAND_MEMBERS

# RAFT=1 runs the same four orderers with the UNMODIFIED Fabric 2.5 image and a
# four-consenter Raft channel, for the Algorand-vs-Raft comparison.
if [ -n "$RAFT" ]; then
    export ORDERER_IMAGE=hyperledger/fabric-orderer:stock-raft-2.5
    export RAFT4=1
    echo ">>> COMPARISON MODE: stock Fabric Raft orderers ($ORDERER_IMAGE)"
fi

echo ">>> Bringing up network..."
cd "$NET_DIR"
./network.sh up -bft

echo ">>> Waiting 15s for network to stabilize..."
sleep 15

echo ">>> Creating channel..."
./network.sh createChannel -c mychannel -bft

echo ">>> Done. The network is up!"
