---
status: Accepted
date: 2026-09-09
deciders:
  - aaronsb
  - claude
related: []
---

# ADR-101: Snapshot and golden loop: cells interchange, ANSI interpretation, structural lint

## Context

The only visual gate is `test_visual.sh`, which prints every diagram with a
prose `CHECK` hint for a human eye. The Go tests are substring assertions.
Three glyph defects in the current flowchart output went unnoticed by both:
a `╮` corner directly before an arrowhead (`├──╮►│`), a `╭` inside a
horizontal run where two edges cross, and an edge that bleeds one cell past
its arrowhead and merges into the target's top border as `┴`.

An agent working on the renderer cannot see its own output. Terminal
emulators inside tool-result panes drop columns and substitute glyphs, so
the ANSI stream is not a reliable eye and a text dump loses colour. The
renderer has also had serialization bugs the canvas never showed: commit
01a4d68 removed a background stripe that existed only in the emitted escape
sequences.

`~/Projects/games/roguelike/roguemap` solved the same problem for a
terminal game with a cell-dump format, a PNG rasteriser over a bitmap font,
and a fuzzy golden comparator.

## Decision

### The `.cells` frame

A frame is text: a header line `W H`, then `W*H` lines, one per cell,
row-major, each `codepoint fr fg fb br bg bb`. This is roguemap's
`.cells` dump verbatim, so its tooling reads mmaid frames unchanged.

Bold, dim, and italic have no column. They fold into colour the way a
classic terminal renders them: dim scales the foreground to 60 percent,
bold on one of the eight named colours selects the bright variant, italic
has no representation. Unstyled cells carry the default pair, foreground
(229,229,229) and background (0,0,0). Named colours map to the xterm
palette.

A wide character occupies its first cell; each continuation cell carries
codepoint 0. Until ADR-401 lands, every rune advances one cell.

### The frame comes from the ANSI stream

`mmaid --cells FILE` renders as usual, then interprets the string that
would have gone to stdout and writes the frame to FILE. The interpreter
lives in `internal/cells` and is the smallest terminal that understands
mmaid's own output: SGR 0, 1, 2, 3, 22, 23, 39, 49, the named foreground
and background codes 30-37, 40-47, 90-97, 100-107, and the extended forms
`38;2;r;g;b`, `48;2;r;g;b`, `38;5;n`, `48;5;n`. It skips OSC sequences
terminated by BEL or ST so that ADR-500's hyperlinks pass through. Any
other escape is an error, and the golden test fails on it.

The frame is built from the stream and never from the canvas. A bug in
`ToColorString` is a bug the user sees, and the frame must show it.

Frame width is the widest line. Shorter lines are padded with default
cells.

### PNG

`tools/cells2png.py` is a copy of roguemap's script with two changes:
codepoint 0 draws nothing, and the font path reads `MMAID_SNAP_FONT` with
`/usr/share/fonts/OTF/unscii-16-full.otf` as the default. It needs Python
3 and Pillow. `make snap FILE=path.mmd [ARGS="-t blueprint"]` writes
`.snap/<stem>.cells` and `.snap/<stem>.png` and prints the lint findings
for the frame. `.snap/` is ignored by git.

The edit loop is: change, `make snap`, Read the PNG, adjust.

### Goldens

Fixtures are `testdata/fixtures/<name>.mmd`. A fixture's first line may be
a directive `%% mmaid: <flags>` naming the theme, width, ASCII, or
orientation to render with; the harness parses that line and defaults to
`-t default -w 120`. Width is always set explicitly so the frame does not
depend on terminal detection. References are `testdata/golden/<name>.cells`.

`TestGolden` in the root package renders each fixture through
`mmaid.Render`, interprets the result, and compares it with its reference.
The comparator reports the percentage of identical cells, the percentage
of cells with the same glyph, the mean six-channel colour distance, and
lists the first ten differing cells with expected and actual glyph and
colours.

A frame passes when every glyph matches, identical cells are at least
`GOLDEN_MIN_IDENTICAL` percent (default 98), and the mean distance is at
most `GOLDEN_MAX_DISTANCE` (default 2.0). `GOLDEN_STRICT=1` requires every
cell identical. Glyph strictness is a departure from roguemap: this is a
glyph renderer, and a one-cell glyph regression is exactly the class of
defect above. The colour tolerance lets a theme tweak pass.

`GOLDEN_RECORD=1` rewrites the references; `make golden-record` is the
alias. The commit that re-records says which frames changed and why. A
reference with no fixture fails the test.

### Structural lint

`cells.Lint(frame)` walks the glyph grid with a rune-to-arms table and
returns one finding per violation, each with row, column, glyph, and rule:

1. Every arm of a box-drawing glyph must meet, in the neighbouring cell,
   an arm pointing back, an arrowhead, or text. Text is any glyph that is
   neither a space, a box-drawing glyph, nor an arrowhead.
2. An arrowhead must have an arm feeding it from its tail side.
3. An arm may meet an arrowhead only from the tail side.

The lint reads Unicode box-drawing glyphs only. ASCII output is ambiguous
with text and is covered by the goldens.

Rule 2 flags `╮►`. Rule 1 flags `─╭─`. Rule 3 flags `▼` over `┴`. The
lint knows nothing about layout: a `┼` where an edge crosses a subgraph
border is structurally sound and is left to the goldens.

`TestFixturesLint` runs the lint over every fixture. Fixtures listed in
`testdata/fixtures/known-bad.txt` are expected to fail and the test fails
if one of them passes. ADR-400's implementation removes the flowchart
fixtures from that file; the chart renderers leave it as each is fixed.

### What this replaces

`test_visual.sh` stays until the fixtures cover its cases, then goes.

## Consequences

### Positive

- The renderer's output is visible to an agent through a PNG and
  diffable through a text frame.
- Serialization bugs show in the frame.
- The three glyph defects are detectable with no reference frames.
- A golden re-record is an explicit, reviewable act.

### Negative

- Italic is invisible to the frame and the goldens.
- The PNG path needs Python, Pillow, and a font outside the Go toolchain.
- References grow the repository by about 20 KB per 120x40 frame.

### Neutral

- `internal/cells` is a new package with an interpreter, a parser, a
  writer, a comparator, and a lint.
- Every later ADR in this series lands with fixtures and goldens.
- The `%% mmaid:` directive is a test-harness convention, not a parser
  feature.

## Alternatives Considered

- **Dump the canvas directly.** Rejected: the canvas is not what the user
  sees, and the phantom stripe of 01a4d68 lived only in the stream.
- **Binary `.frame` as roguemap's goldens use.** Rejected: the text form
  diffs in a pull request and roguemap's own tools read it.
- **Extend `.cells` with an attribute column for italic.** Rejected:
  interchange with roguemap's tools is worth more than italic in the
  goldens.
- **Tolerant glyph matching as in roguemap.** Rejected: a one-cell glyph
  change is the defect class this loop exists to catch.
- **Render PNG in Go with an embedded font.** Rejected for now: Pillow
  and Unscii are installed and the script is twenty lines. Revisit if the
  loop needs to run where Python is absent.
