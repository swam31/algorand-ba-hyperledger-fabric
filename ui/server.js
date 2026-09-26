const express = require('express');
const http = require('http');
const WebSocket = require('ws');
const path = require('path');
const fs = require('fs');
const grpc = require('@grpc/grpc-js');
const { connect, signers } = require('@hyperledger/fabric-gateway');
const fetch = require('node-fetch');
const { History, follow } = require('./history');

// ---------------------------------------------------------
// CONFIGURATION
// ---------------------------------------------------------
const PORT = process.env.PORT || 3001;
const ORDERERS = [
    'http://127.0.0.1:7070',
    'http://127.0.0.1:7071',
    'http://127.0.0.1:7072',
    'http://127.0.0.1:7073',
];

// Test network crypto paths
const cryptoPath = path.resolve(__dirname, '../../teluskohf/fabric-samples/test-network-algorand/organizations/peerOrganizations/org1.example.com');
const keyPath = path.join(cryptoPath, 'users', 'User1@org1.example.com', 'msp', 'keystore', 'priv_sk');
const certPath = path.join(cryptoPath, 'users', 'User1@org1.example.com', 'msp', 'signcerts', 'User1@org1.example.com-cert.pem');
const tlsCertPath = path.join(cryptoPath, 'peers', 'peer0.org1.example.com', 'tls', 'ca.crt');
const peerEndpoint = 'localhost:7051';
const peerHostAlias = 'peer0.org1.example.com';
const channelName = 'mychannel';

// ---------------------------------------------------------
// STATE
// ---------------------------------------------------------
const state = {
    orderers: {},
    rounds: [],
    blocks: [],
    votes: [],
};

// ---------------------------------------------------------
// EXPRESS + WS SERVER
// ---------------------------------------------------------
const app = express();
app.use(express.static(path.join(__dirname, 'public')));

// ---------------------------------------------------------
// HISTORY API (rebuilt from the ledger; see history.js)
// ---------------------------------------------------------
const history = new History();
const benchFile = (name) => path.join(__dirname, 'bench', name);
const readJSON = (file, fallback) => {
    try { return JSON.parse(fs.readFileSync(file)); } catch (e) { return fallback; }
};

app.get('/api/rounds', (req, res) => {
    res.json(history.listRounds({
        before: req.query.before !== undefined ? Number(req.query.before) : undefined,
        limit: Math.min(500, Number(req.query.limit) || 50),
        hideEmpty: req.query.hideEmpty === '1',
    }));
});
app.get('/api/rounds/:round', (req, res) => {
    const d = history.roundDetail(Number(req.params.round));
    return d ? res.json(d) : res.status(404).json({ error: 'no such round' });
});
app.get('/api/tx/:id', (req, res) => {
    const t = history.tx(req.params.id);
    return t ? res.json(t) : res.status(404).json({ error: 'transaction not on the ledger (refused before ordering, or unknown)' });
});
app.get('/api/accounts', (req, res) => res.json(history.accountList()));
app.get('/api/accounts/:id', (req, res) => {
    const a = history.account(req.params.id);
    return a ? res.json(a) : res.status(404).json({ error: 'no such account' });
});
// Transfers refused before ordering never reach the ledger; the payments
// load generator's log of its last run is the only record of them.
app.get('/api/refused', (req, res) => {
    const run = readJSON(benchFile('payments_last_run.json'), null);
    if (!run) return res.json({ summary: null, refused: [] });
    res.json({ summary: run.summary, refused: run.transfers.filter((t) => t.outcome.startsWith('refused')) });
});
app.get('/api/crash', (req, res) => res.json(readJSON(benchFile('crash_results.json'), [])));
app.get('/api/badnet', (req, res) => res.json(readJSON(benchFile('badnet_results.json'), [])));
// Runs from earlier builds, kept to show what each change bought.
app.get('/api/bench/archive', (req, res) => res.json([
    ...readJSON(benchFile('results_before_fast_start.json'), []),
    ...readJSON(benchFile('results_intermediate_builds.json'), []),
]));
app.get('/api/scale', (req, res) => res.json(readJSON(benchFile('scale_results.json'), [])));
app.get('/api/scale/:nodes', (req, res) => {
    const name = `scale_history_${req.params.nodes.replace(/[^0-9a-z_]/gi, '')}.json`;
    const d = readJSON(benchFile(name), null);
    return d ? res.json(d) : res.status(404).json({ error: 'no simulation run for that size' });
});

app.get('/benchmarks', (req, res) => {
    try {
        const data = fs.readFileSync(path.join(__dirname, 'bench', 'results.json'));
        res.json(JSON.parse(data));
    } catch(e) {
        res.json([]);
    }
});

const server = http.createServer(app);
const wss = new WebSocket.Server({ server });

wss.on('connection', ws => {
    ws.send(JSON.stringify(state));
});

let broadcastTimeout = null;
function broadcastState() {
    if (broadcastTimeout) return;
    broadcastTimeout = setTimeout(() => {
        const data = JSON.stringify(state);
        wss.clients.forEach(client => {
            if (client.readyState === WebSocket.OPEN) {
                client.send(data);
            }
        });
        broadcastTimeout = null;
    }, 250);
}

// ---------------------------------------------------------
// ORDERER POLLING (Status + Events)
// ---------------------------------------------------------
async function pollOrderer(url, id) {
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), 2000);
    try {
        const res = await fetch(`${url}/status`, { signal: controller.signal });
        if (res.ok) {
            const data = await res.json();
            // Uptime going backwards means the orderer process restarted:
            // its streams now belong to a new process, so reconnect them.
            const prev = state.orderers[id];
            if (prev && prev.up && data.uptimeSeconds < prev.uptimeSeconds) {
                streams[id].forEach((s) => s.restart());
            }
            state.orderers[id] = { ...data, up: true };
        } else {
            state.orderers[id] = { up: false, nodeId: `orderer${id+1}` };
        }
    } catch (err) {
        state.orderers[id] = { up: false, nodeId: `orderer${id+1}` };
    } finally {
        clearTimeout(timeoutId);
    }
    broadcastState();
}

for (let i = 0; i < ORDERERS.length; i++) {
    setInterval(() => pollOrderer(ORDERERS[i], i), 1000);
}

// Listen to orderer SSE streams for rounds and votes.
//
// Each orderer replays its recent history whenever a client connects, so on
// every (re)connect we first drop what we hold from that orderer and let the
// replay refill it. That keeps a restarted network from leaving the previous
// network's rounds on screen. A stream that goes quiet is reconnected too,
// because a container that is removed does not always close its TCP stream.
const STREAM_IDLE_MS = 20000;
const VOTE_ROUNDS_KEPT = 30;
const streams = ORDERERS.map(() => []);

function openStream(i, route, onEvent, purge) {
    const url = ORDERERS[i];
    let controller = null;
    let lastData = 0;
    let buffered = '';

    const connect = () => {
        controller = new AbortController();
        const mine = controller;
        purge();
        lastData = Date.now();
        buffered = '';
        fetch(`${url}${route}`, { signal: mine.signal }).then((res) => {
            res.body.on('data', (chunk) => {
                lastData = Date.now();
                buffered += chunk.toString();
                const lines = buffered.split('\n');
                buffered = lines.pop(); // keep a partial line for the next chunk
                for (const line of lines) {
                    if (!line.startsWith('data: ')) continue;
                    try { onEvent(JSON.parse(line.slice(6))); } catch (e) {}
                }
                broadcastState();
            });
            const retry = () => { if (controller === mine) setTimeout(connect, 2000); };
            res.body.on('end', retry);
            res.body.on('error', retry);
        }).catch(() => { if (controller === mine) setTimeout(connect, 2000); });
    };

    const restart = () => {
        if (controller) controller.abort();
        connect();
    };
    setInterval(() => { if (Date.now() - lastData > STREAM_IDLE_MS) restart(); }, 5000);
    connect();
    return { restart };
}

ORDERERS.forEach((url, i) => {
    const nodeId = `orderer${i + 1}`;
    streams[i].push(openStream(i, '/events', (data) => {
        data.nodeId = nodeId;
        history.addRoundEvent(data);
        state.rounds.unshift(data);
        // Kept long enough for the flow view to replay as many rounds as the
        // vote buffer below holds.
        if (state.rounds.length > 400) state.rounds.pop();
    }, () => {
        state.rounds = state.rounds.filter((r) => r.nodeId !== nodeId);
    }));
    streams[i].push(openStream(i, '/votes', (data) => {
        history.addVote(data);
        state.votes.push(data);
        // Trim by whole rounds, not by count: a count cap applied while four
        // history replays interleave can keep one orderer's votes and drop
        // another's, which the flow view would then draw as lost messages.
        if (state.votes.length > 4000) {
            let newest = 0;
            for (const v of state.votes) if (v.round > newest) newest = v.round;
            state.votes = state.votes.filter((v) => v.round > newest - VOTE_ROUNDS_KEPT);
        }
    }, () => {
        state.votes = state.votes.filter((v) => v.nodeId !== nodeId);
    }));
});

// ---------------------------------------------------------
// FABRIC GATEWAY
// ---------------------------------------------------------
async function startBlockListener() {
    // Read the certificates on every connection attempt: the test network
    // regenerates them each time it is rebuilt.
    const connectPeer = () => {
        const client = new grpc.Client(peerEndpoint, grpc.credentials.createSsl(fs.readFileSync(tlsCertPath)), {
            'grpc.ssl_target_name_override': peerHostAlias,
        });
        const gateway = connect({
            client,
            identity: { mspId: 'Org1MSP', credentials: fs.readFileSync(certPath) },
            signer: signers.newPrivateKeySigner(require('crypto').createPrivateKey(fs.readFileSync(keyPath))),
        });
        return {
            network: gateway.getNetwork(channelName),
            close: () => { gateway.close(); client.close(); },
        };
    };
    follow(connectPeer, history, () => {
        state.blocks = history.recentBlocks(30);
        broadcastState();
    });
}

startBlockListener();

server.listen(PORT, () => {
    console.log(`Dashboard running on http://localhost:${PORT}`);
});
