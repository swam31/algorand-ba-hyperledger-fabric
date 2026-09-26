"""Diagrams 1-5."""
import math
from reportlab.platypus import Flowable
from pdfbase_kit import *
from pdfdraw import *

class Dia(Flowable):
    H = 150
    CAP = ""
    def wrap(self, aw, ah):
        self.W = aw
        return (aw, self.H + (14 if self.CAP else 0))
    def draw(self):
        if self.CAP:
            caption(self.canv, 0, 3, self.W, self.CAP)
            self.canv.translate(0, 14)
        self.render(self.canv, self.W)

# ---------------------------------------------------------- 1. two kinds of ledger
class D1Ledger(Dia):
    H = 168
    CAP = "Figure 1 — The same list, kept two different ways."
    def render(self, c, W):
        half = W/2 - 12
        # ---- left: the bank
        txt(c, 0, self.H-8, "A. THE BANK'S WAY", "Label", 8, INK)
        txt(c, 0, self.H-20, "One trusted keeper. One copy.", "Body-I", 7.8, INK_FAINT)
        bx, by, bw, bh = half/2-38, self.H-108, 76, 62
        box(c, bx, by, bw, bh, PAPER, INK, 1.1)
        txt(c, bx+bw/2, by+bh-14, "BANK", "Label", 9, INK, "c")
        c.setStrokeColor(RULE); c.setLineWidth(0.5)
        for k, row in enumerate(["Swayam  9,500", "Alice   2,500", "Bob     7,000"]):
            c.line(bx+7, by+bh-22-k*13, bx+bw-7, by+bh-22-k*13)
            txt(c, bx+9, by+bh-20-k*13, row, "Mono", 6.4, INK_SOFT)
        for k in range(3):
            px = 8 + k*26
            node(c, px, self.H-32, 7, PAPER, INK_SOFT, "", 0.8)
            arrow(c, px+6, self.H-36, bx-3, by+bh-12-k*4, INK_FAINT, 0.8, 3.4)
        txt(c, 4, by-14, "If the bank lies, or fails,", "Body-I", 7.4, SIGNAL)
        txt(c, 4, by-23, "there is no second opinion.", "Body-I", 7.4, SIGNAL)

        # ---- divider
        c.setStrokeColor(RULE); c.setLineWidth(0.8); c.setDash(2,3)
        c.line(W/2, 6, W/2, self.H-4); c.setDash()

        # ---- right: the network
        ox = W/2 + 12
        txt(c, ox, self.H-8, "B. THE BLOCKCHAIN'S WAY", "Label", 8, ACCENT)
        txt(c, ox, self.H-20, "Everyone keeps their own copy.", "Body-I", 7.8, INK_FAINT)
        cx, cy, R = ox + half/2, self.H-74, 40
        pts = []
        for k in range(6):
            a = math.pi/2 + k*math.pi/3
            pts.append((cx + R*math.cos(a), cy + R*math.sin(a)))
        c.setStrokeColor(ACCENT_MID); c.setLineWidth(0.5)
        for a in range(6):
            for bb in range(a+1, 6):
                c.line(pts[a][0], pts[a][1], pts[bb][0], pts[bb][1])
        for (px, py) in pts:
            node(c, px, py, 10, ACCENT_SOFT, ACCENT, "", 1.0)
            c.setStrokeColor(ACCENT); c.setLineWidth(0.5)
            for k in range(3):
                c.line(px-5, py+2.5-k*2.6, px+5, py+2.5-k*2.6)
        txt(c, ox, cy-R-20, "No keeper. To rewrite history you must", "Body-I", 7.4, ACCENT)
        txt(c, ox, cy-R-29, "convince most of the network at once.", "Body-I", 7.4, ACCENT)

# ---------------------------------------------------------- 2. double spending
class D2DoubleSpend(Dia):
    H = 190
    CAP = "Figure 2 — One coin, two promises. Both look perfectly valid on their own."
    def render(self, c, W):
        # Explicit bottom-y for every element; the earlier relative offsets let the
        # summary box collide with the recipient labels.
        Y_BOB   = self.H - 48       # bottom of Bob's transaction box
        Y_SUM   = self.H - 104      # bottom of the dashed summary box
        Y_CAROL = self.H - 156      # bottom of Carol's transaction box
        BH      = 28
        alice_y = self.H - 96

        node(c, 30, alice_y, 15, GOLD_SOFT, GOLD, "", 1.1)
        txt(c, 30, alice_y-3, "A", "Label", 9, GOLD, "c")
        txt(c, 30, alice_y-26, "Alice", "Body-B", 8.4, INK, "c")
        txt(c, 30, alice_y-36, "has 1 coin", "Body-I", 7.4, INK_FAINT, "c")

        tx, tw = 128, 150
        for name, ybot in (("Bob", Y_BOB), ("Carol", Y_CAROL)):
            box(c, tx, ybot, tw, BH, SIGNAL_SOFT, SIGNAL, 0.9)
            txt(c, tx+8, ybot+16, f"pay {name}  1 coin", "Mono", 7.6, INK)
            txt(c, tx+8, ybot+5,  "signed by Alice   [VALID]", "Mono", 6.4, SIGNAL)
            arrow(c, 48, alice_y, tx-5, ybot+BH/2, SIGNAL, 1.0)
            node(c, tx+tw+34, ybot+BH/2, 13, PAPER, INK_SOFT, "", 1.0)
            txt(c, tx+tw+34, ybot+BH/2-3, name[0], "Label", 8, INK, "c")
            arrow(c, tx+tw+4, ybot+BH/2, tx+tw+18, ybot+BH/2, INK_SOFT, 0.9)
            txt(c, tx+tw+34, ybot-11, name, "Body-B", 7.6, INK, "c")

        box(c, tx, Y_SUM, W-tx, 30, SURFACE, INK_FAINT, 0.8, dash=(2,2))
        cxm = tx + (W-tx)/2
        txt(c, cxm, Y_SUM+18, "Both signatures verify. Both spend the same coin.",
            "Body-BI", 8.6, INK, "c")
        txt(c, cxm, Y_SUM+7, "Nothing in either message says which came first.",
            "Body-I", 7.6, SIGNAL, "c")

        txt(c, 0, 6, "THE ONLY FIX IS AGREEING ON ORDER \u2014 WHICH IS WHAT CONSENSUS MEANS.",
            "Label", 8, ACCENT)

# ---------------------------------------------------------- 3. hash function
class D3Hash(Dia):
    H = 194
    CAP = "Figure 3 — A hash is a fingerprint: same input, same output; one letter different, everything different."
    def render(self, c, W):
        y = self.H - 34
        box(c, 0, y-16, 96, 26, PAPER, INK_SOFT, 0.9)
        txt(c, 48, y-8, '"hello"', "Mono", 9, INK, "c")
        arrow(c, 100, y-3, 128, y-3, INK_SOFT, 1.1)
        box(c, 132, y-22, 92, 38, INK, INK, 1.0)
        txt(c, 178, y-4, "SHA-256", "Label", 10, PAPER, "c")
        txt(c, 178, y-15, "one-way", "Body-I", 6.8, ACCENT_MID, "c")
        arrow(c, 228, y-3, 256, y-3, INK_SOFT, 1.1)
        box(c, 260, y-22, W-260, 38, ACCENT_SOFT, ACCENT, 0.9)
        txt(c, 266, y-4, "2cf24dba5fb0a30e26e83b2a", "Mono", 7.2, ACCENT)
        txt(c, 266, y-14, "c5b9e29e1b161e5c1fa7425e", "Mono", 7.2, ACCENT)

        c.setStrokeColor(RULE); c.setLineWidth(0.6); c.setDash(2,2)
        c.line(0, y-40, W, y-40); c.setDash()
        txt(c, 0, y-56, "THE AVALANCHE EFFECT", "Label", 8, SIGNAL)
        txt(c, 0, y-68, "Change one letter — o becomes p — and the fingerprint is unrecognisable.",
            "Body", 8.2, INK)

        yy = y - 88
        for lbl, dig, col in [('"hello"', "2cf24dba 5fb0a30e 26e83b2a c5b9e29e", ACCENT),
                              ('"hellp"', "9d3d3b3b 71e4c9a1 04c0f6ed 22ab8ff3", SIGNAL)]:
            box(c, 0, yy-15, 74, 22, PAPER, col, 0.8)
            txt(c, 37, yy-8, lbl, "Mono", 8, col, "c")
            arrow(c, 78, yy-4, 100, yy-4, col, 0.9)
            box(c, 104, yy-15, W-104, 22, PAPER, RULE, 0.7)
            txt(c, 110, yy-8, dig, "Mono", 8, col)
            yy -= 30
        txt(c, 0, yy+2, "Nothing about the second output hints that the input was almost identical.",
            "Body-I", 7.8, INK_FAINT)

# ---------------------------------------------------------- 4. signatures
class D4Signature(Dia):
    H = 172
    CAP = "Figure 4 — Only you can produce it. Everyone can check it. Your secret never moves."
    def render(self, c, W):
        # key pair
        txt(c, 0, self.H-10, "ONE KEY PAIR", "Label", 8, INK)
        box(c, 0, self.H-64, 118, 46, SIGNAL_SOFT, SIGNAL, 0.9)
        txt(c, 8, self.H-32, "PRIVATE KEY", "Label", 7.6, SIGNAL)
        txt(c, 8, self.H-44, "sk — never shared", "Body-I", 7.4, INK)
        txt(c, 8, self.H-56, "this IS your money", "Body-BI", 7.4, SIGNAL)
        arrow(c, 59, self.H-68, 59, self.H-84, INK_FAINT, 0.9)
        txt(c, 64, self.H-79, "derive (one-way)", "Body-I", 6.8, INK_FAINT)
        box(c, 0, self.H-130, 118, 44, ACCENT_SOFT, ACCENT, 0.9)
        txt(c, 8, self.H-99, "PUBLIC KEY", "Label", 7.6, ACCENT)
        txt(c, 8, self.H-111, "pk — tell everyone", "Body-I", 7.4, INK)
        txt(c, 8, self.H-123, "this is your address", "Body-I", 7.4, ACCENT)

        c.setStrokeColor(RULE); c.setDash(2,3); c.setLineWidth(0.7)
        c.line(133, 8, 133, self.H-6); c.setDash()

        # signing
        ox = 150
        txt(c, ox, self.H-10, "SIGNING", "Label", 8, SIGNAL)
        box(c, ox, self.H-46, 150, 28, PAPER, INK_SOFT, 0.8)
        txt(c, ox+6, self.H-30, "pay Alice 500", "Mono", 7.6, INK)
        txt(c, ox+6, self.H-40, "the message", "Body-I", 6.6, INK_FAINT)
        arrow(c, ox+155, self.H-32, ox+178, self.H-32, SIGNAL, 1.0)
        box(c, ox+182, self.H-48, 132, 32, SIGNAL_SOFT, SIGNAL, 0.9)
        txt(c, ox+188, self.H-30, "Sign(sk, message)", "Mono", 7.4, SIGNAL)
        txt(c, ox+188, self.H-42, "only the holder of sk can", "Body-I", 6.6, INK)

        # verifying
        txt(c, ox, self.H-72, "VERIFYING", "Label", 8, ACCENT)
        box(c, ox, self.H-112, 314, 34, ACCENT_SOFT, ACCENT, 0.9)
        txt(c, ox+8, self.H-92, "Verify(pk, message, signature)  ->  true / false",
            "Mono", 7.8, ACCENT)
        txt(c, ox+8, self.H-105, "anyone on earth can run this, using only the PUBLIC key",
            "Body-I", 7, INK)
        box(c, ox, self.H-146, 314, 26, PAPER, INK_FAINT, 0.7, dash=(2,2))
        txt(c, ox+157, self.H-131, "Proves WHO authorised it — and that not one character changed.",
            "Body-BI", 7.6, INK, "c")
        txt(c, ox+157, self.H-141, "Still says nothing about WHEN. Hence Figure 2.",
            "Body-I", 7, SIGNAL, "c")

# ---------------------------------------------------------- 5. the chain + tamper
class D5Chain(Dia):
    H = 236
    CAP = "Figure 5 — Each block seals the one before it, so a single edit unravels everything after."
    def render(self, c, W):
        bw, bh, gap = 96, 54, 22
        def blk(x, y, n, txns, ph, phcol=ACCENT, bcol=INK_SOFT, fill=PAPER, hcol=ACCENT):
            box(c, x, y, bw, bh, fill, bcol, 1.0)
            c.setFillColor(SURFACE); c.setStrokeColor(bcol); c.setLineWidth(0.5)
            c.rect(x, y+bh-15, bw, 15, stroke=1, fill=1)
            txt(c, x+5, y+bh-11, f"BLOCK {n}", "Label", 7.4, INK)
            txt(c, x+5, y+bh-26, "prev_hash", "Body-I", 6.2, INK_FAINT)
            txt(c, x+5, y+bh-35, ph, "Mono", 6.6, phcol)
            txt(c, x+5, y+bh-46, txns, "Mono", 6.4, INK_SOFT)

        # ---- healthy chain
        txt(c, 0, self.H-9, "A. AN INTACT CHAIN", "Label", 8, ACCENT)
        y1 = self.H - 76
        hashes = ["0000...", "a91f...", "7c04...", "e5b8..."]
        for k in range(4):
            x = k*(bw+gap)
            blk(x, y1, k, f"{3+k} txns", hashes[k])
            if k < 3:
                arrow(c, x+bw, y1+bh/2, x+bw+gap-2, y1+bh/2, ACCENT, 1.1)
        txt(c, 0, y1-14, "Each arrow means: \"I contain the hash of the block before me.\"",
            "Body-I", 7.6, INK_FAINT)

        c.setStrokeColor(RULE); c.setLineWidth(0.6); c.setDash(2,2)
        c.line(0, y1-26, W, y1-26); c.setDash()

        # ---- tampered chain
        txt(c, 0, y1-44, "B. SOMEONE EDITS BLOCK 1", "Label", 8, SIGNAL)
        y2 = y1 - 118
        for k in range(4):
            x = k*(bw+gap)
            if k == 0:
                blk(x, y2, 0, "3 txns", "0000...")
            elif k == 1:
                blk(x, y2, 1, "EDITED!", "a91f...", SIGNAL, SIGNAL, SIGNAL_SOFT)
                txt(c, x+bw/2, y2-11, "changed", "Body-BI", 6.8, SIGNAL, "c")
            else:
                blk(x, y2, k, f"{3+k} txns", hashes[k], SIGNAL, SIGNAL, SIGNAL_SOFT)
                txt(c, x+bw/2, y2-11, "now invalid", "Body-BI", 6.8, SIGNAL, "c")
            if k < 3:
                col = ACCENT if k == 0 else SIGNAL
                arrow(c, x+bw, y2+bh/2, x+bw+gap-2, y2+bh/2, col, 1.1)
                if k >= 1:
                    txt(c, x+bw+gap/2, y2+bh/2+5, "X", "Label", 9, SIGNAL, "c")
        txt(c, 0, y2-28,
            "Editing block 1 changes its hash. Block 2 still stores the OLD hash, so block 2 breaks.",
            "Body", 7.8, INK)
        txt(c, 0, y2-38,
            "Fixing block 2 breaks block 3, and so on to the present day.",
            "Body-B", 7.8, SIGNAL)
