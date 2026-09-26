let tpsChart, latChart;

function initCharts() {
    const ctxTps = document.getElementById('bench-tps-chart');
    const ctxLat = document.getElementById('bench-lat-chart');
    if (!ctxTps || !ctxLat) return;
    // Chart.js comes from a CDN; if it did not load, keep the tables working.
    if (typeof Chart === 'undefined') {
        ctxTps.innerHTML = ctxLat.innerHTML = '<p class="hint">Chart library did not load (CDN unreachable); the table below has every number.</p>';
        return;
    }
    
    // Instead of using Chart.js, since we didn't add a canvas for charts (we used a div), we need to create canvases
    ctxTps.innerHTML = '<canvas></canvas>';
    ctxLat.innerHTML = '<canvas></canvas>';
    
    tpsChart = new Chart(ctxTps.querySelector('canvas'), {
        type: 'bar',
        data: {
            labels: [],
            datasets: [
                { label: 'E2E TPS', backgroundColor: '#3b82f6', data: [] },
                { label: 'Busy TPS (first submit to last commit)', backgroundColor: '#10b981', data: [] }
            ]
        },
        options: { responsive: true, maintainAspectRatio: false, scales: { y: { beginAtZero: true } } }
    });
    
    latChart = new Chart(ctxLat.querySelector('canvas'), {
        type: 'bar',
        data: {
            labels: [],
            datasets: [
                { label: 'p50 Latency', backgroundColor: '#f59e0b', data: [] },
                { label: 'p95 Latency', backgroundColor: '#ef4444', data: [] }
            ]
        },
        options: { responsive: true, maintainAspectRatio: false, scales: { y: { beginAtZero: true } } }
    });
}

initCharts();

async function updateBenchmarks() {
    try {
        const res = await fetch('/benchmarks');
        const results = await res.json();
        
        const tbody = document.getElementById('bench-tbody');
        if (!tbody) return;
        
        let html = '';
        const labels = [];
        const e2eData = [];
        const ordData = [];
        const p50Data = [];
        const p95Data = [];
        
        for (const row of results) {
            labels.push(`${row.consensus === 'raft' ? 'Raft ' : ''}${row.workload === 'payments' ? 'pay ' : ''}${row.scale} @ ${row.concurrency}`);
            e2eData.push(row.e2eTps);
            ordData.push(row.ordTps);
            p50Data.push(row.p50);
            p95Data.push(row.p95);
            
            // Rows written by older load generators lack the measured
            // fields; show a dash rather than inventing a value.
            const measured = row.rounds !== undefined && row.rounds !== null;
            const failed = (row.invalid || 0) + (row.errors || 0);
            const payments = row.workload === 'payments';
            let result = row.complete ? 'Complete'
                : `Incomplete${failed ? ` (${failed} failed)` : ''}`;
            let ok = row.complete;
            if (payments) {
                // Bad transfers are refused or invalidated on purpose; the
                // pass/fail here is the balance audit.
                ok = row.complete && row.audit.passed && row.doubleSpendsBothCommitted === 0;
                result = `${ok ? 'Audit passed' : 'Audit FAILED'} (${row.invalid} invalidated, ${row.errors} refused)`;
            }
            html += `
                <tr>
                    <td>${row.consensus === 'raft' ? 'Raft <small>(stock 2.5)</small>' : 'Algorand BA*'}</td>
                    <td>${payments ? `Payments <small>(${row.accounts} accts)</small>` : 'CreateAsset'}</td>
                    <td>${row.scale}</td>
                    <td>${row.concurrency}</td>
                    <td>${row.totalTxs} / ${row.scale}</td>
                    <td>${row.e2eTps.toFixed(1)}</td>
                    <td>${row.ordTps.toFixed(1)}</td>
                    <td>${row.durationSec.toFixed(1)}</td>
                    <td>${row.blocks !== undefined && row.txPerBlock !== undefined ? `${row.blocks} (${row.txPerBlock.toFixed(0)})` : '-'}</td>
                    <td>${measured ? `${row.rounds} (${row.emptyRounds}, ${row.wastedPct.toFixed(1)}%)` : '-'}</td>
                    <td>${row.p50} / ${row.p95}</td>
                    <td class="${ok ? 'outcome-final' : 'outcome-failed'}">${result}</td>
                </tr>
            `;
        }
        tbody.innerHTML = html;
        
        if (tpsChart && latChart && results.length > 0) {
            tpsChart.data.labels = labels;
            tpsChart.data.datasets[0].data = e2eData;
            tpsChart.data.datasets[1].data = ordData;
            tpsChart.update();
            
            latChart.data.labels = labels;
            latChart.data.datasets[0].data = p50Data;
            latChart.data.datasets[1].data = p95Data;
            latChart.update();
        }
    } catch (e) {
        // ignore
    }
}

setInterval(updateBenchmarks, 5000);
updateBenchmarks();
