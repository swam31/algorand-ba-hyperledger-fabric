// Consensus flow: the votes each orderer sent in one BA* round, which of them
// reached which peer, and what everyone voted for at every step.
//
// Data comes from each orderer's /votes stream (merged by server.js). Every
// vote appears twice: once as "sent" by its author and once as "recv" at each
// peer that got it. Matching the two is what shows delivery and loss.

const NODE_IDS = ['orderer1', 'orderer2', 'orderer3', 'orderer4'];

const canvas = document.getElementById('flow-canvas');
const container = document.getElementById('flow-canvas-container');
const ctx = canvas.getContext('2d');
const btnPause = document.getElementById('flow-pause');
const scrubber = document.getElementById('flow-scrubber');
const labelRound = document.getElementById('flow-round-label');
const labelStep = document.getElementById('flow-step-label');
const summaryEl = document.getElementById('flow-round-summary');
const stepsEl = document.getElementById('flow-steps');

let votes = [];
let roundOutcomes = [];
let isPaused = false;
let viewRound = null;     // the round on screen
let positions = {};
let width = 0;
let height = 0;
let particles = [];       // live view: votes in flight
const animated = new Set();
let primed = false;       // don't animate the backlog present at page load

function stepRank(step) {
    if (step === 'reduction-1') return 1;
    if (step === 'reduction-2') return 2;
    if (step === 'FINAL') return 1e6;
    if (step && step.startsWith('binary-')) return 2 + parseInt(step.slice(7), 10);
    return 0;
}

function stepLabel(step) {
    if (step === 'reduction-1') return 'Reduction 1';
    if (step === 'reduction-2') return 'Reduction 2';
    if (step === 'FINAL') return 'FINAL';
    if (step && step.startsWith('binary-')) return `BinaryBA* ${step.slice(7)}`;
    return step;
}

function valueColor(value) {
    if (!value || value === 'empty') return '#64748b';
    let hue = 0;
    for (let i = 0; i < value.length; i++) hue = (hue * 31 + value.charCodeAt(i)) % 360;
    return `hsl(${hue}, 75%, 60%)`;
}

function voteKey(v) {
    return `${v.ts}|${v.nodeId}|${v.dir}|${v.peer}|${v.round}|${v.step}`;
}

function layout() {
    const dpr = window.devicePixelRatio || 1;
    width = container.clientWidth;
    height = container.clientHeight;
    canvas.width = Math.round(width * dpr);
    canvas.height = Math.round(height * dpr);
    canvas.style.width = `${width}px`;
    canvas.style.height = `${height}px`;
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    const dx = Math.min(width * 0.32, 280);
    const dy = Math.min(height * 0.30, 120);
    const cx = width / 2;
    const cy = height / 2;
    positions = {
        orderer1: { x: cx - dx, y: cy - dy },
        orderer2: { x: cx + dx, y: cy - dy },
        orderer3: { x: cx + dx, y: cy + dy },
        orderer4: { x: cx - dx, y: cy + dy },
    };
}

// Everything needed to draw one round.
function summarise(round) {
    const inRound = votes.filter((v) => v.round === round);
    const sent = inRound.filter((v) => v.dir === 'sent');
    const recv = new Set(inRound.filter((v) => v.dir === 'recv').map((v) => `${v.peer}>${v.nodeId}>${v.step}`));

    const links = {};           // "a>b" -> { sent, delivered }
    const cells = {};           // step -> node -> { value, seats }
    const lastSeats = {};
    for (const v of sent) {
        (cells[v.step] = cells[v.step] || {})[v.nodeId] = { value: v.value, seats: v.seats };
        lastSeats[v.nodeId] = v.seats;
        for (const to of NODE_IDS) {
            if (to === v.nodeId) continue;
            const k = `${v.nodeId}>${to}`;
            links[k] = links[k] || { sent: 0, delivered: 0 };
            links[k].sent++;
            if (recv.has(`${v.nodeId}>${to}>${v.step}`)) links[k].delivered++;
        }
    }
    const outcomes = {};
    for (const r of roundOutcomes) {
        if (r.round === round && !outcomes[r.nodeId]) outcomes[r.nodeId] = r;
    }
    const steps = Object.keys(cells).sort((a, b) => stepRank(a) - stepRank(b));
    return { links, cells, steps, lastSeats, outcomes };
}

function latestRound() {
    let r = null;
    for (const v of votes) if (v.dir === 'sent' && (r === null || v.round > r)) r = v.round;
    return r;
}

function spawnParticles() {
    for (const v of votes) {
        const k = voteKey(v);
        if (animated.has(k)) continue;
        animated.add(k);
        if (!primed || v.dir !== 'sent' || v.round !== viewRound) continue;
        for (const to of NODE_IDS) {
            if (to === v.nodeId) continue;
            particles.push({ from: v.nodeId, to, step: v.step, value: v.value, start: performance.now() + Math.random() * 120 });
        }
    }
    primed = true;
    if (animated.size > 20000) animated.clear();
}

function renderSteps(round, s) {
    const outcome = (id) => {
        const o = s.outcomes[id];
        if (!o) return '<span style="color: var(--text-secondary);">running…</span>';
        return `<span class="outcome-${o.outcome.toLowerCase()}">${o.outcome}</span>${o.txCount ? ` · ${o.txCount} txs` : ''}`;
    };
    let html = '<table><thead><tr><th>Step</th>';
    for (const id of NODE_IDS) html += `<th>${id}<br><small>${outcome(id)}</small></th>`;
    html += '</tr></thead><tbody>';
    for (const step of s.steps) {
        html += `<tr><td>${stepLabel(step)}</td>`;
        for (const id of NODE_IDS) {
            const c = s.cells[step][id];
            html += c
                ? `<td><span style="display:inline-block;width:10px;height:10px;border-radius:2px;background:${valueColor(c.value)};margin-right:6px;"></span>${c.value === 'empty' ? 'empty block' : c.value}<small style="color: var(--text-secondary);"> · ${c.seats} seats</small></td>`
                : '<td style="color: var(--text-secondary);">no vote</td>';
        }
        html += '</tr>';
    }
    html += '</tbody></table>';
    if (s.steps.length === 0) html = '<p style="color: var(--text-secondary);">No votes recorded for this round yet.</p>';
    stepsEl.innerHTML = html;

    const decided = Object.values(s.outcomes);
    const final = decided.find((o) => o.outcome === 'FINAL' || o.outcome === 'TENTATIVE');
    const empty = decided.find((o) => o.outcome === 'EMPTY');
    summaryEl.textContent = final
        ? `Round ${round}: agreed on a block with ${final.txCount} transactions (${final.outcome}) - decided by ${decided.length} of 4 orderers so far`
        : empty
            ? `Round ${round}: agreed on the EMPTY block (no transactions written this round)`
            : `Round ${round}: agreement in progress`;
    labelRound.textContent = isPaused ? `Replay: round ${round}` : `Live: round ${round}`;
    labelStep.textContent = s.steps.length ? stepLabel(s.steps[s.steps.length - 1]) : '';
}

function arrow(from, to, side) {
    // Offset each direction to its own side so a->b and b->a don't overlap.
    const dx = to.x - from.x;
    const dy = to.y - from.y;
    const len = Math.hypot(dx, dy) || 1;
    const nx = (-dy / len) * 7 * side;
    const ny = (dx / len) * 7 * side;
    const r = 30;
    return {
        x1: from.x + (dx / len) * r + nx, y1: from.y + (dy / len) * r + ny,
        x2: to.x - (dx / len) * r + nx, y2: to.y - (dy / len) * r + ny,
    };
}

function drawShape(step, x, y, size, color) {
    ctx.fillStyle = color;
    ctx.beginPath();
    if (step === 'FINAL') {
        ctx.moveTo(x, y - size);
        ctx.lineTo(x - size, y + size);
        ctx.lineTo(x + size, y + size);
        ctx.closePath();
    } else if (step.startsWith('binary')) {
        ctx.rect(x - size, y - size, size * 2, size * 2);
    } else {
        ctx.arc(x, y, size, 0, Math.PI * 2);
    }
    ctx.fill();
}

function draw() {
    ctx.clearRect(0, 0, width, height);
    if (viewRound === null) {
        ctx.fillStyle = '#94a3b8';
        ctx.font = '14px sans-serif';
        ctx.textAlign = 'center';
        ctx.fillText('Waiting for votes - submit a transaction to start a round', width / 2, height / 2);
        requestAnimationFrame(draw);
        return;
    }
    const s = summarise(viewRound);

    for (const [k, l] of Object.entries(s.links)) {
        const [a, b] = k.split('>');
        const p = arrow(positions[a], positions[b], 1);
        const ok = l.delivered === l.sent;
        ctx.strokeStyle = ok ? 'rgba(16, 185, 129, 0.7)' : 'rgba(239, 68, 68, 0.9)';
        ctx.setLineDash(ok ? [] : [6, 5]);
        ctx.lineWidth = 2;
        ctx.beginPath();
        ctx.moveTo(p.x1, p.y1);
        ctx.lineTo(p.x2, p.y2);
        ctx.stroke();
        ctx.setLineDash([]);
        const ang = Math.atan2(p.y2 - p.y1, p.x2 - p.x1);
        ctx.fillStyle = ctx.strokeStyle;
        ctx.beginPath();
        ctx.moveTo(p.x2, p.y2);
        ctx.lineTo(p.x2 - 9 * Math.cos(ang - 0.4), p.y2 - 9 * Math.sin(ang - 0.4));
        ctx.lineTo(p.x2 - 9 * Math.cos(ang + 0.4), p.y2 - 9 * Math.sin(ang + 0.4));
        ctx.fill();
        ctx.fillStyle = '#cbd5e1';
        ctx.font = '11px sans-serif';
        ctx.textAlign = 'center';
        ctx.fillText(`${l.delivered}/${l.sent}`, p.x1 + (p.x2 - p.x1) * 0.35, p.y1 + (p.y2 - p.y1) * 0.35 - 4);
    }

    const now = performance.now();
    particles = particles.filter((q) => now - q.start < 900);
    for (const q of particles) {
        const t = (now - q.start) / 900;
        if (t < 0) continue;
        const p = arrow(positions[q.from], positions[q.to], 1);
        drawShape(q.step, p.x1 + (p.x2 - p.x1) * t, p.y1 + (p.y2 - p.y1) * t, 5, q.step === 'FINAL' ? 'gold' : valueColor(q.value));
    }

    for (const id of NODE_IDS) {
        const n = positions[id];
        const o = s.outcomes[id];
        ctx.beginPath();
        ctx.arc(n.x, n.y, 26, 0, Math.PI * 2);
        ctx.fillStyle = '#1e293b';
        ctx.fill();
        ctx.lineWidth = 3;
        ctx.strokeStyle = !o ? '#475569' : o.outcome === 'FINAL' ? '#10b981' : o.outcome === 'EMPTY' ? '#64748b' : '#f59e0b';
        ctx.stroke();
        ctx.fillStyle = 'white';
        ctx.textAlign = 'center';
        ctx.font = 'bold 14px sans-serif';
        ctx.fillText(id, n.x, n.y - 36);
        ctx.font = '12px sans-serif';
        ctx.fillStyle = '#94a3b8';
        ctx.fillText(o ? o.outcome : '…', n.x, n.y + 4);
        if (s.lastSeats[id] !== undefined) ctx.fillText(`${s.lastSeats[id]} seats`, n.x, n.y + 44);
    }
    requestAnimationFrame(draw);
}

function refresh() {
    const latest = latestRound();
    if (latest === null) return;
    let minRound = latest;
    for (const v of votes) if (v.round < minRound) minRound = v.round;
    scrubber.min = minRound;
    scrubber.max = latest;
    if (!isPaused) {
        viewRound = latest;
        scrubber.value = latest;
    }
    spawnParticles();
    renderSteps(viewRound, summarise(viewRound));
}

btnPause.addEventListener('click', () => {
    isPaused = !isPaused;
    btnPause.textContent = isPaused ? 'RESUME' : 'PAUSE';
    scrubber.disabled = !isPaused;
    particles = [];
    refresh();
});

scrubber.addEventListener('input', (e) => {
    if (!isPaused) return;
    viewRound = parseInt(e.target.value, 10);
    renderSteps(viewRound, summarise(viewRound));
});

window.addEventListener('resize', layout);
layout();
requestAnimationFrame(draw);

// Called by app.js for every state update. (Wrapping renderDashboard does not
// work: app.js loads after this file and its function declaration replaces
// any wrapper installed here.)
window.updateFlow = function (state) {
    if (state.votes) votes = state.votes;
    if (state.rounds) roundOutcomes = state.rounds;
    refresh();
};
