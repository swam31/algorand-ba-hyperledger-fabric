"""Phase 1 study PDF — sections 6-13."""
from reportlab.platypus import Paragraph, Spacer, PageBreak, Table, TableStyle
from pdfbase_kit import *
from pdfdiagrams_a import *
from pdfdiagrams_b import *
from build_p1_part1 import P, H1, H2, H3, EY, SP, BU, NUM, term_table, grid_table

def sec6():
    f = [SectionMark("6 • Blocks and the chain")]
    f.append(EY("SECTION 6"))
    f.append(H1("Blocks, and why it is a chain"))
    f.append(SP(4))
    f.append(P("Agreeing on transactions one at a time would be far too slow, so we batch "
               "them into a " + b("block") + ".", "lead"))
    f.append(CodeBlock([
        "BLOCK #42",
        "  prev_hash:    <hash of block #41>    # the chain link",
        "  round:        42",
        "  timestamp:    2026-09-03T10:14:22Z",
        "  transactions: [tx1, tx2, tx3, ... tx2000]",
    ]))
    f.append(SP(9))
    f.append(P("The critical field is " + mono("prev_hash") + ": every block contains the hash "
        "of the block before it. That single line is what makes the past immutable."))
    f.append(P("Suppose an attacker wants to alter a transaction back in block 1. Changing it "
        "changes block 1's hash — by the avalanche property, completely. But block 2 stores "
        "block 1's " + i("old") + " hash, so block 2 is now inconsistent. Repairing block 2 "
        "changes block 2's hash, which breaks block 3. And so on, all the way to the present."))
    f.append(SP(5))
    f.append(D5Chain())
    f.append(SP(5))
    f.append(Callout("The key insight", [
        "<b>You cannot change the past without rewriting everything after it.</b> That is the "
        "entire meaning of the word <i>chain</i> in blockchain. It is not a metaphor and it is "
        "not a database structure — it is just hashes pointing backwards.",
        "The word <b>immutable</b> gets used for this. It does not mean the data physically "
        "cannot be edited. It means editing it is detectable by anyone, instantly, and that "
        "hiding the edit requires redoing all the work that came after.",
    ], "key"))
    f.append(SecBreak())
    return f

def sec7():
    f = [SectionMark("7 • Nodes and gossip")]
    f.append(EY("SECTION 7"))
    f.append(H1("The network: nodes and gossip"))
    f.append(SP(4))
    f.append(P("There is no server. There are " + b("nodes") + " — computers, each holding a "
               "full copy of the blockchain and each running the same software.", "lead"))
    f.append(P("They communicate by " + b("gossip") + ", which works exactly like a rumour:"))
    f.append(BU("Each node connects to a handful of random peers. Your project's engine uses "
        "about eight, and so did the original Algorand prototype."))
    f.append(BU("When you hear a new message, you forward it to all your peers."))
    f.append(BU("You never forward the same message twice, or it would loop forever."))
    f.append(BU("Within a few hops, the message has reached the entire network."))
    f.append(SP(4))
    f.append(D6Gossip())
    f.append(SP(4))
    f.append(P("No node knows the whole network. There is no central broadcaster. Yet "
        "everything reaches everyone. This design is why blockchains survive nodes going "
        "offline — and also " + b("why they are slow") + ": information travels at gossip "
        "speed, with a delay at every hop, not at the speed of writing to a database."))
    f.append(SP(4))
    f.append(Callout("A word you will meet: peer-to-peer", [
        "<b>Peer-to-peer</b> (P2P) means every participant is both a client and a server — "
        "there is no separate class of machine that serves the others. Every node asks for "
        "data and provides it.",
        "<b>Propagation delay</b> is the time for a message to reach the whole network. "
        "Measurements of Bitcoin's network put roughly one second for a small message to "
        "reach 90% of peers. Your project's timeouts are chosen around this number.",
    ], "note"))
    f.append(SecBreak())
    return f

def sec8():
    f = [SectionMark("8 • Consensus")]
    f.append(EY("SECTION 8"))
    f.append(H1("What consensus actually means"))
    f.append(SP(4))
    f.append(Paragraph("Consensus means: all honest nodes agree on the same ledger, in the "
        "same order, forever.", S["quote"]))
    f.append(P("That is the whole job. It splits into two requirements which pull against "
               "each other."))
    f.append(SP(3))
    f.append(grid_table(
        ["PROPERTY", "MEANING", "IF IT BREAKS"],
        [["<b>Safety</b>", "No two honest nodes ever accept different histories.",
          "Double-spending becomes possible. Money can be forged. <b>Fatal.</b>"],
         ["<b>Liveness</b>", "The chain keeps making progress; new blocks keep being added.",
          "Nothing ever confirms. The system is frozen but not corrupt."]],
        [0.16, 0.40, 0.44]))
    f.append(SP(10))
    f.append(P("The distinction matters more than it looks. A system that stops is annoying. "
        "A system that disagrees is broken beyond repair. So when a consensus protocol must "
        "choose, " + b("it always sacrifices liveness to preserve safety") + " — it would "
        "rather halt than let two versions of history exist."))
    f.append(P("You will see this choice made explicitly in your project: when Algorand cannot "
        "prove a decision is unique, it marks it " + i("tentative") + " and refuses to confirm "
        "it, rather than risk being wrong."))
    f.append(SP(5))
    f.append(H2("Why it is hard: nodes that lie"))
    f.append(P("If every node were honest and merely occasionally offline, this would be a "
        "solved problem with decades-old textbook answers. The difficulty is that some nodes "
        "are actively hostile. They lie, they send different messages to different peers, "
        "they stay silent at the worst moment, and they collude."))
    f.append(P("The technical term is a " + b("Byzantine fault") + ", and it is the subject of "
        "the next section."))
    f.append(SecBreak())
    return f

def sec9():
    f = [SectionMark("9 • Byzantine faults")]
    f.append(EY("SECTION 9"))
    f.append(H1("Byzantine faults and the two-thirds rule"))
    f.append(SP(4))
    f.append(P("Computer scientists split failures into two kinds, and the gap between them "
               "is enormous.", "lead"))
    f.append(D7Byzantine())
    f.append(SP(5))
    f.append(P("The name comes from a 1982 paper by Lamport, Shostak and Pease, which framed "
        "it as a story. Several generals surround a city. They must " + i("all") + " attack "
        "or " + i("all") + " retreat — a split is a catastrophe. They can only communicate by "
        "messenger. And some generals are traitors who will tell half the group to attack and "
        "the other half to retreat."))
    f.append(SP(3))
    f.append(Paragraph("The proven result: agreement is possible if and only if more than "
        "two thirds of participants are honest.", S["quote"]))
    f.append(SP(2))
    f.append(D8TwoThirds())
    f.append(SP(6))
    f.append(Callout("Remember this number", [
        "The <b>two-thirds</b> threshold appears everywhere in this field, and it will appear "
        "throughout your project — in the vote threshold <font face='Mono' size='8'>T</font>, "
        "in the honest-stake assumption <font face='Mono' size='8'>h</font>, in the committee "
        "sizing.",
        "In Phase 3 you will derive it properly rather than take it on trust, and you will see "
        "that it is not a rule someone chose — it falls out of the arithmetic as the only "
        "region where safety and liveness can both hold.",
        "<b>PBFT</b> (Practical Byzantine Fault Tolerance, 1999) is the classic protocol that "
        "achieves this. It works beautifully for perhaps 20 to 50 known servers, and collapses "
        "beyond that, because every node must message every other node.",
    ], "key"))
    f.append(SecBreak())
    return f

def sec10():
    f = [SectionMark("10 • Sybil attacks")]
    f.append(EY("SECTION 10"))
    f.append(H1("The Sybil attack"))
    f.append(SP(4))
    f.append(P("There is a hole in everything said so far, and it is a big one.", "lead"))
    f.append(P("The network is open — anyone can join. So I create " + b("one million fake "
        "identities") + ". Now \"more than two thirds of participants are honest\" is a "
        "meaningless sentence, because I " + i("am") + " 99% of the participants. I vote "
        "however I like, as often as I like."))
    f.append(SP(3))
    f.append(Callout("Why the odd name", [
        "Named after a 1973 case study of a woman with multiple personality disorder, "
        "published under the pseudonym Sybil. The term entered computer science in a 2002 "
        "paper by John Douceur.",
        "A <b>Sybil attack</b> is any attack where one party gains disproportionate influence "
        "by pretending to be many parties.",
    ], "note"))
    f.append(SP(7))
    f.append(P("So every blockchain must answer one question before anything else works:"))
    f.append(Paragraph("What makes an identity expensive?", S["quote"]))
    f.append(P("Because identities are free, influence must be tied to something that is not. "
        "There are two established answers, and they are the subjects of the next two sections."))
    f.append(SP(4))
    f.append(grid_table(
        ["ANSWER", "INFLUENCE IS TIED TO", "USED BY"],
        [["<b>Proof of Work</b>", "electricity burned — computation you must actually perform",
          "Bitcoin, and most early cryptocurrencies"],
         ["<b>Proof of Stake</b>", "money held — value you must actually own and risk",
          "Algorand, Ethereum today, and <b>your project</b>"]],
        [0.22, 0.46, 0.32]))
    f.append(SecBreak())
    return f

def sec11():
    f = [SectionMark("11 • Proof of Work")]
    f.append(EY("SECTION 11"))
    f.append(H1("Proof of Work, and the price it pays"))
    f.append(SP(4))
    f.append(P("Bitcoin's answer, from 2008: make votes cost electricity rather than "
               "identities.", "lead"))
    f.append(P("To add a block you must find a number — called a " + b("nonce") + ", meaning "
        "a number used once — such that the hash of the block plus that nonce is below some "
        "target, which in practice means it starts with a run of zeros:"))
    f.append(CodeBlock(["SHA-256( block_contents + nonce )  <  target"]))
    f.append(SP(8))
    f.append(P("Because hashes are one-way (section 3), there is no clever method. Your only "
        "option is to guess, trillions of times per second. This is " + b("mining") + ". It is "
        "a lottery where the tickets cost real electricity, and " + b("fake identities buy you "
        "nothing") + ": one computer split into a million identities has exactly the hash power "
        "of one computer. The Sybil attack is defeated by physics."))
    f.append(SP(5))
    f.append(D9PoW())
    f.append(SP(6))
    f.append(H2("What this costs"))
    f.append(term_table([
        ("Forks are normal", "Two miners can win at nearly the same moment. Both broadcast, "
         "the network splits, and one branch is later discarded along with its transactions."),
        ("Blocks must be slow", "Bitcoin targets ten minutes per block specifically so that "
         "forks stay rare enough to resolve."),
        ("Confirmation is slow", "You wait about six blocks — roughly an hour — before "
         "treating a payment as settled."),
        ("Finality is only probabilistic", "A transaction is never mathematically final. It "
         "just becomes exponentially unlikely to be reversed. <b>This is the point Algorand "
         "attacks.</b>"),
        ("Energy", "The entire network burns power continuously so that one participant wins."),
        ("Throughput", "About seven transactions per second. Visa handles roughly 65,000."),
    ]))
    f.append(SecBreak())
    return f

def sec12():
    f = [SectionMark("12 • Proof of Stake")]
    f.append(EY("SECTION 12"))
    f.append(H1("Proof of Stake"))
    f.append(SP(4))
    f.append(P("The alternative: make votes cost money instead of electricity.", "lead"))
    f.append(P("Your influence is proportional to the coins you hold. Fake identities are "
        "again useless — splitting 1,000 coins across 100 accounts still leaves you with "
        "1,000 coins of influence, not 100,000. The Sybil attack is defeated by arithmetic "
        "rather than physics."))
    f.append(SP(3))
    f.append(P(b("The advantages are obvious:") + " no electricity waste, and blocks can be "
        "fast because there is no puzzle to solve."))
    f.append(P(b("The difficulty is subtler.") + " Producing a block is now free, so a cheater "
        "can sign two conflicting blocks at no cost whatsoever. This is called the " +
        b("nothing-at-stake") + " problem. Most proof-of-stake systems answer it with " +
        b("slashing") + ": misbehave provably, and your deposit is destroyed."))
    f.append(SP(5))
    f.append(Callout("How Algorand differs — and why this matters to you", [
        "Algorand is proof of stake, but it <b>never punishes anyone</b>. There is no slashing "
        "in the paper at all.",
        "It does not need any, because it does not make cheating <i>expensive</i> — it makes "
        "cheating <b>impossible</b>. Stake is used only to stop an attacker amplifying their "
        "power with fake identities. As long as they hold less than a third of the money, the "
        "protocol guarantees no fork can form, so there is nothing to punish them for.",
        "That is a genuinely different design philosophy from most proof-of-stake systems, and "
        "it is worth being able to state clearly — it is the kind of thing you will be asked "
        "about.",
    ], "key"))
    f.append(SecBreak())
    return f

def sec13():
    f = [SectionMark("13 • Public vs permissioned")]
    f.append(EY("SECTION 13"))
    f.append(H1("Public, private, and where Fabric sits"))
    f.append(SP(4))
    f.append(P("Everything so far assumed an open network anyone can join. That is one option, "
               "not the only one.", "lead"))
    f.append(D10Networks())
    f.append(SP(6))
    f.append(grid_table(
        ["", "PUBLIC (PERMISSIONLESS)", "PRIVATE (PERMISSIONED)"],
        [["<b>Who can join</b>", "Anyone, anonymously", "Only approved, identified members"],
         ["<b>Examples</b>", "Bitcoin, Ethereum", "Hyperledger Fabric, Corda"],
         ["<b>Sybil risk</b>", "Severe — needs PoW or PoS", "Low — membership is controlled by a CA"],
         ["<b>Typical size</b>", "Thousands of nodes", "5 to 100 nodes"],
         ["<b>Speed</b>", "Slow, by necessity", "Fast"],
         ["<b>Trust model</b>", "Trust nobody", "Members are known, but may still misbehave"]],
        [0.20, 0.40, 0.40]))
    f.append(SP(10))
    f.append(H2("What this means for your project"))
    f.append(P(b("Hyperledger Fabric is permissioned.") + " Every node holds an identity issued "
        "by a certificate authority, so the network layer already resists Sybil attacks — you "
        "cannot join at all without being admitted."))
    f.append(P("You might reasonably ask: if members are known and vetted, why bother with "
        "Byzantine fault tolerance at all?"))
    f.append(SP(3))
    f.append(Callout("The answer, and the justification for your whole project", [
        "<b>Known does not mean trustworthy.</b> A permissioned member can still be hacked, "
        "bribed, coerced, or simply run buggy software. Identity tells you <i>who</i> misbehaved "
        "— it does not stop them.",
        "Fabric 2.5 ships only <b>Raft</b> as its ordering service. Raft survives nodes that "
        "<b>crash</b>. It does not survive nodes that <b>lie</b> — one malicious leader can "
        "corrupt the log. Solo and Kafka are deprecated, and a Byzantine option is not available.",
        "So Fabric 2.5 has <b>no Byzantine-fault-tolerant consensus at all</b>. That gap is "
        "precisely what you are filling by implementing Algorand's BA* as a custom ordering "
        "service. When someone asks why the project is worth doing, this paragraph is the answer.",
    ], "key"))
    f.append(PageBreak())
    return f
