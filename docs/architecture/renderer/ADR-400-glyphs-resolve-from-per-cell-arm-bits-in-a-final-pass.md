---
status: Accepted
date: 2026-09-09
deciders:
  - aaronsb
  - claude
related:
  - ADR-101
  - ADR-102
  - ADR-500
---

# ADR-400: Glyphs resolve from per-cell arm bits in a final pass

## Context

The canvas writes glyphs the moment a drawer asks for them. A horizontal
edge segment writes `─` cell by cell; a bend writes a corner chosen by
`getCornerChar` from the incoming and outgoing directions; a tee at a node
border and a crossing are produced by `Canvas.Put` looking up the pair
(existing glyph, new glyph) in a 100-entry `junctionTable` in
`internal/renderer/canvas.go`. Any pair the table does not list is
resolved by overwriting.

Three defects in the current flowchart output come from that table.
`drawEdgeLines` draws a path's segments first and its corners second, so
a bend cell receives the path's own horizontal and vertical runs, which
merge to `┼`, and the corner glyph then replaces `┼` because that pair is
not listed. Any arm another edge had through the cell is lost. Two edges
leaving one port render `├──╮►│`, and a rounded corner crossing a
horizontal run renders `╭` inside `───`. The third finding is not a
merging defect: an outgoing edge's source tee and an incoming edge's
arrowhead share one port cell on the node border, so the tee's arm meets
the arrowhead's tip (`┴` under `▼`). ADR-102's port spreading is the fix
for that one.

`getCornerChar` in `draw.go` lines 322 to 407 carries a reasoning
transcript in its comments. `DrawDiamond` uses U+27CB and U+27CD from the
Mathematical Operators block for its chamfers, and `DrawHexagon` uses
U+2B21 for its indicator; both have poor font coverage.

The set of correct glyphs for a cell is a function of which of its four
sides a line leaves through. A pair table approximates that function and
grows quadratically with the glyph set.

## Decision

### Arms, not glyphs

The canvas gains a per-cell line layer beside the glyph grid: four arm
bits (north 1, east 2, south 4, west 8), a weight (light, heavy, double,
dashed), and a rounded flag. Drawers of lines call `Arm(row, col, bits,
style)` and `Segment(r1, c1, r2, c2, style)`; they never write a
box-drawing rune. `Segment` gives every interior cell both along-axis
arms and each endpoint cell only its inward arm. `Arm` ORs bits into the
cell. Weight merges to the heaviest contributor; dashed yields to any
solid contributor; rounded is set when any contributor asks for it and
applies only when the cell resolves to a two-arm corner.

A cell's style is set by the first arm written into it. A tee on a node
border therefore keeps the node's style, and `restoreNodeBorderStyles` is
deleted.

### The resolve pass

Before `ToString` or `ToColorString`, `Resolve(cs)` walks the line layer
and writes one glyph per armed cell from a 16-entry table indexed by the
arm bits. `CharSet` carries one table per weight (`Light`, `Heavy`,
`Double`, `Dashed`) and a four-entry `Rounded` override for the corners.
Single-arm cells resolve to the half-line glyphs `╴╵╶╷` in light and
heavy, and to the full line in double and dashed, which have no half
glyphs. ASCII tables map straight runs to the stroke's pair (`-`/`|`,
`=`/`|`, `.`/`:`) and every other entry to `+`.

A literal glyph already in the cell wins over the arms: arrowheads, shape
indicators, endpoint markers, and text are written with `Put` as now and
are never replaced by the resolver. `junctionTable`, `boxChars`, and
`getCornerChar` are deleted. Corners, tees, and crossings are outcomes of
the table.

`FlipVertical` and `FlipHorizontal` swap arm bits before the resolve pass
and keep their rune maps for arrowheads and other literals.

### The edge invariant

An edge writes arms into exactly two node cells at most: the source attach
cell on the node border and the target attach cell, each only when that
end has no arrowhead. A segment toward an arrowhead runs to the arrowhead's cell,
where the arrowhead literal wins, so the tail cell keeps both arms and no
arm reaches the node border. The lint rule 3 of ADR-101 fails any build
that breaks this.

### Glyph swaps carried in the same change

The diamond chamfers become U+2571 `╱` and U+2572 `╲` from Box Drawing.
The hexagon indicator becomes U+2394 `⎔` from Miscellaneous Technical.
ADR-500 makes both subject to family fallback.

### Gate

`flowchart` and `subgraph-cross` leave `known-bad.txt`; `flowchart-cross`
stays until ADR-102 spreads the ports. Every golden is re-recorded with
the commit naming which frames changed and why. Any frame whose only
change is a corrected junction is expected.

## Consequences

### Positive

- Junction correctness is a property of a 16-entry table, not a 100-pair
  list; the three defects cannot recur through merging.
- Heavy, double, dashed, rounded, and ASCII are tables instead of code
  paths; ADR-500's families slot in as further tables.
- Flips are exact.
- Three functions and one hack are deleted.

### Negative

- Every drawer of lines changes: shapes, subgraph borders, edges, and the
  diagram renderers outside the flowchart engine that draw boxes directly
  through `Put` with merge. Those renderers migrate in the same change or
  keep writing literal glyphs, which the resolver leaves alone. Literal
  box glyphs no longer merge with anything.
- Mixed-weight junction glyphs (a heavy line meeting a light box) resolve
  to the heaviest weight for the whole cell. Unicode has mixed glyphs,
  but their coverage is worse than the problem.

### Neutral

- `Put` loses its `merge` parameter.
- ASCII keeps `.`/`:` for dashed and `=` for heavy and double runs; every
  ASCII junction is `+`.
- Visual output of `├──┬►│` for two edges sharing a port is structurally
  sound and still ugly. ADR-102 spreads the ports.
- A port that one edge leaves through and another enters keeps a rule 3
  finding: the source tee's arm meets the incoming arrowhead's tip, and
  both are what this decision draws. `flowchart-cross` has three such
  ports and stays in `known-bad.txt` until ADR-102 spreads them; `flowchart`
  and `subgraph-cross` leave.
- A line's free end resolves to a half glyph. Renderers that ran a rune
  up to a corner they wrote themselves now run the segment through the
  corner cell, whose arms close it; the class and ER routers, whose
  endpoints sit one cell past a border, join that cell to the border.

## Alternatives Considered

- **Complete the pair table.** Rejected: the table is the wrong
  representation; every new glyph set multiplies it.
- **Arm bits with per-arm weight.** Rejected: mixed-weight glyphs have
  poor font coverage, and no current diagram needs them.
- **Resolve lazily inside `Put` by reading neighbours.** Rejected: a
  cell's glyph depends on writes that have not happened yet; only a final
  pass sees the whole layer.
- **Keep `getCornerChar` and fix its three cases.** Rejected: the corner
  is already determined by which arms the two segments contribute.
