#!/bin/bash
set -e

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FABRIC_DIR="$DIR/../../../teluskohf/fabric-samples"
TEST_NET_DIR="$FABRIC_DIR/test-network-algorand"

cd "$TEST_NET_DIR"

export FABRIC_CFG_PATH="$FABRIC_DIR/config"
export CORE_PEER_TLS_ENABLED=true
export CORE_PEER_LOCALMSPID="Org1MSP"
export CORE_PEER_TLS_ROOTCERT_FILE="$TEST_NET_DIR/organizations/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/ca.crt"
export CORE_PEER_MSPCONFIGPATH="$TEST_NET_DIR/organizations/peerOrganizations/org1.example.com/users/Admin@org1.example.com/msp"
export CORE_PEER_ADDRESS=localhost:7051

echo "=> Querying initial ledger height..."
INFO=$(../bin/peer channel getinfo -c mychannel)
HEIGHT1=$(echo $INFO | grep -o '"height":[0-9]*' | cut -d':' -f2)
echo "   Height 1: $HEIGHT1"

echo "=> Stopping orderer4..."
docker stop orderer4.example.com > /dev/null

echo "=> Invoking a transaction with orderer4 DOWN (f=1 fault tolerance active)..."
"$DIR/invoke.sh" "test-$(date +%s%N)" "blue" "10" "tom" "100" > /dev/null
sleep 10

echo "=> Querying ledger height..."
INFO=$(../bin/peer channel getinfo -c mychannel)
HEIGHT2=$(echo $INFO | grep -o '"height":[0-9]*' | cut -d':' -f2)
echo "   Height 2: $HEIGHT2"

if [ "$HEIGHT2" -le "$HEIGHT1" ]; then
    echo "ERROR: Ledger did not advance with 1 orderer down!"
    exit 1
fi
echo "=> SUCCESS! Ledger advanced to $HEIGHT2"

echo "=> Restarting orderer4..."
docker start orderer4.example.com > /dev/null
sleep 5

echo "=> Invoking a transaction with orderer4 restarted..."
"$DIR/invoke.sh" "test-$(date +%s%N)" "blue" "10" "tom" "100" > /dev/null
sleep 10

echo "=> Querying final ledger height..."
INFO=$(../bin/peer channel getinfo -c mychannel)
HEIGHT3=$(echo $INFO | grep -o '"height":[0-9]*' | cut -d':' -f2)
echo "   Height 3: $HEIGHT3"

if [ "$HEIGHT3" -le "$HEIGHT2" ]; then
    echo "ERROR: Ledger did not advance after orderer4 restarted!"
    exit 1
fi
echo "=> SUCCESS! Ledger advanced to $HEIGHT3"
echo "All fault-tolerance checks passed!"
