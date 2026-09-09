package routing

import (
	"sort"

	"github.com/aaronsb/mmaid-go/internal/layout"
)

// Crossing is where an edge's draw path passes through a subgraph border:
// the border cell, the subgraph and side, and the unit step the edge takes
// through the cell toward its target. The renderer breaks the line one cell
// before the border and restarts it at the border cell, so the border
// resolves to a tee whose stem points the way the edge is going.
type Crossing struct {
	Subgraph string
	Side     layout.Side
	At       Point
	Step     Point
}

// along returns the crossing's coordinate along its side.
func (c Crossing) along() int {
	if c.Side.Vertical() {
		return c.At.Row
	}
	return c.At.Col
}

// sideKey names one side of one subgraph's box.
type sideKey struct {
	sg   string
	side layout.Side
}

// Anchor points of a path: the run at index 0 starts at the source port,
// the last run ends at the target port.
const (
	noConflict   = -1
	sourceRun    = 0
	targetRun    = 1
	bothAnchored = 2
)

// borderPorts hands out the cells of each subgraph side that edges cross
// at. A crossing takes the cell where the run naturally meets the border;
// when another edge holds it, a run between two turns moves to the
// nearest free cell of the same gap on the same side of every border line,
// keeping its neighbours' directions and running along no other edge's
// line. A run anchored at a node port cannot move; the router then gives
// that end another port.
type borderPorts struct {
	l     *layout.GridLayout
	taken map[sideKey]map[int]bool
}

func newBorderPorts(l *layout.GridLayout) *borderPorts {
	return &borderPorts{l: l, taken: make(map[sideKey]map[int]bool)}
}

// hitsOn returns the border cells the run a-b passes through, between the
// box's corners, in travel order.
func (bp *borderPorts) hitsOn(a, b Point) []Crossing {
	var out []Crossing
	step := Point{sign(b.Col - a.Col), sign(b.Row - a.Row)}
	for _, sb := range bp.l.SubgraphBounds {
		if sb.Width <= 0 || sb.Height <= 0 {
			continue
		}
		id := sb.Subgraph.ID
		x0, y0, x1, y1 := sb.X, sb.Y, sb.X+sb.Width-1, sb.Y+sb.Height-1
		switch {
		case a.Col == b.Col:
			x := a.Col
			if x <= x0 || x >= x1 {
				continue
			}
			lo, hi := min(a.Row, b.Row), max(a.Row, b.Row)
			if lo < y0 && y0 < hi {
				out = append(out, Crossing{id, layout.Top, Point{x, y0}, step})
			}
			if lo < y1 && y1 < hi {
				out = append(out, Crossing{id, layout.Bottom, Point{x, y1}, step})
			}
		case a.Row == b.Row:
			y := a.Row
			if y <= y0 || y >= y1 {
				continue
			}
			lo, hi := min(a.Col, b.Col), max(a.Col, b.Col)
			if lo < x0 && x0 < hi {
				out = append(out, Crossing{id, layout.Left, Point{x0, y}, step})
			}
			if lo < x1 && x1 < hi {
				out = append(out, Crossing{id, layout.Right, Point{x1, y}, step})
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		p, q := out[i].At, out[j].At
		return (p.Col-q.Col)*step.Col+(p.Row-q.Row)*step.Row < 0
	})
	return out
}

func (bp *borderPorts) free(cs []Crossing) bool {
	for _, c := range cs {
		if bp.taken[sideKey{c.Subgraph, c.Side}][c.along()] {
			return false
		}
	}
	return true
}

// take marks the crossings' cells as held.
func (bp *borderPorts) take(cs []Crossing) {
	for _, c := range cs {
		k := sideKey{c.Subgraph, c.Side}
		if bp.taken[k] == nil {
			bp.taken[k] = make(map[int]bool)
		}
		bp.taken[k][c.along()] = true
	}
}

// shifted returns the path's run i moved to pos on its cross axis.
func shifted(pts []Point, i int, vertical bool, pos int) (Point, Point) {
	a, b := pts[i], pts[i+1]
	if vertical {
		a.Col, b.Col = pos, pos
	} else {
		a.Row, b.Row = pos, pos
	}
	return a, b
}

// cross moves each crossing run between two turns to a free cell of every
// side it meets and returns the path, its crossings in path order, and
// which anchored run, if any, meets a taken cell: sourceRun, targetRun, or
// bothAnchored for a path of one run. Nothing is taken until take is
// called.
func (bp *borderPorts) cross(pts []Point, line map[Point]bool, arrows [2]bool) ([]Point, []Crossing, int) {
	conflict := noConflict
	for i := 0; i+1 < len(pts); i++ {
		hits := bp.hitsOn(pts[i], pts[i+1])
		if len(hits) == 0 || bp.free(hits) {
			continue
		}
		if i >= 1 && i+2 < len(pts) {
			vertical := pts[i].Col == pts[i+1].Col
			if pos, ok := bp.freePos(pts, i, vertical, line, arrows); ok {
				pts[i], pts[i+1] = shifted(pts, i, vertical, pos)
			}
			continue
		}
		if conflict == noConflict {
			switch {
			case len(pts) == 2:
				conflict = bothAnchored
			case i == 0:
				conflict = sourceRun
			default:
				conflict = targetRun
			}
		}
	}
	var out []Crossing
	for i := 0; i+1 < len(pts); i++ {
		out = append(out, bp.hitsOn(pts[i], pts[i+1])...)
	}
	return pts, out, conflict
}

// sameSide reports whether a run at pos lies on the same side as at
// natural of every box line parallel to it that the run's extent meets,
// and on none of them.
func (bp *borderPorts) sameSide(a, b Point, vertical bool, pos, natural int) bool {
	lo, hi := min(a.Row, b.Row), max(a.Row, b.Row)
	if !vertical {
		lo, hi = min(a.Col, b.Col), max(a.Col, b.Col)
	}
	for _, sb := range bp.l.SubgraphBounds {
		if sb.Width <= 0 || sb.Height <= 0 {
			continue
		}
		var lines [2]int
		var span [2]int
		if vertical {
			lines = [2]int{sb.X, sb.X + sb.Width - 1}
			span = [2]int{sb.Y, sb.Y + sb.Height - 1}
		} else {
			lines = [2]int{sb.Y, sb.Y + sb.Height - 1}
			span = [2]int{sb.X, sb.X + sb.Width - 1}
		}
		if hi < span[0] || lo > span[1] {
			continue
		}
		for _, v := range lines {
			if pos == v || sign(pos-v) != sign(natural-v) {
				return false
			}
		}
	}
	return true
}

// freePos searches outward from run i's position for one where every side
// the run crosses is free, the run stays in its grid cell and on the same
// side of every border line, its neighbours keep their directions and a
// neighbour ending in an arrowhead keeps a cell for it, and no cell inside
// the run lies on another edge's line.
func (bp *borderPorts) freePos(pts []Point, i int, vertical bool, line map[Point]bool, arrows [2]bool) (int, bool) {
	a := pts[i]
	minPrev, minNext := 1, 1
	if i == 1 && arrows[0] {
		minPrev = 2
	}
	if i == len(pts)-3 && arrows[1] {
		minNext = 2
	}
	natural := a.Col
	if !vertical {
		natural = a.Row
	}
	cell := func(pos int) int {
		if vertical {
			c, _ := bp.l.DrawToGrid(pos, a.Row)
			return c
		}
		_, r := bp.l.DrawToGrid(a.Col, pos)
		return r
	}
	home := cell(natural)
	prev, next := pts[i-1], pts[i+2]
	coord := func(p Point) int {
		if vertical {
			return p.Col
		}
		return p.Row
	}
	valid := func(pos int) bool {
		if cell(pos) != home {
			return false
		}
		if sign(pos-coord(prev)) != sign(natural-coord(prev)) || sign(coord(next)-pos) != sign(coord(next)-natural) {
			return false
		}
		if abs(pos-coord(prev)) < minPrev || abs(coord(next)-pos) < minNext {
			return false
		}
		na, nb := shifted(pts, i, vertical, pos)
		if !bp.sameSide(na, nb, vertical, pos, natural) {
			return false
		}
		hits := bp.hitsOn(na, nb)
		if len(hits) == 0 || !bp.free(hits) {
			return false
		}
		// The run's corners may join another edge's line; its interior
		// may not.
		d := Point{sign(nb.Col - na.Col), sign(nb.Row - na.Row)}
		for p := (Point{na.Col + d.Col, na.Row + d.Row}); p != nb; p = (Point{p.Col + d.Col, p.Row + d.Row}) {
			if line[p] {
				return false
			}
		}
		return true
	}
	for d := 1; d <= 16; d++ {
		for _, pos := range []int{natural - d, natural + d} {
			if valid(pos) {
				return pos, true
			}
		}
	}
	return natural, false
}

// route converts a grid path to draw coordinates with its ends at their
// ports, anchors a subgraph end on the subgraph's border, and settles the
// path's border crossings. When a run anchored at a node port meets a cell
// another edge holds, that end takes the next free port on its side and
// the path is converted again.
func (bp *borderPorts) route(e *edgeEnds, ends []*edgeEnds, l *layout.GridLayout, path []Point, srcSG, tgtSG *layout.SubgraphBounds, line map[Point]bool, arrows [2]bool) ([]Point, []Crossing) {
	tried := [2]map[int]bool{{}, {}}
	for {
		pts := drawPath(l, path, e.sides, e.ports)
		pts, ports := bp.anchor(pts, e, srcSG, tgtSG)
		pts, crossings, conflict := bp.cross(pts, line, arrows)
		which := -1
		switch conflict {
		case sourceRun:
			which = 0
		case targetRun:
			which = 1
		case bothAnchored:
			which = 0
			if srcSG != nil || len(tried[0]) > 0 {
				which = 1
			}
		}
		if which == 0 && srcSG != nil || which == 1 && tgtSG != nil {
			which = -1
		}
		if which >= 0 {
			if port, ok := nextPort(l, ends, e, which, tried[which]); ok {
				tried[which][port] = true
				e.ports[which] = port
				continue
			}
		}
		bp.take(crossings)
		return pts, append(ports, crossings...)
	}
}

// nextPort returns the free port on end which's side nearest its current
// port that no other edge end holds and this edge has not tried.
func nextPort(l *layout.GridLayout, ends []*edgeEnds, e *edgeEnds, which int, tried map[int]bool) (int, bool) {
	node, side := e.src, e.sides[0]
	if which == 1 {
		node, side = e.tgt, e.sides[1]
	}
	taken := make(map[int]bool)
	for _, o := range ends {
		if o == nil || o == e {
			continue
		}
		if o.src.NodeID == node.NodeID && o.sides[0] == side {
			taken[o.ports[0]] = true
		}
		if o.tgt.NodeID == node.NodeID && o.sides[1] == side {
			taken[o.ports[1]] = true
		}
	}
	lo, hi := l.PortRange(node, side)
	cur := e.ports[which]
	for d := 1; d <= hi-lo; d++ {
		for _, off := range []int{cur - d, cur + d} {
			if off >= lo && off <= hi && !taken[off] && !tried[off] {
				return off, true
			}
		}
	}
	return 0, false
}

// borderCell returns the cell on a subgraph box's side at coordinate along
// the side: the column for the top and bottom, the row for the left and
// right.
func borderCell(sb *layout.SubgraphBounds, side layout.Side, along int) Point {
	switch side {
	case layout.Top:
		return Point{along, sb.Y}
	case layout.Bottom:
		return Point{along, sb.Y + sb.Height - 1}
	case layout.Left:
		return Point{sb.X, along}
	default:
		return Point{sb.X + sb.Width - 1, along}
	}
}

// anchor moves an end that belongs to a subgraph onto the subgraph's
// border: the path's first point, which drawPath put at the port's
// position in the gap outside the box, is preceded by the border cell in
// line with it, and the last point likewise followed. The border cells are
// taken as ports of their sides and returned.
func (bp *borderPorts) anchor(pts []Point, ends *edgeEnds, srcSG, tgtSG *layout.SubgraphBounds) ([]Point, []Crossing) {
	var ports []Crossing
	if srcSG != nil && len(pts) >= 2 {
		side := ends.sides[0]
		along := pts[0].Row
		if !side.Vertical() {
			along = pts[0].Col
		}
		at := borderCell(srcSG, side, along)
		step := Point{sign(pts[0].Col - at.Col), sign(pts[0].Row - at.Row)}
		ports = append(ports, Crossing{srcSG.Subgraph.ID, side, at, step})
		pts = append([]Point{at}, pts...)
	}
	if tgtSG != nil && len(pts) >= 2 {
		side := ends.sides[1]
		last := pts[len(pts)-1]
		along := last.Row
		if !side.Vertical() {
			along = last.Col
		}
		at := borderCell(tgtSG, side, along)
		step := Point{sign(at.Col - last.Col), sign(at.Row - last.Row)}
		ports = append(ports, Crossing{tgtSG.Subgraph.ID, side, at, step})
		pts = append(pts, at)
	}
	bp.take(ports)
	return straighten(pts), ports
}
