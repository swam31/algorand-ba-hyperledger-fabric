#!/bin/bash
set -e

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
NET_DIR="$DIR/../../../teluskohf/fabric-samples/test-network-algorand"

echo "Deploying chaincode..."
cd "$NET_DIR"
./network.sh deployCC -ccn basic -ccp ../asset-transfer-basic/chaincode-javascript/ -ccl javascript

echo "Running benchmarks..."
cd "$DIR"

for txs in 1000 10000 100000; do
    echo "================================================================"
    echo "Running scale: $txs txs with concurrency 500"
    echo "================================================================"
    node loadgen.js $txs 500
    echo "Waiting a bit before next run..."
    sleep 10
done
