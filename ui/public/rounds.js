// Round History and Accounts & Payments panels. Data comes from the server's
// /api routes, which rebuild everything from the ledger (see ui/history.js).
(() => {
    const $ = (id) => document.getElementById(id);
    const esc = (s) => String(s ?? '').replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
    const money = (n) => `$${Number(n).toLocaleString()}`;
    const time = (ms) => (ms ? new Date(ms).toLocaleTimeString() : '-');
    const short = (id) => (id ? `${id.slice(0, 10)}…` : '-');
    // The server may be restarting; a failed poll just waits for the next.
    const getJSON = async (url) => {
        try {
            const res = await fetch(url);
            return res.ok ? res.json() : null;
        } catch {
            return null;
        }
    };
    const statusCell = (t) => `<span class="${t.valid ? 'tx-valid' : 'tx-invalid'}">${esc(t.status)}</span>`;
    const NODES = ['orderer1', 'orderer2', 'orderer3', 'orderer4'];
    const stepRank = (s) => (s === 'reduction-1' ? 1 : s === 'reduction-2' ? 2 : s === 'FINAL' ? 1e6 : 2 + parseInt(s.split('-')[1], 10));
    const stepLabel = (s) => (s === 'reduction-1' ? 'Reduction 1' : s === 'reduction-2' ? 'Reduction 2' : s === 'FINAL' ? 'FINAL' : `BinaryBA* ${s.split('-')[1]}`);
    const valueColor = (v) => {
        if (!v || v === 'empty') return '#64748b';
        let hue = 0;
        for (let i = 0; i < v.length; i++) hue = (hue * 31 + v.charCodeAt(i)) % 360;
        return `hsl(${hue}, 75%, 60%)`;
    };

    // ---------------------------------------------------------- rounds list
    let pageTop = null;       // null = follow the newest round
    const older = [];         // stack of page tops for "Newer"
    let lastShown = null;
    let selectedRound = null;

    async function loadRounds() {
        const q = new URLSearchParams({ limit: '50' });
        if (pageTop !== null) q.set('before', String(pageTop + 1));
        if ($('hist-hide-empty').checked) q.set('hideEmpty', '1');
        const d = await getJSON(`/api/rounds?${q}`);
        if (!d) return;
        $('hist-status').textContent = `${d.lastRound ?? 0} rounds · ${d.height} blocks · ${d.counts.transfers} transfers (${d.counts.valid} valid, ${d.counts.invalid} invalid)`;
        $('hist-body').innerHTML = d.rounds.map((r) => `
            <tr class="clickable${r.round === selectedRound ? ' selected' : ''}" data-round="${r.round}">
                <td>${r.round}</td>
                <td class="outcome-${r.outcome.toLowerCase()}">${r.outcome}</td>
                <td>${r.block ?? '-'}</td>
                <td>${esc(r.proposer ?? '-')}</td>
                <td>${r.block === null ? '-' : `<span class="tx-valid">${r.valid}</span>${r.invalid ? ` / <span class="tx-invalid">${r.invalid} invalid</span>` : ''}`}</td>
                <td>${r.transferred ? money(r.transferred) : '-'}</td>
                <td>${r.steps ?? '-'}</td>
                <td>${time(r.time)}</td>
            </tr>`).join('') || '<tr><td colspan="8" class="hint">No rounds yet. Submit a transaction to start one.</td></tr>';
        lastShown = d.rounds.length ? d.rounds[d.rounds.length - 1].round : null;
        $('hist-newer').disabled = pageTop === null;
        $('hist-older').disabled = lastShown === null || lastShown <= 1;
    }

    $('hist-body').addEventListener('click', (e) => {
        const tr = e.target.closest('tr[data-round]');
        if (tr) showRound(Number(tr.dataset.round));
    });
    $('hist-older').addEventListener('click', () => {
        older.push(pageTop);
        pageTop = lastShown - 1;
        loadRounds();
    });
    $('hist-newer').addEventListener('click', () => {
        pageTop = older.length ? older.pop() : null;
        loadRounds();
    });
    $('hist-hide-empty').addEventListener('change', () => { pageTop = null; older.length = 0; loadRounds(); });

    // ---------------------------------------------------------- round detail
    function votesTable(votes) {
        if (!votes) return '<p class="hint">Vote details were not captured for this round: the dashboard only records votes while it is running.</p>';
        const steps = [...new Set(Object.values(votes).flatMap((byStep) => Object.keys(byStep)))].sort((a, b) => stepRank(a) - stepRank(b));
        return `<table><thead><tr><th>Step</th>${NODES.map((n) => `<th>${n}</th>`).join('')}</tr></thead><tbody>${steps.map((s) => `
            <tr><td>${stepLabel(s)}</td>${NODES.map((n) => {
                const c = votes[n] && votes[n][s];
                return c ? `<td><span class="chip" style="background:${valueColor(c.value)}"></span><span class="mono">${c.value === 'empty' ? 'empty block' : esc(c.value)}</span> <small>${c.seats} seats</small></td>` : '<td class="hint">no vote</td>';
            }).join('')}</tr>`).join('')}</tbody></table>`;
    }

    function txRow(t, i, highlight) {
        const what = t.type === 'transfer'
            ? `<span class="mono">${esc(t.from)}</span> → <span class="mono">${esc(t.to)}</span>`
            : `${esc(t.type)}${t.fn ? `: ${esc(t.fn)}(${esc((t.args || []).join(', ')).slice(0, 80)})` : ''}`;
        return `<tr${t.id === highlight ? ' class="selected"' : ''}>
            <td>${i + 1}</td>
            <td class="mono" title="${esc(t.id)}">${short(t.id)}</td>
            <td>${what}</td>
            <td>${t.amount !== undefined ? money(t.amount) : '-'}</td>
            <td>${statusCell(t)}</td>
            <td>${esc(t.creator ?? '-')}</td>
        </tr>`;
    }

    async function showRound(r, highlightTx) {
        selectedRound = r;
        const d = await getJSON(`/api/rounds/${r}`);
        if (!d) {
            $('hist-detail').innerHTML = `<div class="detail">Round ${r} is not on this chain yet.</div>`;
            return;
        }
        document.querySelectorAll('#hist-body tr').forEach((tr) => tr.classList.toggle('selected', Number(tr.dataset.round) === r));
        const head = d.outcome === 'EMPTY'
            ? `<p>No block was written. Either no orderer won proposer sortition, or the proposal reached too few orderers inside the proposal window (250 ms on this network), so BA* agreed on the empty block. Nothing is lost: the transactions are proposed again next round.</p>`
            : `<p>Block <b>${d.block}</b>, proposed by <b>${esc(d.proposer)}</b>, reached <b class="outcome-${d.outcome.toLowerCase()}">${d.outcome}</b> after ${d.steps} BinaryBA* step${d.steps === 1 ? '' : 's'} at ${time(d.time)}.
               ${d.valid} transaction${d.valid === 1 ? '' : 's'} valid${d.invalid ? `, <span class="tx-invalid">${d.invalid} invalidated at commit</span> (MVCC_READ_CONFLICT means another transaction in the chain changed the same account first; this is how Fabric stops a double spend)` : ''}.</p>`;
        const nodes = d.nodes ? `<p class="hint">Per orderer: ${NODES.filter((n) => d.nodes[n]).map((n) => `${n} ${d.nodes[n].outcome} in ${d.nodes[n].durationMs} ms`).join(' · ')}</p>` : '';
        $('hist-detail').innerHTML = `<div class="detail">
            <h3>Round ${d.round} · <span class="outcome-${d.outcome.toLowerCase()}">${d.outcome}</span></h3>
            ${head}${nodes}
            <h4>Votes</h4>${votesTable(d.votes)}
            ${d.txs.length ? `<h4>Transactions (${d.txs.length})</h4>
            <div class="table-container scroll-box"><table>
                <thead><tr><th>#</th><th>Tx ID</th><th>What</th><th>Amount</th><th>Status</th><th>Signed by</th></tr></thead>
                <tbody>${d.txs.map((t, i) => txRow(t, i, highlightTx)).join('')}</tbody>
            </table></div>` : ''}
        </div>`;
        if (highlightTx) $('hist-detail').scrollIntoView({ behavior: 'smooth', block: 'start' });
    }
    window.showRound = showRound; // the Transaction Flow feed links here

    // ---------------------------------------------------------- search
    let refusedCache = [];
    $('hist-search').addEventListener('keydown', async (e) => {
        if (e.key !== 'Enter') return;
        const q = e.target.value.trim();
        if (!q) return;
        if (/^\d+$/.test(q)) return showRound(Number(q));
        if (/^acct\d+$/i.test(q)) return showAccount(q.toLowerCase());
        const t = await getJSON(`/api/tx/${encodeURIComponent(q)}`);
        if (t && t.round !== null) return showRound(t.round, t.id);
        const refused = refusedCache.find((r) => r.txId === q);
        $('hist-detail').innerHTML = `<div class="detail">${refused
            ? `Transaction <span class="mono">${esc(q)}</span> never reached the ledger: <span class="tx-invalid">${esc(refused.outcome)}</span> (${esc(refused.kind)}, ${esc(refused.from)} → ${esc(refused.to)}, ${money(refused.amount)}).`
            : `No round, account or transaction matches <span class="mono">${esc(q)}</span>.`}</div>`;
    });

    // ---------------------------------------------------------- accounts
    let selectedAccount = null;

    async function loadAccounts() {
        const d = await getJSON('/api/accounts');
        if (!d) return;
        if (!d.accounts.length) {
            $('acct-summary').innerHTML = '<span class="hint">No payment accounts on this chain yet. Run <span class="mono">node paygen.js 1000</span> in ui/bench to open 1,000 accounts with $1,000 each.</span>';
            return;
        }
        $('acct-summary').innerHTML = `${d.accounts.length.toLocaleString()} accounts · money supply ${money(d.supply)} of ${money(d.issued)} issued
            <span class="${d.conserved ? 'tx-valid' : 'tx-invalid'}">${d.conserved ? '✓ conserved' : '✗ NOT conserved'}</span>
            · ${d.counts.transfers.toLocaleString()} transfers on the ledger (<span class="tx-valid">${d.counts.valid.toLocaleString()} valid</span>, <span class="tx-invalid">${d.counts.invalid.toLocaleString()} invalidated</span>)`;
        $('acct-body').innerHTML = d.accounts.map((a) => `
            <tr class="clickable${a.id === selectedAccount ? ' selected' : ''}" data-acct="${esc(a.id)}">
                <td class="mono">${esc(a.id)}</td><td>${money(a.balance)}</td><td>${a.txCount}</td>
            </tr>`).join('');
    }

    $('acct-body').addEventListener('click', (e) => {
        const tr = e.target.closest('tr[data-acct]');
        if (tr) showAccount(tr.dataset.acct);
    });

    async function showAccount(id) {
        selectedAccount = id;
        const a = await getJSON(`/api/accounts/${encodeURIComponent(id)}`);
        if (!a) {
            $('acct-detail').innerHTML = `<div class="detail">No account ${esc(id)} on this chain.</div>`;
            return;
        }
        document.querySelectorAll('#acct-body tr').forEach((tr) => tr.classList.toggle('selected', tr.dataset.acct === id));
        $('acct-detail').innerHTML = `<div class="detail">
            <h3><span class="mono">${esc(a.id)}</span> · ${money(a.balance)}</h3>
            <p class="hint">Opened with $1,000 by ${esc(a.owner ?? 'unknown')}. Most recent ${a.txs.length} transfers; click a round to open it.</p>
            <div class="table-container scroll-box"><table>
                <thead><tr><th>Round</th><th>Tx ID</th><th>Direction</th><th>Counterparty</th><th>Amount</th><th>Status</th></tr></thead>
                <tbody>${a.txs.map((t) => {
                    const out = t.from === a.id;
                    return `<tr class="clickable" data-round="${t.round}" data-tx="${esc(t.id)}">
                        <td>${t.round}</td><td class="mono">${short(t.id)}</td>
                        <td class="${out ? 'tx-invalid' : 'tx-valid'}">${out ? 'sent' : 'received'}</td>
                        <td class="mono">${esc(out ? t.to : t.from)}</td>
                        <td>${out ? '−' : '+'}${money(t.amount)}</td><td>${statusCell(t)}</td></tr>`;
                }).join('')}</tbody>
            </table></div></div>`;
        $('acct-detail').querySelector('tbody').addEventListener('click', (e) => {
            const tr = e.target.closest('tr[data-round]');
            if (tr) {
                showRound(Number(tr.dataset.round), tr.dataset.tx);
            }
        });
        $('acct-detail').scrollIntoView({ behavior: 'smooth', block: 'nearest' });
    }

    async function loadRefused() {
        const d = await getJSON('/api/refused');
        if (!d || !d.summary) {
            $('refused').innerHTML = '<p class="hint">No payments run recorded yet.</p>';
            return;
        }
        refusedCache = d.refused;
        const s = d.summary;
        const kinds = Object.entries(s.kinds || {});
        $('refused').innerHTML = `
            <table><thead><tr><th>Kind</th><th>Sent</th><th>Committed</th><th>Invalidated</th><th>Refused</th></tr></thead><tbody>
            ${kinds.map(([k, v]) => `<tr><td>${esc(k)}</td><td>${v.sent}</td><td class="tx-valid">${v.committed}</td><td class="${v.invalid ? 'tx-invalid' : ''}">${v.invalid}</td><td class="${v.refused ? 'tx-invalid' : ''}">${v.refused}</td></tr>`).join('')}
            </tbody></table>
            <p class="hint" style="margin-top:10px;">Double-spend pairs with both halves committed: <b class="${s.doubleSpendsBothCommitted ? 'tx-invalid' : 'tx-valid'}">${s.doubleSpendsBothCommitted}</b>.
            Audit after the run: ${s.audit.accountsChecked} balances checked, ${s.audit.balanceMismatches} mismatches, supply ${s.audit.supplyConserved ? 'conserved' : 'NOT conserved'} → <b class="${s.audit.passed ? 'tx-valid' : 'tx-invalid'}">${s.audit.passed ? 'PASSED' : 'FAILED'}</b></p>
            <div class="table-container scroll-box"><table>
                <thead><tr><th>Kind</th><th>From → To</th><th>Amount</th><th>Why it was stopped</th></tr></thead>
                <tbody>${d.refused.slice(0, 200).map((r) => `<tr><td>${esc(r.kind)}</td><td class="mono">${esc(r.from)} → ${esc(r.to)}</td><td>${money(r.amount)}</td><td class="tx-invalid">${esc(r.outcome.replace(/^refused: /, ''))}</td></tr>`).join('')}</tbody>
            </table></div>`;
    }

    // Follow the newest rounds only while the user is on the first page.
    setInterval(() => { if (pageTop === null) loadRounds(); }, 3000);
    setInterval(loadAccounts, 5000);
    setInterval(loadRefused, 15000);
    loadRounds();
    loadAccounts();
    loadRefused();
})();
