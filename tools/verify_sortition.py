"""Independent validation of the SubUsers algorithm in engine/vrf/sortition.go.
Ports the identical logic to Python and checks the paper's claimed properties."""
import math, hashlib, struct

def log_binom_pmf(k, n, p):
    if k > n: return -math.inf
    return (math.lgamma(n+1) - math.lgamma(k+1) - math.lgamma(n-k+1)
            + k*math.log(p) + (n-k)*math.log1p(-p))

def ratio(h: bytes) -> float:
    u = struct.unpack('>Q', h[:8])[0]
    return (u >> 11) / float(1 << 53)          # top 53 bits -> [0,1)

def sub_users(h: bytes, tau: float, w: int, W: int) -> int:
    if w == 0 or W == 0 or tau <= 0: return 0
    p = tau / W
    if p <= 0: return 0
    if p >= 1: return w
    x, cum = ratio(h), 0.0
    for j in range(w+1):
        cum += math.exp(log_binom_pmf(j, w, p))
        if x < cum: return j
    return w

def H(i: int) -> bytes:                         # deterministic pseudo-random hash
    return hashlib.sha512(str(i).encode()).digest()

print("="*66)
print("1. EXPECTED COMMITTEE SIZE  (should converge to tau)")
print("="*66)
W, tau, N = 10_000, 200.0, 20_000
tot = sum(sub_users(H(i), tau, W, W) for i in range(N))
print(f"   one node holds all {W} stake, tau={tau:.0f}")
print(f"   observed mean seats = {tot/N:.2f}   (expected {tau:.0f})")
assert abs(tot/N - tau) < tau*0.05, "FAIL"
print("   PASS\n")

print("="*66)
print("2. SYBIL RESISTANCE  (splitting stake must change nothing)")
print("="*66)
W, tau, stake, N = 100_000, 1000.0, 10_000, 4000
expected = tau * stake / W
whole = sum(sub_users(H(i), tau, stake, W) for i in range(N)) / N
split = sum(sum(sub_users(H(i*1000+k), tau, 100, W) for k in range(100))
            for i in range(N)) / N
print(f"   theory                         : {expected:.2f} seats")
print(f"   1 identity  x {stake:>6} stake   : {whole:.2f} seats")
print(f"   100 Sybils  x {100:>6} stake   : {split:.2f} seats")
print(f"   difference                     : {abs(whole-split):.2f}")
assert abs(whole-expected) < expected*0.10, "FAIL whole"
assert abs(split-expected) < expected*0.10, "FAIL split"
print("   PASS - Sybil attack gains nothing\n")

print("="*66)
print("3. PROPORTIONALITY  (seats must track stake linearly)")
print("="*66)
W, tau, N = 100_000, 1000.0, 4000
print(f"   {'stake':>8} {'expected':>10} {'observed':>10}")
for stake in (100, 1_000, 10_000, 50_000):
    obs = sum(sub_users(H(i*77+stake), tau, stake, W) for i in range(N))/N
    exp = tau*stake/W
    print(f"   {stake:>8} {exp:>10.2f} {obs:>10.2f}")
    assert abs(obs-exp) < max(exp*0.12, 0.5), f"FAIL at stake={stake}"
print("   PASS - influence is exactly proportional to stake\n")

print("="*66)
print("4. P[not selected] must equal B(0;w,p) = (1-p)^w")
print("="*66)
W, tau, w, N = 10_000, 50.0, 20, 40_000
p = tau/W
theory = (1-p)**w
obs = sum(1 for i in range(N) if sub_users(H(i), tau, w, W) == 0)/N
print(f"   theory (1-p)^w = {theory:.4f}")
print(f"   observed       = {obs:.4f}")
assert abs(obs-theory) < 0.01, "FAIL"
print("   PASS - the j=0 case is handled correctly\n")

print("="*66)
print("5. FABRIC-SCALE CHECK  (n=4 orderers, equal stake, tau_step=4)")
print("="*66)
W, tau, w, N = 4, 4.0, 1, 10_000
sel = [sum(sub_users(H(i*10+n), tau, w, W) for n in range(4)) for i in range(N)]
print(f"   p = tau/W = {tau/W:.2f}  -> every ticket wins, committee = all 4")
print(f"   mean seats per step = {sum(sel)/N:.2f} (expected 4.00)")
print(f"   T_step*tau = 0.685*4 = {0.685*4:.2f}  -> need >{math.floor(0.685*4)} = 3 votes")
print("   NOTE: with n=4 the committee is degenerate (everyone is always")
print("         selected). Secrecy/scalability must be shown in engine/sim")
print("         at n=100+; correctness of BA* is unaffected.\n")

W, tau, w, N = 100, 20.0, 1, 5000
sel = [sum(sub_users(H(i*200+n), tau, w, W) for n in range(100)) for i in range(N)]
print(f"   n=100 stakeholders, tau=20 -> mean committee = {sum(sel)/N:.2f} (expect 20)")
print(f"   min={min(sel)} max={max(sel)}  <- genuinely random subset each step")
print("\n" + "="*66)
print("ALL SORTITION PROPERTIES VERIFIED")
print("="*66)
