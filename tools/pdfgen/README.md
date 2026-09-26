# Phase 1 study PDF generator

Regenerate with:

    python3 build_p1_part3.py

Requires only `reportlab` (already installed). Output:
`Phase1_Blockchain_Fundamentals.pdf`, 22 pages.

Layout:
- `pdfbase_kit.py`   fonts, palette, page furniture, Callout/CodeBlock/Rule flowables
- `pdfdraw.py`       low-level canvas primitives (box, arrow, node, chip)
- `pdfdiagrams_a.py` figures 1-5   (ledgers, double spend, hashing, signatures, the chain)
- `pdfdiagrams_b.py` figures 6-10  (gossip, Byzantine faults, 2/3 bound, PoW, network types)
- `build_p1_part*.py` content and assembly

Fonts are macOS system TrueType files (Georgia, Monaco, DIN Alternate). On another
OS, change the paths at the top of `pdfbase_kit.py`.

Notes for anyone editing this:
- Middot (U+00B7) renders BLANK in DIN Alternate Bold. Use a bullet instead.
- The running section label is drawn in `on_page_end`, not `on_page`. reportlab
  calls `onPage` before any flowable on that page is laid down, so drawing it
  there prints the PREVIOUS page's section.
- Sections flow with `SecBreak()` rather than `PageBreak()`. A tall diagram that
  does not fit jumps whole to the next page, so forcing a break after every
  section left large blank areas.

## Phase 2

    python3 build_p2_part3.py     ->  Phase2_Cryptography_and_VRFs.pdf, 22 pages

- `p2diagrams_a.py`  figures 1-5  (hash resistances, birthday bound, Merkle, curves, Ed25519)
- `p2diagrams_b.py`  figures 6-10 (VRF comparison, sortition, binomial, Sybil, the shortcut)
- `build_p2_part*.py` content and assembly
- `phase2_exercises.py` every exercise printed in the PDF, runnable

`pdfbase_kit.set_phase(running, cover)` switches the running header, so both
phases share one toolkit.

### Two checks worth re-running after any diagram edit

**1. Out-of-bounds drawing.** A diagram Flowable draws from its own origin, and
anything drawn at y < 0 lands on top of the figure caption. This is invisible in
the source and easy to introduce by changing one offset. Probe every diagram by
stubbing the canvas and recording the minimum draw Y — it must stay above ~4.
This caught five Phase 2 diagrams and one Phase 1 diagram (`D3Hash`, drawing at
y = -4 straight through its caption).

**2. Under-filled pages.** Render each page, mask off the header and footer
bands, and find the lowest row containing ink. Anything below about 60% is a
forced `PageBreak()` that is not earning its place. Remaining gaps in both
documents are deliberate: the checkpoint starts on a fresh page so the reader
meets the questions cold, and the final page closes the document.

### Also verify the code the PDF prints

The exercises are printed as text, so they can drift from the output shown
beside them. Extract the printed lines, run them, and diff against the stated
output. This caught Exercise 4 printing counts one lower than the document
claimed (the printed version uses `enumerate`, the original used its own
counter), and a 100-character line that nearly overflowed the code block.
