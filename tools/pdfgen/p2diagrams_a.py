"""Phase 2 diagrams 1-5: hashing depth, Merkle, one-way functions, Ed25519."""
import math
from pdfbase_kit import *
from pdfdraw import *
from pdfdiagrams_a import Dia

# ------------------------------------------------- 1. three hash resistances
class P1Resistances(Dia):
    H = 204
    CAP = "Figure 1 — Three different promises, each harder to break than it sounds."
    def render(self, c, W):
        panel = (W - 20) / 3
        specs = [
            ("PREIMAGE", "Given the OUTPUT,\nfind ANY input.", "h", "? -> h",
             "2^256 work", ACCENT),
            ("SECOND PREIMAGE", "Given ONE input,\nfind a DIFFERENT\none with the same\noutput.", "x", "x' != x",
             "2^256 work", ACCENT),
            ("COLLISION", "Find ANY TWO\ninputs that\nmatch.", "any", "x, x'",
             "2^128 work", SIGNAL),
        ]
        for k, (name, desc, a, bl, cost, col) in enumerate(specs):
            ox = k*(panel+10)
            box(c, ox, 34, panel, self.H-42, PAPER, col, 0.9)
            c.setFillColor(col); c.rect(ox, self.H-24, panel, 16, stroke=0, fill=1)
            txt(c, ox+panel/2, self.H-19, name, "Label", 7.6, PAPER, "c")
            yy = self.H - 42
            for line in desc.split("\n"):
                txt(c, ox+9, yy, line, "Body", 8, INK); yy -= 11
            # little schematic
            sy = 62
            box(c, ox+12, sy, 40, 20, SURFACE, INK_FAINT, 0.7)
            txt(c, ox+32, sy+7, a, "Mono", 7.4, INK, "c")
            arrow(c, ox+55, sy+10, ox+panel-56, sy+10, INK_FAINT, 0.9)
            box(c, ox+panel-52, sy, 40, 20, SURFACE, INK_FAINT, 0.7)
            txt(c, ox+panel-32, sy+7, bl, "Mono", 7.4, INK, "c")
            chip(c, ox+12, 42, cost, col, PAPER if col==SIGNAL else ACCENT_SOFT, 7)
        txt(c, 0, 20, "COLLISION IS THE WEAK ONE — AND IT IS THE ONE A BLOCKCHAIN LEANS ON.",
            "Label", 7.6, SIGNAL)
        txt(c, 0, 10, "Half the bit-length, because of the birthday bound. See Figure 2.",
            "Body-I", 7.4, INK_FAINT)

# ------------------------------------------------- 2. birthday bound
class P2Birthday(Dia):
    H = 176
    CAP = "Figure 2 — Measured, not asserted: collisions appear after roughly the square root of the space."
    def render(self, c, W):
        data = [(16, 251, 256), (24, 8864, 4096), (32, 95303, 65536), (40, 1526146, 1048576)]
        txt(c, 0, self.H-10, "WHY 'HALF THE BITS'", "Label", 8, INK)
        txt(c, 0, self.H-24,
            "With n possible outputs, you might expect to try n times for a match. You do not.",
            "Body", 8.2, INK)
        txt(c, 0, self.H-35,
            "Every NEW item is compared against every one already seen, so pairs grow quadratically.",
            "Body", 8.2, INK)

        # bar chart, log scale
        x0, y0, hgt = 8, 26, 78
        maxv = math.log10(2_000_000)
        colw = (W - 80) / 4
        for k, (bits, tries, theory) in enumerate(data):
            bx = x0 + k*colw
            ht = (math.log10(tries)/maxv) * hgt
            ht2 = (math.log10(theory)/maxv) * hgt
            box(c, bx, y0, colw*0.34, ht, ACCENT_SOFT, ACCENT, 0.8)
            box(c, bx+colw*0.40, y0, colw*0.34, ht2, SURFACE, INK_FAINT, 0.8, dash=(2,2))
            txt(c, bx+colw*0.17, y0+ht+4, f"{tries:,}", "Mono", 6.4, ACCENT, "c")
            txt(c, bx+colw*0.57, y0+ht2+4, f"{theory:,}", "Mono", 6.4, INK_FAINT, "c")
            txt(c, bx+colw*0.37, y0-11, f"{bits}-bit", "Label", 7.4, INK, "c")
        chip(c, W-70, y0+hgt-6, "MEASURED", ACCENT, ACCENT_SOFT, 6.6)
        chip(c, W-70, y0+hgt-22, "2^(n/2)", INK_FAINT, SURFACE, 6.6)
        txt(c, 0, 6, "SHA-256 IS THEREFORE A 128-BIT SECURITY LEVEL AGAINST COLLISIONS, NOT 256.",
            "Label", 7.6, SIGNAL)

# ------------------------------------------------- 3. Merkle tree
class P3Merkle(Dia):
    H = 210
    CAP = "Figure 3 — Prove one transaction is in a block of a thousand, using ten hashes."
    def render(self, c, W):
        lw, lh, gap = 46, 18, 8
        n = 8
        base_y = 96
        xs = [k*(lw+gap) for k in range(n)]
        span = xs[-1]+lw
        off = (W - span)/2
        path = {5: True}                      # the leaf we prove
        sibs = {4: True}                      # its sibling
        # leaves
        for k in range(n):
            x = off+xs[k]
            on   = k == 5
            sib  = k == 4
            box(c, x, base_y, lw, lh,
                ACCENT_SOFT if on else (GOLD_SOFT if sib else SURFACE),
                ACCENT if on else (GOLD if sib else RULE), 0.9 if (on or sib) else 0.7)
            txt(c, x+lw/2, base_y+5.5, f"tx{k}", "Mono", 6.8,
                ACCENT if on else (GOLD if sib else INK_SOFT), "c")
        # level 1
        l1y = base_y + 34
        for k in range(4):
            x = off + xs[2*k] + (lw+gap)/2
            on  = k == 2
            hl  = k == 3
            box(c, x, l1y, lw, lh, ACCENT_SOFT if on else (GOLD_SOFT if hl else SURFACE),
                ACCENT if on else (GOLD if hl else RULE), 0.9 if (on or hl) else 0.7)
            txt(c, x+lw/2, l1y+5.5, "H", "Mono", 6.8,
                ACCENT if on else (GOLD if hl else INK_SOFT), "c")
            for src in (2*k, 2*k+1):
                arrow(c, off+xs[src]+lw/2, base_y+lh, x+lw/2, l1y-2,
                      ACCENT if (on and src==5) else RULE, 0.7, 3)
        # level 2
        l2y = l1y + 34
        for k in range(2):
            x = off + xs[4*k] + (3*(lw+gap))/2
            on = k == 1
            hl = k == 0
            box(c, x, l2y, lw, lh, ACCENT_SOFT if on else (GOLD_SOFT if hl else SURFACE),
                ACCENT if on else (GOLD if hl else RULE), 0.9)
            txt(c, x+lw/2, l2y+5.5, "H", "Mono", 6.8,
                ACCENT if on else (GOLD if hl else INK_SOFT), "c")
        # root
        rty = l2y + 34
        rx = off + span/2 - lw/2
        box(c, rx, rty, lw, lh, INK, INK, 1.0)
        txt(c, rx+lw/2, rty+5.5, "ROOT", "Label", 6.6, PAPER, "c")
        txt(c, rx+lw+10, rty+5.5, "goes in the block header", "Body-I", 7.4, INK_SOFT)

        txt(c, 0, 74, "TO PROVE tx5 IS IN THIS BLOCK YOU SEND:", "Label", 7.6, ACCENT)
        txt(c, 0, 62, "tx5 itself, plus the three gold hashes. Not the other seven transactions.",
            "Body", 8, INK)
        txt(c, 0, 48, "The verifier re-hashes upward and checks it lands on the root they already trust.",
            "Body", 8, INK)
        chip(c, 0, 26, "8 LEAVES -> 3 HASHES", ACCENT, ACCENT_SOFT, 7)
        chip(c, 118, 26, "1024 LEAVES -> 10 HASHES", ACCENT, ACCENT_SOFT, 7)
        chip(c, 268, 26, "GROWTH IS log2(n)", INK, SURFACE, 7)
        txt(c, 0, 8, "This is how a light client verifies a payment without downloading the chain.",
            "Body-I", 7.6, INK_FAINT)

# ------------------------------------------------- 4. one-way / trapdoor
class P4OneWay(Dia):
    H = 198
    CAP = "Figure 4 — Easy one way, infeasible the other. Every public-key system is built on such a gap."
    def render(self, c, W):
        txt(c, 0, self.H-10, "THE SHAPE OF THE PROBLEM", "Label", 8, INK)
        # easy direction
        y1 = self.H - 54
        box(c, 0, y1, 96, 30, SURFACE, INK_SOFT, 0.9)
        txt(c, 48, y1+11, "secret k", "Mono", 8, INK, "c")
        arrow(c, 100, y1+15, 208, y1+15, ACCENT, 1.4)
        txt(c, 154, y1+21, "multiply: microseconds", "Body-I", 7.4, ACCENT, "c")
        box(c, 212, y1, 118, 30, ACCENT_SOFT, ACCENT, 0.9)
        txt(c, 271, y1+11, "public point  k·G", "Mono", 8, ACCENT, "c")

        y2 = y1 - 44
        arrow(c, 208, y2+15, 100, y2+15, SIGNAL, 1.4)
        txt(c, 154, y2+21, "reverse: longer than the universe", "Body-I", 7.4, SIGNAL, "c")
        txt(c, 154, y2+3, "the discrete logarithm problem", "Body-I", 6.8, INK_FAINT, "c")

        c.setStrokeColor(RULE); c.setLineWidth(0.6); c.setDash(2,2)
        c.line(0, y2-14, W, y2-14); c.setDash()

        txt(c, 0, y2-32, "WHAT 'k·G' MEANS, WITHOUT THE ALGEBRA", "Label", 7.8, INK)
        # dots on a curve-ish arc
        cx, cy = 84, y2-74
        c.setStrokeColor(RULE_SOFT); c.setLineWidth(1.0)
        p = c.beginPath(); p.moveTo(cx-64, cy-20)
        p.curveTo(cx-30, cy+26, cx+18, cy-30, cx+64, cy+22)
        c.drawPath(p, stroke=1, fill=0)
        for k, (dx, dy, lab) in enumerate([(-52,-9,"G"), (-14,7,"2G"), (26,-6,"3G"), (58,16,"4G")]):
            node(c, cx+dx, cy+dy, 4.2, ACCENT, ACCENT, "", 0.8)
            txt(c, cx+dx, cy+dy+8, lab, "Mono", 6.2, ACCENT, "c")
        txt(c, 160, y2-46,
            "A curve is just a set of points with a rule for \"adding\" two of them.", "Body", 7.8, INK)
        txt(c, 160, y2-57,
            "Start at a fixed point G and add it to itself k times: that is k·G.", "Body", 7.8, INK)
        txt(c, 160, y2-68,
            "Doing it is fast even for astronomically large k (double-and-add).", "Body", 7.8, INK)
        txt(c, 160, y2-79,
            "Recovering k from the landing point is the hard part. That gap IS the key pair:",
            "Body", 7.8, INK)
        txt(c, 160, y2-90, "k is your private key, k·G is your public key.",
            "Body-B", 7.8, ACCENT)

# ------------------------------------------------- 5. Ed25519 determinism
class P5Determinism(Dia):
    H = 178
    CAP = "Figure 5 — Determinism is a safety feature in ECDSA. In Algorand it is load-bearing."
    def render(self, c, W):
        half = W/2 - 12
        # randomized
        txt(c, 0, self.H-10, "RANDOMISED SIGNING (classic ECDSA)", "Label", 7.8, SIGNAL)
        y = self.H - 40
        for k in range(2):
            box(c, 0, y-k*30, 92, 22, SURFACE, INK_FAINT, 0.8)
            txt(c, 46, y-k*30+7, '"pay alice"', "Mono", 7, INK, "c")
            arrow(c, 96, y-k*30+11, 122, y-k*30+11, SIGNAL, 0.9)
            box(c, 126, y-k*30, half-126, 22, SIGNAL_SOFT, SIGNAL, 0.8)
            txt(c, 132, y-k*30+7, ["3a91f0...", "c74b28..."][k], "Mono", 7, SIGNAL)
        txt(c, 0, y-72, "Same key, same message, DIFFERENT signature", "Body-B", 7.8, SIGNAL)
        txt(c, 0, y-83, "each time, because a fresh random nonce k is", "Body", 7.6, INK)
        txt(c, 0, y-94, "drawn per signature. Reuse k even once and", "Body", 7.6, INK)
        txt(c, 0, y-105, "your private key can be recovered outright.", "Body", 7.6, INK)

        c.setStrokeColor(RULE); c.setDash(2,3); c.setLineWidth(0.7)
        c.line(W/2, 6, W/2, self.H-4); c.setDash()

        ox = W/2 + 12
        txt(c, ox, self.H-10, "ED25519 (RFC 8032)", "Label", 7.8, ACCENT)
        for k in range(2):
            box(c, ox, y-k*30, 92, 22, SURFACE, INK_FAINT, 0.8)
            txt(c, ox+46, y-k*30+7, '"pay alice"', "Mono", 7, INK, "c")
            arrow(c, ox+96, y-k*30+11, ox+122, y-k*30+11, ACCENT, 0.9)
            box(c, ox+126, y-k*30, half-126, 22, ACCENT_SOFT, ACCENT, 0.8)
            txt(c, ox+132, y-k*30+7, "eae436f7...", "Mono", 7, ACCENT)
        txt(c, ox, y-72, "Same key, same message, IDENTICAL signature", "Body-B", 7.8, ACCENT)
        txt(c, ox, y-83, "every time. The nonce is derived by HASHING", "Body", 7.6, INK)
        txt(c, ox, y-94, "the key together with the message, so there is", "Body", 7.6, INK)
        txt(c, ox, y-105, "no randomness left to get wrong.", "Body", 7.6, INK)
        chip(c, ox, y-124, "VERIFIED IN EXERCISE 1", ACCENT, ACCENT_SOFT, 6.8)
