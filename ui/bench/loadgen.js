// Load generator: submits N CreateAsset transactions through the Fabric
// Gateway and reports what actually happened.
//
//   node loadgen.js <txs> [concurrency=100] [timeoutSec=max(300, txs/50)]
//
// Every number it records is measured: blocks and BA* rounds come from the
// orderers' telemetry before and after the run (on stock Raft, which has no
// telemetry, blocks come from the peer's ledger and there are no rounds).
const grpc = require('@grpc/grpc-js');
const { connect, signers } = require('@hyperledger/fabric-gateway');
const fs = require('fs');
const path = require('path');
const crypto = require('crypto');

const cryptoPath = path.resolve(__dirname, '../../../teluskohf/fabric-samples/test-network-algorand/organizations/peerOrganizations/org1.example.com');
const keyPath = path.join(cryptoPath, 'users', 'User1@org1.example.com', 'msp', 'keystore', 'priv_sk');
const certPath = path.join(cryptoPath, 'users', 'User1@org1.example.com', 'msp', 'signcerts', 'User1@org1.example.com-cert.pem');
const tlsCertPath = path.join(cryptoPath, 'peers', 'peer0.org1.example.com', 'tls', 'ca.crt');

const peerEndpoint = 'localhost:7051';
const peerHostAlias = 'peer0.org1.example.com';
const channelName = 'mychannel';
const chaincodeName = 'basic';
const { settledState, orderingDelta } = require('./chainstate');

async function main() {
    const scaleStr = process.argv[2] || '1000';
    const totalTxs = parseInt(scaleStr, 10);
    const concurrency = parseInt(process.argv[3] || '100', 10);
    const timeoutSec = parseInt(process.argv[4] || String(Math.max(300, Math.ceil(totalTxs / 50))), 10);

    const client = new grpc.Client(peerEndpoint, grpc.credentials.createSsl(fs.readFileSync(tlsCertPath)), {
        'grpc.ssl_target_name_override': peerHostAlias,
    });
    const gateway = connect({
        client,
        identity: { mspId: 'Org1MSP', credentials: fs.readFileSync(certPath) },
        signer: signers.newPrivateKeySigner(crypto.createPrivateKey(fs.readFileSync(keyPath))),
        // Commit waits can legitimately be long while a backlog drains.
        commitStatusOptions: () => ({ deadline: Date.now() + timeoutSec * 1000 }),
    });
    const network = gateway.getNetwork(channelName);
    const contract = network.getContract(chaincodeName);

    const before = await settledState(network);

    console.log(`Starting benchmark on ${before.consensus}: ${totalTxs} txs, concurrency ${concurrency}, timeout ${timeoutSec}s`);
    const runId = Date.now();
    const start = Date.now();
    let stopped = false;
    let next = 0;
    let submitted = 0;
    let committed = 0;
    let invalid = 0;
    let errors = 0;
    let lastCommit = start;
    const latencies = [];
    const errorSamples = new Map();

    const noteError = (msg) => {
        errors++;
        const key = String(msg).split('\n')[0].slice(0, 160);
        errorSamples.set(key, (errorSamples.get(key) || 0) + 1);
    };

    const worker = async () => {
        while (!stopped && next < totalTxs) {
            const i = next++;
            const t0 = Date.now();
            try {
                const proposal = contract.newProposal('CreateAsset', {
                    arguments: [`bench_${runId}_${i}`, 'color', '5', 'owner', '100'],
                });
                const transaction = await proposal.endorse();
                const commit = await transaction.submit();
                submitted++;
                const status = await commit.getStatus();
                if (status.successful) {
                    committed++;
                    lastCommit = Date.now();
                    latencies.push(lastCommit - t0);
                } else {
                    invalid++;
                    noteError(`invalid: validation code ${status.code}`);
                }
            } catch (e) {
                if (!stopped) noteError(e.message);
            }
        }
    };

    const progress = setInterval(() => {
        const s = ((Date.now() - start) / 1000).toFixed(0);
        console.log(`  t=${s}s submitted=${submitted} committed=${committed} invalid=${invalid} errors=${errors}`);
    }, 10000);

    let timedOut = false;
    await Promise.race([
        Promise.all(Array.from({ length: concurrency }, worker)),
        new Promise((resolve) => setTimeout(() => { timedOut = true; resolve(); }, timeoutSec * 1000)),
    ]);
    stopped = true;
    clearInterval(progress);
    const end = Date.now();

    const after = await settledState(network);
    const ord = orderingDelta(before, after);
    const { blocks } = ord;

    const durationSec = (end - start) / 1000;
    latencies.sort((a, b) => a - b);
    const pct = (q) => (latencies.length ? latencies[Math.min(latencies.length - 1, Math.floor(latencies.length * q))] : 0);

    const row = {
        workload: 'CreateAsset',
        consensus: ord.consensus,
        build: process.env.BUILD_TAG || null,
        scale: scaleStr,
        concurrency,
        submitted,
        totalTxs: committed,
        invalid,
        errors,
        complete: committed === totalTxs,
        durationSec,
        e2eTps: committed / durationSec,
        // Throughput over the busy period only (first submit -> last commit).
        ordTps: committed / Math.max(0.001, (lastCommit - start) / 1000),
        blocks,
        txPerBlock: blocks > 0 ? committed / blocks : 0,
        rounds: ord.rounds,
        emptyRounds: ord.emptyRounds,
        tentativeRounds: ord.tentativeRounds,
        wastedPct: ord.wastedPct,
        ordererHeights: ord.ordererHeights,
        p50: pct(0.5),
        p95: pct(0.95),
        stalls: timedOut ? 'Incomplete' : 0,
        timestamp: new Date().toISOString(),
    };

    console.log(`Finished in ${durationSec.toFixed(1)}s: committed ${committed}/${totalTxs}, invalid ${invalid}, errors ${errors}`);
    const roundInfo = ord.rounds === null ? 'no BA* rounds (Raft)' : `${ord.rounds} BA* rounds, ${ord.emptyRounds} empty (${ord.wastedPct.toFixed(1)}% wasted)`;
    console.log(`  e2e ${row.e2eTps.toFixed(1)} tps | ${blocks} blocks (${row.txPerBlock.toFixed(0)} tx/block) | ${roundInfo} | p50 ${row.p50}ms p95 ${row.p95}ms`);
    if (ord.ordererHeights) console.log(`  orderer heights after run: ${JSON.stringify(ord.ordererHeights)}`);
    for (const [msg, n] of errorSamples) console.log(`  error x${n}: ${msg}`);

    const resFile = process.env.RESULTS_FILE || path.join(__dirname, 'results.json');
    let past = [];
    try { past = JSON.parse(fs.readFileSync(resFile)); } catch { /* first run */ }
    past.push(row);
    fs.writeFileSync(resFile, JSON.stringify(past, null, 2));

    gateway.close();
    client.close();
    process.exit(0);
}

main().catch((e) => { console.error(e); process.exit(1); });
