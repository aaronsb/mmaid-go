---
status: Accepted
date: 2026-09-09
deciders:
  - aaronsb
  - claude
related:
  - ADR-101
  - ADR-102
  - ADR-103
---

# ADR-104: Explicit-position layout path for architecture diagrams

## Context

`architecture-beta` is the one Mermaid diagram type
`docs/upstream-parity-audit.md` defers. Its syntax exists for spatial
placement: `db:R -- L:server` says the database's right side meets the
server's left side, and the whole diagram is a set of such adjacency
hints. Upstream builds a graph and hands the layout engine explicit
grid positions derived from the hints.

`internal/layout/grid.go` runs Sugiyama layering unconditionally.
`graph.Graph` has no position field. A port that let the auto-layout
place the nodes would discard the placement the syntax expresses.

## Decision

### Positions on the graph

`graph.Graph` gains `Positions map[string]layout.GridCoord`. When it is
non-empty, `ComputeLayout` skips `assignLayers`, `orderLayers`, and
`placeNodes` and takes the positions as the grid coordinates, then runs
the existing steps from port counting onward: sizes, gap expansion,
subgraph bounds, draw coordinates. Routing, ports, labels, and border
crossings are unchanged.

Positions are normalised so the minimum column and row are zero. Two
nodes on one coordinate is a parser error reported on stderr, and the
second node is placed in the next free column.

### Groups

An architecture `group` is a subgraph. With explicit positions the
ADR-103 block recursion does not run; a group's bounds are the bounding
box of its members' cells plus the subgraph padding. Nested groups
contain their children's boxes. Two groups whose members interleave
overlap, and that is the author's placement.

### Junctions

`graph.ShapeJunction` is a node shape that draws nothing and occupies
one cell. Edges to and from a junction attach at that cell, so a
junction with four edges resolves to `┼` and one with three to a tee
through the ADR-400 tables.

### The parser

`internal/parser/architecture.go` reads `architecture-beta`: `group
id(icon)[Label] in parent`, `service id(icon)[Label] in group`,
`junction id in group`, and edges `a:R -- L:b`, `a:B --> T:b`, with the
optional `{group}` suffix that attaches the edge to the group border.
Icons are kept as a label prefix in square brackets (`[db]`) since the
terminal has no icon set. Direction hints resolve to positions by a
breadth-first walk from the first declared node: each hint places the
other node one cell in the named direction; a conflict between two
hints for one node keeps the first and reports the second on stderr.
Nodes no hint reaches are placed in a row below the placed ones.

### Gate

Fixtures `architecture.mmd` (the Mermaid docs example with a group, a
junction, and four direction hints) and `architecture-nested.mmd` are
recorded and pass `TestFixturesLint`.

## Consequences

### Positive

- The one deferred diagram type ships with the placement its syntax
  encodes.
- The explicit path is an entry condition in `ComputeLayout`, not a
  second layout engine; everything after placement is shared.
- Junctions reuse the arm tables.

### Negative

- Explicit positions bypass ADR-103's non-interleaving guarantee for
  groups.
- Direction hints are a constraint system and the breadth-first walk is
  a first-fit solver; contradictory hints are reported, not solved.
- An edge between two groups whose blocks are adjacent is dropped. The
  attach cells outside their facing borders are the same cell, and a path
  of one cell draws nothing. Positioned blocks are adjacent by
  construction, so `{group}` on both ends of one edge asks for a gap the
  syntax cannot express; the flowchart engine drops the same edge between
  two sibling subgraphs.

### Neutral

- `Positions` is available to any parser; `block-beta` may adopt it
  later for its explicit columns.
- `GridCoord` is defined in `internal/graph`, which `internal/layout`
  names through a type alias, since `graph` cannot import `layout`.
  `go vet`'s composite check then reads every `GridCoord{c, r}` in the
  layout as a literal of an imported type, so those literals carry field
  names.
- The parser emits each edge from its left end, arrowheads travelling
  with the ends. `PreferredSides` reads an edge whose target lies left of
  its source as a back edge and sends it around the bottom, which is the
  layered path's convention and not a positioned graph's.
- A hint pair across the two axes places the far node diagonally.
  Upstream reads the far node's letter as a direction on the column axis
  and as its opposite on the row axis, so its bend resolves differently
  depending on which end the walk reaches first; here both letters name
  the side of their own node the line leaves through.

## Alternatives Considered

- **Let the auto-layout place architecture nodes.** Rejected: it
  discards the placement the syntax exists for, as the audit says.
- **A separate architecture renderer.** Rejected: the diagram is boxes
  and orthogonal edges, which is the flowchart engine.
- **Solve hints as a constraint system with backtracking.** Rejected for
  now: upstream's own resolver is first-fit, and its diagrams are small.
