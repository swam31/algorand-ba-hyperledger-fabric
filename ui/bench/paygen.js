// Payments load generator: opens N accounts with $1,000 each on the real
// Fabric network, fires random transfers between them - including deliberate
// attacks - and audits every balance afterwards.
//
//   node paygen.js <transfers> [concurrency=100] [accounts=1000] [badFraction=0.06] [timeoutSec] [retries=5]
//
// A normal transfer that loses a race (MVCC_READ_CONFLICT at commit, or the two
// peers endorsing against different ledger heights) is resubmitted up to
// `retries` times. Double-spend halves are never retried: one is meant to lose.
//
// Transfer kinds, and who stops the bad ones:
//   normal        owner pays $1-$100 to a random account
//   overdraft     owner sends far more than the balance      -> chaincode refuses
//   theft         an Org2 user spends an Org1-owned account  -> chaincode refuses (not owner)
//   forged        signature does not match the certificate   -> peer refuses before the chaincode runs
//   double-spend  two concurrent transfers that each fit the
//                 balance but not together                   -> Fabric marks the loser MVCC_READ_CONFLICT
//
// Every number is measured. Blocks and BA* rounds come from orderer telemetry.
const grpc = require('@grpc/grpc-js');
const { connect, signers } = require('@hyperledger/fabric-gateway');
const fs = require('fs');
const path = require('path');
const crypto = require('crypto');
const { settledState, orderingDelta } = require('./chainstate');

const NET = path.resolve(__dirname, '../../../teluskohf/fabric-samples/test-network-algorand/organizations/peerOrganizations');
const CHANNEL = 'mychannel';
const CHAINCODE = 'payments';
const START_DOLLARS = 1000;
const PREFIX = 'acct';

function orgConn(org, port, keyOverride) {
    const base = path.join(NET, `${org}.example.com`);
    const user = path.join(base, 'users', `User1@${org}.example.com`, 'msp');
    const client = new grpc.Client(`localhost:${port}`,
        grpc.credentials.createSsl(fs.readFileSync(path.join(base, 'peers', `peer0.${org}.example.com`, 'tls', 'ca.crt'))),
        { 'grpc.ssl_target_name_override': `peer0.${org}.example.com` });
    const key = keyOverride || crypto.createPrivateKey(fs.readFileSync(path.join(user, 'keystore', 'priv_sk')));
    const gateway = connect({
        client,
        identity: { mspId: org === 'org1' ? 'Org1MSP' : 'Org2MSP', credentials: fs.readFileSync(path.join(user, 'signcerts', `User1@${org}.example.com-cert.pem`)) },
        signer: signers.newPrivateKeySigner(key),
        commitStatusOptions: () => ({ deadline: Date.now() + 30 * 60 * 1000 }),
    });
    return { client, gateway, contract: gateway.getNetwork(CHANNEL).getContract(CHAINCODE) };
}

const decode = (bytes) => JSON.parse(Buffer.from(bytes).toString('utf8'));
const acctId = (i) => `${PREFIX}${String(i).padStart(4, '0')}`;

// Gateway errors carry the chaincode's message several layers down.
function reason(e) {
    const details = (e.details || []).map((d) => d.message).join('; ');
    const msg = `${e.message || e}${details ? ` | ${details}` : ''}`;
    const m = msg.match(/(insufficient funds[^|;]*|not your account[^|;]*|cannot pay yourself|access denied[^|;]*|signature[^|;]*invalid[^|;]*|creator[^|;]*signature[^|;]*)/i);
    return (m ? m[1] : msg).split('\n')[0].slice(0, 200);
}

async function ensureAccounts(contract, count) {
    const { accounts } = decode(await contract.evaluateTransaction('TotalSupply'));
    for (let start = accounts; start < count; start += 100) {
        const n = Math.min(100, count - start);
        await contract.submitTransaction('OpenAccounts', PREFIX, String(start), String(n), String(START_DOLLARS));
    }
    if (accounts < count) console.log(`Opened accounts ${acctId(accounts)}..${acctId(count - 1)} with $${START_DOLLARS} each`);
}

async function main() {
    const total = parseInt(process.argv[2] || '1000', 10);
    const concurrency = parseInt(process.argv[3] || '100', 10);
    const nAccounts = parseInt(process.argv[4] || '1000', 10);
    const bad = parseFloat(process.argv[5] || '0.06');
    const timeoutSec = parseInt(process.argv[6] || String(Math.max(300, Math.ceil(total / 40))), 10);
    const maxRetries = parseInt(process.argv[7] || '5', 10);

    const org1 = orgConn('org1', 7051);
    const org2 = orgConn('org2', 9051);
    // Org1's certificate, but a key that does not belong to it.
    const forger = orgConn('org1', 7051, crypto.generateKeyPairSync('ec', { namedCurve: 'prime256v1' }).privateKey);

    await ensureAccounts(org1.contract, nAccounts);
    const baseline = new Map(decode(await org1.contract.evaluateTransaction('AllBalances')).map((a) => [a.id, a.balance]));
    const issued = baseline.size * START_DOLLARS;
    const before = await settledState(org1.gateway.getNetwork(CHANNEL));

    // Plan every transfer up front so the mix is exactly as requested.
    const rnd = (n) => Math.floor(Math.random() * n);
    const pickPair = () => {
        const a = rnd(nAccounts);
        let b = rnd(nAccounts);
        while (b === a) b = rnd(nAccounts);
        return [acctId(a), acctId(b)];
    };
    const plan = [];
    while (plan.length < total) {
        const [from, to] = pickPair();
        if (Math.random() >= bad) {
            plan.push({ kind: 'normal', from, to, amount: 1 + rnd(100) });
            continue;
        }
        const k = ['overdraft', 'theft', 'forged', 'double-spend'][rnd(4)];
        if (k === 'overdraft') plan.push({ kind: k, from, to, amount: 1000000 });
        else if (k === 'theft') plan.push({ kind: k, from, to, amount: 1 + rnd(100) });
        else if (k === 'forged') plan.push({ kind: k, from, to, amount: 1 + rnd(100) });
        else {
            // Both halves read the same balance and are sent together; at
            // most one can be committed.
            let to2 = acctId(rnd(nAccounts));
            while (to2 === from) to2 = acctId(rnd(nAccounts));
            const pairId = plan.length;
            plan.push({ kind: k, from, to, amount: null, pairId }, { kind: k, from, to: to2, amount: null, pairId });
        }
    }
    plan.length = total;

    console.log(`Starting payments benchmark on ${before.consensus}: ${total} transfers across ${nAccounts} accounts ($${issued} issued), concurrency ${concurrency}, ${(bad * 100).toFixed(0)}% deliberately bad, timeout ${timeoutSec}s`);
    const start = Date.now();
    const results = [];
    const latencies = [];
    let next = 0;
    let stopped = false;
    let lastCommit = start;
    const balanceCache = new Map(baseline);

    // A normal transfer that lost a race had no effect, so it is safe to send
    // again with a fresh endorsement - what a real payments client does.
    const lostRace = (outcome) => outcome.startsWith('invalid: MVCC_READ_CONFLICT') || /ProposalResponsePayloads do not match/.test(outcome);

    const attempt = async (p) => {
        const conn = p.kind === 'theft' ? org2 : p.kind === 'forged' ? forger : org1;
        const r = { kind: p.kind, from: p.from, to: p.to, amount: p.amount };
        if (p.pairId !== undefined) r.pair = p.pairId;
        try {
            const proposal = conn.contract.newProposal('Transfer', { arguments: [p.from, p.to, String(p.amount)] });
            r.txId = proposal.getTransactionId();
            const tx = await proposal.endorse();
            const commit = await tx.submit();
            const status = await commit.getStatus();
            r.block = Number(status.blockNumber);
            if (status.successful) {
                r.outcome = 'committed';
                balanceCache.set(p.from, (balanceCache.get(p.from) ?? 0) - p.amount);
                balanceCache.set(p.to, (balanceCache.get(p.to) ?? 0) + p.amount);
                lastCommit = Date.now();
            } else {
                r.outcome = `invalid: ${status.code === 11 ? 'MVCC_READ_CONFLICT' : `code ${status.code}`}`;
            }
        } catch (e) {
            r.outcome = `refused: ${reason(e)}`;
        }
        return r;
    };

    const runOne = async (p) => {
        const t0 = Date.now();
        let r = await attempt(p);
        let attempts = 1;
        while (p.kind === 'normal' && lostRace(r.outcome) && attempts <= maxRetries && !stopped) {
            r = await attempt(p);
            attempts++;
        }
        r.attempts = attempts;
        r.ms = Date.now() - t0;
        if (r.outcome === 'committed') latencies.push(r.ms);
        results.push(r);
    };

    const worker = async () => {
        while (!stopped && next < plan.length) {
            const p = plan[next++];
            if (p.kind === 'double-spend' && p.amount === null) {
                // Size both halves from the balance as it is right now, then
                // send them at the same moment.
                const twin = plan[next];
                next++;
                const bal = balanceCache.get(p.from) ?? START_DOLLARS;
                const half = Math.floor(bal / 2) + 1;
                p.amount = half;
                if (twin) twin.amount = half;
                await Promise.all([runOne(p), twin ? runOne(twin) : null]);
                continue;
            }
            await runOne(p);
        }
    };

    const progress = setInterval(() => {
        const c = results.filter((r) => r.outcome === 'committed').length;
        console.log(`  t=${((Date.now() - start) / 1000).toFixed(0)}s done=${results.length} committed=${c}`);
    }, 10000);
    let timedOut = false;
    await Promise.race([
        Promise.all(Array.from({ length: concurrency }, worker)),
        new Promise((res) => setTimeout(() => { timedOut = true; res(); }, timeoutSec * 1000)),
    ]);
    stopped = true;
    clearInterval(progress);
    const end = Date.now();
    const after = await settledState(org1.gateway.getNetwork(CHANNEL));
    const ord = orderingDelta(before, after);

    // ---- audit: replay committed transfers against the starting balances ----
    const expected = new Map(baseline);
    for (const r of results) {
        if (r.outcome !== 'committed') continue;
        expected.set(r.from, expected.get(r.from) - r.amount);
        expected.set(r.to, expected.get(r.to) + r.amount);
    }
    const onChain = decode(await org1.contract.evaluateTransaction('AllBalances'));
    const supply = decode(await org1.contract.evaluateTransaction('TotalSupply'));
    let mismatches = 0;
    let negative = 0;
    for (const a of onChain) {
        if (a.balance !== expected.get(a.id)) mismatches++;
        if (a.balance < 0) negative++;
    }
    const audit = {
        issued,
        supplyNow: supply.total,
        supplyConserved: supply.total === issued,
        accountsChecked: onChain.length,
        balanceMismatches: mismatches,
        negativeBalances: negative,
        passed: supply.total === issued && mismatches === 0 && negative === 0 && !timedOut,
    };

    // ---- summary ----
    const kinds = {};
    for (const r of results) {
        const k = (kinds[r.kind] = kinds[r.kind] || { sent: 0, committed: 0, invalid: 0, refused: 0, reasons: {} });
        k.sent++;
        const bucket = r.outcome === 'committed' ? 'committed' : r.outcome.startsWith('invalid') ? 'invalid' : 'refused';
        k[bucket]++;
        if (bucket !== 'committed') {
            const why = r.outcome.replace(/: .*has \$\d+.*/, ': insufficient funds').replace(/acct\d+/g, 'acct');
            k.reasons[why] = (k.reasons[why] || 0) + 1;
        }
    }
    // A double-spend pair must never have both halves committed.
    const pairs = new Map();
    for (const r of results.filter((x) => x.kind === 'double-spend')) {
        pairs.set(r.pair, (pairs.get(r.pair) || 0) + (r.outcome === 'committed' ? 1 : 0));
    }
    const doubleSpendsBothCommitted = [...pairs.values()].filter((v) => v > 1).length;

    latencies.sort((a, b) => a - b);
    const pct = (q) => (latencies.length ? latencies[Math.min(latencies.length - 1, Math.floor(latencies.length * q))] : 0);
    const committed = results.filter((r) => r.outcome === 'committed').length;
    const durationSec = (end - start) / 1000;
    const row = {
        workload: 'payments',
        consensus: ord.consensus,
        build: process.env.BUILD_TAG || null,
        scale: String(total),
        concurrency,
        accounts: nAccounts,
        badFraction: bad,
        retries: maxRetries,
        attempts: results.reduce((s, r) => s + r.attempts, 0),
        submitted: results.length,
        totalTxs: committed,
        invalid: results.filter((r) => r.outcome.startsWith('invalid')).length,
        errors: results.filter((r) => r.outcome.startsWith('refused')).length,
        complete: !timedOut && results.length === total,
        durationSec,
        e2eTps: committed / durationSec,
        ordTps: committed / Math.max(0.001, (lastCommit - start) / 1000),
        blocks: ord.blocks,
        txPerBlock: 0,
        rounds: ord.rounds,
        emptyRounds: ord.emptyRounds,
        tentativeRounds: ord.tentativeRounds,
        wastedPct: 0,
        ordererHeights: ord.ordererHeights,
        p50: pct(0.5),
        p95: pct(0.95),
        kinds,
        doubleSpendsBothCommitted,
        audit,
        stalls: timedOut ? 'Incomplete' : 0,
        timestamp: new Date().toISOString(),
    };
    row.txPerBlock = row.blocks > 0 ? results.filter((r) => r.block !== undefined).length / row.blocks : 0;
    row.wastedPct = ord.wastedPct;

    console.log(`Finished in ${durationSec.toFixed(1)}s: ${committed} committed, ${row.invalid} invalidated at commit, ${row.errors} refused before ordering`);
    const roundInfo = row.rounds === null ? 'no BA* rounds (Raft)' : `${row.rounds} BA* rounds, ${row.emptyRounds} empty`;
    console.log(`  ${row.e2eTps.toFixed(1)} committed tps | ${row.blocks} blocks | ${roundInfo} | p50 ${row.p50}ms p95 ${row.p95}ms${row.ordererHeights ? ` | heights ${JSON.stringify(row.ordererHeights)}` : ''}`);
    for (const [k, v] of Object.entries(kinds)) {
        console.log(`  ${k.padEnd(13)} sent ${String(v.sent).padStart(6)}  committed ${String(v.committed).padStart(6)}  invalid ${String(v.invalid).padStart(5)}  refused ${String(v.refused).padStart(5)}  ${JSON.stringify(v.reasons)}`);
    }
    console.log(`  ${row.attempts} submissions for ${results.length} transfers: ${row.attempts - results.length} resubmissions after losing a race (max ${maxRetries} per transfer)`);
    console.log(`  double-spend pairs with BOTH halves committed: ${doubleSpendsBothCommitted} (must be 0)`);
    console.log(`  audit: supply $${audit.supplyNow} of $${audit.issued} issued (conserved=${audit.supplyConserved}), ${audit.accountsChecked} balances checked, ${audit.balanceMismatches} mismatches, ${audit.negativeBalances} negative -> ${audit.passed ? 'PASSED' : 'FAILED'}`);

    const resFile = process.env.RESULTS_FILE || path.join(__dirname, 'results.json');
    let past = [];
    try { past = JSON.parse(fs.readFileSync(resFile)); } catch { /* first run */ }
    past.push(row);
    fs.writeFileSync(resFile, JSON.stringify(past, null, 2));
    // Transfers refused before ordering never reach the ledger, so this log is
    // the only record of them; the dashboard shows it next to the round history.
    fs.writeFileSync(path.join(__dirname, 'payments_last_run.json'), JSON.stringify({ summary: row, transfers: results }, null, 0));

    for (const c of [org1, org2, forger]) { c.gateway.close(); c.client.close(); }
    process.exit(0);
}

main().catch((e) => { console.error(e); process.exit(1); });
