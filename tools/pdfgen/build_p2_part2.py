"""Phase 2 study PDF — sections 7-12, the VRF core."""
from reportlab.platypus import Paragraph, Spacer, PageBreak, Table, TableStyle, KeepTogether
from pdfbase_kit import *
from p2diagrams_a import *
from p2diagrams_b import *
from build_p1_part1 import P, H1, H2, H3, EY, SP, BU, NUM, term_table, grid_table

def sec7():
    f = [SectionMark("7 • Verifiable Random Functions")]
    f.append(EY("SECTION 7  •  THE CENTRAL IDEA"))
    f.append(H1("Verifiable Random Functions"))
    f.append(SP(4))
    f.append(P("Everything so far has been preparation. This is the idea Algorand is built on, "
        "and the reason your project exists in the form it does.", "lead"))
    f.append(SP(2))
    f.append(H2("The problem, stated precisely"))
    f.append(P("We need to choose a random committee, weighted by stake, such that:"))
    f.append(BU("nobody can " + b("predict") + " who will be chosen, so nobody can be attacked "
        "in advance;"))
    f.append(BU("nobody can " + b("influence") + " who is chosen, including themselves;"))
    f.append(BU("a chosen participant can " + b("prove") + " they were chosen, so their vote counts;"))
    f.append(BU("and there is " + b("no organiser") + " — no trusted party running the draw, "
        "because a trusted party is exactly what we removed in Phase 1."))
    f.append(SP(4))
    f.append(P("Each requirement alone is easy. Together they look contradictory: a random draw "
        "with no organiser that is nonetheless provable. The VRF resolves it."))
    f.append(SP(5))
    f.append(H2("What a VRF is"))
    f.append(P("A function taking your private key and any public input, and returning two things:"))
    f.append(CodeBlock([
        "beta, pi  =  VRF(sk, alpha)",
        "",
        "  alpha  the input        (public: everyone knows it)",
        "  beta   the output       (looks completely random)",
        "  pi     the proof        (lets anyone check beta, using only pk)",
    ]))
    f.append(SP(9))
    f.append(H2("Its three properties"))
    f.append(term_table([
        ("Uniqueness", "For a given <i>sk</i> and <i>alpha</i>, there is exactly <b>one</b> "
         "<i>beta</i> that will ever verify. You cannot re-roll, cannot search for a better "
         "answer, cannot produce two valid outputs. <b>This is the property that makes the "
         "lottery unriggable.</b>"),
        ("Pseudorandomness", "To anyone without <i>sk</i>, <i>beta</i> is indistinguishable "
         "from a random string. They cannot predict it, cannot narrow it down, cannot tell "
         "whether you won before you say so."),
        ("Provability", "The proof <i>pi</i> convinces anyone holding your <b>public</b> key "
         "that <i>beta</i> really is the correct output for <i>alpha</i>. Your secret is never "
         "transmitted and never inferable."),
    ]))
    f.append(SP(10))
    f.append(P6ThreeTools())
    f.append(SP(6))
    f.append(Callout("The sentence worth memorising", [
        "A signature proves <b>you agreed to a message you chose</b>.",
        "A VRF gives you <b>a random number you could not choose, and can prove you did not "
        "choose</b>.",
        "That difference is the whole thing. If you can say this cleanly, you understand "
        "Phase 2.",
    ], "key"))
    f.append(SP(8))
    f.append(H2("Why it feels impossible, and why it is not"))
    f.append(P("The apparent paradox: how can a number be both unpredictable to others and "
        "fixed in advance for you?"))
    f.append(P("Because it is determined by something only you hold. The output is a fixed "
        "function of " + mono("(sk, alpha)") + ", so it existed, mathematically, before you "
        "computed it — you are discovering it, not choosing it. And since nobody else has "
        + mono("sk") + ", nobody else can discover it early. The proof then lets them confirm, "
        "afterwards, that you reported honestly."))
    f.append(P("Ed25519's determinism (section 6) is exactly this shape, which is why it can be "
        "borrowed to build one."))
    f.append(SecBreak())
    return f

def sec8():
    f = [SectionMark("8 • Sortition")]
    f.append(EY("SECTION 8"))
    f.append(H1("Sortition: running the secret lottery"))
    f.append(SP(4))
    f.append(P("\"Sortition\" means selection by lot. Algorand's version, from section 5 of the "
        "paper, is where the VRF earns its place.", "lead"))
    f.append(P7Sortition())
    f.append(SP(6))
    f.append(H2("The input is the whole design"))
    f.append(CodeBlock([
        "beta, pi  =  VRF(sk, seed || role)",
        "",
        "  seed  a public random value for this round, the same for everyone",
        "  role  a label:  <\"committee\", round 42, step 3>",
    ]))
    f.append(SP(9))
    f.append(P("Change any part of " + mono("role") + " and you get a completely different, "
        "independent lottery. That is how Algorand draws a " + b("fresh committee for every "
        "single step") + " at zero coordination cost — no messages, no organiser, no setup. "
        "Exercise 3 in section 14 demonstrates it in four lines."))
    f.append(SP(4))
    f.append(grid_table(
        ["PROPERTY NEEDED", "WHICH VRF PROPERTY DELIVERS IT"],
        [["Nobody can predict the committee", "Pseudorandomness — without <i>sk</i>, "
          "<i>beta</i> is unguessable"],
         ["Nobody can rig their own selection", "Uniqueness — there is one answer and you "
          "cannot re-roll"],
         ["A winner's vote is counted", "Provability — <i>pi</i> convinces everyone, using "
          "only <i>pk</i>"],
         ["No trusted organiser exists", "Every node computes its own draw, alone"]],
        [0.44, 0.56]))
    f.append(SP(10))
    f.append(Callout("The consequence that makes BA* possible", [
        "Because selection is private until you speak, an attacker cannot silence the committee "
        "in advance — they do not know who is on it.",
        "And because a fresh committee is drawn every step, by the time your message reveals "
        "you, <b>you no longer matter</b>. Attacking you afterwards accomplishes nothing.",
        "This is what the Algorand paper calls resistance to an <i>adaptive adversary</i>, and "
        "it is the property that earlier systems like ByzCoin could not achieve. In Phase 4 you "
        "will meet it again as the reason committee members speak exactly once.",
    ], "key"))
    f.append(SecBreak())
    return f

def sec9():
    f = [SectionMark("9 • The binomial")]
    f.append(EY("SECTION 9"))
    f.append(H1("From stake to votes: the binomial"))
    f.append(SP(4))
    f.append(P("A VRF gives you one random number. But influence must be proportional to "
        "stake — so how does one number become <i>many</i> votes?", "lead"))
    f.append(P("The answer is to stop thinking of a node as a participant, and start thinking "
        "of each " + b("coin") + " as one. A node with 10,000 coins holds 10,000 lottery "
        "tickets, each winning independently with probability " + mono("p = tau / W") +
        ", where " + mono("tau") + " is the target committee size and " + mono("W") +
        " the total stake."))
    f.append(P("The number of tickets that win therefore follows the " + b("binomial "
        "distribution") + " — the same distribution as \"how many heads in " + mono("w") +
        " tosses of a biased coin\". Algorand calls each winning ticket a " + b("sub-user") + "."))
    f.append(SP(4))
    f.append(P8Binomial())
    f.append(SP(6))
    f.append(H2("How the code does it"))
    f.append(P("Your engine's " + mono("SubUsers()") + " walks the cumulative distribution:"))
    f.append(CodeBlock([
        "p := tau / float64(W)",
        "x := ratio(hash)              // the VRF hash, normalised to [0,1)",
        "cum := 0.0",
        "for j := uint64(0); j <= w; j++ {",
        "    cum += math.Exp(logBinomPMF(j, w, p))",
        "    if x < cum {",
        "        return j              // this node won j seats",
        "    }",
        "}",
    ]))
    f.append(SP(9))
    f.append(Callout("Two details in that code worth understanding", [
        "<b>Why logarithms.</b> <font face='Mono' size='8'>logBinomPMF</font> works in log "
        "space via log-gamma. Computing <font face='Mono' size='8'>C(n,k)·p^k·q^(n-k)</font> "
        "directly overflows and loses all precision once <i>n</i> reaches the thousands — which "
        "is exactly the regime here, since <i>W</i> is the entire stake.",
        "<b>Why 53 bits.</b> <font face='Mono' size='8'>ratio()</font> takes the top 53 bits of "
        "the hash, the mantissa width of a float64. Using more adds no precision, and the naive "
        "<font face='Mono' size='8'>float64(u)/2^64</font> can round up to exactly 1.0, which "
        "would fall off the end of the CDF walk and return the wrong seat count.",
    ], "note"))
    f.append(SecBreak())
    return f

def sec10():
    f = [SectionMark("10 • Sybil resistance")]
    f.append(EY("SECTION 10"))
    f.append(H1("Why the binomial kills Sybil attacks"))
    f.append(SP(4))
    f.append(P("Phase 1 said proof of stake defeats Sybils \"arithmetically\". This is the "
        "arithmetic.", "lead"))
    f.append(P9Sybil())
    f.append(SP(6))
    f.append(P("The binomial distribution has an addition property:"))
    f.append(CodeBlock(["B(k1; n1, p)  +  B(k2; n2, p)   =   B(k1+k2; n1+n2, p)"]))
    f.append(SP(9))
    f.append(P("In words: " + b("two independent binomial draws with the same p add up to a "
        "single binomial draw over the combined size") + ". Ten thousand tickets held by one "
        "account, or split across a hundred accounts of a hundred tickets each, produce the "
        "same distribution of total wins."))
    f.append(P("So a Sybil attacker gains " + b("nothing") + ". Not less than before — exactly "
        "the same. Splitting is invisible to the mathematics."))
    f.append(SP(4))
    f.append(Callout("This is checkpoint question 2, and it is worth rehearsing", [
        "The naive design — one vote per node — would be catastrophic here: 100 fake accounts "
        "would give 100 votes instead of 1. It is precisely the per-coin draw that removes the "
        "incentive.",
        "You can verify this yourself. <font face='Mono' size='8'>tools/verify_sortition.py</font> "
        "in your own project runs it as a Monte Carlo: one identity with 10,000 stake averaged "
        "<b>99.84</b> seats, a hundred Sybils with 100 each averaged <b>100.32</b>, against a "
        "theoretical 100.00.",
        "Note carefully what this does <b>not</b> do: it does not make identities expensive, and "
        "it does not detect Sybils. It makes them <i>pointless</i>, which is stronger — there "
        "is nothing to detect and nothing to punish.",
    ], "key"))
    f.append(SecBreak())
    return f

def sec11():
    f = [SectionMark("11 • Your engine's shortcut")]
    f.append(EY("SECTION 11"))
    f.append(H1("What your engine actually ships"))
    f.append(SP(4))
    f.append(P("Time to be precise about the code you already have, including where it is "
        "weaker than the paper.", "lead"))
    f.append(P10Shortcut())
    f.append(SP(6))
    f.append(H2("The construction"))
    f.append(CodeBlock([
        "func (Ed25519VRF) Prove(sk, m) (hash, proof []byte) {",
        "    proof = ed25519.Sign(sk, m)      // deterministic, per RFC 8032",
        "    h := sha512.Sum512(proof)        // pseudorandom output",
        "    return h[:], proof",
        "}",
    ]))
    f.append(SP(9))
    f.append(P("Check it against the three properties. " + b("Pseudorandomness") + ": the "
        "output is SHA-512 of a signature, so it is indistinguishable from random without "
        + mono("sk") + ". " + b("Provability") + ": verification is signature verification "
        "plus a hash comparison, needing only " + mono("pk") + ". " + b("Uniqueness") +
        ": Ed25519 signing is deterministic, so for honest keys there is exactly one signature "
        "and therefore one output."))
    f.append(SP(3))
    f.append(Callout("And here is the gap — state this plainly in your writeup", [
        "That uniqueness argument holds only for <b>honestly generated keys</b>. A true VRF "
        "guarantees uniqueness even when the key was chosen adversarially.",
        "Ed25519 does not promise that. Using non-canonical scalar encodings or small-order "
        "curve points, a malicious key holder can — in some verifier implementations — produce "
        "more than one signature that verifies, and therefore more than one \"random\" draw. "
        "They could pick whichever wins them more seats.",
        "<b>Why it is nonetheless the right call here:</b> Fabric is permissioned. Orderer keys "
        "are issued by a certificate authority under an MSP, not generated by an attacker. The "
        "threat the gap opens is one your deployment model already closes.",
        "And <font face='Mono' size='8'>vrf.go</font> defines a <font face='Mono' size='8'>VRF</font> "
        "interface, so a standards-track implementation drops in without touching another file.",
    ], "warn"))
    f.append(SecBreak())
    return f

def sec12():
    f = [SectionMark("12 • Real VRFs")]
    f.append(EY("SECTION 12"))
    f.append(H1("Real VRFs, and when you would need one"))
    f.append(SP(4))
    f.append(P("The standards-track construction is " + b("ECVRF") + ", specified in RFC 9381 "
        "(2023). Algorand's paper uses an earlier elliptic-curve VRF of the same family.", "lead"))
    f.append(P("You do not need to implement it, and for this project you almost certainly "
        "should not. But you should be able to say what it adds:"))
    f.append(SP(3))
    f.append(grid_table(
        ["", "YOUR Ed25519 SHORTCUT", "ECVRF (RFC 9381)"],
        [["<b>Pseudorandomness</b>", "Yes", "Yes"],
         ["<b>Provability</b>", "Yes", "Yes"],
         ["<b>Uniqueness, honest keys</b>", "Yes", "Yes"],
         ["<b>Uniqueness, adversarial keys</b>", "<b>No</b>", "Yes — proven"],
         ["<b>Lines of code</b>", "About 15", "Several hundred, easy to get subtly wrong"],
         ["<b>Right for a permissioned lab network</b>", "Yes", "Overkill"],
         ["<b>Right for an open network</b>", "No", "Yes"]],
        [0.34, 0.33, 0.33]))
    f.append(SP(10))
    f.append(P("ECVRF works by hashing the input " + i("onto the curve") + " to get a point, "
        "multiplying that point by the secret scalar, and attaching a zero-knowledge proof that "
        "the same scalar was used as in the public key. The uniqueness is then a theorem about "
        "the curve group, not a property of one signing algorithm's conventions."))
    f.append(SP(4))
    f.append(Callout("The honest position for your report", [
        "\"We implement the VRF using Ed25519's deterministic signatures, which satisfies "
        "pseudorandomness and provability, and satisfies uniqueness for honestly generated "
        "keys. Since Hyperledger Fabric is permissioned and orderer keys are issued by a CA, "
        "adversarial key generation is outside our threat model. The VRF is defined behind an "
        "interface so an RFC 9381 ECVRF can replace it without changes elsewhere.\"",
        "That is a stronger answer than pretending the gap is not there. Knowing exactly where "
        "your implementation departs from the paper — and why the departure is safe in your "
        "setting — is the thing that distinguishes a considered project from a copied one.",
    ], "key"))
    f.append(SecBreak())
    return f
