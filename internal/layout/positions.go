package layout

import (
	"fmt"
	"io"

	"github.com/aaronsb/mmaid-go/internal/graph"
)

// layoutPositions is the explicit-position entry path of ADR-104: the
// graph's own positions stand in for layering, ordering and placement.
// Positions are normalised so the least column and row are zero, two nodes
// on one cell are reported and the second takes the next free column of its
// row, and a node the graph gives no position lands in a row below the
// rest. A subgraph's block is the bounding box of its members' cells and
// its children's blocks.
func layoutPositions(g *graph.Graph, warn io.Writer) (map[string]GridCoord, map[*graph.Subgraph]posRect) {
	minCol, minRow, first := 0, 0, true
	for _, nid := range g.NodeOrder {
		p, ok := g.Positions[nid]
		if !ok {
			continue
		}
		if first {
			minCol, minRow, first = p.Col, p.Row, false
			continue
		}
		minCol, minRow = min(minCol, p.Col), min(minRow, p.Row)
	}

	positions := make(map[string]GridCoord, len(g.NodeOrder))
	taken := make(map[GridCoord]string, len(g.NodeOrder))
	maxRow := 0
	var loose []string
	for _, nid := range g.NodeOrder {
		p, ok := g.Positions[nid]
		if !ok {
			loose = append(loose, nid)
			continue
		}
		at := GridCoord{Col: p.Col - minCol, Row: p.Row - minRow}
		if held, clash := taken[at]; clash {
			if warn != nil {
				fmt.Fprintf(warn, "mmaid: %s and %s are both at column %d row %d\n", held, nid, at.Col, at.Row)
			}
			for taken[at] != "" {
				at.Col++
			}
		}
		taken[at] = nid
		positions[nid] = at
		maxRow = max(maxRow, at.Row)
	}
	for i, nid := range loose {
		positions[nid] = GridCoord{Col: i, Row: maxRow + 1}
	}

	rects := make(map[*graph.Subgraph]posRect)
	var box func(sg *graph.Subgraph) (posRect, bool)
	box = func(sg *graph.Subgraph) (posRect, bool) {
		r, ok := posRect{}, false
		grow := func(c0, r0, c1, r1 int) {
			if !ok {
				r, ok = posRect{c0, r0, c1, r1}, true
				return
			}
			r = posRect{min(r.c0, c0), min(r.r0, r0), max(r.c1, c1), max(r.r1, r1)}
		}
		for _, child := range sg.Children {
			if cr, has := box(child); has {
				grow(cr.c0, cr.r0, cr.c1, cr.r1)
			}
		}
		for _, nid := range sg.NodeIDs {
			if p, has := positions[nid]; has {
				grow(p.Col, p.Row, p.Col, p.Row)
			}
		}
		if ok {
			rects[sg] = r
		}
		return r, ok
	}
	for _, sg := range g.Subgraphs {
		box(sg)
	}
	return positions, rects
}

// markPoints flags the placements of nodes that occupy a single cell and
// releases the eight cells around them, so an edge reaches the cell itself
// and its arms resolve there to a tee or a cross (ADR-104).
func markPoints(g *graph.Graph, layout *GridLayout) {
	for nid, p := range layout.Placements {
		node, ok := g.Nodes[nid]
		if !ok || node.Shape != graph.ShapeJunction {
			continue
		}
		p.Point = true
		for dc := -1; dc <= 1; dc++ {
			for dr := -1; dr <= 1; dr++ {
				if dc == 0 && dr == 0 {
					continue
				}
				delete(layout.GridOccupied, GridCoord{Col: p.Grid.Col + dc, Row: p.Grid.Row + dr})
			}
		}
	}
}
