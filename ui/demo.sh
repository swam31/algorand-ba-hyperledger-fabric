#!/bin/bash

# Ensure we're in the right directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/../fabric/scripts"

echo "=========================================================="
echo "  ALGORAND BA* CONSENSUS DASHBOARD DEMO"
echo "  Ensure you have the dashboard open in a browser"
echo "  http://localhost:3000"
echo "=========================================================="
echo ""

# Helper for transactions
submit_tx() {
    local i=$1
    echo "  -> Submitting transaction $i..."
    ./invoke.sh "asset_demo_$i" "blue" "10" "tom" "100" > /dev/null 2>&1
    sleep 5
}

echo "PHASE 1: Normal Operation (n=4, f=1)"
echo "Watch the 'Live Consensus Rounds' and 'Committed Blocks' tables populate."
for i in 1 2 3; do
    submit_tx $i
done
echo ""

echo "PHASE 2: Simulating Node Failure"
echo "Stopping orderer4.example.com... watch the Orderer Health panel."
echo "The Network Summary banner will turn red indicating 'CONSENSUS AT RISK'."
docker stop orderer4.example.com > /dev/null 2>&1
sleep 5
echo ""

echo "PHASE 3: Tolerating f=1 Byzantine Fault"
echo "With orderer4 offline, the network still has 3 honest nodes (h=0.75)."
echo "Since the threshold is 3-of-4, transactions MUST continue to commit."
for i in 4 5 6; do
    submit_tx $i
done
echo ""

echo "PHASE 4: Node Recovery (The Known Limitation)"
echo "Starting orderer4.example.com back up..."
docker start orderer4.example.com > /dev/null 2>&1
sleep 5
echo "Watch the Orderer Health panel for orderer4."
echo "Because there is no block replication in this PoC, orderer4 will reconnect to the TCP mesh,"
echo "but it is stuck at an old ledger height. It will show as LAGGING and will not participate"
echo "in future consensus rounds."
echo ""

echo "PHASE 5: Continued Operation with Lagging Node"
echo "The network is now operating as a 3-node network effectively."
for i in 7 8; do
    submit_tx $i
done

echo ""
echo "=========================================================="
echo "  DEMO COMPLETE"
echo "=========================================================="
