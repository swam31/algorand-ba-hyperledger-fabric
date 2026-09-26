#!/bin/bash
set -e

# This script pulls Hyperledger Fabric and injects our Algorand consensus module.

FABRIC_DIR="fabric-src"
ALG_DIR=$(cd ../.. && pwd)

echo ">>> Setting up Stage 3: Fabric Integration"

if [ ! -d "$FABRIC_DIR" ]; then
    echo ">>> Cloning Hyperledger Fabric (release-2.5)..."
    git clone -b release-2.5 https://github.com/hyperledger/fabric.git $FABRIC_DIR
fi

cd $FABRIC_DIR

echo ">>> Linking our Algorand Engine into Fabric's Go modules..."
# This tells Fabric to use our local code whenever it sees our module path
go mod edit -replace github.com/swayamchavan/algorand-fabric=$ALG_DIR
go mod edit -replace github.com/Shopify/sarama=github.com/Shopify/sarama@v1.20.1
go mod edit -replace github.com/hyperledger/fabric-protos-go=github.com/hyperledger/fabric-protos-go@v0.2.2

MAIN_FILE="orderer/common/server/main.go"

echo ">>> Injecting our Consenter into orderer/common/server/main.go..."

# Clean any previous injections to be idempotent
awk '!/algoconsenter/' $MAIN_FILE > tmp.go && mv tmp.go $MAIN_FILE

# 1. Add the import
awk '/import \(/ { print; print "\talgoconsenter \"github.com/swayamchavan/algorand-fabric/fabric/consenter\""; next }1' $MAIN_FILE > tmp.go && mv tmp.go $MAIN_FILE

# 2. Register the consenter for etcdraft and BFT
awk '/registrar\.Initialize\(consenters\)/ { print "\tconsenters[\"etcdraft\"] = algoconsenter.New()\n\tconsenters[\"BFT\"] = algoconsenter.New()"; print; next }1' $MAIN_FILE > tmp.go && mv tmp.go $MAIN_FILE

echo ">>> Injection complete."

echo ">>> Ensuring vendor directory is up to date..."
go mod edit -require=github.com/swayamchavan/algorand-fabric@v0.0.0-00010101000000-000000000000
rm -rf vendor/github.com/swayamchavan/algorand-fabric
go mod vendor -e

echo ">>> Building custom fabric-orderer Docker image..."
echo ">>> Using local cross-compilation on darwin/arm64..."
GOOS=linux GOARCH=arm64 go build -o orderer ./cmd/orderer
cat << 'EOF' > Dockerfile.algorand
FROM hyperledger/fabric-orderer:2.5
USER root
# iproute2 provides tc, used by netem.sh to inject network delay and loss
RUN apt-get update && apt-get install -y binutils iproute2 || true
COPY orderer /usr/local/bin/
EOF
docker build -t hyperledger/fabric-orderer:algorand -f Dockerfile.algorand .

echo ">>> Verifying image contents..."
if ! docker run --rm --entrypoint sh hyperledger/fabric-orderer:algorand -c "grep -a ALGORAND_MEMBERS /usr/local/bin/orderer > /dev/null"; then
    echo ">>> FATAL: ALGORAND_MEMBERS not found in the built image!"
    exit 1
fi
echo ">>> Verification successful!"

echo ">>> Build complete! You now have a hyperledger/fabric-orderer image powered by Algorand."
