#!/bin/bash
# Creates test-network-algorand inside a fabric-samples checkout: a copy of
# the stock test-network with our four changed files laid over it.
#
#   ./install.sh [path/to/fabric-samples]     (default: ../../teluskohf/fabric-samples)
#
# Tested against hyperledger/fabric-samples commit 05edea01d4cf24dd4087bd3750c36e690dc4d6ff.
# The network has to live inside fabric-samples because it uses that
# repository's bin/ (peer, configtxgen, osnadmin), config/ and sample chaincode.
#
# What the four files change (full diff: changes-from-stock-test-network.diff):
#   compose/compose-bft-test-net.yaml  four orderers running our image (or stock
#                                      Raft via ORDERER_IMAGE), ALGORAND_* settings,
#                                      telemetry ports 7070-7073, orderer keys volume
#   bft-config/configtx.yaml           channel profiles, including the 500-tx batch
#                                      profile and the four-consenter Raft profile
#   configtx/configtx.yaml             the same batch profiles for the non-BFT path
#   scripts/createChannel.sh           BATCH500 / RAFT4 switches that pick a profile
set -e

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SAMPLES="${1:-$HERE/../../teluskohf/fabric-samples}"
SAMPLES="$(cd "$SAMPLES" && pwd)"

if [ ! -d "$SAMPLES/test-network" ]; then
    echo "No test-network in $SAMPLES - is that a fabric-samples checkout?"
    exit 1
fi

TARGET="$SAMPLES/test-network-algorand"
if [ ! -d "$TARGET" ]; then
    echo ">>> Copying the stock test-network to $TARGET"
    cp -R "$SAMPLES/test-network" "$TARGET"
fi

echo ">>> Applying our four files"
(cd "$HERE/overlay" && find . -type f) | while read -r f; do
    cp "$HERE/overlay/$f" "$TARGET/$f"
    echo "    $f"
done
echo ">>> Done. Start it with: fabric/scripts/run-network.sh (see README.md)"
