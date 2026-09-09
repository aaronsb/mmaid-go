package layout

import (
	"math"

	"github.com/aaronsb/mmaid-go/internal/graph"
	"github.com/aaronsb/mmaid-go/internal/textwidth"
)

// borderRun is one side of a subgraph box lying in a gap cell: opening
// when the block's first position follows the gap, closing when its last
// position precedes it. lo and hi are the positions the side spans along
// the gap.
type borderRun struct {
	opening bool
	lo, hi  int
	// excess is how far a closing side moves outward for a label wider
	// than the block.
	excess int
}

// gapCells returns the cells of a gap that border lines occupy, given the
// borders in it and its size. Closing sides take the cells nearest the
// block that precedes the gap, opening sides the cells nearest the block
// that follows; each nested side adds SGBorderPad, and the innermost
// opening side on the row axis adds the label height.
func gapCells(runs []borderRun, size int, rows bool) map[int]bool {
	cells := make(map[int]bool)
	closing, opening, excess := 0, 0, 0
	for _, r := range runs {
		if r.opening {
			opening++
		} else {
			closing++
			excess = max(excess, r.excess)
		}
	}
	for i := range closing {
		cells[SGBorderPad-1+SGBorderPad*i+excess] = true
	}
	first := SGBorderPad
	if rows {
		first += SGLabelHeight
	}
	for i := range opening {
		cells[size-first-SGBorderPad*i] = true
	}
	return cells
}

// gapSize returns the smallest size at least min whose centre cell holds no
// border line.
func gapSize(runs []borderRun, min int, rows bool) int {
	size := min
	for gapCells(runs, size, rows)[size/2] {
		size++
	}
	return size
}

// expandGapsForSubgraphs sizes every gap column and row for the subgraph
// borders that run through it, so that the corridor an edge follows never
// lies on a border line, and widens the gap after a block whose label is
// wider than the block.
func expandGapsForSubgraphs(g *graph.Graph, layout *GridLayout) {
	if len(layout.blocks) == 0 {
		return
	}
	cols := make(map[int][]borderRun)
	rows := make(map[int][]borderRun)
	maxCol, maxRow := 0, 0
	for sg, r := range layout.blocks {
		maxCol = max(maxCol, r.c1)
		maxRow = max(maxRow, r.r1)
		excess := 0
		width := 0
		for c := r.c0 * Stride; c <= r.c1*Stride+2; c++ {
			width += layout.ColWidths[c]
		}
		if need := textwidth.String(sg.Label) + 4; need > width+2*SGBorderPad {
			excess = need - width - 2*SGBorderPad
		}
		if r.c0 > 0 {
			gc := r.c0*Stride - 1
			cols[gc] = append(cols[gc], borderRun{opening: true, lo: r.r0, hi: r.r1})
		}
		gc := r.c1*Stride + 3
		cols[gc] = append(cols[gc], borderRun{lo: r.r0, hi: r.r1, excess: excess})
		if r.r0 > 0 {
			gr := r.r0*Stride - 1
			rows[gr] = append(rows[gr], borderRun{opening: true, lo: r.c0, hi: r.c1})
		}
		gr := r.r1*Stride + 3
		rows[gr] = append(rows[gr], borderRun{lo: r.c0, hi: r.c1})
	}

	size := func(runs []borderRun, cur, maxPos int, isRows bool) int {
		best := cur
		for pos := 0; pos <= maxPos; pos++ {
			var at []borderRun
			for _, r := range runs {
				if r.lo <= pos && pos <= r.hi {
					at = append(at, r)
				}
			}
			if len(at) == 0 {
				continue
			}
			best = max(best, gapSize(at, max(cur, len(at)*SGGapPerLevel), isRows))
		}
		return best
	}
	for gc, runs := range cols {
		layout.ColWidths[gc] = size(runs, layout.ColWidths[gc], maxRow, false)
	}
	for gr, runs := range rows {
		layout.RowHeights[gr] = size(runs, layout.RowHeights[gr], maxCol, true)
	}
}

// computeSubgraphBounds sets each subgraph's box from its block of
// positions, the boxes of its children, and the padding. Sibling blocks are
// disjoint, so sibling boxes do not overlap; a child block lies inside its
// parent's, so its box does too.
func computeSubgraphBounds(g *graph.Graph, layout *GridLayout) {
	var compute func(sg *graph.Subgraph) *SubgraphBounds
	compute = func(sg *graph.Subgraph) *SubgraphBounds {
		var childBounds []SubgraphBounds
		for _, child := range sg.Children {
			if cb := compute(child); cb != nil {
				childBounds = append(childBounds, *cb)
				layout.SubgraphBounds = append(layout.SubgraphBounds, *cb)
			}
		}

		r, ok := layout.blocks[sg]
		if !ok {
			return nil
		}
		minX, minY := layout.GridToDraw(r.c0*Stride, r.r0*Stride)
		maxX, maxY := layout.GridToDraw(r.c1*Stride+3, r.r1*Stride+3)
		for _, cb := range childBounds {
			minX = min(minX, cb.X)
			minY = min(minY, cb.Y)
			maxX = max(maxX, cb.X+cb.Width)
			maxY = max(maxY, cb.Y+cb.Height)
		}
		if minX == math.MaxInt {
			return nil
		}

		width := max((maxX-minX)+SGBorderPad*2, textwidth.String(sg.Label)+4)
		return &SubgraphBounds{
			Subgraph: sg,
			X:        minX - SGBorderPad,
			Y:        minY - SGBorderPad - SGLabelHeight,
			Width:    width,
			Height:   (maxY - minY) + SGBorderPad*2 + SGLabelHeight,
		}
	}

	for _, sg := range g.Subgraphs {
		if bounds := compute(sg); bounds != nil {
			layout.SubgraphBounds = append(layout.SubgraphBounds, *bounds)
		}
	}
}
