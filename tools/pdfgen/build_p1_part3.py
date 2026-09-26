"""Phase 1 study PDF — glossary, exercise, checkpoint, and document assembly."""
from reportlab.platypus import (BaseDocTemplate, PageTemplate, Frame,
                                NextPageTemplate, Paragraph, Spacer, PageBreak, KeepTogether,
                                Table, TableStyle)
from reportlab.lib.units import mm
from pdfbase_kit import *
from pdfdiagrams_a import *
from pdfdiagrams_b import *
from build_p1_part1 import (P, H1, H2, H3, EY, SP, BU, NUM, term_table, grid_table,
                            cover, how_to_use, sec1, sec2, sec3, sec4, sec5)
from build_p1_part2 import sec6, sec7, sec8, sec9, sec10, sec11, sec12, sec13

# =========================================================== 14. GLOSSARY
GLOSSARY = [
 ("Avalanche effect", "Changing one bit of a hash function's input changes the whole output. "
  "Why you cannot tell from a hash whether two inputs were similar. <i>Section 3</i>"),
 ("Block", "A batch of transactions plus metadata, including the hash of the previous block. "
  "<i>Section 6</i>"),
 ("Byzantine fault", "A node that behaves arbitrarily — lying, equivocating, colluding — "
  "rather than simply stopping. Named after the Byzantine Generals problem. <i>Section 9</i>"),
 ("Central authority", "A single trusted party holding the definitive copy of a record. "
  "<i>Section 1</i>"),
 ("Consensus", "All honest nodes agreeing on the same ledger in the same order. "
  "<i>Section 8</i>"),
 ("Crash fault", "A node that stops responding. Far easier to tolerate than a Byzantine "
  "fault, because it never says anything false. <i>Section 9</i>"),
 ("Decentralised", "No single party holds the true copy; the network collectively does. "
  "<i>Section 1</i>"),
 ("Double spending", "Spending the same money twice by telling different parts of the network "
  "different things. The core problem. <i>Section 2</i>"),
 ("Equivocation", "Sending contradictory messages to different peers. The signature "
  "characteristic of a Byzantine node. <i>Section 9</i>"),
 ("Finality", "The guarantee that a committed transaction can never be reversed. "
  "<b>Probabilistic</b> finality (Bitcoin) means reversal grows unlikely; <b>absolute</b> "
  "finality (Algorand) means it is impossible. <i>Sections 11, 12</i>"),
 ("Fork", "Two valid but incompatible versions of the chain existing at once. "
  "<i>Section 11</i>"),
 ("Gossip", "A protocol where each node forwards messages to a few random peers, so news "
  "spreads without any central broadcaster. <i>Section 7</i>"),
 ("Hash function", "A one-way function producing a fixed-size fingerprint of any input. "
  "<i>Section 3</i>"),
 ("Hexadecimal", "Base-16 notation using 0-9 and a-f, used to print raw bytes compactly. "
  "<i>Section 3</i>"),
 ("Immutable", "Not literally unchangeable, but changeable only in a way that is immediately "
  "detectable by everyone. <i>Section 6</i>"),
 ("Ledger", "A record of who owns what and how it moved. A list. <i>Section 1</i>"),
 ("Liveness", "The property that the system keeps making progress. <i>Section 8</i>"),
 ("Mining", "Repeatedly guessing nonces to find one satisfying the proof-of-work target. "
  "<i>Section 11</i>"),
 ("Node", "One computer running the blockchain software and holding a copy of the chain. "
  "<i>Section 7</i>"),
 ("Nonce", "A \"number used once\" — the value miners search for. <i>Section 11</i>"),
 ("Nothing at stake", "The problem that in proof of stake, signing conflicting blocks costs "
  "nothing. <i>Section 12</i>"),
 ("Peer-to-peer", "Every participant is both client and server; there is no privileged "
  "machine. <i>Section 7</i>"),
 ("Permissioned", "A network where membership is controlled and identities are known. "
  "Fabric is one. <i>Section 13</i>"),
 ("Private key (sk)", "The secret half of a key pair. Effectively, it is your money. "
  "<i>Section 4</i>"),
 ("Proof of Stake", "Sybil resistance by tying influence to money held. <i>Section 12</i>"),
 ("Proof of Work", "Sybil resistance by tying influence to computation performed. "
  "<i>Section 11</i>"),
 ("Public key (pk)", "The shareable half of a key pair. Effectively, your address. "
  "<i>Section 4</i>"),
 ("Safety", "The property that honest nodes never accept conflicting histories. The one you "
  "never trade away. <i>Section 8</i>"),
 ("SHA-256", "The specific hash function used by Bitcoin and by your project. Produces 256 "
  "bits, shown as 64 hex characters. <i>Section 3</i>"),
 ("Slashing", "Destroying a misbehaving participant's deposit as punishment. Notably "
  "<b>absent</b> from Algorand. <i>Section 12</i>"),
 ("Sybil attack", "Gaining disproportionate influence by creating many fake identities. "
  "<i>Section 10</i>"),
 ("Throughput", "How much the system can process per unit time — transactions per second, or "
  "megabytes per hour. <i>Section 11</i>"),
]

def sec14():
    f = [SectionMark("14 • Glossary"), PageBreak()]
    f = [SectionMark("14 • Glossary")]
    f.append(EY("SECTION 14"))
    f.append(H1("Glossary"))
    f.append(SP(3))
    f.append(P("Every technical term used in this document, with the section that introduces "
               "it. Come back here whenever a word stops meaning anything.", "lead"))
    f.append(SP(4))
    f.append(term_table([(a, bd) for a, bd in GLOSSARY], widths=(120, None)))
    f.append(PageBreak())
    return f

# =========================================================== 15. BUILD IT
TOY = [
"import hashlib, json, time",
"",
"def sha(data: str) -> str:",
"    return hashlib.sha256(data.encode()).hexdigest()",
"",
"class Block:",
"    def __init__(self, index, transactions, prev_hash):",
"        self.index        = index",
"        self.timestamp    = time.time()",
"        self.transactions = transactions",
"        self.prev_hash    = prev_hash",
"",
"    def hash(self) -> str:",
"        # the hash must cover EVERY field, or a field could be",
"        # edited without detection. sort_keys makes it deterministic.",
"        payload = json.dumps({",
"            'index':        self.index,",
"            'timestamp':    self.timestamp,",
"            'transactions': self.transactions,",
"            'prev_hash':    self.prev_hash,",
"        }, sort_keys=True)",
"        return sha(payload)",
"",
"class Chain:",
"    def __init__(self):",
"        # the genesis block: the only one with no predecessor",
"        self.blocks = [Block(0, ['genesis'], '0'*64)]",
"",
"    def add(self, transactions):",
"        prev = self.blocks[-1]",
"        self.blocks.append(Block(len(self.blocks), transactions, prev.hash()))",
"",
"    def validate(self) -> bool:",
"        for i in range(1, len(self.blocks)):",
"            cur, prev = self.blocks[i], self.blocks[i-1]",
"            if cur.prev_hash != prev.hash():",
"                print(f'  BROKEN at block {i}:')",
"                print(f'    block {i} stores prev_hash {cur.prev_hash[:16]}...')",
"                print(f'    block {i-1} actually hashes to {prev.hash()[:16]}...')",
"                return False",
"        return True",
"",
"# ---- try it -------------------------------------------------",
"c = Chain()",
"c.add(['swayam pays alice 500'])",
"c.add(['alice pays bob 200'])",
"c.add(['bob pays carol 50'])",
"",
"print('intact chain valid? ', c.validate())",
"",
"# now tamper with block 1 and watch the damage propagate",
"c.blocks[1].transactions = ['swayam pays alice 5000000']",
"print('after tampering:    ', c.validate())",
]

def sec15():
    f = [SectionMark("15 \u2022 Build it yourself")]
    f.append(EY("SECTION 15  \u2022  HANDS-ON"))
    f.append(H1("Build a blockchain in 60 lines"))
    f.append(SP(4))
    f.append(P("Reading about hash chains and building one are different kinds of knowing. "
        "This needs nothing installed \u2014 Python 3 is already on your machine.", "lead"))
    f.append(P("Save the three parts below into one file, " + mono("toychain.py") + ", "
               "then run " + mono("python3 toychain.py") + "."))
    f.append(SP(6))

    f.append(KeepTogether([H3("Part 1 \u2014 a block, and how it hashes itself"),
                           CodeBlock(TOY[:23], size=7.4)]))
    f.append(SP(9))
    f.append(P("The only subtle line is " + mono("sort_keys=True") + ". Dictionaries have no "
        "guaranteed order, so without it the same block could serialise two different ways and "
        "produce two different hashes \u2014 and validation would fail at random. Note also that "
        "the hash covers " + b("every") + " field: leave one out and that field could be edited "
        "undetected."))
    f.append(SP(4))

    f.append(KeepTogether([H3("Part 2 \u2014 the chain, and the validation walk"),
                           CodeBlock(TOY[23:42], size=7.4)]))
    f.append(SP(9))
    f.append(P(mono("validate()") + " is the whole idea of section 6 in six lines: walk the "
        "chain, and at each step check that what a block " + i("claims") + " its predecessor "
        "hashed to still matches what that predecessor " + i("actually") + " hashes to now."))
    f.append(SP(4))

    f.append(KeepTogether([H3("Part 3 \u2014 run it, then break it"),
                           CodeBlock(TOY[42:], size=7.4)]))
    f.append(SP(10))

    f.append(KeepTogether([
        H2("What you should see"),
        CodeBlock([
            "intact chain valid?  True",
            "  BROKEN at block 2:",
            "    block 2 stores prev_hash 3f8a1c04b2e97d55...",
            "    block 1 actually hashes to c710d9e4a8f30126...",
            "after tampering:     False",
        ], size=7.6),
    ]))
    f.append(SP(10))
    f.append(Callout("Read the output carefully \u2014 this is section 6 made real", [
        "You edited block <b>1</b>, but the error is reported at block <b>2</b>. That is the "
        "cascade: block 2 is the first place the damage becomes <i>visible</i>, because block 2 "
        "is what stores block 1's old hash.",
        "Now try to repair it. Set <font face='Mono' size='8'>c.blocks[2].prev_hash = "
        "c.blocks[1].hash()</font> and validate again. It will fail at block 3 instead. Repair "
        "that, and it fails at block 4. <b>You have to redo the entire chain.</b>",
    ], "key"))
    f.append(SP(8))
    f.append(H2("Three extensions, if you want to go further"))
    f.append(NUM(1, b("Add signatures.") + " Use " + mono("cryptography") + "'s Ed25519 so each "
        "transaction is signed, and have " + mono("validate()") + " reject unsigned ones. This "
        "makes section 4 concrete."))
    f.append(NUM(2, b("Add proof of work.") + " Give " + mono("Block") + " a " + mono("nonce") +
        " and loop until the hash starts with four zeros. Time it, then try five zeros. You "
        "will feel section 11 in your laptop fan."))
    f.append(NUM(3, b("Make two chains disagree.") + " Build two " + mono("Chain") + " objects, "
        "add different blocks to each, and write the rule that picks a winner. You have just "
        "written fork resolution."))
    f.append(PageBreak())
    return f

# =========================================================== 16. CHECKPOINT
QUESTIONS = [
 ("Why is a bank able to prevent double spending trivially, while a decentralised network "
  "cannot?", "2"),
 ("What are the four properties of a hash function, and which one makes mining possible?", "3"),
 ("A signature proves who authorised a transaction. Name precisely what it does <b>not</b> "
  "prove, and why that gap matters.", "4, 5"),
 ("You edit a transaction in block 10 of a 20-block chain. Describe exactly what breaks, and "
  "what an attacker would have to do to hide it.", "6"),
 ("Why is a Byzantine fault harder to tolerate than a crash fault? Give a concrete example "
  "where signatures do not help.", "9"),
 ("Explain the reasoning behind <font face='Mono' size='9'>n &gt; 3f</font> — why two thirds "
  "and not one half?", "9"),
 ("What is a Sybil attack, and how do proof of work and proof of stake each defeat it? Note "
  "they defeat it in genuinely different ways.", "10, 11, 12"),
 ("Why does Bitcoin make you wait roughly an hour? Name the two separate design decisions "
  "that add up to that number.", "11"),
 ("What is the difference between probabilistic and absolute finality, and which does "
  "Algorand provide?", "11, 12"),
 ("Hyperledger Fabric is permissioned, so every member is known and vetted. Why does it still "
  "need Byzantine fault tolerance? (This is the justification for your entire project.)", "13"),
]

ANSWERS = [
 "The bank is a single point of decision: it sees both transactions and processes whichever "
 "arrived first. A decentralised network has no single observer, so two halves can each "
 "legitimately believe they saw the first one. Nothing inside a signed transaction records "
 "when it was created relative to another.",

 "Deterministic, fixed size, one-way, avalanche. <b>One-way</b> makes mining possible: since "
 "you cannot invert the function, the only way to find a nonce producing a small hash is to "
 "guess, which is what makes the work real.",

 "It does not prove <b>ordering</b> — when this transaction happened relative to any other. "
 "I can sign two conflicting transactions and both signatures verify perfectly. Authenticity "
 "and ordering are separate problems, and only the second needs consensus.",

 "Editing block 10 changes its hash completely (avalanche). Block 11 stores block 10's old "
 "hash, so block 11 no longer matches. Repairing block 11 changes block 11's hash, breaking "
 "block 12, and so on. To hide it the attacker must recompute every block from 10 to 20 — "
 "and in proof of work, redo all of that mining faster than the honest network extends the "
 "real chain.",

 "A crashed node is silent, so everyone still sees a consistent (if incomplete) world. A "
 "Byzantine node tells different peers different things, so honest nodes end up confidently "
 "believing contradictory facts. Signatures do not help because the liar signs both messages "
 "honestly — each is genuinely from them. The signature proves authorship, not consistency.",

 "You must be able to decide without hearing from <font face='Mono' size='9'>f</font> nodes, "
 "since they may be silent — so you act on <font face='Mono' size='9'>n - f</font> replies. "
 "Among those, up to <font face='Mono' size='9'>f</font> may be lies. For the honest replies "
 "to still outnumber the lies you need "
 "<font face='Mono' size='9'>(n - f) - f &gt; f</font>, that is "
 "<font face='Mono' size='9'>n &gt; 3f</font>. One half is not enough because a half-sized "
 "honest majority can be exactly cancelled by liars plus the unreachable.",

 "A Sybil attack is one party gaining outsized influence by posing as many. Proof of work "
 "defeats it <b>physically</b> — a million identities on one machine still have that "
 "machine's hash rate. Proof of stake defeats it <b>arithmetically</b> — splitting a balance "
 "across accounts leaves the total unchanged. Neither makes identities scarce; both make "
 "identities irrelevant.",

 "Two decisions compound. First, blocks are deliberately slow (ten minutes) so that "
 "simultaneous wins, and therefore forks, stay rare. Second, because forks still happen, you "
 "wait about six blocks for the branch to be settled. Six times ten minutes is roughly an "
 "hour.",

 "Probabilistic finality means a transaction becomes exponentially <i>unlikely</i> to be "
 "reversed as blocks pile on, but never certain — that is Bitcoin. Absolute finality means "
 "reversal is mathematically impossible once committed — that is Algorand, and it is the "
 "central claim of the paper you are implementing.",

 "Known does not mean trustworthy. A vetted member can still be compromised, coerced, or "
 "simply buggy — identity tells you who misbehaved, it does not prevent it. Fabric 2.5 ships "
 "only Raft, which tolerates nodes that <b>crash</b> but not nodes that <b>lie</b>; Solo and "
 "Kafka are deprecated and no BFT option exists. So Fabric 2.5 has no Byzantine-tolerant "
 "ordering service at all — which is the gap your project fills.",
]

def sec16():
    f = [SectionMark("16 • Checkpoint")]
    f.append(EY("SECTION 16"))
    f.append(H1("Checkpoint"))
    f.append(SP(3))
    f.append(P("Answer these out loud, without notes, before moving to Phase 2. The section to "
        "revisit is named beside each one. Model answers follow — resist reading them first.",
        "lead"))
    f.append(SP(4))
    rows = []
    for k, (q, s) in enumerate(QUESTIONS, 1):
        rows.append([
            Paragraph(f'<font face="Mono" size="8.5" color="#0E6B61">{k:02d}</font>', S["tbl"]),
            Paragraph(q, S["tbl"]),
            Paragraph(f'<font size="8" color="#767C96">§{s}</font>', S["tbl"]),
        ])
    t = Table(rows, colWidths=[20, CONTENT_W-64, 44])
    t.setStyle(TableStyle([
        ("VALIGN", (0,0), (-1,-1), "TOP"),
        ("LINEBELOW", (0,0), (-1,-2), 0.4, RULE),
        ("TOPPADDING", (0,0), (-1,-1), 6),
        ("BOTTOMPADDING", (0,0), (-1,-1), 6),
        ("LEFTPADDING", (0,0), (-1,-1), 0),
    ]))
    f.append(t)
    f.append(PageBreak())

    f.append(SectionMark("16 • Answers"))
    f.append(EY("SECTION 16  •  MODEL ANSWERS"))
    f.append(H1("Answers"))
    f.append(SP(3))
    f.append(P("If your answer covered the same ground in your own words, you have it. Exact "
               "wording does not matter; the mechanism does.", "lead"))
    f.append(SP(3))
    for k, a in enumerate(ANSWERS, 1):
        f.append(Paragraph(f'<font face="Mono" size="9" color="#0E6B61">{k:02d}</font>  ' + a,
                           ParagraphStyle("ans", parent=S["body"], fontSize=9.6, leading=14.4,
                                          spaceAfter=9)))
    f.append(SP(8))
    f.append(Callout("You are done with Phase 1", [
        "Next is <b>Phase 2 — Cryptography and VRFs</b>, about ten hours. It goes deeper on "
        "hashing and signatures, then introduces the Verifiable Random Function, which is the "
        "single idea Algorand is built on.",
        "If you would rather keep momentum on the tooling side instead, the Homebrew and Go "
        "installs are still outstanding, and Phase 5 can start the moment they finish.",
    ], "note"))
    return f

# =========================================================== ASSEMBLE
def build(path="Phase1_Blockchain_Fundamentals.pdf"):
    doc = BaseDocTemplate(
        path, pagesize=(PAGE_W, PAGE_H),
        leftMargin=MARGIN_L, rightMargin=MARGIN_R,
        topMargin=MARGIN_T, bottomMargin=MARGIN_B,
        title="Blockchain Fundamentals - Phase 1",
        author="Study guide for the Algorand-on-Fabric project",
        subject="Phase 1 of 8: blockchain fundamentals from first principles",
    )
    frame = Frame(MARGIN_L, MARGIN_B, CONTENT_W, PAGE_H - MARGIN_T - MARGIN_B,
                  id="body", leftPadding=0, rightPadding=0,
                  topPadding=0, bottomPadding=0)
    doc.addPageTemplates([
        PageTemplate(id="cover", frames=[frame], onPage=on_cover),
        # onPageEnd, not onPage, so the running section label reflects the page
        # it is printed on rather than the previous one
        PageTemplate(id="body", frames=[frame], onPage=on_page, onPageEnd=on_page_end),
    ])

    story = [NextPageTemplate("body")]
    story += cover()
    story += how_to_use()
    for s in (sec1, sec2, sec3, sec4, sec5, sec6, sec7, sec8,
              sec9, sec10, sec11, sec12, sec13, sec14, sec15, sec16):
        story += s()
    doc.build(story)
    return path

if __name__ == "__main__":
    p = build()
    import pypdf
    r = pypdf.PdfReader(p)
    print(f"built {p}: {len(r.pages)} pages")
