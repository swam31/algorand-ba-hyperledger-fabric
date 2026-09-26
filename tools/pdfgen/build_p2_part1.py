"""Phase 2 study PDF — front matter and sections 1-6."""
from reportlab.platypus import Paragraph, Spacer, PageBreak, Table, TableStyle, KeepTogether
from reportlab.lib.units import mm
from pdfbase_kit import *
from p2diagrams_a import *
from p2diagrams_b import *
from build_p1_part1 import P, H1, H2, H3, EY, SP, BU, NUM, term_table, grid_table

set_phase("PHASE 2  •  CRYPTOGRAPHY AND VRFs", "STUDY GUIDE  •  PHASE 2 OF 8")

def cover():
    f = [Spacer(1, 34*mm)]
    f.append(Paragraph("Cryptography", S["cover_t"]))
    f.append(Paragraph('and <font color="#0E6B61">VRFs</font>', S["cover_t"]))
    f.append(Spacer(1, 7*mm))
    f.append(Rule(color=INK, thickness=1.6, space=10))
    f.append(Spacer(1, 3*mm))
    f.append(Paragraph(
        "The single idea Algorand is built on, and the maths that makes fake identities "
        "worthless. Everything here becomes code you can read in "
        "<font face='Mono' size='11'>engine/vrf/</font>.", S["cover_s"]))
    f.append(Spacer(1, 14*mm))
    rows = [
        ("1", "What Phase 2 is for", "the one idea everything points at"),
        ("2", "Hashing, properly", "three resistances, not one"),
        ("3", "The birthday bound", "why collisions cost half the bits"),
        ("4", "Merkle trees", "proving membership without the data"),
        ("5", "One-way functions", "and a gentle look at elliptic curves"),
        ("6", "Ed25519", "and why determinism matters here"),
        ("7", "Verifiable Random Functions", "the heart of the whole project"),
        ("8", "Sortition", "how Algorand runs a secret lottery"),
        ("9", "The binomial and sub-users", "turning stake into votes"),
        ("10", "Sybil resistance", "the identity that kills fake accounts"),
        ("11", "Your engine's shortcut", "what it does, and where it falls short"),
        ("12", "Real VRFs", "RFC 9381, and when you would need it"),
        ("13", "Glossary", "every new term in one place"),
        ("14", "Build it yourself", "five exercises, all runnable today"),
        ("15", "Checkpoint", "questions, then answers"),
    ]
    data = [[Paragraph(f'<font face="Mono" size="8" color="#0E6B61">{n.rjust(2)}</font>', S["tbl"]),
             Paragraph(f"<b>{t}</b>  <font color='#767C96'>— {s}</font>", S["tbl"])]
            for n, t, s in rows]
    t = Table(data, colWidths=[22, CONTENT_W-22])
    t.setStyle(TableStyle([("VALIGN",(0,0),(-1,-1),"TOP"),
                           ("TOPPADDING",(0,0),(-1,-1),3.4),
                           ("BOTTOMPADDING",(0,0),(-1,-1),3.4),
                           ("LEFTPADDING",(0,0),(-1,-1),0)]))
    f.append(Paragraph("CONTENTS", S["eyebrow"])); f.append(SP(3)); f.append(t)
    f.append(PageBreak())
    return f

def how_to_use():
    f = [SectionMark("How to use this")]
    f.append(EY("BEFORE YOU START"))
    f.append(H1("How to read this document"))
    f.append(SP(4))
    f.append(P("Phase 2 of your eight-phase plan. Budget about ten hours — it is the "
        "second-longest phase, and the only one whose subject is a single idea.", "lead"))
    f.append(P(b("This document assumes Phase 1.") + " If \"hash\", \"signature\", \"Sybil\" or "
        "\"consensus\" are not yet comfortable words, go back — nothing here will land "
        "otherwise."))
    f.append(P(b("Sections 1 to 6 are the run-up.") + " They deepen what Phase 1 introduced. "
        "Read them properly, but do not linger: they exist to make section 7 possible."))
    f.append(P(b("Section 7 is the point of the entire phase.") + " If you read one section "
        "twice, read that one. Everything in your project's " + mono("engine/vrf/") +
        " package is section 7 turned into Go."))
    f.append(P(b("Section 14 has five exercises, and all five run on the Python you already "
        "have.") + " No installs. They were each executed while this document was written, "
        "and the outputs shown are the real ones."))
    f.append(SP(6))
    f.append(Callout("What you will be able to do afterwards", [
        "State the three properties of a VRF, and say exactly what a VRF gives you that a "
        "signature does not.",
        "Explain why splitting a stake across a hundred fake accounts wins an attacker nothing "
        "— using the actual identity, not a hand-wave.",
        "Read <font face='Mono' size='8'>engine/vrf/sortition.go</font> line by line and say "
        "what every line is for, including the binomial walk.",
        "Say precisely why the Ed25519 shortcut in that file is not a real VRF, and why it is "
        "still the right call for a permissioned Fabric network.",
    ], "note"))
    f.append(SP(8))
    f.append(Callout("A note on the maths", [
        "There is more arithmetic here than in Phase 1, and none of it is optional — the "
        "binomial is the Sybil defence, not decoration.",
        "But you need no proofs and no advanced background. If you can follow \"probability of "
        "k heads in n tosses\", you can follow all of it. Elliptic curves are described in "
        "section 5 without a single field equation, because you do not need one to use them.",
    ], "key"))
    f.append(PageBreak())
    return f

def sec1():
    f = [SectionMark("1 • What Phase 2 is for")]
    f.append(EY("SECTION 1"))
    f.append(H1("What this phase is for"))
    f.append(SP(4))
    f.append(P("Phase 1 ended with a problem left open.", "lead"))
    f.append(P("You know that consensus needs a committee, that the committee must be chosen "
        "fairly, and that it must be weighted by stake so fake identities gain nothing. But "
        "how do you actually run that lottery?"))
    f.append(SP(3))
    f.append(Paragraph("Whoever runs the draw can rig the draw. And if the winners are "
        "announced in advance, an attacker knows exactly whom to silence.", S["quote"]))
    f.append(P("Algorand's answer is a cryptographic primitive most people have never heard "
        "of: the " + b("Verifiable Random Function") + ". It lets every participant run the "
        "lottery privately, on their own machine, and later prove the result — with no "
        "organiser, no announcement, and no way to cheat."))
    f.append(P("That is section 7. Sections 2 through 6 build the vocabulary it needs."))
    f.append(SP(5))
    f.append(grid_table(
        ["SECTION", "WHAT IT ADDS", "WHY SECTION 7 NEEDS IT"],
        [["2 – 3", "Hashing, in real detail",
          "A VRF's output must be indistinguishable from random"],
         ["4", "Merkle trees",
          "Not needed for VRFs — but Fabric uses them, and Phase 7 will assume you know"],
         ["5", "One-way functions, curves",
          "A VRF is built on the same hardness as a key pair"],
         ["6", "Ed25519 and determinism",
          "Your engine's VRF <b>is</b> an Ed25519 signature. The determinism is the trick"]],
        [0.14, 0.34, 0.52]))
    f.append(SP(9))
    f.append(Callout("Keep one question in your head throughout", [
        "<b>How do you produce a random number that you cannot choose, that nobody can predict, "
        "and that you can nonetheless prove is the right one?</b>",
        "Every section from here to section 7 is a piece of that answer. When something feels "
        "like a detour, ask how it serves that question.",
    ], "key"))
    f.append(SecBreak())
    return f

def sec2():
    f = [SectionMark("2 • Hashing, properly")]
    f.append(EY("SECTION 2"))
    f.append(H1("Hashing, properly this time"))
    f.append(SP(4))
    f.append(P("Phase 1 said a hash is one-way. That was true but incomplete: \"one-way\" is "
        "actually three separate promises, and they are not equally strong.", "lead"))
    f.append(P1Resistances())
    f.append(SP(5))
    f.append(term_table([
        ("Preimage resistance", "Given an output <i>h</i>, you cannot find any input that "
         "hashes to it. This is what stops someone reversing a hash back into the data."),
        ("Second-preimage resistance", "Given a specific input <i>x</i>, you cannot find a "
         "different <i>x'</i> with the same hash. This is what stops someone swapping one "
         "transaction for another inside a block while keeping the block's hash intact."),
        ("Collision resistance", "You cannot find <b>any</b> two inputs that hash to the same "
         "value. Note the freedom: the attacker gets to choose <i>both</i>. That extra freedom "
         "is exactly why this is the weakest of the three."),
    ]))
    f.append(SP(9))
    f.append(P("The distinction matters for your project. Every block stores its predecessor's "
        "hash, so the chain's immutability rests on " + b("second-preimage resistance") +
        ". But a system where an attacker can craft two colliding blocks from scratch — "
        "one benign, one malicious — is broken by " + b("collision resistance") +
        " alone. That is the one that gives way first, and section 3 shows by how much."))
    f.append(SecBreak())
    return f

def sec3():
    f = [SectionMark("3 • The birthday bound")]
    f.append(EY("SECTION 3"))
    f.append(H1("The birthday bound"))
    f.append(SP(4))
    f.append(P("Here is a fact that surprises almost everyone the first time.", "lead"))
    f.append(P("In a room of 23 people, there is a better-than-even chance that two share a "
        "birthday. Not that someone shares " + i("yours") + " — that would need about 253 "
        "people. Just that " + b("some") + " pair matches."))
    f.append(P("The reason is that you are not making 23 comparisons, you are making every "
        "possible pair: 23 × 22 / 2 = 253 of them. Pairs grow with the " + i("square") +
        " of the group."))
    f.append(SP(4))
    f.append(P2Birthday())
    f.append(SP(5))
    f.append(P("Applied to hashes: with an " + mono("n") + "-bit output there are "
        + mono("2^n") + " possible values, but a collision shows up after roughly "
        + mono("2^(n/2)") + " attempts, not " + mono("2^n") + "."))
    f.append(SP(3))
    f.append(Callout("What this means in practice", [
        "SHA-256 gives <b>256-bit</b> preimage resistance but only <b>128-bit</b> collision "
        "resistance. 128 bits is still far beyond reach, so this is a design fact, not a "
        "weakness — but it is why hash outputs are twice as long as the security level "
        "they are chosen for.",
        "It is also why truncating a hash to \"save space\" is so dangerous. Cut SHA-256 down "
        "to 64 bits and you have 32-bit collision resistance: about four billion tries, which "
        "is minutes on a laptop. The numbers in Figure 2 were produced exactly that way.",
    ], "warn"))
    f.append(SecBreak())
    return f

def sec4():
    f = [SectionMark("4 • Merkle trees")]
    f.append(EY("SECTION 4"))
    f.append(H1("Merkle trees"))
    f.append(SP(4))
    f.append(P("A way to commit to a whole set of items with one hash, and then prove any "
        "single item was in that set without revealing or transmitting the rest.", "lead"))
    f.append(P3Merkle())
    f.append(SP(5))
    f.append(P("Hash the items in pairs, then hash those hashes in pairs, and keep going until "
        "one value remains: the " + b("root") + ". Change any leaf and the root changes, by "
        "the avalanche property."))
    f.append(P("The useful part is the " + b("inclusion proof") + ". To convince someone that "
        + mono("tx5") + " is in the set, you send " + mono("tx5") + " plus one sibling hash per "
        "level. They recompute upward and compare against the root they already trust."))
    f.append(SP(4))
    f.append(Callout("Where this shows up in your project", [
        "Fabric puts a Merkle-style hash over a block's transactions in the block header, so a "
        "peer can verify a block's contents are intact without re-reading every transaction.",
        "It is <b>not</b> part of BA*, and your engine does not implement one. It is here "
        "because Phase 7 will show you Fabric block structure and assume you recognise it.",
        "The wider idea — <b>commit to a lot with a little, then prove membership cheaply</b> "
        "— is worth carrying with you. It is the same instinct as a VRF proof: a small "
        "token that lets someone else check a claim they could not compute themselves.",
    ], "note"))
    f.append(SecBreak())
    return f

def sec5():
    f = [SectionMark("5 • One-way functions")]
    f.append(EY("SECTION 5"))
    f.append(H1("One-way functions and elliptic curves"))
    f.append(SP(4))
    f.append(P("Phase 1 said you can compute a public key from a private key but never the "
        "reverse. This section says what that actually is — without the algebra.", "lead"))
    f.append(P4OneWay())
    f.append(SP(5))
    f.append(H2("What you genuinely need to know"))
    f.append(NUM(1, "An elliptic curve is a set of points, plus a rule for \"adding\" two "
        "points to get a third. The rule is geometric and you do not need it."))
    f.append(NUM(2, "Fix one point " + mono("G") + ", the " + b("generator") + ". Adding it to "
        "itself " + mono("k") + " times gives a point written " + mono("k·G") + "."))
    f.append(NUM(3, "Computing " + mono("k·G") + " is fast even when " + mono("k") + " has 256 "
        "bits, because you can double repeatedly instead of adding one at a time."))
    f.append(NUM(4, "Recovering " + mono("k") + " from " + mono("k·G") + " is the " +
        b("discrete logarithm problem") + ", and nobody knows how to do it in reasonable time."))
    f.append(SP(4))
    f.append(P("That asymmetry " + b("is") + " the key pair. Your private key is the number "
        + mono("k") + "; your public key is the point " + mono("k·G") + ". Curve25519 — "
        "the curve Algorand's VRF and your engine both use — is one specific well-studied "
        "choice of curve, designed to be fast and hard to implement insecurely."))
    f.append(SP(4))
    f.append(Callout("You will not need to implement any of this", [
        "Every operation is provided by a library. What matters is knowing <i>what hardness "
        "assumption you are relying on</i>, because that is the sentence a reviewer will ask "
        "you for: <b>\"if the discrete logarithm problem on Curve25519 were solved tomorrow, "
        "my scheme would break, and so would most of the internet.\"</b>",
    ], "note"))
    f.append(SecBreak())
    return f

def sec6():
    f = [SectionMark("6 • Ed25519")]
    f.append(EY("SECTION 6"))
    f.append(H1("Ed25519, and why determinism matters"))
    f.append(SP(4))
    f.append(P(b("Ed25519") + " is a specific signature scheme built on Curve25519. It is what "
        "your engine uses, and it has one property that turns out to be load-bearing.", "lead"))
    f.append(P5Determinism())
    f.append(SP(5))
    f.append(P("Most older signature schemes need a fresh random number — a " + b("nonce") +
        " — for every signature. If that randomness is ever repeated or predictable, the "
        "private key can be recovered outright. This has caused real, expensive failures."))
    f.append(P("Ed25519 removes the risk by deriving the nonce deterministically: it hashes the "
        "private key together with the message. Same key, same message, same signature, forever."))
    f.append(SP(3))
    f.append(Callout("Why this matters far more than usual for you", [
        "For most systems, Ed25519's determinism is just a safety feature — one less thing "
        "to get wrong.",
        "For your project it is <b>the mechanism itself</b>. A VRF must produce exactly one "
        "output per (key, input), or a node could re-roll until it won the lottery. Ed25519's "
        "determinism is what your engine borrows to get that property for free.",
        "That is the trick in <font face='Mono' size='8'>engine/vrf/vrf.go</font>, and section "
        "11 is about exactly how far the trick can be trusted.",
    ], "key"))
    f.append(SecBreak())
    return f
