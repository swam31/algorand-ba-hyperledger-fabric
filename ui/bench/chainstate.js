// Chain state before and after a benchmark run, for either ordering service.
//
// On the Algorand network every orderer exposes telemetry (ports 7070-7073)
// with its height and BA* round counts. Stock Raft orderers have no such
// endpoint, so there the height comes from the peer's own ledger (qscc
// GetChainInfo) and round counts do not exist.
const http = require('http');
const { common } = require('@hyperledger/fabric-protos');

const ORDERERS = [7070, 7071, 7072, 7073];

function getStatus(port) {
    return new Promise((resolve) => {
        const req = http.get({ host: '127.0.0.1', port, path: '/status', timeout: 2000 }, (res) => {
            let body = '';
            res.on('data', (d) => { body += d; });
            res.on('end', () => { try { resolve(JSON.parse(body)); } catch { resolve(null); } });
        });
        req.on('error', () => resolve(null));
        req.on('timeout', () => { req.destroy(); resolve(null); });
    });
}

async function peerHeight(network) {
    const raw = await network.getContract('qscc').evaluateTransaction('GetChainInfo', network.getName());
    return Number(common.BlockchainInfo.deserializeBinary(raw).getHeight());
}

// Waits until the chain stops moving (all orderers at one height on Algorand;
// the peer's height unchanged for 2 s on Raft), then reports it.
async function settledState(network, maxWaitMs = 15000) {
    const deadline = Date.now() + maxWaitMs;
    const snap = await Promise.all(ORDERERS.map(getStatus));
    if (snap[0]) {
        let s = snap;
        for (;;) {
            const hs = s.filter(Boolean).map((x) => x.ledgerHeight);
            if ((hs.length && hs.every((h) => h === hs[0])) || Date.now() > deadline) break;
            await new Promise((r) => setTimeout(r, 500));
            s = await Promise.all(ORDERERS.map(getStatus));
        }
        return {
            consensus: 'algorand',
            height: s[0].ledgerHeight,
            rounds: s[0].rounds,
            ordererHeights: s.map((x) => (x ? x.ledgerHeight : null)),
            lastHashes: s.map((x) => (x ? x.lastBlockHash : null)),
        };
    }
    let h = await peerHeight(network);
    for (;;) {
        await new Promise((r) => setTimeout(r, 2000));
        const h2 = await peerHeight(network);
        if (h2 === h || Date.now() > deadline) break;
        h = h2;
    }
    return { consensus: 'raft', height: h, rounds: null, ordererHeights: null, lastHashes: null };
}

// Row fields describing what the ordering service did between two states.
function orderingDelta(before, after) {
    const blocks = after.height - before.height;
    if (!before.rounds || !after.rounds) {
        return { consensus: before.consensus, blocks, rounds: null, emptyRounds: null, tentativeRounds: null, wastedPct: null, ordererHeights: null };
    }
    const rounds = after.rounds.total - before.rounds.total;
    const emptyRounds = (after.rounds.empty || 0) - (before.rounds.empty || 0);
    return {
        consensus: before.consensus,
        blocks,
        rounds,
        emptyRounds,
        tentativeRounds: after.rounds.tentative - before.rounds.tentative,
        wastedPct: rounds > 0 ? (100 * emptyRounds) / rounds : 0,
        ordererHeights: after.ordererHeights,
    };
}

module.exports = { settledState, orderingDelta, getStatus, ORDERERS };
