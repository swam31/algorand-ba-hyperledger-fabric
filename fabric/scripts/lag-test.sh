#!/bin/bash
# Catch-up test: freeze one orderer in the middle of a load run, let the other
# three move on without it, then unfreeze it and check it rejoins on its own.
#
# This is the failure that wedged orderer2 (P12) and orderer3 (P13 start): a
# lagging orderer threw away proposals for later rounds, then agreed on blocks
# it no longer had and retried the same round forever.
#
#   ./lag-test.sh [orderer=orderer3] [pauseSec=15] [txs=3000]
set -e

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BENCH="$DIR/../../ui/bench"
VICTIM="${1:-orderer3}"
PAUSE="${2:-15}"
TXS="${3:-3000}"
CONTAINER="$VICTIM.example.com"

heights() {
    for p in 7070 7071 7072 7073; do
        curl -s --max-time 2 "localhost:$p/status" | python3 -c 'import sys,json; print(json.load(sys.stdin)["ledgerHeight"])' 2>/dev/null || echo "down"
    done | paste -sd' ' -
}

echo "=> Heights before: $(heights)"
SINCE=$(date -u +%Y-%m-%dT%H:%M:%SZ)

echo "=> Starting load: $TXS txs"
(cd "$BENCH" && node loadgen.js "$TXS" 200 > "$BENCH/lagtest_load.log" 2>&1) &
LOAD=$!

sleep 8
echo "=> Freezing $VICTIM for ${PAUSE}s (docker pause: process stopped, connections kept)"
docker pause "$CONTAINER" > /dev/null
sleep 3
echo "   heights while frozen: $(heights)"
sleep $((PAUSE - 3))
docker unpause "$CONTAINER" > /dev/null
echo "=> Unfroze $VICTIM; heights: $(heights)"

wait $LOAD
tail -4 "$BENCH/lagtest_load.log"
sleep 5
FINAL_HEIGHTS=$(heights)
echo "=> Heights after load: $FINAL_HEIGHTS"

LOGS=$(docker logs --since "$SINCE" "$CONTAINER" 2>&1)
echo "=> $VICTIM after unfreezing:"
echo "   rounds replayed without waiting (behind=true): $(grep -c 'behind=true' <<<"$LOGS")"
echo "   blocks fetched from peers:                     $(grep -c 'fetched block' <<<"$LOGS")"
echo "   BA* failures:                                  $(grep -c 'BA\* failed' <<<"$LOGS")"

if [ "$(tr ' ' '\n' <<<"$FINAL_HEIGHTS" | sort -u | wc -l | tr -d ' ')" != "1" ]; then
    echo "FAIL: orderers disagree on height: $FINAL_HEIGHTS"
    exit 1
fi
if ! grep -q "committed $TXS/$TXS" "$BENCH/lagtest_load.log"; then
    echo "FAIL: not every transaction committed"
    exit 1
fi
echo "PASS: $VICTIM caught up; all four orderers at the same height and every transaction committed."
