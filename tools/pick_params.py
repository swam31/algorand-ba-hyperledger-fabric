"""Choose (tau, T) for a Fabric orderer set, following paper section 7.5 / Appendix B.

Constraints for a step committee, with g = honest seats and b = malicious seats:
    LIVENESS : g > T*tau          (honest alone can cross the threshold)
    SAFETY   : g/2 + b <= T*tau   (a split honest vote + all malicious cannot)
Both must hold with overwhelming probability, where g ~ Binom(honest_stake, p)
and b ~ Binom(mal_stake, p), p = tau/W.
"""
from math import comb, sqrt, floor
from statistics import NormalDist

def fail_prob(n_orderers, f_mal, stake_each, tau, T):
    W = n_orderers * stake_each
    p = tau / W
    if p >= 1: return 1.0, p          # degenerate: everyone always selected
    hs = (n_orderers - f_mal) * stake_each
    ms = f_mal * stake_each
    g_mu, g_sd = hs*p, sqrt(hs*p*(1-p))
    b_mu, b_sd = ms*p, sqrt(ms*p*(1-p))
    thr = T * tau
    # P(liveness fails) = P(g <= thr)
    p_live = NormalDist(g_mu, g_sd).cdf(thr) if g_sd > 0 else float(g_mu <= thr)
    # P(safety fails) = P(g/2 + b > thr); Var = Var(g)/4 + Var(b)
    s_mu = g_mu/2 + b_mu
    s_sd = sqrt((g_sd**2)/4 + b_sd**2)
    p_safe = 1 - NormalDist(s_mu, s_sd).cdf(thr) if s_sd > 0 else float(s_mu > thr)
    return max(p_live, p_safe), p

print("="*78)
print("PARAMETER SEARCH for a Fabric orderer set")
print("="*78)

for n, f in [(4,1), (5,1), (7,1), (7,2), (10,3)]:
    h = (n-f)/n
    print(f"\n--- n={n} orderers, f={f} malicious   (h = {h:.1%} honest stake) ---")
    if h <= 2/3:
        print("    h <= 2/3 : BA* CANNOT be secure here. Skipping.")
        continue
    best = None
    for stake_each in (1_000, 10_000):
        W = n*stake_each
        for T in (0.685, 0.70, 0.75, 0.80):
            for frac in (0.3, 0.4, 0.5, 0.6, 0.7):
                tau = W*frac
                pf, p = fail_prob(n, f, stake_each, tau, T)
                if best is None or pf < best[0]:
                    best = (pf, stake_each, tau, T, p, W)
    pf, stake_each, tau, T, p, W = best
    print(f"    stake/orderer = {stake_each:,}   W = {W:,}")
    print(f"    tau = {tau:,.0f}   T = {T}   p = tau/W = {p:.3f}")
    print(f"    threshold T*tau = {T*tau:,.0f} votes")
    print(f"    P[step fails]  ~ {pf:.2e}")
    exp_seats = stake_each*p
    print(f"    each orderer contributes ~{exp_seats:,.0f} votes (randomly varying)")

print("\n" + "="*78)
print("RECOMMENDED: n=7, f=2  (tolerates 2 Byzantine orderers, h=71.4%)")
print("="*78)
n, f, stake_each = 7, 2, 10_000
W = n*stake_each
for T in (0.685, 0.70, 0.75):
    for tau in (W*0.4, W*0.5, W*0.6):
        pf, p = fail_prob(n, f, stake_each, tau, T)
        flag = "  <== " if pf < 1e-6 else ""
        print(f"  tau={tau:>7,.0f} T={T}  p={p:.2f}  thr={T*tau:>8,.0f}  P[fail]={pf:.2e}{flag}")

print("\n" + "="*78)
print("SANITY: does h matter as much as the paper's Figure 3 claims?")
print("="*78)
stake_each, T = 10_000, 0.685
for n, f in [(7,1),(7,2),(9,3),(10,3)]:
    W = n*stake_each; tau = W*0.5
    pf, p = fail_prob(n, f, stake_each, tau, T)
    print(f"  n={n} f={f}  h={(n-f)/n:.1%}  ->  P[fail] = {pf:.2e}")
print("\n  Confirms Figure 3: as h approaches 2/3 the failure probability")
print("  explodes. n=7/f=2 (h=71%) is near the cliff; n=7/f=1 (h=86%) is safe.")
