"""Phase 2 diagrams 6-10: the VRF, sortition, and the binomial."""
import math
from pdfbase_kit import *
from pdfdraw import *
from pdfdiagrams_a import Dia

def binom(k, n, p):
    return math.comb(n, k) * p**k * (1-p)**(n-k)

# ------------------------------------------------- 6. hash vs signature vs VRF
class P6ThreeTools(Dia):
    H = 228
    CAP = "Figure 6 — The VRF is the only one of the three that is private first and public afterwards."
    def render(self, c, W):
        panel = (W - 20)/3
        cols = [
            ("HASH", INK_FAINT, SURFACE, [
                ("input", "anyone's data"),
                ("output", "same for everyone"),
                ("secret?", "no secret at all"),
                ("who can compute", "anybody"),
                ("who can verify", "anybody"),
            ], "Public randomness.\nNothing is yours."),
            ("SIGNATURE", GOLD, GOLD_SOFT, [
                ("input", "a message"),
                ("output", "proof of intent"),
                ("secret?", "yes, your sk"),
                ("who can compute", "only you"),
                ("who can verify", "anybody"),
            ], "Proves you agreed.\nBut YOU chose the\nmessage."),
            ("VRF", ACCENT, ACCENT_SOFT, [
                ("input", "a public seed"),
                ("output", "a random number"),
                ("secret?", "yes, your sk"),
                ("who can compute", "only you"),
                ("who can verify", "anybody"),
            ], "A random number you\nCANNOT choose, and\nCAN prove."),
        ]
        for k, (name, col, bg, rows, note) in enumerate(cols):
            ox = k*(panel+10)
            box(c, ox, 44, panel, self.H-52, PAPER, col, 1.0 if k==2 else 0.8)
            c.setFillColor(col); c.rect(ox, self.H-26, panel, 18, stroke=0, fill=1)
            txt(c, ox+panel/2, self.H-20, name, "Label", 9, PAPER, "c")
            yy = self.H - 44
            for lab, val in rows:
                txt(c, ox+8, yy, lab, "Body-I", 6.6, INK_FAINT)
                txt(c, ox+8, yy-9, val, "Body-B" if k==2 else "Body", 7.4, INK)
                yy -= 22
            c.setFillColor(bg); c.rect(ox+1, 45, panel-2, 44, stroke=0, fill=1)
            ny = 76
            for line in note.split("\n"):
                txt(c, ox+8, ny, line, "Body-BI" if k==2 else "Body-I", 7.4,
                    col if k==2 else INK); ny -= 10
        txt(c, 0, 34, "THE ONE PROPERTY THAT MATTERS:", "Label", 7.8, ACCENT)
        txt(c, 0, 22,
            "A signature lets you pick what you sign. A VRF does not — the output is fixed by "
            "your key and the",
            "Body", 8, INK)
        txt(c, 0, 12,
            "seed before you ever touch it. That is why it can run a lottery nobody can rig.",
            "Body-B", 8, INK)

# ------------------------------------------------- 7. VRF in sortition
class P7Sortition(Dia):
    H = 258
    CAP = "Figure 7 — The whole mechanism: you find out alone, everyone finds out afterwards."
    def render(self, c, W):
        # --- stage 1: private
        txt(c, 0, self.H-10, "STAGE 1  ·  ALONE, OFFLINE, IN SECRET", "Label", 8, SIGNAL)
        y1 = self.H - 92
        box(c, 0, y1, 130, 68, SIGNAL_SOFT, SIGNAL, 0.9)
        txt(c, 8, y1+56, "YOUR NODE", "Label", 7.2, SIGNAL)
        txt(c, 8, y1+43, "sk  (private key)", "Mono", 7, INK)
        txt(c, 8, y1+31, "seed  (public)", "Mono", 7, INK)
        txt(c, 8, y1+19, "role = committee|", "Mono", 7, INK)
        txt(c, 8, y1+9,  "       r42|step3", "Mono", 7, INK)
        arrow(c, 134, y1+34, 162, y1+34, SIGNAL, 1.1)
        box(c, 166, y1+14, 92, 40, INK, INK, 1.0)
        txt(c, 212, y1+36, "VRF", "Label", 10, PAPER, "c")
        txt(c, 212, y1+24, "sk, seed||role", "Mono", 6.2, ACCENT_MID, "c")
        arrow(c, 262, y1+34, 290, y1+34, SIGNAL, 1.1)
        box(c, 294, y1+14, W-294, 40, PAPER, SIGNAL, 0.9)
        txt(c, 300, y1+40, "hash  6f195cdb1b70cc8d...", "Mono", 6.8, SIGNAL)
        txt(c, 300, y1+29, "proof eae436f7d3b7698b...", "Mono", 6.8, SIGNAL)
        txt(c, 300, y1+18, "-> 3 seats won", "Mono", 7, ACCENT)
        txt(c, 0, y1-14,
            "Nobody else has run this. Nobody else CAN run it — it needs your sk.",
            "Body-B", 8, SIGNAL)
        txt(c, 0, y1-25,
            "An attacker who wants to silence the committee has no idea you are on it.",
            "Body", 7.8, INK)

        c.setStrokeColor(RULE); c.setLineWidth(0.6); c.setDash(2,2)
        c.line(0, y1-36, W, y1-36); c.setDash()

        # --- stage 2: public
        txt(c, 0, y1-56, "STAGE 2  ·  YOU SPEAK, AND ONLY THEN ARE YOU VISIBLE", "Label", 8, ACCENT)
        y2 = y1 - 126
        box(c, 0, y2, 150, 50, ACCENT_SOFT, ACCENT, 0.9)
        txt(c, 8, y2+38, "GOSSIPED VOTE", "Label", 7.2, ACCENT)
        txt(c, 8, y2+26, "value + hash + proof", "Mono", 6.8, INK)
        txt(c, 8, y2+14, "signed by you", "Mono", 6.8, INK)
        txt(c, 8, y2+4,  "about 200 bytes", "Body-I", 6.6, INK_FAINT)
        for k in range(3):
            arrow(c, 155, y2+25, 196, y2+8+k*17, ACCENT, 0.9, 3.4)
        for k in range(3):
            node(c, 210, y2+8+k*17, 8, PAPER, INK_SOFT, "", 0.9)
        txt(c, 226, y2+38, "every other node runs VerifySort(pk, hash, proof, seed, role)",
            "Body", 7.6, INK)
        txt(c, 226, y2+27, "and independently confirms: yes, 3 seats, legitimately won.",
            "Body", 7.6, INK)
        txt(c, 226, y2+13, "They never see your sk. You never send it.",
            "Body-BI", 7.6, ACCENT)
        txt(c, 0, y2-16,
            "By the time an attacker learns who you are, your vote is already gossiped and you are",
            "Body-B", 8, INK)
        txt(c, 0, y2-27,
            "irrelevant — a fresh committee is drawn for the next step. Attacking you achieves nothing.",
            "Body-B", 8, INK)

# ------------------------------------------------- 8. binomial buckets
class P8Binomial(Dia):
    H = 200
    CAP = "Figure 8 — Your hash is a dart. Where it lands decides how many seats you won."
    def render(self, c, W):
        w, p = 10, 0.30
        txt(c, 0, self.H-10, f"A NODE HOLDING w = {w} TICKETS, EACH WINNING WITH p = {p}",
            "Label", 8, INK)
        txt(c, 0, self.H-24,
            "Slice the line from 0 to 1 into buckets, one per possible number of wins,",
            "Body", 8, INK)
        txt(c, 0, self.H-35,
            "each as wide as its probability. That is the binomial distribution.",
            "Body", 8, INK)

        bar_y, bar_h = self.H-92, 34
        x = 0.0
        landed = 3
        for k in range(w+1):
            pk = binom(k, w, p)
            bw = pk * W
            if bw < 0.6: 
                x += bw; continue
            on = (k == landed)
            box(c, x, bar_y, bw, bar_h, ACCENT_SOFT if on else SURFACE,
                ACCENT if on else RULE, 1.0 if on else 0.6)
            if bw > 15:
                txt(c, x+bw/2, bar_y+bar_h/2-3, str(k), "Mono", 7.4,
                    ACCENT if on else INK_SOFT, "c")
                txt(c, x+bw/2, bar_y-10, f"{pk:.3f}", "Mono", 5.8, INK_FAINT, "c")
            x += bw
        txt(c, 0, bar_y+bar_h+6, "0.0", "Mono", 6.4, INK_FAINT)
        txt(c, W, bar_y+bar_h+6, "1.0", "Mono", 6.4, INK_FAINT, "r")

        # the dart
        dx = sum(binom(k, w, p) for k in range(landed)) * W + binom(landed, w, p)*W*0.5
        arrow(c, dx, bar_y+bar_h+30, dx, bar_y+bar_h+3, SIGNAL, 1.3, 5)
        txt(c, dx, bar_y+bar_h+34, "hash / 2^256  =  0.41", "Mono", 7, SIGNAL, "c")
        txt(c, dx, bar_y-26, "lands in bucket 3", "Body-B", 7.8, ACCENT, "c")
        txt(c, dx, bar_y-36, "-> this node casts 3 votes", "Body-B", 7.8, ACCENT, "c")

        txt(c, 0, 30, "WHY BUCKETS AND NOT A SIMPLE COIN FLIP:", "Label", 7.8, ACCENT)
        txt(c, 0, 18,
            "One vote per node would mean a node with 1000 coins has the same say as one with 1.",
            "Body", 7.8, INK)
        txt(c, 0, 7,
            "One draw per COIN makes influence exactly proportional to stake — and kills Sybils. See Figure 9.",
            "Body", 7.8, INK)

# ------------------------------------------------- 9. Sybil invariance
class P9Sybil(Dia):
    H = 186
    CAP = "Figure 9 — Splitting a stake across fake identities changes nothing. Verified in your own tools."
    def render(self, c, W):
        half = W/2 - 14
        txt(c, 0, self.H-10, "ONE HONEST IDENTITY", "Label", 8, ACCENT)
        node(c, 40, self.H-56, 20, ACCENT_SOFT, ACCENT, "", 1.2)
        txt(c, 40, self.H-59, "1", "Label", 10, ACCENT, "c")
        txt(c, 74, self.H-46, "holds 10,000 coins", "Body", 8, INK)
        txt(c, 74, self.H-58, "= 10,000 lottery tickets", "Body-I", 7.6, INK_FAINT)
        box(c, 0, self.H-104, half, 26, ACCENT_SOFT, ACCENT, 0.9)
        txt(c, half/2, self.H-92, "expected seats:  100.0", "Mono", 8.4, ACCENT, "c")
        txt(c, 0, self.H-120, "measured over 4,000 trials:  99.84", "Mono", 7.2, INK_SOFT)

        c.setStrokeColor(RULE); c.setDash(2,3); c.setLineWidth(0.7)
        c.line(W/2, 20, W/2, self.H-4); c.setDash()

        ox = W/2 + 14
        txt(c, ox, self.H-10, "100 SYBIL IDENTITIES", "Label", 8, SIGNAL)
        for r in range(4):
            for k in range(12):
                node(c, ox+8+k*13, self.H-40-r*13, 4.6, SIGNAL_SOFT, SIGNAL, "", 0.7)
        txt(c, ox+172, self.H-46, "100 accounts,", "Body", 8, INK)
        txt(c, ox+172, self.H-58, "100 coins each", "Body", 8, INK)
        box(c, ox, self.H-104, half, 26, SIGNAL_SOFT, SIGNAL, 0.9)
        txt(c, ox+half/2, self.H-92, "expected seats:  100.0", "Mono", 8.4, SIGNAL, "c")
        txt(c, ox, self.H-120, "measured over 4,000 trials:  100.32", "Mono", 7.2, INK_SOFT)

        box(c, 0, 6, W, 40, SURFACE, INK, 0.9)
        txt(c, 8, 32, "The identity that makes this work:", "Body-B", 8, INK)
        txt(c, 8, 20, "B(k1; n1, p)  +  B(k2; n2, p)   =   B(k1+k2; n1+n2, p)",
            "Mono", 8.6, ACCENT)
        txt(c, 8, 10, "Two small binomials add up to one big one. The split is invisible to the maths.",
            "Body-I", 7.4, INK_SOFT)

# ------------------------------------------------- 10. the shortcut and its gap
class P10Shortcut(Dia):
    H = 214
    CAP = "Figure 10 — What your engine actually ships, and precisely where it falls short."
    def render(self, c, W):
        txt(c, 0, self.H-10, "WHAT engine/vrf/vrf.go DOES", "Label", 8, INK)
        y = self.H - 62
        box(c, 0, y, 108, 42, SURFACE, INK_SOFT, 0.9)
        txt(c, 54, y+27, "sign(sk, alpha)", "Mono", 7, INK, "c")
        txt(c, 54, y+15, "Ed25519", "Body-I", 6.8, INK_FAINT, "c")
        txt(c, 54, y+5,  "deterministic", "Body-I", 6.8, ACCENT, "c")
        arrow(c, 112, y+21, 140, y+21, ACCENT, 1.1)
        box(c, 144, y, 96, 42, ACCENT_SOFT, ACCENT, 0.9)
        txt(c, 192, y+27, "= the PROOF", "Body-B", 7.6, ACCENT, "c")
        txt(c, 192, y+14, "verifiable by pk", "Body-I", 6.8, INK, "c")
        arrow(c, 244, y+21, 272, y+21, ACCENT, 1.1)
        box(c, 276, y, W-276, 42, ACCENT_SOFT, ACCENT, 0.9)
        txt(c, 282, y+27, "SHA-512(proof)", "Mono", 7.4, ACCENT)
        txt(c, 282, y+14, "= the pseudorandom OUTPUT", "Body-I", 6.8, INK)

        c.setStrokeColor(RULE); c.setLineWidth(0.6); c.setDash(2,2)
        c.line(0, y-16, W, y-16); c.setDash()

        txt(c, 0, y-36, "WHERE IT IS NOT A REAL VRF", "Label", 8, SIGNAL)
        box(c, 0, y-116, W, 70, SIGNAL_SOFT, SIGNAL, 0.9)
        txt(c, 10, y-56, "A true VRF guarantees UNIQUENESS even when the key was chosen by an attacker:",
            "Body-B", 7.8, INK)
        txt(c, 10, y-68, "for one (sk, input) there is exactly ONE output that will ever verify.",
            "Body", 7.8, INK)
        txt(c, 10, y-84, "Ed25519 does not promise that. With non-canonical encodings or small-order",
            "Body", 7.8, INK)
        txt(c, 10, y-95, "points, a malicious key holder can, in some verifier implementations, produce",
            "Body", 7.8, INK)
        txt(c, 10, y-106, "more than one signature that verifies — and therefore more than one \"random\" draw.",
            "Body", 7.8, INK)

        txt(c, 0, 36, "WHY IT IS STILL ACCEPTABLE FOR YOUR PROJECT:", "Label", 7.6, ACCENT)
        txt(c, 0, 24, "Fabric is permissioned — orderer keys are issued by a CA under an MSP, not chosen "
                      "by an attacker.", "Body", 7.8, INK)
        txt(c, 0, 12, "The interface is swappable: drop in an RFC 9381 ECVRF and no other file changes.",
            "Body-B", 7.8, ACCENT)
