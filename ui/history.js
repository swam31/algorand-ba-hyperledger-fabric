// Round and transaction history, rebuilt from the ledger itself.
//
// Every block written by the Algorand consenter carries metadata naming the
// BA* round that produced it, the proposer and the time. The gap between two
// blocks' rounds is the number of EMPTY rounds in between (a round either
// writes one block or agrees on the empty block). So streaming the chain from
// block 0 reconstructs the history of every round, and replaying the payments
// in it reconstructs every account balance - independently of the chaincode.
//
// Vote-level detail (who voted what, with how many seats) is not on the
// ledger; it is captured from the orderers' telemetry while this server runs.
const { common, peer, msp } = require('@hyperledger/fabric-protos');

const CODE_NAMES = Object.fromEntries(Object.entries(peer.TxValidationCode).map(([k, v]) => [v, k]));
const utf8 = (u8) => Buffer.from(u8).toString('utf8');

function parseMeta(s) {
    const out = {};
    for (const field of s.split(',')) {
        const [k, v] = field.split('=');
        if (k && v !== undefined) out[k.trim()] = v.trim();
    }
    return out;
}

function roundMetadata(block) {
    try {
        const md = block.getMetadata().getMetadataList_asU8();
        const sig = common.Metadata.deserializeBinary(md[common.BlockMetadataIndex.SIGNATURES]);
        const obm = common.OrdererBlockMetadata.deserializeBinary(sig.getValue_asU8());
        const cm = common.Metadata.deserializeBinary(obm.getConsenterMetadata_asU8());
        const m = parseMeta(utf8(cm.getValue_asU8()));
        if (m.round === undefined) return null;
        return {
            round: Number(m.round),
            steps: Number(m.steps || 0),
            outcome: m.outcome || 'FINAL',
            proposer: m.proposer || 'unknown',
            time: m.time ? Number(m.time) : null,
        };
    } catch {
        return null;
    }
}

function decodeTx(envBytes, code) {
    const tx = { status: CODE_NAMES[code] || `code ${code}`, valid: code === peer.TxValidationCode.VALID };
    try {
        const env = common.Envelope.deserializeBinary(envBytes);
        const payload = common.Payload.deserializeBinary(env.getPayload_asU8());
        const hdr = payload.getHeader();
        const ch = common.ChannelHeader.deserializeBinary(hdr.getChannelHeader_asU8());
        tx.id = ch.getTxId();
        const ts = ch.getTimestamp();
        if (ts) tx.time = ts.getSeconds() * 1000 + Math.floor(ts.getNanos() / 1e6);
        try {
            const sh = common.SignatureHeader.deserializeBinary(hdr.getSignatureHeader_asU8());
            tx.creator = msp.SerializedIdentity.deserializeBinary(sh.getCreator_asU8()).getMspid();
        } catch { /* config transactions may not carry one */ }

        if (ch.getType() === common.HeaderType.CONFIG) {
            tx.type = 'config';
            return tx;
        }
        if (ch.getType() !== common.HeaderType.ENDORSER_TRANSACTION) {
            tx.type = `type ${ch.getType()}`;
            return tx;
        }
        const ptx = peer.Transaction.deserializeBinary(payload.getData_asU8());
        const action = ptx.getActionsList()[0];
        const cap = peer.ChaincodeActionPayload.deserializeBinary(action.getPayload_asU8());
        const cpp = peer.ChaincodeProposalPayload.deserializeBinary(cap.getChaincodeProposalPayload_asU8());
        const spec = peer.ChaincodeInvocationSpec.deserializeBinary(cpp.getInput_asU8()).getChaincodeSpec();
        tx.chaincode = spec.getChaincodeId().getName();
        const args = spec.getInput().getArgsList_asU8().map(utf8);
        tx.fn = args[0];
        tx.args = args.slice(1);
        if (tx.chaincode === 'payments' && tx.fn === 'Transfer') {
            tx.type = 'transfer';
            tx.from = args[1];
            tx.to = args[2];
            tx.amount = Number(args[3]);
        } else if (tx.chaincode === 'payments' && tx.fn === 'OpenAccounts') {
            tx.type = 'open-accounts';
        } else {
            tx.type = tx.chaincode === '_lifecycle' ? 'lifecycle' : 'invoke';
        }
    } catch (e) {
        tx.type = 'undecodable';
        tx.error = e.message;
    }
    return tx;
}

function decodeBlock(block) {
    const md = block.getMetadata().getMetadataList_asU8();
    const filter = md[common.BlockMetadataIndex.TRANSACTIONS_FILTER] || new Uint8Array();
    const txs = block.getData().getDataList_asU8().map((env, i) => decodeTx(env, filter[i]));
    return { number: Number(block.getHeader().getNumber()), meta: roundMetadata(block), txs };
}

class History {
    constructor() {
        this.reset();
        this.votes = new Map();  // round -> { node -> { step -> {value, seats} } }
        this.events = new Map(); // round -> { node -> {outcome, durationMs, txCount, timestamp} }
    }

    reset() {
        this.blocks = [];          // index = block number
        this.rounds = new Map();   // round -> record
        this.lastRound = null;
        this.txIndex = new Map();  // txId -> {block, index}
        this.accounts = new Map(); // id -> {id, balance, owner, txs: [txId...]}
        this.issued = 0;
        this.counts = { transfers: 0, valid: 0, invalid: 0 };
    }

    addBlock(block) {
        const b = decodeBlock(block);
        if (b.number === 0) {
            // A stream always starts from genesis, so rebuild. If genesis
            // itself changed, the network was recreated and round numbers
            // restart: the live vote data belongs to the old chain.
            const g = Buffer.from(block.getHeader().getDataHash_asU8()).toString('hex');
            if (this.genesis && this.genesis !== g) {
                this.votes.clear();
                this.events.clear();
            }
            this.genesis = g;
            this.reset();
        }
        if (b.number !== this.blocks.length) return; // duplicate or out of order: ignore
        this.blocks.push(b);
        b.txs.forEach((tx, i) => {
            if (tx.id) this.txIndex.set(tx.id, { block: b.number, index: i });
            this.applyPayment(tx, b);
        });

        if (!b.meta) return; // genesis, or a block from before round metadata existed
        const r = b.meta.round;
        if (this.lastRound !== null) {
            for (let e = this.lastRound + 1; e < r; e++) {
                this.rounds.set(e, { round: e, outcome: 'EMPTY', block: null, txCount: 0 });
            }
        }
        const transfers = b.txs.filter((t) => t.type === 'transfer');
        this.rounds.set(r, {
            round: r,
            outcome: b.meta.outcome,
            steps: b.meta.steps,
            proposer: b.meta.proposer,
            time: b.meta.time,
            block: b.number,
            txCount: b.txs.length,
            valid: b.txs.filter((t) => t.valid).length,
            invalid: b.txs.filter((t) => !t.valid).length,
            transferred: transfers.filter((t) => t.valid).reduce((s, t) => s + t.amount, 0),
        });
        this.lastRound = r;
    }

    applyPayment(tx, b) {
        if (tx.chaincode !== 'payments') return;
        const touch = (id) => {
            if (!this.accounts.has(id)) this.accounts.set(id, { id, balance: 0, txs: [] });
            return this.accounts.get(id);
        };
        if (tx.type === 'transfer') {
            this.counts.transfers++;
            this.counts[tx.valid ? 'valid' : 'invalid']++;
            touch(tx.from).txs.push(tx.id);
            touch(tx.to).txs.push(tx.id);
            if (tx.valid) {
                touch(tx.from).balance -= tx.amount;
                touch(tx.to).balance += tx.amount;
            }
        } else if (tx.type === 'open-accounts' && tx.valid) {
            const [prefix, start, count, dollars] = [tx.args[0], Number(tx.args[1]), Number(tx.args[2]), Number(tx.args[3])];
            for (let i = start; i < start + count; i++) {
                const a = touch(`${prefix}${String(i).padStart(4, '0')}`);
                a.balance += dollars;
                a.owner = tx.creator;
                this.issued += dollars;
            }
        }
    }

    // Live telemetry, kept for every round seen while the server runs.
    addVote(v) {
        if (v.dir !== 'sent') return;
        if (!this.votes.has(v.round)) this.votes.set(v.round, {});
        const byNode = this.votes.get(v.round);
        (byNode[v.nodeId] = byNode[v.nodeId] || {})[v.step] = { value: v.value, seats: v.seats };
        if (this.votes.size > 20000) this.votes.delete(this.votes.keys().next().value);
    }

    addRoundEvent(e) {
        if (!this.events.has(e.round)) this.events.set(e.round, {});
        this.events.get(e.round)[e.nodeId] = { outcome: e.outcome, durationMs: e.durationMs, txCount: e.txCount, timestamp: e.timestamp };
        if (this.events.size > 20000) this.events.delete(this.events.keys().next().value);
    }

    listRounds({ before, limit = 50, hideEmpty = false }) {
        const out = [];
        const top = before !== undefined ? before - 1 : this.lastRound;
        for (let r = top; r !== null && r >= 1 && out.length < limit; r--) {
            const rec = this.rounds.get(r);
            if (!rec || (hideEmpty && rec.outcome === 'EMPTY')) continue;
            const ev = this.events.get(r);
            const durations = ev ? Object.values(ev).map((x) => x.durationMs) : [];
            out.push({ ...rec, durationMs: durations.length ? Math.max(...durations) : null, votesCaptured: this.votes.has(r) });
        }
        return { rounds: out, lastRound: this.lastRound, height: this.blocks.length, counts: this.counts };
    }

    roundDetail(r) {
        const rec = this.rounds.get(r);
        if (!rec) return null;
        const block = rec.block !== null ? this.blocks[rec.block] : null;
        return { ...rec, txs: block ? block.txs : [], votes: this.votes.get(r) || null, nodes: this.events.get(r) || null };
    }

    tx(id) {
        const where = this.txIndex.get(id);
        if (!where) return null;
        const b = this.blocks[where.block];
        return { ...b.txs[where.index], block: b.number, round: b.meta ? b.meta.round : null };
    }

    accountList() {
        const list = [...this.accounts.values()].map(({ id, balance, owner, txs }) => ({ id, balance, owner, txCount: txs.length }));
        list.sort((a, b) => b.balance - a.balance);
        const supply = list.reduce((s, a) => s + a.balance, 0);
        return { accounts: list, supply, issued: this.issued, conserved: supply === this.issued, counts: this.counts };
    }

    account(id) {
        const a = this.accounts.get(id);
        if (!a) return null;
        return { id, balance: a.balance, owner: a.owner, txs: a.txs.slice(-200).reverse().map((t) => this.tx(t)) };
    }

    // Pushed to every browser several times a second, so it carries counts
    // and a few sample IDs, never whole blocks (a 500-transaction block would
    // make each push megabytes).
    recentBlocks(n = 30) {
        return this.blocks.slice(-n).reverse().map((b) => ({
            number: b.number,
            round: b.meta ? b.meta.round : null,
            txCount: b.txs.length,
            valid: b.txs.filter((t) => t.valid).length,
            txs: b.txs.slice(0, 6).map((t) => ({ id: t.id, valid: t.valid, status: t.status })),
        }));
    }

    // The newest blocks with who paid whom, for the Transaction Flow panel,
    // oldest first. Like recentBlocks it is pushed several times a second, so
    // each block carries only its first few transactions.
    activity(n = 10, perBlock = 12) {
        const out = [];
        for (let i = this.blocks.length - 1; i >= 1 && out.length < n; i--) {
            const b = this.blocks[i];
            const prev = this.blocks[i - 1];
            const m = b.meta || {};
            out.push({
                number: b.number,
                round: b.meta ? m.round : null,
                emptyBefore: b.meta && prev.meta ? m.round - prev.meta.round - 1 : 0,
                proposer: m.proposer || null,
                outcome: m.outcome || null,
                steps: m.steps ?? null,
                time: m.time ?? null,
                txCount: b.txs.length,
                valid: b.txs.filter((t) => t.valid).length,
                txs: b.txs.slice(0, perBlock).map((t) => ({
                    id: t.id, type: t.type, chaincode: t.chaincode, fn: t.fn,
                    from: t.from, to: t.to, amount: t.amount,
                    valid: t.valid, status: t.status, creator: t.creator, time: t.time,
                })),
            });
        }
        return out.reverse();
    }
}

// Streams the whole chain from block 0 and keeps following it. After a
// network restart the stream fails; reconnecting from 0 rebuilds history for
// the new chain. connect() is called afresh on every attempt because a rebuilt
// network has new certificates, so a connection made with the old ones can
// never succeed again.
async function follow(connect, history, onBlock) {
    for (;;) {
        let conn = null;
        try {
            conn = connect();
            const events = await conn.network.getBlockEvents({ startBlock: BigInt(0) });
            try {
                for await (const block of events) {
                    history.addBlock(block);
                    onBlock();
                }
            } finally {
                events.close();
            }
        } catch (e) {
            console.log(`Block stream interrupted (${e.message.split('\n')[0].slice(0, 120)}); reconnecting in 3s`);
        } finally {
            if (conn) conn.close();
        }
        await new Promise((r) => setTimeout(r, 3000));
    }
}

module.exports = { History, follow };
