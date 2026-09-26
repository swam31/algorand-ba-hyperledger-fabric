"""Diagrams 6-11."""
import math, random
from pdfbase_kit import *
from pdfdraw import *
from pdfdiagrams_a import Dia

# ---------------------------------------------------------- 6. gossip
class D6Gossip(Dia):
    H = 150
    CAP = "Figure 6 — Nobody broadcasts to everyone. Each node tells a few peers, and in a few hops it has reached all."
    def render(self, c, W):
        random.seed(7)
        panel = (W - 24) / 3
        titles = ["t = 0   one node knows", "t = 1   its peers know", "t = 2   nearly all know"]
        known_sets = [{0}, {0,1,4,7}, {0,1,2,3,4,5,7,8,10}]
        pos = []
        for k in range(12):
            a = k*math.pi/6 + 0.25
            rr = 30 if k % 2 == 0 else 20
            pos.append((rr*math.cos(a), rr*math.sin(a)))
        edges = [(0,1),(0,4),(0,7),(1,2),(1,5),(4,3),(4,8),(7,10),(7,6),
                 (2,9),(5,11),(8,9),(10,11),(3,6)]
        for p in range(3):
            ox = p*(panel+12) + panel/2
            oy = self.H - 66
            txt(c, ox, self.H-12, titles[p], "Label", 7.2, INK, "c")
            known = known_sets[p]
            c.setStrokeColor(RULE); c.setLineWidth(0.5)
            for (a,bb) in edges:
                c.line(ox+pos[a][0], oy+pos[a][1], ox+pos[bb][0], oy+pos[bb][1])
            c.setStrokeColor(ACCENT); c.setLineWidth(1.0)
            for (a,bb) in edges:
                if a in known and bb in known:
                    c.line(ox+pos[a][0], oy+pos[a][1], ox+pos[bb][0], oy+pos[bb][1])
            for k in range(12):
                on = k in known
                node(c, ox+pos[k][0], oy+pos[k][1], 5.5,
                     ACCENT if on else PAPER, ACCENT if on else INK_FAINT, "", 0.9)
            txt(c, ox, oy-46, f"{len(known)} of 12", "Mono", 7, ACCENT if p else INK_FAINT, "c")
        txt(c, 0, 4, "THIS IS ALSO WHY BLOCKCHAINS ARE SLOW: NEWS TRAVELS AT GOSSIP SPEED, NOT DATABASE SPEED.",
            "Label", 7.4, INK_FAINT)

# ---------------------------------------------------------- 7. byzantine generals
class D7Byzantine(Dia):
    H = 172
    CAP = "Figure 7 — A crashed node says nothing. A Byzantine node says different things to different people."
    def render(self, c, W):
        half = W/2 - 14
        # left: crash fault
        txt(c, 0, self.H-9, "CRASH FAULT — THE EASY CASE", "Label", 7.8, INK)
        cx, cy = half/2, self.H - 62
        node(c, cx, cy, 15, SURFACE, INK_FAINT, "", 1.0)
        txt(c, cx, cy-3, "OFF", "Label", 7, INK_FAINT, "c")
        for k, ang in enumerate([2.4, math.pi/2, 0.75]):
            px, py = cx + 56*math.cos(ang), cy + 44*math.sin(ang)
            node(c, px, py, 12, PAPER, INK_SOFT, "", 1.0)
            txt(c, px, py-3, "OK", "Label", 6.6, INK, "c")
            c.setStrokeColor(INK_FAINT); c.setLineWidth(0.8); c.setDash(2,2)
            c.line(cx+13, cy+4, px-10, py-4); c.setDash()
        txt(c, 0, cy-52, "It is simply silent. Everyone", "Body-I", 7.6, INK_FAINT)
        txt(c, 0, cy-62, "still sees the same world.", "Body-I", 7.6, INK_FAINT)

        c.setStrokeColor(RULE); c.setDash(2,3); c.setLineWidth(0.7)
        c.line(W/2, 6, W/2, self.H-4); c.setDash()

        # right: byzantine
        ox = W/2 + 14
        txt(c, ox, self.H-9, "BYZANTINE FAULT — THE HARD CASE", "Label", 7.8, SIGNAL)
        tx, ty = ox + half/2, self.H - 40
        node(c, tx, ty, 15, SIGNAL_SOFT, SIGNAL, "", 1.2)
        txt(c, tx, ty-3, "LIAR", "Label", 6.6, SIGNAL, "c")
        for k, (lbl, dx) in enumerate([('"ATTACK"', -66), ('"RETREAT"', 66)]):
            px, py = tx + dx, ty - 62
            node(c, px, py, 13, PAPER, INK_SOFT, "", 1.0)
            txt(c, px, py-3, "OK", "Label", 6.6, INK, "c")
            arrow(c, tx + (dx/abs(dx))*12, ty-12, px - (dx/abs(dx))*4, py+15, SIGNAL, 1.0)
            txt(c, tx + dx*0.62, ty-36, lbl, "Mono", 7, SIGNAL, "c")
        txt(c, ox, ty-92, "Both honest nodes are certain.", "Body-I", 7.6, SIGNAL)
        txt(c, ox, ty-102, "They are certain of opposite things.", "Body-BI", 7.6, SIGNAL)
        txt(c, ox, ty-116, "Signatures do not help: the liar", "Body-I", 7.2, INK_FAINT)
        txt(c, ox, ty-125, "signed both messages honestly.", "Body-I", 7.2, INK_FAINT)

# ---------------------------------------------------------- 8. the 2/3 bound
class D8TwoThirds(Dia):
    H = 186
    CAP = "Figure 8 — Why the magic number is two thirds, and not one half."
    def render(self, c, W):
        bar_h = 26
        def bar(y, n, f, label, ok):
            seg = W / n
            for k in range(n):
                bad = k >= n - f
                box(c, k*seg, y, seg-2, bar_h,
                    SIGNAL_SOFT if bad else ACCENT_SOFT,
                    SIGNAL if bad else ACCENT, 0.8)
                txt(c, k*seg + seg/2 - 1, y + 9, "X" if bad else "OK",
                    "Label", 7, SIGNAL if bad else ACCENT, "c")
            txt(c, 0, y + bar_h + 5, label, "Body-B", 8.2, INK)
            v = "WORKS" if ok else "BREAKS"
            chip(c, W - 46, y + bar_h + 2, v, ACCENT if ok else SIGNAL,
                 ACCENT_SOFT if ok else SIGNAL_SOFT, 7)

        txt(c, 0, self.H-10, "n = 9 participants, f of them Byzantine", "Label", 8, INK)
        bar(self.H-58, 9, 2, "f = 2 liars   ->   honest = 7 of 9 = 78%", True)
        bar(self.H-112, 9, 3, "f = 3 liars   ->   honest = 6 of 9 = 67%", False)

        y = self.H - 150
        box(c, 0, y-16, W, 34, SURFACE, RULE, 0.7)
        txt(c, 8, y+9, "The reasoning:", "Body-B", 8.2, INK)
        txt(c, 8, y-1, "you must decide without hearing from f nodes (they may be silent), so you act on n - f replies;",
            "Body", 7.6, INK)
        txt(c, 8, y-11, "of those, f may be lies. For honest replies to still outnumber lies:  n - 2f > f,  i.e.  n > 3f.",
            "Body-B", 7.6, ACCENT)

# ---------------------------------------------------------- 9. proof of work + forks
class D9PoW(Dia):
    H = 206
    CAP = "Figure 9 — Mining is a lottery with real electricity as the ticket price — and it forks."
    def render(self, c, W):
        txt(c, 0, self.H-9, "A. MINING IS GUESSING", "Label", 8, INK)
        y = self.H - 62
        box(c, 0, y, 196, 44, SURFACE, RULE, 0.8)
        rows = ["SHA256(block + 1)    = 8f2a...  no",
                "SHA256(block + 2)    = c41d...  no",
                "SHA256(block + 91847)= 0000...  YES"]
        for k, r in enumerate(rows):
            col = ACCENT if "YES" in r else INK_FAINT
            txt(c, 6, y + 32 - k*12, r, "Mono", 6.5, col)
        txt(c, 204, y+30, "Only way to find it is to guess,", "Body-I", 7.6, INK)
        txt(c, 204, y+20, "trillions of times per second.", "Body-I", 7.6, INK)
        txt(c, 204, y+6, "A million fake identities on one", "Body", 7.4, ACCENT)
        txt(c, 204, y-4, "computer have no more power", "Body", 7.4, ACCENT)
        txt(c, 204, y-14, "than that one computer.", "Body-B", 7.4, ACCENT)

        c.setStrokeColor(RULE); c.setLineWidth(0.6); c.setDash(2,2)
        c.line(0, y-26, W, y-26); c.setDash()

        txt(c, 0, y-44, "B. TWO WINNERS AT ONCE = A FORK", "Label", 8, SIGNAL)
        bw, bh = 62, 26
        base_y = y - 96
        for k in range(2):
            x = k*(bw+18)
            box(c, x, base_y, bw, bh, PAPER, INK_SOFT, 0.9)
            txt(c, x+bw/2, base_y+9, f"#{41+k}", "Label", 7.4, INK, "c")
            if k: arrow(c, x-16, base_y+bh/2, x-3, base_y+bh/2, INK_SOFT, 1.0)
        fx = 2*(bw+18)
        for k, (dy, lab, col, note) in enumerate([
                (26, "#43a", SIGNAL, "some nodes believe this"),
                (-26, "#43b", SIGNAL, "others believe this")]):
            box(c, fx, base_y+dy, bw, bh, SIGNAL_SOFT, col, 0.9)
            txt(c, fx+bw/2, base_y+dy+9, lab, "Label", 7.4, col, "c")
            arrow(c, fx-16, base_y+bh/2, fx-3, base_y+dy+bh/2, col, 1.0)
            txt(c, fx+bw+8, base_y+dy+9, note, "Body-I", 7.2, INK_FAINT)
        box(c, fx+bw+112, base_y+26, bw, bh, ACCENT_SOFT, ACCENT, 0.9)
        txt(c, fx+bw+112+bw/2, base_y+35, "#44", "Label", 7.4, ACCENT, "c")
        arrow(c, fx+bw+2, base_y+39, fx+bw+108, base_y+39, ACCENT, 1.0)
        txt(c, fx+bw+112, base_y+8, "longest chain wins;", "Body-I", 7, ACCENT)
        txt(c, fx+bw+112, base_y-1, "#43b is discarded", "Body-I", 7, SIGNAL)
        txt(c, 0, base_y-24,
            "Every transaction in the losing branch is undone. That is why you wait six blocks — about an hour.",
            "Body-B", 7.8, INK)

# ---------------------------------------------------------- 10. public vs private
class D10Networks(Dia):
    H = 176
    CAP = "Figure 10 — Where your project sits: a small, permissioned network with known members."
    def render(self, c, W):
        random.seed(3)
        half = W/2 - 14
        # public
        txt(c, 0, self.H-9, "PUBLIC  (Bitcoin, Ethereum)", "Label", 7.8, INK)
        cx, cy = half/2, self.H - 74
        pts = [(cx + random.uniform(-62,62), cy + random.uniform(-38,38)) for _ in range(22)]
        c.setStrokeColor(RULE_SOFT); c.setLineWidth(0.4)
        for a in range(22):
            for bb in range(a+1, 22):
                if random.random() < 0.13:
                    c.line(pts[a][0],pts[a][1],pts[bb][0],pts[bb][1])
        for (px,py) in pts: node(c, px, py, 3.6, PAPER, INK_FAINT, "", 0.7)
        for lbl, yy in [("anyone can join", 0), ("identities unknown", 10),
                        ("needs PoW or PoS to stop Sybils", 20), ("thousands of nodes, slow", 30)]:
            txt(c, 0, cy-48-yy, "- " + lbl, "Body", 7.4, INK_FAINT)

        c.setStrokeColor(RULE); c.setDash(2,3); c.setLineWidth(0.7)
        c.line(W/2, 6, W/2, self.H-4); c.setDash()

        # private
        ox = W/2 + 14
        txt(c, ox, self.H-9, "PERMISSIONED  (Hyperledger Fabric)", "Label", 7.8, ACCENT)
        cx2 = ox + half/2
        n = 7
        pts2 = []
        for k in range(n):
            a = math.pi/2 + k*2*math.pi/n
            pts2.append((cx2 + 44*math.cos(a), cy + 30*math.sin(a)))
        c.setStrokeColor(ACCENT_MID); c.setLineWidth(0.6)
        for a in range(n):
            for bb in range(a+1, n):
                c.line(pts2[a][0],pts2[a][1],pts2[bb][0],pts2[bb][1])
        for k,(px,py) in enumerate(pts2):
            node(c, px, py, 9, ACCENT_SOFT, ACCENT, f"{k+1}", 1.0, ACCENT, 6.2)
        for lbl, yy in [("membership issued by a CA", 0), ("every node has a known identity", 10),
                        ("Sybil attack largely off the table", 20),
                        ("a handful of nodes, fast", 30)]:
            txt(c, ox, cy-48-yy, "- " + lbl, "Body", 7.4, ACCENT)
        txt(c, ox, cy-88, "This is your project's world.", "Body-BI", 8, INK)
