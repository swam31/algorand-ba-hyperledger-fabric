// Algorand-vs-Raft comparison, latency before/after, and bad-network panels.
(() => {
    const $ = (id) => document.getElementById(id);
    const esc = (s) => String(s ?? '').replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
    const secs = (ms) => (ms === null || ms === undefined ? '-' : `${(ms / 1000).toFixed(2)} s`);
    const getJSON = async (url, fallback) => {
        try {
            const r = await fetch(url);
            return r.ok ? r.json() : fallback;
        } catch {
            return fallback;
        }
    };

    function cell(r) {
        if (!r) return '<td class="hint">not run</td>';
        const pct = Number(r.scale) ? Math.round((100 * r.totalTxs) / Number(r.scale)) : 0;
        return `<td>${r.totalTxs.toLocaleString()} <small>(${pct}%)</small> · <b>${r.e2eTps.toFixed(1)}</b> tps</td>`;
    }
    const lat = (r) => (r ? `<td>${secs(r.p50)} / ${secs(r.p95)}</td>` : '<td class="hint">-</td>');

    async function loadComparison() {
        const rows = await getJSON('/benchmarks', []);
        const key = (r) => `${r.workload || 'CreateAsset'}|${r.scale}|${r.concurrency}`;
        const groups = new Map();
        for (const r of rows) {
            if (!groups.has(key(r))) groups.set(key(r), {});
            groups.get(key(r))[r.consensus === 'raft' ? 'raft' : 'algorand'] = r; // latest run wins
        }
        $('cmp-body').innerHTML = [...groups.entries()].map(([k, g]) => {
            const [workload, scale, conc] = k.split('|');
            return `<tr><td>${workload === 'payments' ? 'Payments' : 'CreateAsset'}</td><td>${Number(scale).toLocaleString()} @ ${conc}</td>
                ${cell(g.algorand)}${cell(g.raft)}${lat(g.algorand)}${lat(g.raft)}</tr>`;
        }).join('') || '<tr><td colspan="6" class="hint">No benchmark runs yet.</td></tr>';

        const archive = await getJSON('/api/bench/archive', []);
        const find = (list, scale, conc, build, consensus) => list.filter((r) => r.workload !== 'payments'
            && r.scale === scale && r.concurrency === conc
            && (build === undefined || r.build === build)
            && (consensus === undefined || (r.consensus || 'algorand') === consensus)).pop();
        const fmt = (r) => (r ? `<td>${r.e2eTps.toFixed(1)} · ${secs(r.p50)} · ${secs(r.p95)}</td>` : '<td class="hint">not measured</td>');
        const steps = [
            ['Before: rounds wait for Fabric\'s 2 s batch timer', 'before fast-start (2 s batch timer)'],
            ['Start a round as soon as a transaction arrives', 'fast-start only (no tx sharing)'],
            ['+ orderers share pending transactions (the winner includes everyone\'s)', 'tx-sharing, lambda_priority 1s'],
            ['+ proposal window 1 s → 250 ms, still sending whole blocks', 'tx-sharing + 250ms, full-block proposals'],
            ['+ compact proposals: send transaction hashes, not whole blocks', 'compact proposals (before priority queues and catch-up)'],
        ];
        let html = steps.map(([label, build]) => `<tr><td>${label}</td>${fmt(find(archive, '1000', 25, build))}${fmt(find(archive, '10000', 500, build))}</tr>`).join('');
        html += `<tr><td><b>+ robustness fixes: priority queues, heartbeats, vote catch-up (final build)</b></td>${fmt(find(rows, '1000', 25, undefined, 'algorand'))}${fmt(find(rows, '10000', 500, undefined, 'algorand'))}</tr>`;
        html += `<tr><td class="hint">Stock Fabric Raft, for reference</td>${fmt(find(rows, '1000', 25, undefined, 'raft'))}${fmt(find(rows, '10000', 500, undefined, 'raft'))}</tr>`;
        $('lat-body').innerHTML = html;
    }

    async function loadBadnet() {
        const rows = await getJSON('/api/badnet', []);
        $('badnet-body').innerHTML = rows.map((r) => {
            const o1 = r.rounds[0] || {};
            return `<tr>
                <td><b>${esc(r.title)}</b><br><small>${esc(r.faults.join('; ') || 'none')}${r.healAfterSec ? `; reconnected after ${r.healAfterSec} s` : ''}</small></td>
                <td>${r.committed.toLocaleString()} <small>(${r.tps.toFixed(1)} tps)</small></td>
                <td>${secs(r.p50)} / ${secs(r.p95)}</td>
                <td><span class="outcome-final">${o1.final ?? '-'}</span> / <span class="outcome-tentative">${o1.tentative ?? '-'}</span> / <span class="outcome-empty">${o1.empty ?? '-'}</span></td>
                <td class="${r.forkFree ? 'tx-valid' : 'tx-invalid'}">${r.forkFree ? `✓ after ${r.convergedAfterSec.toFixed(0)} s` : '✗ still differ'}<br><small>height ${esc(r.finalHeights.join('/'))}</small></td>
            </tr>`;
        }).join('') || '<tr><td colspan="5" class="hint">No experiments yet: node bench/badnet.js</td></tr>';
    }

    async function loadCrash() {
        const rows = await getJSON('/api/crash', []);
        $('crash-body').innerHTML = rows.map((r) => `<tr>
            <td>${r.consensus === 'raft' ? 'Raft (stock 2.5)' : 'Algorand BA*'}</td>
            <td>${esc(r.victim)}${r.wasLeader ? ' <small>(leader)</small>' : ''}</td>
            <td>${r.committed.toLocaleString()}</td>
            <td class="${r.timedOutAfterCrash ? 'tx-invalid' : 'tx-valid'}">${r.timedOutAfterCrash ?? r.errors}</td>
            <td><b>${(r.longestGapMs / 1000).toFixed(2)} s</b></td>
            <td><small class="mono">${r.perSecond.join(' ')}</small></td></tr>`).join('')
            || '<tr><td colspan="6" class="hint">Not run yet: node bench/crash-test.js</td></tr>';
    }

    loadComparison();
    loadBadnet();
    loadCrash();
    setInterval(loadCrash, 15000);
    setInterval(loadComparison, 15000);
    setInterval(loadBadnet, 15000);
})();
