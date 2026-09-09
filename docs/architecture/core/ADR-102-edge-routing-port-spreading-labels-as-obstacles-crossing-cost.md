---
status: Proposed
date: 2026-09-09
deciders:
  - aaronsb
  - claude
related:
  - ADR-101
  - ADR-400
  - ADR-103
---

# ADR-102: Edge routing: port spreading, labels as obstacles, crossing cost

## Context

`internal/routing/router.go` attaches every edge to the centre of a node
side. Two edges leaving one side share the first cells of their paths and
separate at a tee. `FindPath` in `pathfinder.go` is A* over the grid with
a step cost of 1, a corner cost of 0.5, and a soft-obstacle cost of 2 for
cells earlier edges occupy. Two edges in the same corridor pay 2 per
shared cell, which is cheaper than a detour of three cells, so edges
overlap for long runs and crossings are frequent.

Edge labels are placed after routing by `drawEdgeLabel` in `draw.go`,
which tries the cells beside a segment and falls back to forcing the
label at the path midpoint over whatever is there. Labels are neither
routed around nor kept off node borders. The state fixture in ADR-101
prints `stop` over a node's top-right corner.

## Decision

### Ports spread along the side

A node side offers ports at every cell between its corners, excluding the
corner cells. Edges attached to a side are sorted by the position of their
other endpoint along that side's axis and assigned distinct ports from the
centre outward. A side with more edges than ports assigns the centre to
the overflow. `getAttachPoint` takes the port index; the layout grid gains
a per-node port count on each side so that gap sizing accounts for it.

Ports are the reason two edges never share a segment at the node.

### Labels are routed obstacles

Each edge's label reserves a run of cells before the next edge is routed:
the label is placed beside the first segment after the edge's last turn,
one cell off the line on the side away from the nearest node, and its
cells enter the hard-obstacle set for later edges. A label whose reserved
cells would overlap a node border, a subgraph border, or another label
moves along the segment until it fits, then to the segment before it, and
finally shrinks to its first word with an ellipsis. A label never lands
on a border cell. The forced fallback in `drawEdgeLabel` is deleted.

### Crossing costs more than a detour

The soft-obstacle cost rises from 2 to 6 per cell, and crossing a
subgraph border costs 4. A step that enters a cell occupied by another
edge and leaves in the same axis (a shared corridor) pays the full 6; a
step that crosses perpendicular pays 3. Edges prefer their own corridor
and cross each other only where a corridor would be longer than the
crossing is expensive.

### Gate

The `flowchart-cross`, `state-label`, and `subgraph-cross` fixtures of
ADR-101 are re-recorded with this change and the commit names what
moved. `TestFixturesLint` stays clean.

## Consequences

### Positive

- Two edges from one node leave through different cells.
- Labels are always readable and never on a border.
- Corridors are shared only where a detour is longer than the penalty.

### Negative

- Node sides need width for ports, so dense graphs get wider.
- Labels as obstacles can push a later edge onto a longer path.
- Three numbers (6, 3, 4) are tuned by eye on the fixture set and are
  the kind of constant a later diagram will want to move.

### Neutral

- Port assignment is a layout concern and lands in `internal/layout`;
  routing reads it.
- The ellipsis fallback introduces truncation to edge labels for the
  first time.

## Alternatives Considered

- **Keep centre ports and rely on the tee.** Rejected: `├──┬►│` is sound
  and unreadable.
- **Labels after routing with a search for free cells.** Rejected: once
  the edges are routed there may be no free cell near the edge, and the
  forced fallback exists because of that.
- **A hard obstacle for occupied cells.** Rejected: dense graphs would
  fail to route; a high soft cost degrades instead.
