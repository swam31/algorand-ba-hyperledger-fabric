#!/bin/bash
set -e

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
KEYS_DIR="$DIR/keys"

mkdir -p "$KEYS_DIR"

cd "$DIR/../.."

echo ">>> Building genkey..."
go build -o "$DIR/genkey" ./fabric/scripts/cmd/genkey

echo ">>> Generating keys..."
MEMBERS=()
for i in {1..4}; do
    KEY_FILE="$KEYS_DIR/orderer${i}.key"
    if [ ! -f "$KEY_FILE" ]; then
        echo "Generating key for orderer${i}..."
    fi
    PUBKEY=$("$DIR/genkey" "$KEY_FILE" | grep -v '>>>' | tr -d '\n')
    MEMBERS+=("orderer${i}:${PUBKEY}:100000")
done

# Join members with commas
MEMBERS_STR=$(IFS=, ; echo "${MEMBERS[*]}")

echo ">>> Writing members.env..."
echo "ALGORAND_MEMBERS=$MEMBERS_STR" > "$KEYS_DIR/members.env"
echo ">>> ALGORAND_MEMBERS string:"
echo "$MEMBERS_STR"
echo "Done!"
