"""Low-level drawing primitives shared by every diagram."""
import math
from pdfbase_kit import *

def box(c, x, y, w, h, fill=SURFACE, stroke=RULE, lw=0.8, r=0, dash=None):
    c.setFillColor(fill); c.setStrokeColor(stroke); c.setLineWidth(lw)
    if dash: c.setDash(dash)
    if r: c.roundRect(x, y, w, h, r, stroke=1, fill=1)
    else: c.rect(x, y, w, h, stroke=1, fill=1)
    c.setDash()

def txt(c, x, y, s, font="Body", size=8.6, color=INK, anchor="l"):
    c.setFont(font, size); c.setFillColor(color)
    if anchor == "c":   c.drawCentredString(x, y, s)
    elif anchor == "r": c.drawRightString(x, y, s)
    else:               c.drawString(x, y, s)

def arrow(c, x1, y1, x2, y2, color=INK_SOFT, lw=1.0, head=4.2, dash=None):
    c.setStrokeColor(color); c.setFillColor(color); c.setLineWidth(lw)
    if dash: c.setDash(dash)
    ang = math.atan2(y2-y1, x2-x1)
    bx, by = x2 - head*math.cos(ang), y2 - head*math.sin(ang)
    c.line(x1, y1, bx, by); c.setDash()
    p = c.beginPath()
    p.moveTo(x2, y2)
    p.lineTo(x2 - head*1.9*math.cos(ang-0.38), y2 - head*1.9*math.sin(ang-0.38))
    p.lineTo(x2 - head*1.9*math.cos(ang+0.38), y2 - head*1.9*math.sin(ang+0.38))
    p.close(); c.drawPath(p, stroke=0, fill=1)

def node(c, cx, cy, r=9, fill=PAPER, stroke=INK_SOFT, label=None, lw=0.9,
         lcolor=INK, lsize=7):
    c.setFillColor(fill); c.setStrokeColor(stroke); c.setLineWidth(lw)
    c.circle(cx, cy, r, stroke=1, fill=1)
    if label: txt(c, cx, cy-2.4, label, "Mono", lsize, lcolor, "c")

def chip(c, x, y, s, fg=ACCENT, bg=ACCENT_SOFT, size=6.8, padx=4, h=11):
    c.setFont("Label", size)
    w = c.stringWidth(s, "Label", size) + 2*padx
    c.setFillColor(bg); c.setStrokeColor(fg); c.setLineWidth(0.6)
    c.rect(x, y, w, h, stroke=1, fill=1)
    txt(c, x+padx, y+3.2, s, "Label", size, fg)
    return w

def caption(c, x, y, w, s, size=7.6):
    """Small centred caption under a diagram."""
    txt(c, x + w/2, y, s, "Body-I", size, INK_FAINT, "c")

def brace_h(c, x1, x2, y, color=INK_FAINT, depth=4):
    """A flat horizontal brace under a span."""
    c.setStrokeColor(color); c.setLineWidth(0.7)
    c.line(x1, y, x1, y-depth); c.line(x1, y-depth, x2, y-depth); c.line(x2, y-depth, x2, y)
