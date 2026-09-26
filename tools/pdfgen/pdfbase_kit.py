"""Shared typography, palette and page furniture for the Phase 1 study PDF."""
from reportlab.lib.pagesizes import A4
from reportlab.lib.units import mm
from reportlab.lib import colors
from reportlab.lib.styles import ParagraphStyle
from reportlab.lib.enums import TA_LEFT, TA_CENTER, TA_JUSTIFY
from reportlab.pdfbase import pdfmetrics
from reportlab.pdfbase.ttfonts import TTFont
from reportlab.platypus import Flowable

# ---------------------------------------------------------------- fonts
SUP = "/System/Library/Fonts/Supplemental/"
pdfmetrics.registerFont(TTFont("Body",      SUP + "Georgia.ttf"))
pdfmetrics.registerFont(TTFont("Body-B",    SUP + "Georgia Bold.ttf"))
pdfmetrics.registerFont(TTFont("Body-I",    SUP + "Georgia Italic.ttf"))
pdfmetrics.registerFont(TTFont("Body-BI",   SUP + "Georgia Bold Italic.ttf"))
pdfmetrics.registerFont(TTFont("Mono",      "/System/Library/Fonts/Monaco.ttf"))
pdfmetrics.registerFont(TTFont("Mono-B",    SUP + "Courier New Bold.ttf"))
pdfmetrics.registerFont(TTFont("Label",     SUP + "DIN Alternate Bold.ttf"))
pdfmetrics.registerFont(TTFont("LabelCond", SUP + "DIN Condensed Bold.ttf"))
pdfmetrics.registerFontFamily("Body", normal="Body", bold="Body-B",
                              italic="Body-I", boldItalic="Body-BI")

# ---------------------------------------------------------------- palette
INK        = colors.HexColor("#14172B")
INK_SOFT   = colors.HexColor("#4A4F6A")
INK_FAINT  = colors.HexColor("#767C96")
ACCENT     = colors.HexColor("#0E6B61")   # verdigris
ACCENT_MID = colors.HexColor("#5FA79C")
ACCENT_SOFT= colors.HexColor("#E2EFEC")
SIGNAL     = colors.HexColor("#A8412F")   # brick, for warnings/attacks
SIGNAL_SOFT= colors.HexColor("#F7E7E3")
GOLD       = colors.HexColor("#8A6D1F")
GOLD_SOFT  = colors.HexColor("#F5EEDC")
RULE       = colors.HexColor("#D8DDE6")
RULE_SOFT  = colors.HexColor("#ECEFF4")
SURFACE    = colors.HexColor("#F5F6F8")
PAPER      = colors.white

# ---------------------------------------------------------------- geometry
PAGE_W, PAGE_H = A4
MARGIN_L, MARGIN_R = 24*mm, 22*mm
MARGIN_T, MARGIN_B = 24*mm, 20*mm
CONTENT_W = PAGE_W - MARGIN_L - MARGIN_R

# ---------------------------------------------------------------- styles
def _p(name, **kw):
    base = dict(fontName="Body", fontSize=10.4, leading=16.2, textColor=INK,
                alignment=TA_LEFT, spaceBefore=0, spaceAfter=8)
    base.update(kw)
    return ParagraphStyle(name, **base)

S = {
 "body":     _p("body"),
 "body_j":   _p("body_j", alignment=TA_JUSTIFY),
 "lead":     _p("lead", fontSize=12.2, leading=19, textColor=INK_SOFT, spaceAfter=13),
 "h1":       _p("h1", fontName="Body-B", fontSize=21, leading=25, spaceBefore=0,
                spaceAfter=3, textColor=INK),
 "h2":       _p("h2", fontName="Body-B", fontSize=14.2, leading=18.5, spaceBefore=15,
                spaceAfter=6, textColor=INK),
 "h3":       _p("h3", fontName="Body-BI", fontSize=11.6, leading=15.5, spaceBefore=11,
                spaceAfter=4, textColor=INK),
 "eyebrow":  _p("eyebrow", fontName="Label", fontSize=8.2, leading=11, textColor=ACCENT,
                spaceAfter=5),
 "caption":  _p("caption", fontSize=8.7, leading=12.4, textColor=INK_FAINT, spaceAfter=4),
 "bullet":   _p("bullet", leftIndent=13, bulletIndent=2, spaceAfter=4.5, leading=15.4),
 "code":     _p("code", fontName="Mono", fontSize=8.5, leading=12.6, textColor=INK,
                spaceAfter=0, spaceBefore=0),
 "term":     _p("term", fontSize=9.9, leading=14.6, spaceAfter=6),
 "quote":    _p("quote", fontName="Body-I", fontSize=11, leading=16.5,
                leftIndent=12, textColor=INK, spaceBefore=4, spaceAfter=9),
 "cover_t":  _p("cover_t", fontName="Body-B", fontSize=33, leading=37, spaceAfter=0),
 "cover_s":  _p("cover_s", fontSize=13, leading=19, textColor=INK_SOFT),
 "tblh":     _p("tblh", fontName="Label", fontSize=8, leading=10.5, textColor=INK_FAINT),
 "tbl":      _p("tbl", fontSize=9.3, leading=13.4),
 "tblb":     _p("tblb", fontName="Body-B", fontSize=9.3, leading=13.4),
}

# ---------------------------------------------------------------- helpers
class Rule(Flowable):
    """A horizontal hairline."""
    def __init__(self, w=None, color=RULE, thickness=0.6, space=6):
        Flowable.__init__(self); self.w=w; self.color=color; self.t=thickness; self.space=space
    def wrap(self, aw, ah):
        self._w = self.w or aw
        return (self._w, self.t + self.space)
    def draw(self):
        self.canv.setStrokeColor(self.color); self.canv.setLineWidth(self.t)
        self.canv.line(0, self.space/2, self._w, self.space/2)

class SecBreak(Flowable):
    """Separator between flowing sections: a hairline with a centred accent tick.

    Used instead of a forced PageBreak. Breaking every section onto a fresh page
    left large blank areas whenever a section overran by a little, because a tall
    diagram that does not fit jumps whole to the next page.
    """
    def wrap(self, aw, ah):
        self._w = aw
        return (aw, 34)
    def draw(self):
        c = self.canv
        c.setStrokeColor(RULE); c.setLineWidth(0.8)
        c.line(0, 17, self._w, 17)
        c.setFillColor(PAPER); c.rect(self._w/2 - 9, 12, 18, 10, stroke=0, fill=1)
        c.setFillColor(ACCENT); c.circle(self._w/2, 17, 2.1, stroke=0, fill=1)


class Callout(Flowable):
    """A boxed aside. kind: 'note' | 'warn' | 'key'."""
    PAL = {"note": (ACCENT, ACCENT_SOFT), "warn": (SIGNAL, SIGNAL_SOFT),
           "key":  (GOLD, GOLD_SOFT)}
    def __init__(self, title, paras, kind="note", width=None):
        Flowable.__init__(self)
        self.title=title; self.paras=paras; self.kind=kind; self.width=width
        self.pad = 9
    def wrap(self, aw, ah):
        self._w = self.width or aw
        inner = self._w - 2*self.pad - 4
        self._h = 15
        self._laid = []
        from reportlab.platypus import Paragraph
        st = ParagraphStyle("co", parent=S["body"], fontSize=9.6, leading=14.2,
                            spaceAfter=0, textColor=INK)
        for t in self.paras:
            p = Paragraph(t, st); w,h = p.wrap(inner, 10000)
            self._laid.append((p,h)); self._h += h + 5
        self._h += self.pad
        return (self._w, self._h)
    def draw(self):
        c = self.canv; fg, bg = self.PAL[self.kind]
        c.setFillColor(bg); c.setStrokeColor(fg); c.setLineWidth(0.7)
        c.rect(0, 0, self._w, self._h, stroke=1, fill=1)
        c.setFillColor(fg); c.rect(0, 0, 2.6, self._h, stroke=0, fill=1)
        c.setFont("Label", 8.2); c.setFillColor(fg)
        c.drawString(self.pad+4, self._h - 12, self.title.upper())
        y = self._h - 15
        for p,h in self._laid:
            y -= h + 5
            p.drawOn(c, self.pad+4, y)

class CodeBlock(Flowable):
    """Monospaced block on a tinted ground with a hairline border."""
    def __init__(self, lines, width=None, size=8.3, pad=8, tint=SURFACE):
        Flowable.__init__(self)
        self.lines=lines; self.width=width; self.size=size; self.pad=pad; self.tint=tint
        self.lead = size*1.48
    def wrap(self, aw, ah):
        self._w = self.width or aw
        self._h = len(self.lines)*self.lead + 2*self.pad
        return (self._w, self._h)
    def draw(self):
        c=self.canv
        c.setFillColor(self.tint); c.setStrokeColor(RULE); c.setLineWidth(0.6)
        c.rect(0,0,self._w,self._h,stroke=1,fill=1)
        y = self._h - self.pad - self.size
        for ln in self.lines:
            col = INK
            s = ln
            if s.startswith("#"): col = INK_FAINT
            c.setFillColor(col); c.setFont("Mono", self.size)
            c.drawString(self.pad+2, y, s)
            y -= self.lead

def mono(t):  return f'<font face="Mono" size="9">{t}</font>'
def b(t):     return f"<b>{t}</b>"
def i(t):     return f"<i>{t}</i>"
def acc(t):   return f'<font color="#0E6B61">{t}</font>'
def sig(t):   return f'<font color="#A8412F">{t}</font>'

# ---------------------------------------------------------------- page furniture
_state = {"section": "", "phase": "PHASE 1  \u2022  BLOCKCHAIN FUNDAMENTALS",
          "cover": "STUDY GUIDE  \u2022  PHASE 1 OF 8"}
def set_phase(running, cover):
    _state["phase"] = running
    _state["cover"] = cover
def set_section(name): _state["section"] = name

class SectionMark(Flowable):
    """Zero-height flowable that updates the running header."""
    def __init__(self, name): Flowable.__init__(self); self.name=name
    def wrap(self, aw, ah): return (0,0)
    def draw(self): set_section(self.name)

def on_page(canvas, doc):
    """Drawn BEFORE the page's flowables: rules, footer, left-hand header."""
    canvas.saveState()
    y = PAGE_H - MARGIN_T + 7*mm
    canvas.setStrokeColor(RULE); canvas.setLineWidth(0.6)
    canvas.line(MARGIN_L, y-2.5*mm, PAGE_W-MARGIN_R, y-2.5*mm)
    canvas.setFont("Label", 7.4); canvas.setFillColor(INK_FAINT)
    canvas.drawString(MARGIN_L, y, _state["phase"])
    fy = MARGIN_B - 9*mm
    canvas.setStrokeColor(RULE); canvas.line(MARGIN_L, fy+5*mm, PAGE_W-MARGIN_R, fy+5*mm)
    canvas.setFont("Mono", 8); canvas.setFillColor(INK_FAINT)
    canvas.drawRightString(PAGE_W-MARGIN_R, fy+1.5*mm, str(canvas.getPageNumber()))
    canvas.setFont("Body-I", 8); canvas.setFillColor(INK_FAINT)
    canvas.drawString(MARGIN_L, fy+1.5*mm, "Algorand on Hyperledger Fabric")
    canvas.restoreState()

def on_page_end(canvas, doc):
    """Drawn AFTER the page's flowables, so _state["section"] reflects THIS page.

    Drawing it in on_page would print the previous page's section, because
    reportlab calls onPage before any flowable on that page has been laid down.
    """
    sec = _state["section"]
    if not sec:
        return
    canvas.saveState()
    y = PAGE_H - MARGIN_T + 7*mm
    canvas.setFont("Label", 7.4); canvas.setFillColor(ACCENT)
    canvas.drawRightString(PAGE_W-MARGIN_R, y, sec.upper())
    canvas.restoreState()

def on_cover(canvas, doc):
    canvas.saveState()
    canvas.setFillColor(INK)
    canvas.rect(0, PAGE_H-13*mm, PAGE_W, 13*mm, stroke=0, fill=1)
    canvas.setFillColor(ACCENT)
    canvas.rect(0, PAGE_H-13*mm, 78*mm, 13*mm, stroke=0, fill=1)
    canvas.setFont("Label", 8.6); canvas.setFillColor(PAPER)
    canvas.drawString(MARGIN_L, PAGE_H-8.6*mm, "STUDY GUIDE  •  PHASE 1 OF 8")
    canvas.restoreState()
