#!/bin/bash
# Inject network faults into the orderers' CONSENSUS traffic only (the BA*
# mesh on TCP port 7060). Client submissions, peer block delivery and gRPC on
# 7050-7058 are untouched, so what changes is purely how BA* messages travel.
#
#   ./netem.sh apply <orderer|all> <netem args...>
#        e.g. ./netem.sh apply all delay 500ms 300ms
#             ./netem.sh apply orderer3 delay 1500ms 500ms
#             ./netem.sh apply all loss 10%
#             ./netem.sh apply orderer4 loss 100%        (cut it off)
#   ./netem.sh clear [orderer|all]
#   ./netem.sh show
#
# The orderer containers are not modified: tc runs in a throwaway container
# that joins the target's network namespace (needs NET_ADMIN, and the tc
# binary that build.sh installs in the orderer image).
set -e

IMAGE=hyperledger/fabric-orderer:algorand
ALL="orderer orderer2 orderer3 orderer4"

targets() {
    if [ "$1" = "all" ] || [ -z "$1" ]; then echo $ALL; else echo "$1"; fi
}

tc_in() {
    local o=$1; shift
    docker run --rm --net "container:$o.example.com" --cap-add NET_ADMIN --entrypoint sh "$IMAGE" -c "$*"
}

case "$1" in
apply)
    who=$2; shift 2
    for o in $(targets "$who"); do
        # Band 4 of a 4-band prio qdisc is unused by the default priomap, so
        # only packets the filters steer there (port 7060 either way) are hit.
        # A large limit keeps netem's own queue from silently dropping packets
        # when long delays hold many of them at once.
        tc_in "$o" "tc qdisc del dev eth0 root 2>/dev/null; \
            tc qdisc add dev eth0 root handle 1: prio bands 4 && \
            tc qdisc add dev eth0 parent 1:4 handle 40: netem limit 100000 $* && \
            tc filter add dev eth0 protocol ip parent 1:0 prio 1 u32 match ip dport 7060 0xffff flowid 1:4 && \
            tc filter add dev eth0 protocol ip parent 1:0 prio 1 u32 match ip sport 7060 0xffff flowid 1:4"
        echo "$o: $*"
    done
    ;;
clear)
    for o in $(targets "$2"); do
        tc_in "$o" "tc qdisc del dev eth0 root 2>/dev/null || true"
    done
    echo "cleared: $(targets "$2")"
    ;;
show)
    for o in $ALL; do
        echo "$o: $(tc_in "$o" "tc qdisc show dev eth0 | grep netem || echo none")"
    done
    ;;
*)
    sed -n '2,15p' "$0"
    exit 1
    ;;
esac
