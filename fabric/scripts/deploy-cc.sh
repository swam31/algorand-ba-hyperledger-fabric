#!/bin/bash
# Deploys both chaincodes the project uses, then checks each one answers:
#   basic     - the Fabric sample asset chaincode (JavaScript), used by loadgen.js
#   payments  - our $1,000-account chaincode (chaincode/payments), used by paygen.js
#
# Run after every run-network.sh: a rebuilt network has an empty ledger.
set -e

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
ALG_DIR="$(cd "$DIR/../.." && pwd)"
NET_DIR="$ALG_DIR/../teluskohf/fabric-samples/test-network-algorand"

cd "$NET_DIR"

echo ">>> Deploying basic (asset transfer, JavaScript)..."
./network.sh deployCC -ccn basic -ccp ../asset-transfer-basic/chaincode-javascript/ -ccl javascript > deploy-basic.log 2>&1 \
    || { tail -20 deploy-basic.log; exit 1; }

echo ">>> Deploying payments (\$1,000 accounts)..."
./network.sh deployCC -ccn payments -ccp "$ALG_DIR/chaincode/payments" -ccl javascript > deploy-payments.log 2>&1 \
    || { tail -20 deploy-payments.log; exit 1; }

export PATH=${PWD}/../bin:$PATH
export FABRIC_CFG_PATH=${PWD}/../config/
export CORE_PEER_TLS_ENABLED=true
export CORE_PEER_LOCALMSPID="Org1MSP"
export CORE_PEER_TLS_ROOTCERT_FILE=${PWD}/organizations/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/ca.crt
export CORE_PEER_MSPCONFIGPATH=${PWD}/organizations/peerOrganizations/org1.example.com/users/Admin@org1.example.com/msp
export CORE_PEER_ADDRESS=localhost:7051

echo ">>> Checking both chaincodes answer..."
peer chaincode query -C mychannel -n basic -c '{"Args":["GetAllAssets"]}' > /dev/null
peer chaincode query -C mychannel -n payments -c '{"Args":["TotalSupply"]}'
echo ">>> Both chaincodes deployed."
