#!/bin/bash
# Runs the full benchmark set against whichever network is up (Algorand, or
# stock Raft if started with RAFT=1 ../../fabric/scripts/run-network.sh).
# Each run appends a row to results.json, which the dashboard shows.
#
#   ./run_benchmark.sh            # everything (~35 min)
#   ./run_benchmark.sh quick      # skip the 100,000-transaction run
#
# Deploy both chaincodes first: ../../fabric/scripts/deploy-cc.sh
set -e
cd "$(dirname "${BASH_SOURCE[0]}")"
mkdir -p logs

run() { echo; echo "=== $*"; node "$@" 2>&1 | grep -vE "t=[0-9]+s (done|submitted)=" | tee -a logs/run_benchmark.log; sleep 5; }

run crash-test.js 60 20 40
run loadgen.js 1000 25
run loadgen.js 1000 500
run loadgen.js 10000 500
if [ "$1" != "quick" ]; then
    run loadgen.js 100000 500
fi
run paygen.js 1000 500 1000 0.06
run paygen.js 10000 500 1000 0.06
