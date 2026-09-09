package layout

import (
	"sort"

	"github.com/aaronsb/mmaid-go/internal/graph"
	"github.com/aaronsb/mmaid-go/internal/textwidth"
)

// sideDepth is the draw cells one subgraph box adds beyond its content on
// one side: the padding, and on the top the label rows as well. Nested
// boxes stack, so the n-th enclosing box's line lies n*sideDepth cells out.
func sideDepth(top bool) int {
	if top {
		return SGBorderPad + SGLabelHeight
	}
	return SGBorderPad
}

// borderRun is one side of a subgraph box lying in a gap cell: opening
// when the block's first position follows the gap, closing when its last
// position precedes it. lo and hi are the positions the side spans along
// the gap.
type borderRun struct {
	opening bool
	lo, hi  int
}

// gapCells returns the cells of a gap of the given size that border lines
// occupy. Closing sides count from the block that precedes the gap and
// opening sides from the block that follows, each nested side one
// sideDepth further out; on the row axis an opening side is a box top.
func gapCells(runs []borderRun, size int, rows bool) (closing, opening map[int]bool) {
	closing, opening = make(map[int]bool), make(map[int]bool)
	nc, no := 0, 0
	for _, r := range runs {
		if r.opening {
			no++
		} else {
			nc++
		}
	}
	for i := range nc {
		closing[sideDepth(false)*(i+1)-1] = true
	}
	for i := range no {
		opening[size-sideDepth(rows)*(i+1)] = true
	}
	return closing, opening
}

// gapSize returns the smallest size at least min whose centre cell, the
// corridor an edge follows, lies outside every box in the gap with a
// straight cell between it and each border line.
func gapSize(runs []borderRun, min int, rows bool) int {
	for size := max(min, 1); ; size++ {
		c := size / 2
		ok := true
		closing, opening := gapCells(runs, size, rows)
		for b := range closing {
			if c-b < 2 {
				ok = false
			}
		}
		for b := range opening {
			if b-c < 2 {
				ok = false
			}
		}
		if ok {
			return size
		}
	}
}

// depth returns how many subgraphs enclose sg, plus one.
func depth(sg *graph.Subgraph) int {
	d := 0
	for ; sg != nil; sg = sg.Parent {
		d++
	}
	return d
}

// expandGapsForSubgraphs widens the last node column of every block whose
// label is wider than the block, deepest blocks first, then sizes every
// gap column and row for the borders that run through it.
func expandGapsForSubgraphs(g *graph.Graph, layout *GridLayout) {
	if len(layout.blocks) == 0 {
		return
	}
	sgs := make([]*graph.Subgraph, 0, len(layout.blocks))
	for sg := range layout.blocks {
		sgs = append(sgs, sg)
	}
	sort.Slice(sgs, func(i, j int) bool {
		di, dj := depth(sgs[i]), depth(sgs[j])
		if di != dj {
			return di > dj
		}
		return sgs[i].ID < sgs[j].ID
	})
	for _, sg := range sgs {
		r := layout.blocks[sg]
		width := 0
		for c := r.c0 * Stride; c <= r.c1*Stride+2; c++ {
			width += layout.ColWidths[c]
		}
		if need := textwidth.String(sg.Label) + 4; need > width+2*sideDepth(false) {
			layout.ColWidths[r.c1*Stride+1] += need - width - 2*sideDepth(false)
		}
	}

	cols := make(map[int][]borderRun)
	rows := make(map[int][]borderRun)
	maxCol, maxRow := 0, 0
	for _, r := range layout.blocks {
		maxCol = max(maxCol, r.c1)
		maxRow = max(maxRow, r.r1)
		if r.c0 > 0 {
			gc := r.c0*Stride - 1
			cols[gc] = append(cols[gc], borderRun{opening: true, lo: r.r0, hi: r.r1})
		}
		gc := r.c1*Stride + 3
		cols[gc] = append(cols[gc], borderRun{lo: r.r0, hi: r.r1})
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
			if len(at) > 0 {
				best = max(best, gapSize(at, cur, isRows))
			}
		}
		return best
	}
	// Every gap keeps a free cell on each side of its corridor, beyond the
	// cells a port's stub and an arrowhead take, so a crossing run can
	// move off a cell another edge holds.
	for p := 0; p <= maxCol; p++ {
		gc := p*Stride + 3
		layout.ColWidths[gc] = max(layout.ColWidths[gc], SGGapMin)
	}
	for p := 0; p <= maxRow; p++ {
		gr := p*Stride + 3
		layout.RowHeights[gr] = max(layout.RowHeights[gr], SGGapMin)
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

		width := max((maxX-minX)+2*sideDepth(false), textwidth.String(sg.Label)+4)
		return &SubgraphBounds{
			Subgraph: sg,
			X:        minX - sideDepth(false),
			Y:        minY - sideDepth(true),
			Width:    width,
			Height:   (maxY - minY) + sideDepth(true) + sideDepth(false),
		}
	}

	for _, sg := range layout.rootSubgraphs(g) {
		if bounds := compute(sg); bounds != nil {
			layout.SubgraphBounds = append(layout.SubgraphBounds, *bounds)
		}
	}
}

// Block returns the grid cells of the first and last node positions a
// subgraph's block covers, or false for a subgraph with no nodes.
func (l *GridLayout) Block(sg *graph.Subgraph) (min, max GridCoord, ok bool) {
	r, ok := l.blocks[sg]
	if !ok {
		return GridCoord{}, GridCoord{}, false
	}
	return GridCoord{r.c0*Stride + 1, r.r0*Stride + 1}, GridCoord{r.c1*Stride + 1, r.r1*Stride + 1}, true
}
