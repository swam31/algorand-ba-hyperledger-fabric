"""Phase 2 exercises — every one runs, output goes verbatim into the PDF."""
import hashlib, os, itertools
from cryptography.hazmat.primitives.asymmetric.ed25519 import (
    Ed25519PrivateKey, Ed25519PublicKey)
from cryptography.hazmat.primitives import serialization
from cryptography.exceptions import InvalidSignature

def raw_pub(pk):  return pk.public_bytes(serialization.Encoding.Raw,
                                          serialization.PublicFormat.Raw)

print("="*72); print("EX 1 — Ed25519 signing is DETERMINISTIC"); print("="*72)
sk = Ed25519PrivateKey.from_private_bytes(bytes(range(32)))
pk = sk.public_key()
msg = b"pay alice 500"
s1, s2 = sk.sign(msg), sk.sign(msg)
print(f"  signature #1 : {s1.hex()[:48]}...")
print(f"  signature #2 : {s2.hex()[:48]}...")
print(f"  identical?     {s1 == s2}")
print(f"  verifies?      {sk.public_key().verify(s1, msg) is None}")
assert s1 == s2
print("  -> RFC 8032 derives the nonce from the key and message by hashing,")
print("     so there is exactly one signature per (key, message). No randomness.")

print()
print("="*72); print("EX 2 — a toy VRF built from that determinism"); print("="*72)
def vrf_prove(sk, alpha):
    proof = sk.sign(alpha)
    return hashlib.sha512(proof).digest(), proof
def vrf_verify(pk, alpha, beta, proof):
    try: pk.verify(proof, alpha)
    except InvalidSignature: return False
    return hashlib.sha512(proof).digest() == beta

alpha = b"seed-round-42|committee|step-3"
beta, proof = vrf_prove(sk, alpha)
print(f"  input  alpha : {alpha.decode()}")
print(f"  output beta  : {beta.hex()[:48]}...")
print(f"  verify(pk)   : {vrf_verify(pk, alpha, beta, proof)}")
bad = bytearray(beta); bad[0] ^= 0xFF
print(f"  tampered beta: {vrf_verify(pk, alpha, bytes(bad), proof)}")
other = Ed25519PrivateKey.from_private_bytes(bytes(range(1,33))).public_key()
print(f"  wrong pubkey : {vrf_verify(other, alpha, beta, proof)}")
print(f"  wrong input  : {vrf_verify(pk, b'different', beta, proof)}")
print("  -> unpredictable before you hold sk, verifiable by anyone after.")

print()
print("="*72); print("EX 3 — the same seed gives a DIFFERENT output per role"); print("="*72)
for role in [b"committee|r42|s1", b"committee|r42|s2", b"proposer|r42"]:
    bta, _ = vrf_prove(sk, role)
    print(f"  {role.decode():24} -> {bta.hex()[:32]}...")
print("  -> one seed, unlimited independent lotteries, zero coordination.")

print()
print("="*72); print("EX 4 — collision resistance costs HALF your bits"); print("="*72)
def find_collision(nbits):
    seen, n = {}, 0
    for i in itertools.count():
        d = hashlib.sha256(str(i).encode()).digest()
        t = int.from_bytes(d, 'big') >> (256 - nbits)
        n += 1
        if t in seen: return n, seen[t], i
        seen[t] = i
import math
print(f"  {'bits':>6} {'tries needed':>14} {'2^(n/2)':>12}")
for nbits in (16, 24, 32, 40):
    tries, a, b = find_collision(nbits)
    print(f"  {nbits:>6} {tries:>14,} {int(2**(nbits/2)):>12,}")
print("  -> the birthday bound: a collision appears after about 2^(n/2) tries,")
print("     not 2^n. SHA-256 therefore offers 128-bit collision resistance.")

print()
print("="*72); print("EX 5 — Merkle tree and an inclusion proof"); print("="*72)
def H(b): return hashlib.sha256(b).digest()
def merkle(leaves):
    lvl = [H(l) for l in leaves]; levels=[lvl]
    while len(lvl) > 1:
        if len(lvl) % 2: lvl = lvl + [lvl[-1]]
        lvl = [H(lvl[i]+lvl[i+1]) for i in range(0,len(lvl),2)]
        levels.append(lvl)
    return levels
def proof_for(levels, idx):
    path=[]
    for d in range(len(levels)-1):
        lvl = levels[d]
        if len(lvl) % 2: lvl = lvl + [lvl[-1]]
        sib = idx ^ 1
        path.append((lvl[sib], idx % 2 == 1))
        idx //= 2
    return path
def check(leaf, path, root):
    h = H(leaf)
    for sib, right in path:
        h = H(sib + h) if right else H(h + sib)
    return h == root

txs = [f"tx{i}".encode() for i in range(8)]
lv = merkle(txs); root = lv[-1][0]
pr = proof_for(lv, 5)
print(f"  8 transactions -> root {root.hex()[:32]}...")
print(f"  proof for tx5 is {len(pr)} hashes, not 8 transactions")
print(f"  verify tx5     : {check(txs[5], pr, root)}")
print(f"  verify a fake  : {check(b'tx-forged', pr, root)}")
print(f"  proof size grows as log2(n): 1024 txs -> {math.ceil(math.log2(1024))} hashes")
