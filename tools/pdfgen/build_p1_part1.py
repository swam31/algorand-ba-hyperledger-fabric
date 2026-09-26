"""Phase 1 study PDF — front matter and sections 1-5."""
from reportlab.platypus import (SimpleDocTemplate, Paragraph, Spacer, PageBreak,
                                Table, TableStyle, KeepTogether)
from reportlab.lib.units import mm
from reportlab.lib import colors
from pdfbase_kit import *
from pdfdiagrams_a import *
from pdfdiagrams_b import *

def P(t, st="body"):   return Paragraph(t, S[st])
def H1(t):             return Paragraph(t, S["h1"])
def H2(t):             return Paragraph(t, S["h2"])
def H3(t):             return Paragraph(t, S["h3"])
def EY(t):             return Paragraph(t, S["eyebrow"])
def SP(h=6):           return Spacer(1, h)
def BU(t):             return Paragraph(t, S["bullet"], bulletText="•")
def NUM(n, t):         return Paragraph(t, S["bullet"], bulletText=f"{n}.")

def term_table(rows, widths=(112, None), total=CONTENT_W):
    w0 = widths[0]; w1 = total - w0
    data = [[Paragraph(f"<b>{a}</b>", S["tbl"]), Paragraph(bdy, S["tbl"])] for a, bdy in rows]
    t = Table(data, colWidths=[w0, w1])
    t.setStyle(TableStyle([
        ("VALIGN", (0,0), (-1,-1), "TOP"),
        ("LINEBELOW", (0,0), (-1,-2), 0.5, RULE),
        ("TOPPADDING", (0,0), (-1,-1), 5),
        ("BOTTOMPADDING", (0,0), (-1,-1), 5),
        ("LEFTPADDING", (0,0), (0,-1), 0),
        ("RIGHTPADDING", (1,0), (1,-1), 0),
    ]))
    return t

def grid_table(header, rows, widths, total=CONTENT_W):
    cw = [total*x for x in widths]
    data = [[Paragraph(h, S["tblh"]) for h in header]]
    for r in rows:
        data.append([Paragraph(cell, S["tbl"]) for cell in r])
    t = Table(data, colWidths=cw, repeatRows=1)
    t.setStyle(TableStyle([
        ("VALIGN", (0,0), (-1,-1), "TOP"),
        ("BACKGROUND", (0,0), (-1,0), SURFACE),
        ("LINEBELOW", (0,0), (-1,0), 0.8, INK),
        ("LINEBELOW", (0,1), (-1,-2), 0.4, RULE),
        ("TOPPADDING", (0,0), (-1,-1), 6),
        ("BOTTOMPADDING", (0,0), (-1,-1), 6),
        ("LEFTPADDING", (0,0), (-1,-1), 7),
        ("RIGHTPADDING", (0,0), (-1,-1), 7),
    ]))
    return t

# =========================================================== COVER
def cover():
    f = []
    f.append(Spacer(1, 34*mm))
    f.append(Paragraph("Blockchain", S["cover_t"]))
    f.append(Paragraph('<font color="#0E6B61">Fundamentals</font>', S["cover_t"]))
    f.append(Spacer(1, 7*mm))
    f.append(Rule(color=INK, thickness=1.6, space=10))
    f.append(Spacer(1, 3*mm))
    f.append(Paragraph(
        "Everything in Phase 1, explained from nothing. No prior knowledge assumed, "
        "every technical word defined the first time it appears.", S["cover_s"]))
    f.append(Spacer(1, 16*mm))

    rows = [
        ("1", "Money is just a list", "why removing the bank is hard"),
        ("2", "The double-spending problem", "the one problem all of this solves"),
        ("3", "Hash functions", "tamper-evident fingerprints"),
        ("4", "Keys and signatures", "ownership without accounts"),
        ("5", "What a transaction is", "putting the tools together"),
        ("6", "Blocks and the chain", "why the past cannot be rewritten"),
        ("7", "Nodes and gossip", "how the network actually talks"),
        ("8", "Consensus", "safety, liveness, and what can go wrong"),
        ("9", "Byzantine faults", "liars, and the two-thirds rule"),
        ("10", "Sybil attacks", "why fake identities must be made useless"),
        ("11", "Proof of Work", "Bitcoin's answer, and its price"),
        ("12", "Proof of Stake", "the alternative Algorand builds on"),
        ("13", "Public vs permissioned", "where Hyperledger Fabric sits"),
        ("14", "Glossary", "every term in one place"),
        ("15", "Build it yourself", "a working chain in 60 lines of Python"),
        ("16", "Checkpoint", "questions, then answers"),
    ]
    data = []
    for n, title, sub in rows:
        data.append([
            Paragraph(f'<font face="Mono" size="8" color="#0E6B61">{n.rjust(2)}</font>', S["tbl"]),
            Paragraph(f"<b>{title}</b>  <font color='#767C96'>— {sub}</font>", S["tbl"]),
        ])
    t = Table(data, colWidths=[22, CONTENT_W-22])
    t.setStyle(TableStyle([
        ("VALIGN", (0,0), (-1,-1), "TOP"),
        ("TOPPADDING", (0,0), (-1,-1), 3.4),
        ("BOTTOMPADDING", (0,0), (-1,-1), 3.4),
        ("LEFTPADDING", (0,0), (-1,-1), 0),
    ]))
    f.append(Paragraph("CONTENTS", S["eyebrow"]))
    f.append(SP(3)); f.append(t)
    f.append(PageBreak())
    return f

# =========================================================== HOW TO USE
def how_to_use():
    f = [SectionMark("How to use this")]
    f.append(EY("BEFORE YOU START"))
    f.append(H1("How to read this document"))
    f.append(SP(4))
    f.append(P(
        "This covers Phase 1 of your eight-phase plan. It is the foundation everything else "
        "stands on, so it is worth going slowly. Budget about six hours."))
    f.append(SP(2))
    f.append(P(b("Read it in order.") + " Each section uses only ideas already introduced. "
        "Section 6 will not make sense without section 3, and section 11 will not make sense "
        "without section 9."))
    f.append(P(b("Every bold-italic term is defined where it first appears") +
        ", and again in the glossary in section 14. If a word looks like jargon, it is, and "
        "it will have been explained."))
    f.append(P(b("Do section 15.") + " Reading about hash chains and building one are different "
        "kinds of knowing. It is sixty lines and it will take you under an hour."))
    f.append(P(b("Then answer section 16 out loud, without notes.") + " If you cannot, the "
        "section that failed you is named next to each question. Go back to it."))
    f.append(SP(6))
    f.append(Callout("What you will be able to do afterwards", [
        "Explain what a blockchain is to someone who has never heard of one, without hand-waving.",
        "Say precisely why Bitcoin makes you wait an hour, and why Algorand does not.",
        "Read the words <i>ledger, hash, signature, node, fork, consensus, Byzantine, Sybil, "
        "finality</i> in a research paper without pausing.",
        "Understand why your project's consensus algorithm exists at all — which is the "
        "thing that makes Phase 3 possible.",
    ], "note"))
    f.append(SP(8))
    f.append(Callout("One honest warning", [
        "Blockchain writing is full of hype and full of people repeating things they have not "
        "checked. This document sticks to mechanism: what the machine actually does. Where "
        "something is a trade-off rather than a triumph, it says so.",
    ], "warn"))
    f.append(PageBreak())
    return f

# =========================================================== 1. LEDGER
def sec1():
    f = [SectionMark("1 • Money is a list")]
    f.append(EY("SECTION 1"))
    f.append(H1("Money is just a list"))
    f.append(SP(4))
    f.append(P("Forget blockchain entirely for a moment. Think about your bank.", "lead"))
    f.append(P(
        "Your money is not a physical thing in a vault with your name on it. It is a number "
        "in a database the bank owns. When you pay a friend 500 rupees, nothing moves. The "
        "bank edits two rows:"))
    f.append(CodeBlock([
        "Swayam    10,000  ->   9,500",
        "Friend     2,000  ->   2,500",
    ]))
    f.append(SP(9))
    f.append(P(
        "That is all money is: a " + b("ledger") + " — a list of who owns what, and a history "
        "of transfers. It works because everyone trusts the bank to keep the list honestly. "
        "The bank is a " + b("central authority") + ": one organisation whose copy of the list "
        "is, by definition, the correct one."))
    f.append(SP(3))
    f.append(Paragraph(
        "The question that created blockchain is simply: " +
        i("can we keep that list without anyone being in charge of it?"), S["quote"]))
    f.append(P(
        "A list maintained by thousands of strangers, where no single one of them can cheat, "
        "and no single one of them can be shut down. That sounds achievable until you look at "
        "what goes wrong."))
    f.append(SP(6))
    f.append(D1Ledger())
    f.append(SP(6))
    f.append(Callout("Words defined in this section", [
        "<b>Ledger</b> — a record of accounts and transfers. A list. Nothing more mysterious than that.",
        "<b>Central authority</b> — a single party everyone trusts to hold the true copy. "
        "A bank, a government, a company database.",
        "<b>Decentralised</b> — the opposite: no single party holds the true copy; the network "
        "collectively does.",
    ], "key"))
    f.append(SecBreak())
    return f

# =========================================================== 2. DOUBLE SPEND
def sec2():
    f = [SectionMark("2 • Double spending")]
    f.append(EY("SECTION 2"))
    f.append(H1("The double-spending problem"))
    f.append(SP(4))
    f.append(P(
        "Here is what goes wrong the moment you remove the bank.", "lead"))
    f.append(P(
        "Suppose I have one coin. I send it to Alice, and at the same instant I send the same "
        "coin to Bob — announcing each to a different part of the network. Who actually has it?"))
    f.append(P(
        "With a bank this is trivial. The bank sees both requests, processes whichever arrived "
        "first, and rejects the second. " + b("One entity, therefore one order of events.")))
    f.append(P(
        "Without a bank it is brutal. Alice's copy of the ledger says she was paid. Bob's copy "
        "says he was. Both copies are internally consistent. Both payments are correctly signed "
        "by me. Nothing inside either message says which one I sent first — or that the other "
        "exists at all."))
    f.append(SP(5))
    f.append(D2DoubleSpend())
    f.append(SP(5))
    f.append(Callout("This is the whole problem", [
        "<b>Double spending</b> is the core problem every cryptocurrency exists to solve: "
        "spending the same money twice by telling different people different things.",
        "Every mechanism in the rest of this document — hashes, chains, mining, committees, "
        "voting — is machinery built to answer one question: <b>in what order did these events "
        "happen, and who says so?</b>",
        "Keep this in mind. When a later section feels arbitrary, ask how it helps establish "
        "order, and it will usually snap into focus.",
    ], "key"))
    f.append(SecBreak())
    return f

# =========================================================== 3. HASHES
def sec3():
    f = [SectionMark("3 • Hash functions")]
    f.append(EY("SECTION 3  •  TOOL ONE"))
    f.append(H1("Hash functions"))
    f.append(SP(4))
    f.append(P(
        "A hash function is a machine that eats data of any size and produces a fixed-length "
        "fingerprint.", "lead"))
    f.append(D3Hash())
    f.append(SP(4))
    f.append(H2("The four properties that matter"))
    f.append(term_table([
        ("Deterministic", "The same input always produces the same output. Every computer on "
         "earth, run a century apart, gets the same answer."),
        ("Fixed size", "A one-gigabyte film and the word <i>hi</i> both produce the same "
         "64-character output. Size in, size out, unrelated."),
        ("One-way", "Given the output, you cannot work backwards to the input. Your only "
         "option is to guess inputs and check. This property is what mining is built on."),
        ("Avalanche", "Change one bit of input and the entire output changes unrecognisably. "
         "There is no partial similarity, no gradient, no clue."),
    ]))
    f.append(SP(9))
    f.append(H2("What it is actually for"))
    f.append(P(
        "A hash is a " + b("tamper-evident seal") + ". Publish the hash of a document today; "
        "if anyone alters so much as a comma later, the hash will not match. You get a "
        "32-byte proof that a file of any size is unchanged."))
    f.append(P(
        "That is the property section 6 turns into a chain. Hold onto it."))
    f.append(SP(4))
    f.append(Callout("SHA-256", [
        "<b>SHA-256</b> is the specific hash function Bitcoin uses, and the one we use in your "
        "project. The name means <i>Secure Hash Algorithm, 256 bits of output</i>. 256 bits is "
        "64 hexadecimal characters.",
        "<b>Hexadecimal</b> (hex) just means counting in base 16, using 0-9 then a-f. It is a "
        "compact way to print raw bytes. <font face='Mono' size='8'>ff</font> is 255; "
        "<font face='Mono' size='8'>00</font> is zero.",
        "You will see hashes written as <font face='Mono' size='8'>2cf24dba...</font> — that is "
        "just the first few hex characters, shown because the full 64 are unwieldy.",
    ], "note"))
    f.append(SecBreak())
    return f

# =========================================================== 4. SIGNATURES
def sec4():
    f = [SectionMark("4 • Keys and signatures")]
    f.append(EY("SECTION 4  •  TOOL TWO"))
    f.append(H1("Public keys and digital signatures"))
    f.append(SP(4))
    f.append(P(
        "This is the tool that makes ownership possible with no accounts, no usernames, "
        "and no passwords.", "lead"))
    f.append(P(
        "You generate a " + b("key pair") + " — two mathematically linked numbers:"))
    f.append(BU(b("Private key") + " (written " + mono("sk") + ", for <i>secret key</i>) — an "
        "enormous secret number. Only you have it. " + b("This is your money.") + " Lose it and "
        "the funds are gone permanently; there is no reset, because there is nobody to ask."))
    f.append(BU(b("Public key") + " (written " + mono("pk") + ") — derived from the private key "
        "and shared freely. " + b("This is your address.") + " People send you money here."))
    f.append(SP(3))
    f.append(P("The essential asymmetry: you can compute " + mono("pk") + " from " + mono("sk") +
        ", but never " + mono("sk") + " from " + mono("pk") + ". The maths runs one way only."))
    f.append(SP(5))
    f.append(D4Signature())
    f.append(SP(4))
    f.append(P(
        "So I can write " + i("pay Alice 500") + ", sign it with my private key, and shout it "
        "across the world. Anyone can verify — using my " + i("public") + " key — that the "
        "message genuinely came from whoever holds that private key, and that not a single "
        "character was altered in transit. " + b("And I never revealed my secret.")))
    f.append(SP(4))
    f.append(Callout("Why this beats a handwritten signature", [
        "A physical signature looks the same on every document, so it can be traced and forged. "
        "A digital signature is computed <i>from the message itself</i> — sign a different "
        "message and you get a completely different signature.",
        "That means a signature cannot be lifted from one document and pasted onto another. It "
        "proves not just <i>who</i>, but <i>who agreed to exactly this text</i>.",
    ], "note"))
    f.append(SecBreak())
    return f

# =========================================================== 5. TRANSACTION
def sec5():
    f = [SectionMark("5 • Transactions")]
    f.append(EY("SECTION 5"))
    f.append(H1("What a transaction actually is"))
    f.append(SP(4))
    f.append(P("Now we can assemble the two tools into something useful.", "lead"))
    f.append(CodeBlock([
        "Transaction {",
        "    from:      pk_swayam      # my public key",
        "    to:        pk_alice       # her public key",
        "    amount:    500",
        "    signature: Sign(sk_swayam, everything above)",
        "}",
    ]))
    f.append(SP(9))
    f.append(P("Anyone receiving this can check two things independently, with no help from anyone:"))
    f.append(NUM(1, "Is the signature valid for " + mono("pk_swayam") + "? " +
                 "That proves I authorised it."))
    f.append(NUM(2, "Does " + mono("pk_swayam") + " hold at least 500 in the ledger? " +
                 "That proves I am not spending money I do not have."))
    f.append(SP(6))
    f.append(Callout("Notice what is still missing", [
        "Nothing here stops me creating <b>two</b> such transactions spending the same 500. "
        "Both would be perfectly valid on their own. Both would pass both checks.",
        "Signatures solve <b>authenticity</b> — who agreed to what. They do not solve "
        "<b>ordering</b> — what happened first.",
        "That gap is exactly the double-spending problem of section 2, and the next four "
        "sections are about closing it.",
    ], "warn"))
    f.append(SecBreak())
    return f
