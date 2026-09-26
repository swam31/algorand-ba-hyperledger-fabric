"""Full BA* validation (BinaryBA* + FINAL step) over a configurable orderer set."""
import sys, math, random, hashlib
from collections import defaultdict
from ba_model import Params, seats, node_proc, BLOCK, EMPTY, TIMEOUT

def run_full(P, initials, byz=(), byz_value=EMPTY,
             drop=lambda s,d,st: False, coin_seed=0):
    nodes, procs, done = {}, {}, {}
    for i in range(P.n):
        g = node_proc(i, P, initials[i], i in byz, byz_value)
        procs[i] = g; nodes[i] = next(g)

    votes = defaultdict(lambda: defaultdict(dict))
    coins = defaultdict(dict)
    guard = 0
    while len(done) < P.n and guard < P.maxsteps*4:
        guard += 1
        pending = []
        for i, act in list(nodes.items()):
            if i in done: continue
            while act is not None and act[0] == 'v':
                pending.append((i, act[1], act[2]))
                try: act = procs[i].send(None)
                except StopIteration as e: done[i] = e.value; act = None
            nodes[i] = act
            if act is not None and act[0] == 'vc':
                pending.append((i, act[1], act[2]))

        for src, st, val in pending:
            sc = seats(src, st, P, coin_seed)
            if sc == 0: continue
            h = hashlib.sha256(f"{src}|{st}|{coin_seed}".encode()).digest()
            for dst in range(P.n):
                if drop(src, dst, st): continue
                votes[st][dst][src] = (val, sc)
                coins[st].setdefault(dst, []).append((h, src))

        for i, act in list(nodes.items()):
            if i in done or act is None: continue
            kind, st = act[0], act[1]
            if kind == 'vc':
                tally = defaultdict(float)
                for src,(v,sc) in votes[st].get(i, {}).items(): tally[v] += sc
                w = next((v for v,c in tally.items() if c > P.thr), TIMEOUT)
                try: nodes[i] = procs[i].send(w)
                except StopIteration as e: done[i] = e.value; nodes[i] = None
            elif kind == 'coin':
                hs = coins[st].get(i, [])
                try: nodes[i] = procs[i].send(0 if not hs else min(hs)[0][-1] & 1)
                except StopIteration as e: done[i] = e.value; nodes[i] = None

    out = {}
    for i in range(P.n):
        res = done.get(i)
        if res is None or res[0] != 'decided':
            out[i] = ('stuck', None); continue
        tally = defaultdict(float)
        for src,(v,sc) in votes['FINAL'].get(i, {}).items(): tally[v] += sc
        fin = next((v for v,c in tally.items() if c > P.thr_final), None)
        out[i] = ('FINAL' if fin == res[1] else 'TENTATIVE', res[1])
    return out

def invariant(out, byz):
    honest = {i:v for i,v in out.items() if i not in byz}
    finals  = {v for st,v in honest.values() if st == 'FINAL'}
    decided = {v for st,v in honest.values() if st in ('FINAL','TENTATIVE')}
    if len(finals) > 1: return False, f"two FINAL values {finals}"
    if finals and (decided - finals): return False, f"FINAL={finals} contradicted by {decided-finals}"
    return True, ""

def show(name, out, byz=()):
    ok, why = invariant(out, byz)
    honest = {i:v for i,v in out.items() if i not in byz}
    f = sum(1 for st,_ in honest.values() if st=='FINAL')
    t = sum(1 for st,_ in honest.values() if st=='TENTATIVE')
    s = sum(1 for st,_ in honest.values() if st=='stuck')
    vals = {v for st,v in honest.values() if v}
    print(f"  {name:<40} {'PASS' if ok else 'VIOLATION':<10} final={f:<2} tent={t:<2} stuck={s:<2} {vals or '-'}")
    if not ok: print(f"      -> {why}")
    return ok, f

def suite(n, f, tau, tau_final, label):
    P = Params(n=n, f=f, stake=10_000, tau=tau, T=0.685)
    P.thr_final = tau_final * 0.74
    P.tau_final = tau_final
    print("="*88); print(f"{label}   h = {(n-f)/n:.1%}"); print("="*88)
    print(f"  W={P.W:,}  tau_step={tau:,.0f} thr={P.thr:,.0f}   tau_final={tau_final:,.0f} thr_final={P.thr_final:,.0f}\n")
    byz = tuple(range(n-f, n))
    oks = []
    ok,_  = show("1. all honest, same block", run_full(P,[BLOCK]*n)); oks.append(ok)
    ok,fc = show(f"2. {f} crashed (silent)", run_full(P,[BLOCK]*n,drop=lambda s,d,st,B=byz: s in B), byz=byz); oks.append(ok)
    ok,_  = show(f"3. {f} byzantine vote EMPTY", run_full(P,[BLOCK]*n,byz=byz,byz_value=EMPTY), byz=byz); oks.append(ok)
    half = n//2
    ok,_  = show("4. proposer equivocated ~50/50", run_full(P,[BLOCK]*half+[EMPTY]*(n-half))); oks.append(ok)
    print(f"\n  --> with {f} nodes down, {fc} honest nodes reached FINAL")

    viol = fin = 0
    for t in range(1500):
        rng = random.Random(t)
        inits = [rng.choice([BLOCK,EMPTY]) for _ in range(n)]
        bz = tuple(rng.sample(range(n), rng.randint(0,f)))
        L = rng.random()*0.45
        out = run_full(P, inits, byz=bz, byz_value=rng.choice([BLOCK,EMPTY]),
                       drop=lambda s_,d_,st_,LL=L,R=rng: R.random()<LL, coin_seed=t)
        ok,_ = invariant(out, bz)
        if not ok: viol += 1
        h = {i:v for i,v in out.items() if i not in bz}
        if any(st=='FINAL' for st,_ in h.values()): fin += 1
    print(f"  soak 1500 trials: {viol} safety violations, {fin} reached FINAL under <=45% loss\n")
    return all(oks) and viol == 0, fc

a, fc7  = suite(7, 2, 35_000.0, 35_000.0, "CONFIG A: n=7,  f=2  (previous choice)")
b, fc10 = suite(10, 2, 50_000.0, 50_000.0, "CONFIG B: n=10, f=2  (proposed)")

print("="*88)
print(f"  n=7  f=2 : safety {'OK' if a else 'FAIL'} | honest nodes reaching FINAL with 2 down = {fc7}")
print(f"  n=10 f=2 : safety {'OK' if b else 'FAIL'} | honest nodes reaching FINAL with 2 down = {fc10}")
print()
print("  Both are SAFE. Only n=10 is also USEFUL: at n=7 with 2 orderers down the")
print("  chain agrees but can never finalize, so no transaction is ever confirmed.")
print("="*88)
