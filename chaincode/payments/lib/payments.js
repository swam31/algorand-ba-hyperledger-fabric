'use strict';

// Payments between dollar accounts.
//
// Balances are whole dollars stored under "acct:<id>", so one range scan lists
// every account for the supply audit without touching other chaincodes' keys.
//
// Each account belongs to the client identity that opened it, and only that
// identity may spend from it.
//
// Double spending is prevented by Fabric itself, not by this code: Transfer
// reads the sender's balance, and if another transaction changes that balance
// before this one is committed, the peer marks this one MVCC_READ_CONFLICT and
// applies none of it.

const stringify = require('json-stringify-deterministic');
const sortKeysRecursive = require('sort-keys-recursive');
const { Contract } = require('fabric-contract-api');

const key = (id) => `acct:${id}`;

function wholeDollars(value, name) {
    const n = Number(value);
    if (!Number.isInteger(n) || n <= 0 || String(n) !== String(value)) {
        throw new Error(`${name} must be a positive whole number of dollars, got "${value}"`);
    }
    return n;
}

class Payments extends Contract {

    // Opens accounts <prefix>0000, <prefix>0001, ... for n in [start, start+count),
    // each holding `dollars`. Refuses to reopen an existing account, so the total
    // supply is exactly what was deliberately issued.
    async OpenAccounts(ctx, prefix, start, count, dollars) {
        const first = Number(start);
        const n = wholeDollars(count, 'count');
        const each = wholeDollars(dollars, 'dollars');
        if (!Number.isInteger(first) || first < 0) {
            throw new Error(`start must be a non-negative integer, got "${start}"`);
        }
        for (let i = first; i < first + n; i++) {
            const id = `${prefix}${String(i).padStart(4, '0')}`;
            const existing = await ctx.stub.getState(key(id));
            if (existing && existing.length > 0) {
                throw new Error(`account ${id} already exists`);
            }
            await this._put(ctx, { id, balance: each, owner: ctx.clientIdentity.getID() });
        }
        return JSON.stringify({ opened: n, each });
    }

    async Transfer(ctx, from, to, amount) {
        const dollars = wholeDollars(amount, 'amount');
        if (from === to) {
            throw new Error('cannot pay yourself');
        }
        const sender = await this._get(ctx, from);
        const receiver = await this._get(ctx, to);
        if (sender.owner !== ctx.clientIdentity.getID()) {
            throw new Error(`not your account: ${from} belongs to another identity`);
        }
        if (sender.balance < dollars) {
            throw new Error(`insufficient funds: ${from} has $${sender.balance}, tried to send $${dollars}`);
        }
        sender.balance -= dollars;
        receiver.balance += dollars;
        await this._put(ctx, sender);
        await this._put(ctx, receiver);
        return JSON.stringify({ from, to, amount: dollars, fromBalance: sender.balance, toBalance: receiver.balance });
    }

    async Balance(ctx, id) {
        return JSON.stringify(await this._get(ctx, id));
    }

    // The audit: money is only ever moved, so this must always equal what
    // OpenAccounts issued.
    async TotalSupply(ctx) {
        let accounts = 0;
        let total = 0;
        for await (const res of ctx.stub.getStateByRange('acct:', 'acct;')) {
            accounts++;
            total += JSON.parse(Buffer.from(res.value).toString('utf8')).balance;
        }
        return JSON.stringify({ accounts, total });
    }

    async AllBalances(ctx) {
        const out = [];
        for await (const res of ctx.stub.getStateByRange('acct:', 'acct;')) {
            out.push(JSON.parse(Buffer.from(res.value).toString('utf8')));
        }
        return JSON.stringify(out);
    }

    async _get(ctx, id) {
        const raw = await ctx.stub.getState(key(id));
        if (!raw || raw.length === 0) {
            throw new Error(`no such account: ${id}`);
        }
        return JSON.parse(raw.toString());
    }

    async _put(ctx, account) {
        await ctx.stub.putState(key(account.id), Buffer.from(stringify(sortKeysRecursive(account))));
    }
}

module.exports = Payments;
