"""Reference simulation of BinaryBA* mirroring engine/ba/ba.go exactly.

Each node is a generator whose control flow is a line-by-line match for the Go.
A driver steps them in lockstep and applies an adversarial network policy.
Purpose: prove the state machine is correct before compiling the Go.
"""
import math, hashlib, random
from collections import defaultdict

BLOCK, EMPTY = "BLOCK", "EMPTY"
TIMEOUT = None

class Params:
    def __init__(self, n=7, f=2, stake=10_000, tau=35_000.0, T=0.685, maxsteps=30):
        self.n, self.f, self.stake = n, f, stake
        self.tau, self.T, self.maxsteps = tau, T, maxsteps
        self.W = n * stake
        self.thr = T * tau

def seats(node_id, step, P, rng):
    """Binomial sortition: how many seats this node wins at this step."""
    p = P.tau / P.W
    r = random.Random(hashlib.sha256(f"{node_id}|{step}|{rng}".encode()).digest())
    # sum of `stake` Bernoulli(p) == Binomial(stake, p); use normal approx for speed
    mu, sd = P.stake*p, math.sqrt(P.stake*p*(1-p))
    return max(0, int(round(r.gauss(mu, sd))))

def node_proc(nid, P, initial, byzantine=False, byz_value=None):
    """Generator implementing BinaryBA* (Algorithm 8). Mirrors ba.go:binaryBA."""
    r = initial
    step = 1
    while step < P.maxsteps:
        # ---- sub-step A: can only commit to BLOCK ----
        v = yield ('vc', step, byz_value if byzantine else r)
        if v is TIMEOUT:
            r = BLOCK
        elif v != EMPTY:
            r = v
            for s in range(step+1, step+4):
                yield ('v', s, r)
            if step == 1:
                yield ('v', 'FINAL', r)
            return ('decided', r, step)
        else:
            r = v
        step += 1

        # ---- sub-step B: can only commit to EMPTY ----
        v = yield ('vc', step, byz_value if byzantine else r)
        if v is TIMEOUT:
            r = EMPTY
        elif v == EMPTY:
            r = v
            for s in range(step+1, step+4):
                yield ('v', s, r)
            return ('decided', r, step)
        else:
            r = v
        step += 1

        # ---- sub-step C: common coin ----
        v = yield ('vc', step, byz_value if byzantine else r)
        if v is TIMEOUT:
            coin = yield ('coin', step, None)
            r = BLOCK if coin == 0 else EMPTY
        else:
            r = v
        step += 1
    return ('maxsteps', None, step)


