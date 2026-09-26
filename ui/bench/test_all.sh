#!/bin/bash
set -e
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
NET_DIR="$DIR/../../../teluskohf/fabric-samples/test-network-algorand"

echo "Tearing down and bringing up network..."
cd ../../fabric/scripts
./run-network.sh

echo "Deploying chaincode..."
cd "$NET_DIR"
./network.sh deployCC -ccn basic -ccp ../asset-transfer-basic/chaincode-javascript/ -ccl javascript

echo "Running benchmarks..."
cd "$DIR"
for tx in 1000 10000 100000; do
  for conc in 500 25; do
    echo "================================================================"
    echo "Running scale: $tx txs with concurrency $conc"
    echo "================================================================"
    node loadgen.js $tx $conc > "bench_${tx}_${conc}.log"
    sleep 5
  done
done
