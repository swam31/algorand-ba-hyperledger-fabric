// Transaction flow: every new block on the ledger replayed as an animation of
// Fabric's execute-order-validate path. The transactions, their validity, the
// proposer, the round and its outcome are real (server.js pushes them from the
// ledger as state.activity); only the timing is slowed down to be followable.
(() => {
    const $ = (id) => document.getElementById(id);
    const canvas = $('txf-canvas');
    const container = $('txf-canvas-container');
    const ctx = canvas.getContext('2d');
    const statusEl = $('txf-status');
    const captionEl = $('txf-caption');
    const feedEl = $('txf-feed');
    const demoEl = $('txf-demo');
    const sendButtons = [$('txf-send10'), $('txf-send50')];
    const pauseBtn = $('txf-pause');

    const ORDERERS = ['orderer1', 'orderer2', 'orderer3', 'orderer4'];
    const C = {
        pending: '#3b82f6', valid: '#10b981', invalid: '#ef4444', tentative: '#f59e0b',
        node: '#1e293b', edge: '#475569', box: '#0b1220', text: '#f8fafc', dim: '#94a3b8', gold: '#fbbf24',
    };
    const VOTE_COLORS = ['#a78bfa', '#38bdf8', C.gold]; // Reduction, BinaryBA*, FINAL

    // Seconds from the start of a block's replay, at normal speed.
    const T = { endorse: 1.2, order: 1.8, vote: 2.8, pack: 4.0, validate: 4.8, commit: 6.0, done: 7.0 };
    const GAP = 1.6;        // seconds between two blocks' starts: one block per stage at a time
    const MAX_QUEUE = 10;    // blocks waiting beyond this go straight onto the ledger row
    const SHOWN = 10;       // transactions drawn per block; the rest are counted
    const STALE_MS = 30000; // older blocks are history being reloaded, not news
    const CHIP_W = 58, CHIP_H = 18, CELL_H = 22, HEAD_H = 30;
    const SLOT_W = 74, SLOT_H = 44, SLOT_STEP = 100;

    let W = 0, H = 0, L = {};
    let clock = 0, lastFrame = performance.now(), speed = 1, paused = false;
    let known = null;       // highest block number received
    let queue = [], playing = [], chain = [];
    let lastStart = -Infinity, skipped = 0;
    const up = {};

    const clamp = (v) => Math.max(0, Math.min(1, v));
    const ease = (p) => (p < 0.5 ? 2 * p * p : 1 - Math.pow(-2 * p + 2, 2) / 2);
    const lerp = (a, b, p) => a + (b - a) * p;
    const esc = (s) => String(s ?? '').replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
    const outcomeColor = (o) => (o === 'TENTATIVE' ? C.tentative : C.valid);
    const isUp = (id) => up[id] !== false;

    function roundRect(x, y, w, h, r) {
        ctx.beginPath();
        if (ctx.roundRect) ctx.roundRect(x, y, w, h, r);
        else ctx.rect(x, y, w, h);
    }

    function text(str, x, y, { size = 12, color = C.text, bold = false, align = 'center' } = {}) {
        ctx.font = `${bold ? 'bold ' : ''}${size}px sans-serif`;
        ctx.fillStyle = color;
        ctx.textAlign = align;
        ctx.fillText(str, x, y);
    }

    function layout() {
        const dpr = window.devicePixelRatio || 1;
        W = container.clientWidth;
        H = container.clientHeight;
        canvas.width = Math.round(W * dpr);
        canvas.height = Math.round(H * dpr);
        canvas.style.width = `${W}px`;
        canvas.style.height = `${H}px`;
        ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
        const mid = 212;
        L = {
            mid,
            client: W * 0.07, endorse: W * 0.265, order: W * 0.465, block: W * 0.665, validate: W * 0.865,
            chainY: H - 56,
        };
        const d = 62;
        L.orderers = {
            orderer1: { x: L.order - d, y: mid - d }, orderer2: { x: L.order + d, y: mid - d },
            orderer3: { x: L.order + d, y: mid + d }, orderer4: { x: L.order - d, y: mid + d },
        };
        L.endorsers = [{ x: L.endorse, y: mid - 112, name: 'Org1 peer' }, { x: L.endorse, y: mid + 112, name: 'Org2 peer' }];
        L.validators = [{ x: L.validate, y: mid - 112, name: 'Org1 peer' }, { x: L.validate, y: mid + 112, name: 'Org2 peer' }];
    }

    // ------------------------------------------------------------ data in
    function prepare(b) {
        const shown = b.txs.slice(0, SHOWN);
        const rows = Math.max(1, Math.ceil(shown.length / 2));
        return {
            ...b,
            shown,
            start: null,
            fed: false,
            captioned: false,
            blockW: 2 * CHIP_W + 18,
            blockH: HEAD_H + rows * CELL_H + 8,
            pool: shown.map(() => ({ dx: (Math.random() - 0.5) * 64, dy: (Math.random() - 0.5) * 64 })),
        };
    }

    const chainItem = (b) => ({ number: b.number, round: b.round, outcome: b.outcome, txCount: b.txCount, valid: b.valid, emptyBefore: b.emptyBefore });

    function addToChain(item) {
        chain.push(item);
        chain.sort((a, b) => a.number - b.number);
        if (chain.length > 60) chain.splice(0, chain.length - 60);
    }

    function onActivity(list) {
        if (!list || !list.length) return;
        const newest = list[list.length - 1].number;
        if (known === null || newest < known) {
            // First data, or the network was rebuilt: show the chain as it is.
            known = newest;
            chain = list.map(chainItem);
            queue = [];
            playing = [];
            feedEl.innerHTML = '';
            list.slice(-3).forEach((b) => feed(prepare(b), false));
            return;
        }
        const fresh = list.filter((b) => b.number > known);
        if (!fresh.length) return;
        known = newest;
        const catchingUp = fresh.length > MAX_QUEUE;
        for (const b of fresh) {
            if (catchingUp || (b.time && Date.now() - b.time > STALE_MS)) addToChain(chainItem(b));
            else queue.push(prepare(b));
        }
    }

    // ------------------------------------------------------------ feed
    function feed(b, animate = true) {
        const rows = b.shown.map((tx) => {
            const what = tx.type === 'transfer'
                ? `<span class="mono">${esc(tx.from)}</span> → <span class="mono">${esc(tx.to)}</span> <b>$${esc(tx.amount)}</b>`
                : `${esc(tx.chaincode || tx.type)}${tx.fn ? ` · ${esc(tx.fn)}` : ''}`;
            const verdict = tx.valid ? '<span class="txf-ok">✓ valid</span>' : `<span class="txf-bad">✗ ${esc(tx.status)}</span>`;
            const why = !tx.valid && tx.status === 'MVCC_READ_CONFLICT'
                ? '<br><span class="hint">Another transfer changed this balance first, so this one was voided. This is how Fabric stops a double spend.</span>'
                : '';
            return `<div class="txf-row${tx.valid ? '' : ' bad'}${animate ? ' new' : ''}" data-round="${b.round ?? ''}" data-tx="${esc(tx.id)}" title="Open round ${b.round ?? '-'} in Round History">
                ${verdict} ${what} <span class="hint">· block ${b.number} · round ${b.round ?? '-'} · proposed by ${esc(b.proposer ?? '-')}</span>${why}</div>`;
        }).join('');
        const more = b.txCount > b.shown.length
            ? `<div class="txf-row more${animate ? ' new' : ''}">+${b.txCount - b.shown.length} more transactions in block ${b.number}</div>`
            : '';
        feedEl.insertAdjacentHTML('afterbegin', rows + more);
        while (feedEl.children.length > 16) feedEl.lastElementChild.remove();
    }

    feedEl.addEventListener('click', (e) => {
        const row = e.target.closest('.txf-row[data-round]');
        if (row && row.dataset.round && window.showRound) window.showRound(Number(row.dataset.round), row.dataset.tx);
    });

    function caption(b) {
        const invalid = b.txCount - b.valid;
        captionEl.innerHTML = `Round ${b.round ?? '-'}: <b>${esc(b.proposer ?? '-')}</b> won the proposer lottery, the orderers voted with BA* and reached
            <span class="outcome-${String(b.outcome || 'final').toLowerCase()}">${esc(b.outcome ?? 'FINAL')}</span>.
            Block ${b.number} holds ${b.txCount} transaction${b.txCount === 1 ? '' : 's'}${invalid ? `, <span class="tx-invalid">${invalid} rejected at commit</span>` : ''}.`;
    }

    // ------------------------------------------------------------ drawing
    function drawNode(x, y, r, label, { ring = C.edge, down = false } = {}) {
        ctx.beginPath();
        ctx.arc(x, y, r, 0, Math.PI * 2);
        ctx.fillStyle = C.node;
        ctx.fill();
        ctx.lineWidth = 2.5;
        ctx.strokeStyle = down ? C.invalid : ring;
        ctx.setLineDash(down ? [4, 4] : []);
        ctx.stroke();
        ctx.setLineDash([]);
        text(label, x, y + 4, { size: 12, bold: true, color: down ? C.dim : C.text });
    }

    function drawScene(voting) {
        const labels = [
            [L.client, '1 · Client', 'sends a transaction'],
            [L.endorse, '2 · Endorse', 'both peers run it and sign'],
            [L.order, '3 · Order', '4 orderers agree with BA*'],
            [L.block, '4 · Block', 'the agreed block'],
            [L.validate, '5 · Validate', 'peers check it and commit'],
        ];
        for (const [x, title, sub] of labels) {
            text(title, x, 22, { size: 14, bold: true });
            text(sub, x, 39, { size: 11, color: C.dim });
        }

        roundRect(L.client - 44, L.mid - 32, 88, 64, 10);
        ctx.fillStyle = C.box;
        ctx.fill();
        ctx.strokeStyle = C.edge;
        ctx.lineWidth = 2;
        ctx.stroke();
        text('Client', L.client, L.mid - 2, { size: 13, bold: true });
        text('app', L.client, L.mid + 15, { size: 11, color: C.dim });

        for (const p of [...L.endorsers, ...L.validators]) {
            drawNode(p.x, p.y, 22, 'peer');
            text(p.name, p.x, p.y + (p.y < L.mid ? -30 : 38), { size: 11, color: C.dim });
        }

        ORDERERS.forEach((id, i) => {
            const p = L.orderers[id];
            const isProposer = voting && voting.proposer === id;
            if (isProposer) {
                ctx.beginPath();
                ctx.arc(p.x, p.y, 30, 0, Math.PI * 2);
                ctx.strokeStyle = C.gold;
                ctx.lineWidth = 3;
                ctx.stroke();
                text('proposer', p.x, p.y + (p.y < L.mid ? -36 : 46), { size: 11, bold: true, color: C.gold });
            }
            drawNode(p.x, p.y, 22, `O${i + 1}`, { down: !isUp(id) });
            if (!isUp(id)) text('down', p.x, p.y + (p.y < L.mid ? -30 : 38), { size: 11, color: C.invalid });
        });

        text('Ledger: the same chain on every peer and orderer', 16, L.chainY - 36, { size: 12, color: C.dim, align: 'left' });
    }

    function drawChain(shift) {
        const n = chain.length;
        const right = W - 50 - SLOT_W / 2;
        for (let j = n - 1; j >= 0; j--) {
            const x = right - (n - 1 - j) * SLOT_STEP - shift;
            if (x < -SLOT_W) break;
            const c = chain[j];
            const y = L.chainY;
            if (j > 0) {
                const x1 = x - SLOT_STEP + SLOT_W / 2;
                const x2 = x - SLOT_W / 2;
                ctx.beginPath();
                ctx.moveTo(x1, y);
                ctx.lineTo(x2, y);
                ctx.strokeStyle = c.emptyBefore ? C.dim : C.edge;
                ctx.setLineDash(c.emptyBefore ? [3, 4] : []);
                ctx.lineWidth = 2;
                ctx.stroke();
                ctx.setLineDash([]);
                if (c.emptyBefore) text(`${c.emptyBefore} empty`, (x1 + x2) / 2, y - SLOT_H / 2 - 6, { size: 10, color: C.dim });
            }
            roundRect(x - SLOT_W / 2, y - SLOT_H / 2, SLOT_W, SLOT_H, 6);
            ctx.fillStyle = C.box;
            ctx.fill();
            ctx.strokeStyle = c.round === null ? C.edge : outcomeColor(c.outcome);
            ctx.lineWidth = 2;
            ctx.stroke();
            text(`#${c.number}`, x, y - 3, { size: 13, bold: true });
            text(c.round === null ? 'setup' : `round ${c.round}`, x, y + 13, { size: 10, color: C.dim });
            text(`${c.txCount} tx`, x, y + SLOT_H / 2 + 14, { size: 10, color: C.dim });
        }
    }

    function chipLabel(tx) {
        if (tx.type === 'transfer') return `$${tx.amount}`;
        if (tx.type === 'open-accounts') return 'open';
        if (tx.type === 'config') return 'config';
        return tx.fn ? tx.fn.slice(0, 8) : 'tx';
    }

    function drawChip(x, y, tx, color, alpha, signed, mark, scale = 1) {
        const w = CHIP_W * scale;
        const h = CHIP_H * scale;
        ctx.globalAlpha = alpha;
        roundRect(x - w / 2, y - h / 2, w, h, h / 2);
        ctx.fillStyle = color;
        ctx.fill();
        text(mark ? `${mark} ${chipLabel(tx)}` : chipLabel(tx), x, y + 4 * scale, { size: 11 * scale, bold: true });
        if (signed) {
            ctx.fillStyle = C.gold;
            for (const dx of [-5, 3]) {
                ctx.beginPath();
                ctx.arc(x + w / 2 + dx, y - h / 2 + 1, 3 * scale, 0, Math.PI * 2);
                ctx.fill();
            }
        }
        ctx.globalAlpha = 1;
    }

    function drawVoting(b, t) {
        const p = clamp((t - T.vote) / (T.pack - T.vote - 0.1));
        for (let w = 0; w < 3; w++) {
            const wp = clamp(p * 3 - w);
            if (wp <= 0 || wp >= 1) continue;
            const q = ease(wp);
            ctx.fillStyle = VOTE_COLORS[w];
            for (const a of ORDERERS) {
                if (!isUp(a)) continue;
                for (const c of ORDERERS) {
                    if (a === c) continue;
                    const A = L.orderers[a];
                    const B = L.orderers[c];
                    ctx.beginPath();
                    ctx.arc(lerp(A.x, B.x, q), lerp(A.y, B.y, q), 3.5, 0, Math.PI * 2);
                    ctx.fill();
                }
            }
        }
        const stage = p < 1 / 3 ? 'Reduction votes' : p < 2 / 3 ? 'BinaryBA* votes' : 'FINAL votes';
        text(p < 1 ? stage : `Round ${b.round ?? '-'} · ${b.outcome ?? 'FINAL'}`, L.order, L.mid + 122,
            { size: 13, bold: true, color: p < 1 ? C.dim : outcomeColor(b.outcome) });
        if (p >= 1) {
            for (const id of ORDERERS) {
                if (!isUp(id)) continue;
                const o = L.orderers[id];
                text('✓', o.x + 20, o.y - 14, { size: 14, bold: true, color: C.valid });
            }
        }
    }

    function blockPos(t) {
        if (t < T.validate) return { x: L.block, y: L.mid, g: 0 };
        if (t < T.validate + 0.7) return { x: lerp(L.block, L.validate, ease((t - T.validate) / 0.7)), y: L.mid, g: 0 };
        if (t < T.commit) return { x: L.validate, y: L.mid, g: 0 };
        const g = ease(clamp((t - T.commit) / (T.done - T.commit)));
        return { x: lerp(L.validate, W - 50 - SLOT_W / 2, g), y: lerp(L.mid, L.chainY, g), g };
    }

    function cell(b, i) {
        const col = i % 2;
        const row = Math.floor(i / 2);
        return { dx: (col === 0 ? -1 : 1) * (CHIP_W / 2 + 4), dy: -b.blockH / 2 + HEAD_H + row * CELL_H + CELL_H / 2 };
    }

    function drawBlock(b, t) {
        const k = b.shown.length;
        const spacing = Math.min(22, 180 / Math.max(k, 1));
        const lane = (i) => L.mid + (i - (k - 1) / 2) * spacing;
        const bp = blockPos(t);
        const reveal = t >= T.validate + 0.9;

        if (t >= T.vote && t < T.pack + 0.5) drawVoting(b, t);

        if (t >= T.pack - 0.2) {
            const w = lerp(b.blockW, SLOT_W, bp.g);
            const h = lerp(b.blockH, SLOT_H, bp.g);
            ctx.globalAlpha = clamp((t - (T.pack - 0.2)) / 0.4);
            roundRect(bp.x - w / 2, bp.y - h / 2, w, h, 8);
            ctx.fillStyle = C.box;
            ctx.fill();
            ctx.strokeStyle = outcomeColor(b.outcome);
            ctx.lineWidth = 2;
            ctx.stroke();
            if (bp.g < 0.5) text(`Block #${b.number}`, bp.x, bp.y - h / 2 + 19, { size: 13, bold: true });
            else text(`#${b.number}`, bp.x, bp.y + 4, { size: 13, bold: true });
            ctx.globalAlpha = 1;
        }

        // Both peers validate the block.
        if (t >= T.validate + 0.5 && t < T.commit) {
            const pulse = 0.35 + 0.35 * Math.sin((t - T.validate) * 10);
            ctx.strokeStyle = `rgba(16, 185, 129, ${pulse})`;
            ctx.lineWidth = 2;
            for (const v of L.validators) {
                ctx.beginPath();
                ctx.moveTo(v.x, v.y + (v.y < L.mid ? 22 : -22));
                ctx.lineTo(bp.x, bp.y + (v.y < L.mid ? -b.blockH / 2 : b.blockH / 2));
                ctx.stroke();
            }
        }

        let lastY = null;
        b.shown.forEach((tx, i) => {
            const start = i * 0.07;
            if (t < start) return;
            let x;
            let y;
            let alpha = 1;
            let scale = 1;
            const pool = { x: L.order + b.pool[i].dx, y: L.mid + b.pool[i].dy };
            if (t < T.endorse) {
                const p = ease(clamp((t - start) / 0.9));
                x = lerp(L.client + 44, L.endorse, p);
                y = lane(i);
                alpha = clamp((t - start) / 0.2);
            } else if (t < T.order) {
                x = L.endorse;
                y = lane(i);
            } else if (t < T.vote) {
                const p = ease(clamp((t - T.order) / (T.vote - T.order)));
                x = lerp(L.endorse, pool.x, p);
                y = lerp(lane(i), pool.y, p);
            } else if (t < T.pack) {
                x = pool.x;
                y = pool.y;
            } else {
                const c = cell(b, i);
                if (t < T.validate) {
                    const p = ease(clamp((t - T.pack) / 0.7));
                    x = lerp(pool.x, bp.x + c.dx, p);
                    y = lerp(pool.y, bp.y + c.dy, p);
                } else {
                    scale = 1 - bp.g * 0.6;
                    x = bp.x + c.dx * scale;
                    y = bp.y + c.dy * scale;
                    alpha = 1 - clamp((t - T.commit) / 0.5);
                }
            }
            // The two endorsing peers sign each transaction.
            if (t >= T.endorse - 0.1 && t < T.order) {
                ctx.strokeStyle = 'rgba(251, 191, 36, 0.45)';
                ctx.lineWidth = 1;
                for (const e of L.endorsers) {
                    ctx.beginPath();
                    ctx.moveTo(x, y);
                    ctx.lineTo(e.x, e.y + (e.y < L.mid ? 22 : -22));
                    ctx.stroke();
                }
            }
            const color = reveal ? (tx.valid ? C.valid : C.invalid) : C.pending;
            const mark = reveal ? (tx.valid ? '✓' : '✗') : '';
            if (alpha > 0) drawChip(x, y, tx, color, alpha, t >= T.endorse + 0.3 && !reveal, mark, scale);
            lastY = y;
        });

        const extra = b.txCount - k;
        if (extra > 0 && lastY !== null && t < T.commit) {
            const inBlock = t >= T.pack;
            text(`+${extra} more`, inBlock ? bp.x : t < T.order ? (t < T.endorse ? lerp(L.client + 44, L.endorse, ease(clamp(t / 0.9))) : L.endorse) : L.order,
                inBlock ? bp.y + b.blockH / 2 + 16 : t < T.vote ? lane(k - 1) + 22 : L.mid + 50,
                { size: 11, bold: true, color: C.dim });
        }
    }

    // ------------------------------------------------------------ loop
    let lastStatus = '';
    function updateStatus() {
        const active = playing.length ? playing[playing.length - 1] : null;
        const s = [
            paused ? 'Paused' : active ? `Replaying block #${active.number}` : 'Waiting for new blocks. Send some payments to see them move.',
            queue.length ? `${queue.length} waiting` : '',
            skipped ? `${skipped} block${skipped === 1 ? '' : 's'} arrived faster than the replay and went straight onto the ledger row` : '',
        ].filter(Boolean).join(' · ');
        if (s !== lastStatus) {
            statusEl.textContent = s;
            lastStatus = s;
        }
    }

    function frame(now) {
        const dt = Math.min(0.1, (now - lastFrame) / 1000);
        lastFrame = now;
        if (!paused) clock += dt * speed;

        while (queue.length > MAX_QUEUE) {
            addToChain(chainItem(queue.shift()));
            skipped++;
        }
        if (!paused && queue.length && clock - lastStart >= GAP) {
            const b = queue.shift();
            b.start = clock;
            lastStart = clock;
            playing.push(b);
        }

        let shift = 0;
        let voting = null;
        for (const b of playing) {
            const t = clock - b.start;
            if (t >= T.order && t < T.pack + 0.5) voting = b;
            if (t >= T.commit && t < T.done) shift = ease(clamp((t - T.commit) / (T.done - T.commit))) * SLOT_STEP;
            if (t >= T.vote && !b.captioned) {
                b.captioned = true;
                caption(b);
            }
            if (t >= T.validate + 0.9 && !b.fed) {
                b.fed = true;
                feed(b);
            }
        }

        ctx.clearRect(0, 0, W, H);
        drawScene(voting);
        drawChain(shift);
        for (const b of playing) drawBlock(b, clock - b.start);

        playing = playing.filter((b) => {
            if (clock - b.start < T.done) return true;
            addToChain(chainItem(b));
            return false;
        });
        updateStatus();
        requestAnimationFrame(frame);
    }

    // ------------------------------------------------------------ controls
    $('txf-speed').addEventListener('change', (e) => { speed = Number(e.target.value); });
    pauseBtn.addEventListener('click', () => {
        paused = !paused;
        pauseBtn.textContent = paused ? 'Resume' : 'Pause';
    });
    sendButtons.forEach((btn) => btn.addEventListener('click', async () => {
        const n = btn.dataset.count;
        sendButtons.forEach((b) => { b.disabled = true; });
        demoEl.textContent = `Sending ${n} payments…`;
        try {
            const res = await fetch(`/api/demo/payments/${n}`, { method: 'POST' });
            if (!res.ok) demoEl.textContent = (await res.json().catch(() => ({}))).error || `Could not start (HTTP ${res.status})`;
        } catch {
            demoEl.textContent = 'Could not reach the dashboard server.';
        }
    }));

    window.addEventListener('resize', layout);
    layout();
    requestAnimationFrame(frame);

    // Called by app.js for every state update.
    window.updateTxFlow = function (state) {
        for (const k in state.orderers) {
            const o = state.orderers[k];
            up[o.nodeId || `orderer${Number(k) + 1}`] = !!o.up;
        }
        onActivity(state.activity);
        if (state.demo) {
            sendButtons.forEach((b) => { b.disabled = state.demo.running; });
            if (state.demo.output) demoEl.textContent = state.demo.output;
        }
    };
})();
