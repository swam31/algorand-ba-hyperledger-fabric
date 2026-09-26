// Crash one orderer under steady load and measure the gap in commits.
//
//   node crash-test.js [seconds=60] [stopAt=20] [startAt=40]
//
// On Raft the victim is the current leader - its worst case, since the rest
// must notice and elect a new one. Algorand has no leader (a new proposer is
// drawn by lottery every round), so the victim is simply an orderer that
// neither peer pulls blocks from.
const grpc = require('@grpc/grpc-js');
const { connect, signers } = require('@hyperledger/fabric-gateway');
const { execFileSync } = require('child_process');
const fs = require('fs');
const path = require('path');
const crypto = require('crypto');
const { settledState } = require('./chainstate');

const base = path.resolve(__dirname, '../../../teluskohf/fabric-samples/test-network-algorand/organizations/peerOrganizations/org1.example.com');
const NAMES = ['orderer', 'orderer2', 'orderer3', 'orderer4'];
const sh = (cmd) => execFileSync('sh', ['-c', cmd], { maxBuffer: 1 << 26 }).toString();

function raftLeader() {
    // The leader logs "<id> became leader at term <t>"; the latest term wins.
    let best = null;
    for (const n of NAMES) {
        for (const m of sh(`docker logs ${n}.example.com 2>&1 | grep 'became leader at term' || true`).matchAll(/became leader at term (\d+)/g)) {
            const term = Number(m[1]);
            if (!best || term > best.term) best = { name: n, term };
        }
    }
    return best && best.name;
}

function deliverSources() {
    const used = new Set();
    for (const p of ['peer0.org1.example.com', 'peer0.org2.example.com']) {
        const m = [...sh(`docker logs ${p} 2>&1 | grep 'BlockReceiver starting' || true`).matchAll(/orderer-address=(orderer\d?)\.example\.com/g)];
        if (m.length) used.add(m[m.length - 1][1]);
    }
    return used;
}

async function main() {
    const seconds = Number(process.argv[2] || 60);
    const stopAt = Number(process.argv[3] || 20);
    const startAt = Number(process.argv[4] || 40);

    const client = new grpc.Client('localhost:7051', grpc.credentials.createSsl(fs.readFileSync(path.join(base, 'peers/peer0.org1.example.com/tls/ca.crt'))), { 'grpc.ssl_target_name_override': 'peer0.org1.example.com' });
    const gateway = connect({
        client,
        identity: { mspId: 'Org1MSP', credentials: fs.readFileSync(path.join(base, 'users/User1@org1.example.com/msp/signcerts/User1@org1.example.com-cert.pem')) },
        signer: signers.newPrivateKeySigner(crypto.createPrivateKey(fs.readFileSync(path.join(base, 'users/User1@org1.example.com/msp/keystore/priv_sk')))),
        // A normal client: if a transaction is not committed within 10 s it
        // gives up and submits the next one. (A transaction accepted by an
        // orderer that then crashes may simply be lost.)
        submitOptions: () => ({ deadline: Date.now() + 10000 }),
        commitStatusOptions: () => ({ deadline: Date.now() + 10000 }),
    });
    const network = gateway.getNetwork('mychannel');
    const contract = network.getContract('basic');
    const state = await settledState(network);
    const consensus = state.consensus;
    const victim = consensus === 'raft' ? raftLeader() : NAMES.find((n) => !deliverSources().has(n));
    if (!victim) throw new Error('could not choose a victim orderer');
    console.log(`${consensus}: stopping ${victim}${consensus === 'raft' ? ' (the Raft leader)' : ''} at t=${stopAt}s, restarting at t=${startAt}s`);

    const start = Date.now();
    const commits = [];
    let errors = 0;
    let lostAfterCrash = 0;
    let stopped = false;
    let n = 0;
    const worker = async () => {
        while (!stopped) {
            try {
                const p = contract.newProposal('CreateAsset', { arguments: [`crash_${start}_${n++}`, 'c', '1', 'o', '1'] });
                const sub = await (await p.endorse()).submit();
                const st = await sub.getStatus();
                if (st.successful) commits.push(Date.now() - start);
            } catch {
                errors++;
                if (Date.now() - start >= stopAt * 1000) lostAfterCrash++;
            }
        }
    };
    setTimeout(() => { console.log(`  t=${stopAt}s docker stop ${victim}`); sh(`docker stop ${victim}.example.com`); }, stopAt * 1000);
    setTimeout(() => { console.log(`  t=${startAt}s docker start ${victim}`); sh(`docker start ${victim}.example.com`); }, startAt * 1000);
    setTimeout(() => { stopped = true; }, seconds * 1000);
    await Promise.all(Array.from({ length: 20 }, worker));

    // Longest stretch with no commit, starting from the moment of the crash.
    const after = commits.filter((t) => t >= stopAt * 1000).sort((a, b) => a - b);
    let gap = after.length ? after[0] - stopAt * 1000 : (seconds - stopAt) * 1000;
    for (let i = 1; i < after.length; i++) gap = Math.max(gap, after[i] - after[i - 1]);
    const perSecond = Array.from({ length: seconds }, (_, s) => commits.filter((t) => t >= s * 1000 && t < (s + 1) * 1000).length);
    await new Promise((r) => setTimeout(r, 15000));
    const end = await settledState(network, 30000);

    const result = {
        consensus,
        victim,
        wasLeader: consensus === 'raft',
        stopAt,
        startAt,
        seconds,
        committed: commits.length,
        errors,
        timedOutAfterCrash: lostAfterCrash,
        longestGapMs: gap,
        perSecond,
        ordererHeightsAfter: end.ordererHeights,
        timestamp: new Date().toISOString(),
    };
    console.log(`  committed ${commits.length}; ${errors} transactions failed or timed out (${lostAfterCrash} after the crash); longest gap in commits after the crash ${(gap / 1000).toFixed(2)} s`);
    console.log(`  commits per second: ${perSecond.join(' ')}`);
    if (end.ordererHeights) console.log(`  orderer heights 15 s after the run: ${end.ordererHeights.join('/')}`);

    const out = path.join(__dirname, 'crash_results.json');
    let rows = [];
    try { rows = JSON.parse(fs.readFileSync(out)); } catch { /* first run */ }
    rows = rows.filter((r) => r.consensus !== consensus).concat([result]);
    fs.writeFileSync(out, JSON.stringify(rows, null, 2));
    gateway.close();
    client.close();
    process.exit(0);
}

main().catch((e) => { console.error(e); process.exit(1); });
