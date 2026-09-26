const ws = new WebSocket(`ws://${window.location.host}`);

ws.onmessage = (event) => {
    const state = JSON.parse(event.data);
    renderDashboard(state);
    if (window.updateFlow) window.updateFlow(state);
};

function renderDashboard(state) {
    renderNetworkSummary(state);
    renderOrderers(state);
    renderRounds(state.rounds);
    renderBlocks(state.blocks);
}

function renderNetworkSummary(state) {
    const summaryEl = document.getElementById('network-summary');
    
    // Find the first healthy orderer to extract network params
    let params = null;
    let upCount = 0;
    
    for (const key in state.orderers) {
        if (state.orderers[key] && state.orderers[key].up) {
            upCount++;
            if (!params) {
                params = state.orderers[key];
            }
        }
    }

    if (!params) {
        summaryEl.innerHTML = '<div class="banner at-risk">Waiting for network data...</div>';
        return;
    }

    const { n, w, tau, threshold, h, p, toleratedFaults } = params;
    
    let html = '';
    
    if (upCount >= n - toleratedFaults) {
        html += `<div class="banner healthy">BYZANTINE FAULT TOLERANT — ${upCount} of ${n} orderers healthy, tolerating f=${toleratedFaults}</div>`;
    } else {
        html += `<div class="banner at-risk">CONSENSUS AT RISK — Only ${upCount} of ${n} orderers healthy, cannot tolerate failures</div>`;
    }

    html += `
        <div style="display:flex; justify-content:space-between; margin-top: 10px; color: var(--text-secondary);">
            <span><b>Stake (W):</b> ${w}</span>
            <span><b>Tau:</b> ${tau}</span>
            <span><b>Threshold:</b> ${threshold}</span>
            <span><b>h:</b> ${(h * 100).toFixed(1)}%</span>
            <span><b>p:</b> ${(p * 100).toFixed(1)}%</span>
        </div>
    `;
    
    summaryEl.innerHTML = html;
}

function renderOrderers(state) {
    const orderersEl = document.getElementById('orderers');
    let html = '';

    // Find max final round across all orderers
    let maxFinalRound = 0;
    for (const key in state.orderers) {
        const o = state.orderers[key];
        if (o.up && o.lastFinalRound > maxFinalRound) {
            maxFinalRound = o.lastFinalRound;
        }
    }

    for (let i = 0; i < 4; i++) {
        const o = state.orderers[i] || { up: false, nodeId: `orderer${i+1}` };
        
        const isUp = o.up;
        const statusClass = isUp ? 'up' : 'down';
        const statusText = isUp ? 'UP' : 'DOWN';
        
        // A node is lagging if it's up but its lastFinalRound is > 2 behind the max
        const isLagging = isUp && o.lastFinalRound !== undefined && (maxFinalRound - o.lastFinalRound) > 2;
        const cardClass = isLagging ? 'orderer-card lagging' : 'orderer-card';

        let statsHtml = '';
        if (isUp) {
            statsHtml = `
                <div class="stat"><span class="stat-label">Ledger Height:</span> <span>${o.ledgerHeight || 0}</span></div>
                <div class="stat"><span class="stat-label">Last FINAL:</span> <span>${o.lastFinalRound || 0}</span></div>
                <div class="stat"><span class="stat-label">Peers:</span> <span>${o.peersConnected}/${o.peersConfigured}</span></div>
                <div class="stat"><span class="stat-label">Seats Won:</span> <span>${o.seatsLastRound || 0}</span></div>
            `;
        }

        html += `
            <div class="${cardClass}">
                <div class="lagging-banner">LAGGING — not participating in consensus</div>
                <h3>
                    ${o.nodeId} 
                    <span class="status-badge ${statusClass}">${statusText}</span>
                </h3>
                ${statsHtml}
            </div>
        `;
    }
    
    orderersEl.innerHTML = html;
}

function renderRounds(rounds) {
    const roundsEl = document.getElementById('rounds-body');
    if (!rounds || rounds.length === 0) return;

    let html = '';
    for (const r of rounds.slice(0, 50)) {
        const outcomeClass = `outcome-${r.outcome.toLowerCase()}`;
        html += `
            <tr>
                <td>${r.nodeId || '-'}</td>
                <td>${r.round}</td>
                <td class="${outcomeClass}">${r.outcome}</td>
                <td>${r.steps}</td>
                <td>${r.txCount}</td>
                <td>${r.durationMs}</td>
            </tr>
        `;
    }
    roundsEl.innerHTML = html;
}

function renderBlocks(blocks) {
    const blocksEl = document.getElementById('blocks-body');
    if (!blocks || blocks.length === 0) return;

    let html = '';
    for (const b of blocks) {
        const valid = b.valid ?? b.txs.filter(tx => tx.valid).length;
        const invalid = b.txCount - valid;
        // A block is never "invalid" as a whole: Fabric validates each
        // transaction separately and records a code for each.
        const shown = b.txs.slice(0, 6).map(tx => {
            const cls = tx.valid ? 'tx-valid' : 'tx-invalid';
            return `<span class="${cls}" title="${tx.status}">${(tx.id || 'config').substring(0, 8)}</span>`;
        }).join(', ');
        const more = b.txCount > b.txs.length ? ` +${b.txCount - b.txs.length} more` : '';
        html += `
            <tr>
                <td>${b.number}${b.round !== null ? ` <small>(round ${b.round})</small>` : ''}</td>
                <td>${b.txCount}</td>
                <td><span class="tx-valid">${valid} valid</span>${invalid ? ` / <span class="tx-invalid">${invalid} invalid</span>` : ''}</td>
                <td>${shown || '-'}${more}</td>
            </tr>
        `;
    }
    blocksEl.innerHTML = html;
}
