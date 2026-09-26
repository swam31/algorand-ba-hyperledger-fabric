// Scale Simulation panel: results of cmd/scalesim for 4 to 10,000 nodes.
(() => {
    const $ = (id) => document.getElementById(id);
    const esc = (s) => String(s ?? '').replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
    const n0 = (x) => Math.round(x).toLocaleString();
    const money = (n) => `$${Number(n).toLocaleString()}`;
    let chart = null;
    let rows = [];

    const keyOf = (r) => (r.online < r.nodes ? `${r.nodes}_offline${Math.round(100 * (1 - r.online / r.nodes))}` : String(r.nodes));
    const label = (r) => (r.online < r.nodes ? `${r.nodes.toLocaleString()} (${Math.round(100 * (1 - r.online / r.nodes))}% offline)` : r.nodes.toLocaleString());

    async function load() {
        const res = await fetch('/api/scale').catch(() => null);
        if (!res || !res.ok) return;
        rows = (await res.json()).sort((a, b) => a.nodes - b.nodes || b.online - a.online);
        if (!rows.length) {
            $('scale-body').innerHTML = '<tr><td colspan="11" class="hint">No simulation runs yet: go run ./cmd/scalesim -nodes 1000</td></tr>';
            return;
        }
        $('scale-body').innerHTML = rows.map((r) => `
            <tr class="clickable" data-key="${keyOf(r)}">
                <td><b>${label(r)}</b></td>
                <td><span class="outcome-final">${r.finalRounds} FINAL</span>${r.tentativeRounds ? ` / ${r.tentativeRounds} T` : ''}${r.emptyRounds ? ` / ${r.emptyRounds} empty` : ''}${r.allAgreed ? '' : ' <span class="tx-invalid">DISAGREED</span>'}</td>
                <td>${r.avgProposers.toFixed(0)}</td>
                <td>${n0(r.avgVotersPerStep)}</td>
                <td>${n0(r.avgSeatsPerStep)}</td>
                <td>${n0(r.votesPerRound)}</td>
                <td>${(r.kbReceivedPerNodePerRound / 1024).toFixed(2)} MB</td>
                <td>${(r.nodeVerifyMsPerRound / 1000).toFixed(2)} s CPU</td>
                <td><span class="tx-valid">${n0(r.paymentsCommitted)}</span> / <span class="tx-invalid">${n0(r.paymentsRejected)} refused</span></td>
                <td class="${r.supplyConserved && r.auditPassed ? 'tx-valid' : 'tx-invalid'}">${money(r.supplyEnd)} ${r.supplyConserved && r.auditPassed ? '✓' : '✗'}</td>
                <td>${r.simSecondsPerRound.toFixed(1)} s</td>
            </tr>`).join('');

        const full = rows.filter((r) => r.online === r.nodes);
        const data = {
            labels: full.map((r) => r.nodes.toLocaleString()),
            datasets: [
                { type: 'bar', label: 'Committee seats per step (target 2,000)', data: full.map((r) => r.avgSeatsPerStep), backgroundColor: '#10b981', yAxisID: 'y' },
                { type: 'bar', label: 'Distinct nodes voting per step', data: full.map((r) => r.avgVotersPerStep), backgroundColor: '#3b82f6', yAxisID: 'y' },
                { type: 'line', label: 'Total nodes (log scale)', data: full.map((r) => r.nodes), borderColor: '#f59e0b', backgroundColor: '#f59e0b', yAxisID: 'nodes' },
            ],
        };
        if (typeof Chart === 'undefined') return;
        if (!chart) {
            chart = new Chart($('scale-chart'), {
                data,
                options: {
                    maintainAspectRatio: false,
                    plugins: { legend: { labels: { color: '#cbd5e1' } } },
                    scales: {
                        x: { title: { display: true, text: 'Nodes in the network', color: '#94a3b8' }, ticks: { color: '#94a3b8' } },
                        y: { beginAtZero: true, title: { display: true, text: 'Per BA* step', color: '#94a3b8' }, ticks: { color: '#94a3b8' } },
                        nodes: { type: 'logarithmic', position: 'right', title: { display: true, text: 'Nodes', color: '#f59e0b' }, ticks: { color: '#f59e0b' }, grid: { drawOnChartArea: false } },
                    },
                },
            });
        } else {
            chart.data = data;
            chart.update();
        }
    }

    $('scale-body').addEventListener('click', async (e) => {
        const tr = e.target.closest('tr[data-key]');
        if (!tr) return;
        document.querySelectorAll('#scale-body tr').forEach((x) => x.classList.toggle('selected', x === tr));
        const res = await fetch(`/api/scale/${tr.dataset.key}`);
        if (!res.ok) return;
        const run = await res.json();
        const s = run.summary;
        const kinds = Object.entries(s.kinds || {});
        $('scale-detail').innerHTML = `<div class="detail">
            <h3>${label(s)} nodes · ${money(s.startBalance)} each · W = ${money(s.w)}</h3>
            <p>τ proposer / step / final = ${s.tauProposer} / ${s.tauStep} / ${s.tauFinal}. One vote costs ${s.verifyMicrosPerVote.toFixed(0)} µs to verify here.
               <span class="${s.auditPassed ? 'tx-valid' : 'tx-invalid'}">Audit ${s.auditPassed ? 'passed' : 'FAILED'}: ${esc(s.auditNote)}</span></p>
            <table><thead><tr><th>Payment kind</th><th>Committed</th><th>Refused</th></tr></thead><tbody>
                ${kinds.map(([k, v]) => `<tr><td>${esc(k)}</td><td class="tx-valid">${v.committed}</td><td class="${v.refused ? 'tx-invalid' : ''}">${v.refused}</td></tr>`).join('')}
            </tbody></table>
            <p class="hint">An "overdraft" or double-spend attempt can legitimately commit if the sender was paid earlier in the same block; the audit replays every committed payment to prove none overdrew.</p>
            <h4>Rounds</h4>
            <div class="table-container scroll-box"><table>
                <thead><tr><th>Round</th><th>Outcome</th><th>Proposer (of N winners)</th><th>Payments</th><th>Refused</th><th>Votes</th><th>Committee per step (voters / seats)</th></tr></thead>
                <tbody id="scale-rounds-body">${run.rounds.map((r) => `
                    <tr class="clickable" data-round="${r.round}">
                        <td>${r.round}</td><td class="outcome-${r.outcome.toLowerCase()}">${r.outcome}${r.agreed ? '' : ' (disagreed)'}</td>
                        <td class="mono">${esc(r.proposer)} <small>of ${r.proposers}</small></td>
                        <td>${r.payments.length}</td><td>${r.rejected.length}</td><td>${n0(r.votes)}</td>
                        <td><small>${r.committee.map((c) => `${esc(c.step)} ${n0(c.voters)}/${n0(c.seats)}`).join(' · ')}</small></td>
                    </tr>`).join('')}</tbody>
            </table></div>
            <div id="scale-round"></div>
        </div>`;
        $('scale-rounds-body').addEventListener('click', (ev) => {
            const row = ev.target.closest('tr[data-round]');
            if (!row) return;
            const r = run.rounds.find((x) => x.round === Number(row.dataset.round));
            const all = [...r.payments, ...r.rejected];
            $('scale-round').innerHTML = `<h4>Round ${r.round}: ${r.payments.length} payments committed, ${r.rejected.length} refused</h4>
                <div class="table-container scroll-box"><table>
                    <thead><tr><th>Tx</th><th>From → To</th><th>Amount</th><th>Kind</th><th>Result</th></tr></thead>
                    <tbody>${all.map((p) => `<tr><td class="mono">${esc(p.id)}</td><td class="mono">${esc(p.from)} → ${esc(p.to)}</td><td>${money(p.amount)}</td>
                        <td>${esc(p.kind)}</td><td class="${p.status === 'committed' ? 'tx-valid' : 'tx-invalid'}">${esc(p.status)}${p.reason ? `: ${esc(p.reason)}` : ''}</td></tr>`).join('')}</tbody>
                </table></div>`;
        });
    });

    load();
    setInterval(load, 30000);
})();
