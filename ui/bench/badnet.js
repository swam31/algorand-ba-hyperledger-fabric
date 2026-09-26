// Bad-network experiments (paper section 7.4, "weak synchrony").
//
// For each scenario: inject a fault into the orderers' consensus traffic with
// netem, run 90 s of load, record what every orderer's BA* rounds did, remove
// the fault, and measure how long until all four orderers again hold the same
// chain (same height AND same last block hash - blocks are hash-chained, so
// that means identical ledgers, i.e. no fork).
//
//   node badnet.js [scenarioId ...]      (default: all)
const { execFileSync, spawn } = require('child_process');
const fs = require('fs');
const path = require('path');
const { getStatus, ORDERERS } = require('./chainstate');

const NETEM = path.resolve(__dirname, '../../fabric/scripts/netem.sh');
const OUT = path.join(__dirname, 'badnet_results.json');
const LOAD_SECONDS = 90;
const NAMES = ['orderer', 'orderer2', 'orderer3', 'orderer4'];

const netem = (...args) => execFileSync(NETEM, args, { stdio: 'pipe' }).toString().trim();
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const snapshot = () => Promise.all(ORDERERS.map(getStatus));

// A peer pulls blocks from one orderer and does not switch while that orderer
// is merely slow or partitioned. Faults go to orderers no peer depends on, so
// the measurement shows consensus, not a stalled block feed.
function deliverSources() {
    const used = new Set();
    for (const peer of ['peer0.org1.example.com', 'peer0.org2.example.com']) {
        const log = execFileSync('sh', ['-c', `docker logs ${peer} 2>&1 | grep 'BlockReceiver starting'`], { maxBuffer: 1 << 26 }).toString();
        const m = [...log.matchAll(/BlockReceiver starting.*orderer-address=(orderer\d?)\.example\.com/g)];
        if (m.length) used.add(m[m.length - 1][1]);
    }
    return used;
}

function scenarios(victims) {
    const [v1, v2] = victims;
    return [
        { id: 'baseline', title: 'Healthy network (no fault)', faults: [] },
        { id: 'wan-100', title: 'Every consensus message delayed 100 ms ± 20 ms (a continent-wide WAN)', faults: [['all', 'delay', '100ms', '20ms']] },
        { id: 'wan-500', title: 'Every consensus message delayed 500 ms ± 300 ms (bad WAN; packets reordered)', faults: [['all', 'delay', '500ms', '300ms']] },
        { id: 'loss-10', title: '10% of consensus packets dropped (TCP retransmits them late)', faults: [['all', 'loss', '10%']] },
        { id: 'slow-node', title: `One orderer (${v1}) delayed 1.5 s ± 0.5 s: its proposals miss the 1 s window`, faults: [[v1, 'delay', '1500ms', '500ms']] },
        { id: 'very-slow', title: 'All consensus messages delayed 1.5 s ± 0.5 s: longer than the 1 s proposal window', faults: [['all', 'delay', '1500ms', '500ms']] },
        { id: 'isolate-1', title: `One orderer (${v1}) cut off completely: f = 1 of 4 missing`, faults: [[v1, 'loss', '100%']] },
        { id: 'partition-2', title: `Two orderers (${v1}, ${v2}) cut off for 45 s, then reconnected: no 3-of-4 quorum`, faults: [[v1, 'loss', '100%'], [v2, 'loss', '100%']], healAfterSec: 45 },
    ];
}

function runLoad() {
    return new Promise((resolve) => {
        const tmp = path.join(__dirname, `.badnet_row_${process.pid}.json`);
        fs.writeFileSync(tmp, '[]');
        const child = spawn('node', ['loadgen.js', '100000', '50', String(LOAD_SECONDS)], {
            cwd: __dirname,
            env: { ...process.env, RESULTS_FILE: tmp },
            stdio: ['ignore', 'pipe', 'pipe'],
        });
        let out = '';
        child.stdout.on('data', (d) => { out += d; });
        child.stderr.on('data', (d) => { out += d; });
        child.on('close', () => {
            const rows = JSON.parse(fs.readFileSync(tmp));
            fs.unlinkSync(tmp);
            resolve({ row: rows[rows.length - 1] || null, out });
        });
    });
}

function roundDelta(a, b) {
    if (!a || !b) return null;
    return {
        total: b.rounds.total - a.rounds.total,
        final: b.rounds.final - a.rounds.final,
        tentative: b.rounds.tentative - a.rounds.tentative,
        empty: (b.rounds.empty || 0) - (a.rounds.empty || 0),
        failed: b.rounds.failed - a.rounds.failed,
    };
}

async function converge(maxSec = 180) {
    const start = Date.now();
    for (;;) {
        const s = await snapshot();
        const ok = s.every(Boolean)
            && s.every((x) => x.ledgerHeight === s[0].ledgerHeight)
            && s.every((x) => x.lastBlockHash === s[0].lastBlockHash);
        if (ok) return { converged: true, seconds: (Date.now() - start) / 1000, state: s };
        if (Date.now() - start > maxSec * 1000) return { converged: false, seconds: maxSec, state: s };
        await sleep(1000);
    }
}

async function runScenario(sc) {
    console.log(`\n=== ${sc.id}: ${sc.title}`);
    netem('clear', 'all');
    await converge(60);
    const before = await snapshot();

    for (const [who, ...args] of sc.faults) console.log(`  ${netem('apply', who, ...args)}`);
    let healTimer = null;
    if (sc.healAfterSec) {
        healTimer = setTimeout(() => {
            console.log(`  t=${sc.healAfterSec}s: reconnecting (${netem('clear', 'all')})`);
        }, sc.healAfterSec * 1000);
    }
    const heightsAt = [];
    const sampler = setInterval(async () => {
        const s = await snapshot();
        heightsAt.push(s.map((x) => (x ? x.ledgerHeight : null)));
    }, 5000);

    const { row } = await runLoad();
    clearInterval(sampler);
    if (healTimer) clearTimeout(healTimer);
    const during = await snapshot();

    netem('clear', 'all');
    const rec = await converge();
    const perOrderer = NAMES.map((n, i) => ({ orderer: n, ...roundDelta(before[i], during[i]) }));
    const result = {
        id: sc.id,
        title: sc.title,
        faults: sc.faults.map((f) => f.join(' ')),
        healAfterSec: sc.healAfterSec || null,
        loadSeconds: LOAD_SECONDS,
        committed: row ? row.totalTxs : 0,
        tps: row ? row.e2eTps : 0,
        p50: row ? row.p50 : null,
        p95: row ? row.p95 : null,
        blocks: Math.max(...during.map((x) => (x ? x.ledgerHeight : 0))) - Math.max(...before.map((x) => (x ? x.ledgerHeight : 0))),
        rounds: perOrderer,
        heightsEvery5s: heightsAt,
        forkFree: rec.converged,
        convergedAfterSec: rec.seconds,
        finalHeights: rec.state.map((x) => (x ? x.ledgerHeight : null)),
        finalHashes: rec.state.map((x) => (x ? (x.lastBlockHash || '').slice(0, 12) : null)),
        timestamp: new Date().toISOString(),
    };
    const r1 = perOrderer[0];
    console.log(`  committed ${result.committed} in ${LOAD_SECONDS}s (${result.tps.toFixed(1)} tps), p50 ${result.p50} ms, p95 ${result.p95} ms, ${result.blocks} blocks`);
    console.log(`  rounds per orderer (FINAL/TENTATIVE/EMPTY): ${perOrderer.map((p) => `${p.orderer} ${p.final}/${p.tentative}/${p.empty}`).join(' · ')}`);
    console.log(`  after removing the fault: ${rec.converged ? `all four identical after ${rec.seconds.toFixed(0)} s` : 'DID NOT CONVERGE'} at height ${result.finalHeights.join('/')} hash ${result.finalHashes[0]}`);
    return result;
}

async function main() {
    const used = deliverSources();
    const victims = NAMES.filter((n) => !used.has(n));
    if (victims.length < 2) throw new Error(`need two orderers no peer delivers from; peers use ${[...used]}`);
    console.log(`Peers pull blocks from: ${[...used].join(', ')}; faults target ${victims.slice(0, 2).join(', ')}`);

    const wanted = process.argv.slice(2);
    // BADNET_TAG labels a run made with non-default settings, e.g. the 1 s
    // proposal window, so it sits beside the default run instead of replacing it.
    const tag = process.env.BADNET_TAG;
    const list = scenarios(victims).filter((s) => !wanted.length || wanted.includes(s.id))
        .map((s) => (tag ? { ...s, id: `${s.id}@${tag}`, title: `${s.title} [${tag}]` } : s));
    let results = [];
    try { results = JSON.parse(fs.readFileSync(OUT)); } catch { /* first run */ }
    try {
        for (const sc of list) {
            const r = await runScenario(sc);
            results = results.filter((x) => x.id !== r.id).concat([r]);
            fs.writeFileSync(OUT, JSON.stringify(results, null, 2));
        }
    } finally {
        netem('clear', 'all');
    }
}

main().catch((e) => {
    console.error(e);
    try { netem('clear', 'all'); } catch { /* best effort */ }
    process.exit(1);
});
