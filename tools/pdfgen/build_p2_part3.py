"""Phase 2 study PDF — glossary, exercises, checkpoint, assembly."""
from reportlab.platypus import (BaseDocTemplate, PageTemplate, Frame, NextPageTemplate,
                                Paragraph, Spacer, PageBreak, Table, TableStyle, KeepTogether)
from pdfbase_kit import *
from p2diagrams_a import *
from p2diagrams_b import *
from build_p1_part1 import P, H1, H2, H3, EY, SP, BU, NUM, term_table, grid_table
from build_p2_part1 import cover, how_to_use, sec1, sec2, sec3, sec4, sec5, sec6
from build_p2_part2 import sec7, sec8, sec9, sec10, sec11, sec12

GLOSSARY = [
 ("alpha", "Conventional name for a VRF's input. Public. In Algorand it is "
  "<font face='Mono' size='8'>seed || role</font>. <i>Section 7</i>"),
 ("beta", "Conventional name for a VRF's output — the pseudorandom value. <i>Section 7</i>"),
 ("Binomial distribution", "The distribution of \"how many successes in n independent trials\". "
  "Governs how many committee seats a stake wins. <i>Section 9</i>"),
 ("Birthday bound", "A collision in an n-bit hash appears after about 2^(n/2) tries, not 2^n, "
  "because every new item is compared against all previous ones. <i>Section 3</i>"),
 ("Collision resistance", "Cannot find <b>any</b> two inputs with the same hash. The weakest of "
  "the three hash properties, because the attacker chooses both inputs. <i>Section 2</i>"),
 ("Curve25519", "The specific elliptic curve used by Ed25519, Algorand's VRF, and your engine. "
  "Chosen for speed and for being hard to implement insecurely. <i>Section 5</i>"),
 ("Discrete logarithm", "Recovering k from the point k·G. Believed infeasible; this is the "
  "hardness every elliptic-curve key pair rests on. <i>Section 5</i>"),
 ("ECVRF", "The standards-track VRF construction, RFC 9381. Provides uniqueness even for "
  "adversarially generated keys. <i>Section 12</i>"),
 ("Ed25519", "A signature scheme on Curve25519 whose signing is fully deterministic. "
  "<i>Section 6</i>"),
 ("Generator (G)", "A fixed, publicly agreed starting point on the curve. <i>Section 5</i>"),
 ("Inclusion proof", "The sibling hashes needed to show a leaf belongs to a Merkle root, "
  "without sending the other leaves. <i>Section 4</i>"),
 ("Merkle tree", "A tree of hashes committing to a set of items with a single root value. "
  "<i>Section 4</i>"),
 ("Nonce", "A number used once. In signatures, the per-signature random value that Ed25519 "
  "replaces with a deterministic hash. <i>Section 6</i>"),
 ("Preimage resistance", "Given an output, cannot find any input producing it. <i>Section 2</i>"),
 ("Proof (pi)", "The VRF's third output: convinces a verifier holding pk that beta is correct "
  "for alpha. <i>Section 7</i>"),
 ("Pseudorandomness", "Indistinguishable from random to anyone lacking the secret key. One of "
  "the three VRF properties. <i>Section 7</i>"),
 ("Role", "The label distinguishing one sortition lottery from another — name, round, step. "
  "Changing it yields an independent draw. <i>Section 8</i>"),
 ("Second-preimage resistance", "Given one input, cannot find a different one with the same "
  "hash. What the block chain's immutability rests on. <i>Section 2</i>"),
 ("Seed", "The public per-round randomness fed into sortition, identical for all nodes. "
  "<i>Section 8</i>"),
 ("Sortition", "Selection by lot. Algorand's private, verifiable, stake-weighted committee "
  "draw. <i>Section 8</i>"),
 ("Sub-user", "One winning lottery ticket. A node holding many coins may win many, and casts "
  "that many votes. <i>Section 9</i>"),
 ("tau", "The target committee size — the expected total number of seats handed out. "
  "<i>Section 9</i>"),
 ("Uniqueness", "For one (sk, alpha) there is exactly one beta that verifies. The property "
  "that makes the lottery unriggable — and the one the Ed25519 shortcut only partly "
  "guarantees. <i>Sections 7, 11</i>"),
 ("VRF", "Verifiable Random Function. Produces a random value only you can compute and anyone "
  "can verify. The idea Algorand is built on. <i>Section 7</i>"),
 ("W", "Total stake across all participants. Sets p = tau/W. <i>Section 9</i>"),
]

def sec13():
    f = [SectionMark("13 • Glossary")]
    f.append(EY("SECTION 13"))
    f.append(H1("Glossary"))
    f.append(SP(3))
    f.append(P("New terms from this phase. Phase 1's glossary still applies underneath.", "lead"))
    f.append(SP(4))
    f.append(term_table(GLOSSARY, widths=(122, None)))
    f.append(PageBreak())
    return f

EX1 = [
"from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey",
"import hashlib",
"",
"sk  = Ed25519PrivateKey.from_private_bytes(bytes(range(32)))",
"pk  = sk.public_key()",
"msg = b'pay alice 500'",
"",
"s1, s2 = sk.sign(msg), sk.sign(msg)",
"print('signature #1 :', s1.hex()[:32], '...')",
"print('signature #2 :', s2.hex()[:32], '...')",
"print('identical?   :', s1 == s2)",
]
OUT1 = [
"signature #1 : eae436f7d3b7698b1ea031c37ef813aa ...",
"signature #2 : eae436f7d3b7698b1ea031c37ef813aa ...",
"identical?   : True",
]
EX2 = [
"from cryptography.exceptions import InvalidSignature",
"",
"def vrf_prove(sk, alpha):",
"    proof = sk.sign(alpha)                      # deterministic",
"    return hashlib.sha512(proof).digest(), proof",
"",
"def vrf_verify(pk, alpha, beta, proof):",
"    try:",
"        pk.verify(proof, alpha)",
"    except InvalidSignature:",
"        return False",
"    return hashlib.sha512(proof).digest() == beta",
"",
"alpha = b'seed-round-42|committee|step-3'",
"beta, proof = vrf_prove(sk, alpha)",
"print('beta        :', beta.hex()[:32], '...')",
"print('verify      :', vrf_verify(pk, alpha, beta, proof))",
"print('tampered    :', vrf_verify(pk, alpha, b'\\x00'*64, proof))",
"print('wrong input :', vrf_verify(pk, b'different', beta, proof))",
]
OUT2 = [
"beta        : 0ce7266e59515a5d3fb8b43055c9ea93 ...",
"verify      : True",
"tampered    : False",
"wrong input : False",
]
EX3 = [
"for role in [b'committee|r42|s1', b'committee|r42|s2', b'proposer|r42']:",
"    b, _ = vrf_prove(sk, role)",
"    print(f'{role.decode():22} -> {b.hex()[:24]}...')",
]
OUT3 = [
"committee|r42|s1       -> 043d10fbc4c55a444b640cd4...",
"committee|r42|s2       -> 599a7e1df67d7c723b886cba...",
"proposer|r42           -> 6f195cdb1b70cc8d08d34265...",
]
EX4 = [
"import itertools",
"",
"def find_collision(nbits):",
"    seen = {}",
"    for n, i in enumerate(itertools.count()):",
"        d = hashlib.sha256(str(i).encode()).digest()",
"        t = int.from_bytes(d, 'big') >> (256 - nbits)   # truncate",
"        if t in seen:",
"            return n",
"        seen[t] = i",
"",
"for bits in (16, 24, 32, 40):",
"    tries  = find_collision(bits)",
"    theory = int(2 ** (bits / 2))",
"    print(f'{bits:>3} bits: {tries:>10,} tries   (2^(n/2) = {theory:>9,})')",
]
OUT4 = [
" 16 bits:        251 tries   (2^(n/2) =       256)",
" 24 bits:      8,864 tries   (2^(n/2) =     4,096)",
" 32 bits:     95,303 tries   (2^(n/2) =    65,536)",
" 40 bits:  1,526,146 tries   (2^(n/2) = 1,048,576)",
]

def sec14():
    f = [SectionMark("14 • Build it yourself")]
    f.append(EY("SECTION 14  •  HANDS-ON"))
    f.append(H1("Four exercises, all runnable now"))
    f.append(SP(4))
    f.append(P("These need only the <font face='Mono' size='10'>cryptography</font> package, "
        "which is already installed on your machine. Every output below is real — each was "
        "executed while this document was written.", "lead"))
    f.append(SP(4))

    for title, code, out, note in [
        ("Exercise 1 — prove Ed25519 is deterministic", EX1, OUT1,
         "Run it twice more. The bytes never change. That single fact is what section 6 said "
         "is load-bearing: it means one (key, message) has exactly one signature, so a node "
         "cannot re-roll its lottery."),
        ("Exercise 2 — build a VRF from that determinism", EX2, OUT2,
         "You have just written the same construction as "
         "<font face='Mono' size='8'>engine/vrf/vrf.go</font>. Note what verification needs: "
         "only the public key. The secret never travels."),
        ("Exercise 3 — one seed, many independent lotteries", EX3, OUT3,
         "Three completely unrelated outputs from one key, with no coordination and no extra "
         "messages. This is how Algorand affords a fresh committee for every step."),
        ("Exercise 4 — measure the birthday bound yourself", EX4, OUT4,
         "The measured counts hug 2^(n/2), not 2^n. This is the experiment that produced "
         "Figure 2 — you are reproducing the document's own data."),
    ]:
        f.append(KeepTogether([H3(title), CodeBlock(code, size=7.2)]))
        f.append(SP(8))
        f.append(KeepTogether([
            Paragraph('<font face="Label" size="7.4" color="#767C96">OUTPUT</font>',
                      S["caption"]),
            CodeBlock(out, size=7.2, tint=ACCENT_SOFT)]))
        f.append(SP(7))
        f.append(P(note))
        f.append(SP(10))

    f.append(H2("Exercise 5 — the one that matters most"))
    f.append(P("Open your own project and run the sortition validator that already exists:"))
    f.append(CodeBlock([
        "cd algorand-fabric/tools",
        "python3 verify_sortition.py",
    ]))
    f.append(SP(9))
    f.append(P("It runs five Monte Carlo checks on the exact algorithm in "
        + mono("engine/vrf/sortition.go") + ". Read the output against section 9 and section 10 "
        "and confirm you can explain each result — particularly test 2, where one identity "
        "holding 10,000 stake and a hundred Sybils holding 100 each both average almost exactly "
        "100 seats."))
    f.append(SP(4))
    f.append(Callout("Then read the Go", [
        "With sections 7 to 11 fresh, open <font face='Mono' size='8'>engine/vrf/sortition.go</font> "
        "and read it top to bottom. It is 170 lines and you should now recognise every one: "
        "the role encoding, the VRF call, the binomial CDF walk, the log-space arithmetic, the "
        "53-bit ratio.",
        "If any line is still opaque, the section that explains it is named in the comment above "
        "it. That file was written to be read alongside this document.",
    ], "key"))
    f.append(PageBreak())
    return f

QUESTIONS = [
 ("Name the three properties of a VRF, and say which one makes the lottery impossible to rig.", "7"),
 ("A signature and a VRF both need your private key and both are publicly verifiable. State the "
  "difference in one sentence.", "7"),
 ("Why can a node not simply keep re-computing its VRF until it wins a seat?", "7, 6"),
 ("Explain why splitting 10,000 coins across 100 accounts wins an attacker no extra committee "
  "seats. Use the binomial identity.", "9, 10"),
 ("Sortition's input is <font face='Mono' size='9'>seed || role</font>. What breaks if you drop "
  "the role and use the seed alone?", "8"),
 ("Why does SHA-256 give 128-bit collision resistance rather than 256-bit?", "3"),
 ("Which hash property does a blockchain's immutability actually depend on — preimage, "
  "second-preimage, or collision resistance? Justify it.", "2, 6 of Phase 1"),
 ("What is the discrete logarithm problem, and what in your project would break if it were "
  "solved?", "5"),
 ("Your engine builds its VRF from an Ed25519 signature. State precisely which VRF property is "
  "only partly guaranteed, and why that is acceptable for Fabric.", "11"),
 ("An attacker learns you are on the committee the moment you broadcast your vote. Why does "
  "attacking you at that point gain them nothing?", "8"),
]

ANSWERS = [
 "<b>Uniqueness</b>, <b>pseudorandomness</b>, and <b>provability</b>. Uniqueness makes it "
 "unriggable: for a given key and input there is exactly one output that will ever verify, so "
 "there is no re-rolling and no searching for a better draw.",

 "A signature proves you agreed to a message <b>you chose</b>; a VRF gives you a random number "
 "you <b>could not choose</b>. The output is fixed by your key and the public input before you "
 "ever compute it.",

 "Because the output is deterministic. There is only one valid answer for a given (sk, input), "
 "so recomputing produces the identical result every time. Re-rolling would require a different "
 "input — and the input is fixed by the public seed and the role, both of which everyone else "
 "can check.",

 "Because the binomial distribution adds: "
 "<font face='Mono' size='9'>B(k1;n1,p) + B(k2;n2,p) = B(k1+k2;n1+n2,p)</font>. Each coin is an "
 "independent ticket with the same win probability p, so a hundred draws over 100 tickets have "
 "exactly the same combined distribution as one draw over 10,000. The split is invisible to the "
 "maths — measured at 100.32 seats versus 99.84 unsplit, against a theoretical 100.",

 "Every step of every round would use the same lottery, so the same nodes would be selected "
 "every time. The committee would stop being fresh, which destroys the two properties that "
 "matter: an attacker could learn the membership once and target it thereafter, and the "
 "replacement of members after each step — which is what makes attacking a revealed member "
 "pointless — would no longer happen.",

 "The birthday bound. An attacker seeking a collision gets to choose both inputs, and every new "
 "candidate is compared against all previously seen ones, so matching pairs accumulate "
 "quadratically. A collision therefore appears after roughly 2^(n/2) attempts rather than 2^n.",

 "<b>Second-preimage resistance.</b> The chain's guarantee is that you cannot swap the contents "
 "of an existing block while keeping its hash — that is, given a specific existing input, you "
 "cannot find a different one with the same hash. Collision resistance is a separate and "
 "stronger requirement, needed to stop an attacker crafting two colliding blocks from scratch "
 "before either is committed.",

 "Recovering the scalar k from the curve point k·G. It is believed infeasible, and it is what "
 "makes a private key unrecoverable from a public key. If it were solved, anyone could derive "
 "any node's private key from its public key — forging signatures, forging VRF proofs, and "
 "letting an attacker claim any number of committee seats. The entire project would collapse, "
 "along with most of the internet's transport security.",

 "<b>Uniqueness</b>, and only for adversarially generated keys. Ed25519 signing is "
 "deterministic, so an honest key yields exactly one signature and therefore one output — but "
 "Ed25519 does not guarantee signature uniqueness when the key itself was chosen maliciously "
 "(non-canonical encodings, small-order points). It is acceptable because Fabric is "
 "permissioned: orderer keys are issued by a CA under an MSP, so adversarial key generation is "
 "outside the threat model. The VRF sits behind an interface so a real ECVRF can replace it.",

 "Because committee membership is per-step, and a fresh committee is drawn for the next step "
 "from an independent lottery. Your vote is already gossiped by the time you are visible, and "
 "you have no special role in any future step — so silencing you removes nothing. This is what "
 "makes Algorand resistant to an adaptive adversary, and it is why members speak exactly once.",
]

def sec15():
    f = [SectionMark("15 • Checkpoint")]
    f.append(EY("SECTION 15"))
    f.append(H1("Checkpoint"))
    f.append(SP(3))
    f.append(P("Answer these out loud, without notes, before Phase 3. Questions 1, 4 and 9 are "
        "the ones you are most likely to be asked about this project.", "lead"))
    f.append(SP(4))
    rows = [[Paragraph(f'<font face="Mono" size="8.5" color="#0E6B61">{k:02d}</font>', S["tbl"]),
             Paragraph(q, S["tbl"]),
             Paragraph(f'<font size="8" color="#767C96">§{s}</font>', S["tbl"])]
            for k, (q, s) in enumerate(QUESTIONS, 1)]
    t = Table(rows, colWidths=[20, CONTENT_W-74, 54])
    t.setStyle(TableStyle([("VALIGN",(0,0),(-1,-1),"TOP"),
                           ("LINEBELOW",(0,0),(-1,-2),0.4,RULE),
                           ("TOPPADDING",(0,0),(-1,-1),6),
                           ("BOTTOMPADDING",(0,0),(-1,-1),6),
                           ("LEFTPADDING",(0,0),(-1,-1),0)]))
    f.append(t)
    f.append(PageBreak())

    f.append(SectionMark("15 • Answers"))
    f.append(EY("SECTION 15  •  MODEL ANSWERS"))
    f.append(H1("Answers"))
    f.append(SP(3))
    f.append(P("Same ground in your own words counts. The mechanism matters, the wording does "
        "not.", "lead"))
    f.append(SP(3))
    for k, a in enumerate(ANSWERS, 1):
        f.append(Paragraph(f'<font face="Mono" size="9" color="#0E6B61">{k:02d}</font>  ' + a,
                 ParagraphStyle("ans", parent=S["body"], fontSize=9.5, leading=14.2,
                                spaceAfter=9)))
    f.append(SP(6))
    f.append(Callout("Phase 2 done — what changes now", [
        "You can read <font face='Mono' size='8'>engine/vrf/</font> in full. That is a third of "
        "your project's consensus engine no longer a black box.",
        "Next is <b>Phase 3 — Distributed systems and consensus theory</b>, sixteen hours and "
        "the hardest of the eight. It is also the one the roadmap says never to cut: it is where "
        "the 2/3 bound stops being a rule you were told and becomes something you can derive.",
        "If you would rather build for a while, Phase 5 (Go) needs only the Homebrew and Go "
        "installs, which are still outstanding.",
    ], "note"))
    return f

def build(path="Phase2_Cryptography_and_VRFs.pdf"):
    doc = BaseDocTemplate(
        path, pagesize=(PAGE_W, PAGE_H),
        leftMargin=MARGIN_L, rightMargin=MARGIN_R,
        topMargin=MARGIN_T, bottomMargin=MARGIN_B,
        title="Cryptography and VRFs - Phase 2",
        author="Study guide for the Algorand-on-Fabric project",
        subject="Phase 2 of 8: hashing, signatures, and Verifiable Random Functions",
    )
    frame = Frame(MARGIN_L, MARGIN_B, CONTENT_W, PAGE_H - MARGIN_T - MARGIN_B,
                  id="body", leftPadding=0, rightPadding=0, topPadding=0, bottomPadding=0)
    doc.addPageTemplates([
        PageTemplate(id="cover", frames=[frame], onPage=on_cover),
        PageTemplate(id="body", frames=[frame], onPage=on_page, onPageEnd=on_page_end),
    ])
    story = [NextPageTemplate("body")]
    story += cover(); story += how_to_use()
    for s in (sec1, sec2, sec3, sec4, sec5, sec6, sec7, sec8,
              sec9, sec10, sec11, sec12, sec13, sec14, sec15):
        story += s()
    doc.build(story)
    return path

if __name__ == "__main__":
    p = build()
    import pypdf
    print(f"built {p}: {len(pypdf.PdfReader(p).pages)} pages")
