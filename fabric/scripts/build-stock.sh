#!/bin/bash
# Builds an UNMODIFIED Fabric 2.5 orderer (stock Raft) for comparison.
#
# It compiles the same fabric-src tree as build.sh, with the same compiler and
# flags, but without the lines that register our Algorand consenter - so the
# only difference between the two images is the consensus module. The image
# reuses hyperledger/fabric-orderer:algorand as its base, so the container
# environment is identical too. Run build.sh first.
set -e

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$DIR/fabric-src"

MAIN_FILE="orderer/common/server/main.go"
cp "$MAIN_FILE" "$MAIN_FILE.algorand"
trap 'mv "$MAIN_FILE.algorand" "$MAIN_FILE"' EXIT

awk '!/algoconsenter/' "$MAIN_FILE.algorand" > "$MAIN_FILE"
if grep -q algoconsenter "$MAIN_FILE"; then
    echo ">>> FATAL: could not remove the Algorand consenter registration"
    exit 1
fi

echo ">>> Building stock Fabric 2.5 orderer (Raft)..."
GOOS=linux GOARCH=arm64 go build -o orderer-stock ./cmd/orderer

cat << 'EOF' > Dockerfile.stock
FROM hyperledger/fabric-orderer:algorand
COPY orderer-stock /usr/local/bin/orderer
EOF
docker build -q -t hyperledger/fabric-orderer:stock-raft-2.5 -f Dockerfile.stock .

if docker run --rm --entrypoint sh hyperledger/fabric-orderer:stock-raft-2.5 -c "grep -aq ALGORAND_MEMBERS /usr/local/bin/orderer"; then
    echo ">>> FATAL: stock image still contains the Algorand consenter"
    exit 1
fi
docker run --rm hyperledger/fabric-orderer:stock-raft-2.5 orderer version | head -3
echo ">>> Built hyperledger/fabric-orderer:stock-raft-2.5 (no Algorand code)"
