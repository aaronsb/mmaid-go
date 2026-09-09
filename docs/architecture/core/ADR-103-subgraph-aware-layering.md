---
status: Proposed
date: 2026-09-09
deciders:
  - aaronsb
  - claude
related:
  - ADR-101
  - ADR-102
---

# ADR-103: Subgraph-aware layering

## Context

`assignLayers` in `internal/layout/grid.go` assigns a layer to every node
by longest path from the sources and ignores subgraph membership.
`computeSubgraphBounds` then takes the bounding box of each subgraph's
placed nodes. Two subgraphs whose nodes interleave in layer order get
overlapping boxes: the `subgraph-cross` fixture of ADR-101 places S1's A
in layer 0, S2's C in layer 1, S1's B in layer 1, and S2's D in layer 2,
so S1's box spans layers 0 to 1 and S2's box spans 1 to 2 and they
overlap on layer 1. Edges from A to C and B to D then cross the shared
border and leave `┼` in it.

`expandGapsForSubgraphs` already inserts gap cells between layers when
adjacent layers belong to different subgraphs, which handles the case
where the interleaving is absent.

## Decision

### Clusters are laid out as units

Layering becomes hierarchical. Each subgraph is collapsed to one
compound node in its parent's graph, with the edges between its members
and the rest of the parent rewritten to the compound node. The parent
graph is layered and ordered as now. Each subgraph is then laid out
recursively in its own coordinate space, and its members occupy a block
of consecutive layers in the parent's layer sequence starting at the
compound node's layer. Nodes of one subgraph therefore never interleave
with nodes of a sibling.

The recursion bottoms out at subgraphs with no children. A subgraph's
`direction` applies inside its own space, as ADR-100 scopes it.

### Bounds come from the block, not the nodes

`computeSubgraphBounds` takes the block's layer range and cross-axis
range plus padding. Sibling boxes cannot overlap because sibling blocks
are disjoint in layer space; nested boxes are contained because a child
block lies inside its parent's block.

### Edges cross borders at a port

An edge between a subgraph member and an outside node crosses the
subgraph border once. The crossing cell is a port on the border chosen
as in ADR-102, and the border there resolves to a tee toward the inside
and the outside. A crossing never resolves to `┼`.

### Gate

`subgraph-cross` and the nested subgraph fixtures are re-recorded.
`TestFixturesLint` stays clean.

## Consequences

### Positive

- Sibling subgraph boxes never overlap.
- Nested subgraphs are laid out by the same rule as the top level.
- Borders are crossed at tees.

### Negative

- Longest-path layering of the parent graph with compound nodes can
  produce longer diagrams than the flat layering when a subgraph spans
  many layers.
- `assignLayers`, `orderLayers`, `placeNodes`, and
  `computeSubgraphBounds` all change, and the orthogonal-subgraph
  special cases in them are re-expressed through the recursion.

### Neutral

- The compound-node rewrite is the standard approach to clustered
  layered layout and matches what Mermaid's dagre does.
- A border port is the cell where the crossing run meets the border. A
  run whose cell another edge holds moves to the nearest free cell of
  the same gap that keeps its neighbours' directions and lies on no other
  edge's line; a run that starts or ends at a node port keeps the node's
  port. Sorting from the side's centre as ADR-102 does for nodes would
  pull every crossing of a wide box toward its middle.
- The tee's stem points toward the edge's target: the line stops one cell
  before the border, where its free end resolves to a half glyph, and
  starts again at the border cell. With the stem this way an arrowhead may
  sit directly inside the border.
- An edge that starts or ends at a subgraph starts at the border cell it
  leaves through and ends with its arrowhead just outside the border it
  enters. Before, it ran from the member node nearest the box's centre.
- Every gap column and row is sized from the borders in it so that the
  corridor an edge follows never lies on a border line. The corridor of a
  gap row holding one opening top border lies on the box's label row.
  Cross-axis gaps take `SGGapPerLevel` per border instead of a flat 8.
- Two edges that share a grid corridor still share its draw cells, as
  ADR-102 notes; `subgraph-cross` shows it between B and C.
- A subgraph's `direction` is normalized inside its block; `BT` and `RL`
  reverse only at the top level, as before.

## Alternatives Considered

- **Keep flat layering and post-process the bounds to not overlap.**
  Rejected: the boxes would be correct but the nodes inside them would
  still interleave, so a box would contain another subgraph's node.
- **Constrain layering so that all members of a subgraph share
  consecutive layers, without recursion.** Rejected: it is the same
  constraint expressed as a patch on the flat algorithm and does not
  handle nesting.
