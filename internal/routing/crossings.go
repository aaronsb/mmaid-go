package routing

import (
	"sort"

	"github.com/aaronsb/mmaid-go/internal/layout"
)

// Crossing is where an edge's draw path passes through a subgraph border:
// the border cell, the subgraph, and the unit step the edge takes through
// the cell toward its target. The renderer breaks the line one cell before
// the border and restarts it at the border cell, so the border resolves to
// a tee whose stem points the way the edge is going.
type Crossing struct {
	Subgraph string
	At       Point
	Step     Point
}

// sideKey names one side of one subgraph's box.
type sideKey struct {
	sg   string
	side layout.Side
}

// hit is one border cell a run passes through.
type hit struct {
	key sideKey
	at  Point
}

// borderPorts hands out the cells of each subgraph side that edges cross
// at. A crossing takes the cell where the run naturally meets the border;
// when another edge holds it, the run moves to the nearest free cell in
// the same gap that keeps its neighbours' directions and runs along no
// other edge's line.
type borderPorts struct {
	l     *layout.GridLayout
	taken map[sideKey]map[int]bool
}

func newBorderPorts(l *layout.GridLayout) *borderPorts {
	return &borderPorts{l: l, taken: make(map[sideKey]map[int]bool)}
}

// hitsOn returns the border cells the run a-b passes through, between the
// box's corners.
func (bp *borderPorts) hitsOn(a, b Point) []hit {
	var out []hit
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
				out = append(out, hit{sideKey{id, layout.Top}, Point{x, y0}})
			}
			if lo < y1 && y1 < hi {
				out = append(out, hit{sideKey{id, layout.Bottom}, Point{x, y1}})
			}
		case a.Row == b.Row:
			y := a.Row
			if y <= y0 || y >= y1 {
				continue
			}
			lo, hi := min(a.Col, b.Col), max(a.Col, b.Col)
			if lo < x0 && x0 < hi {
				out = append(out, hit{sideKey{id, layout.Left}, Point{x0, y}})
			}
			if lo < x1 && x1 < hi {
				out = append(out, hit{sideKey{id, layout.Right}, Point{x1, y}})
			}
		}
	}
	return out
}

// along returns a hit's coordinate along its side.
func (h hit) along() int {
	if h.key.side.Vertical() {
		return h.at.Row
	}
	return h.at.Col
}

func (bp *borderPorts) free(hits []hit) bool {
	for _, h := range hits {
		if bp.taken[h.key][h.along()] {
			return false
		}
	}
	return true
}

func (bp *borderPorts) take(h hit) {
	if bp.taken[h.key] == nil {
		bp.taken[h.key] = make(map[int]bool)
	}
	bp.taken[h.key][h.along()] = true
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

// cross finds the borders the path crosses, moves each crossing run to a
// free cell of every side it meets, and returns the path with its
// crossings in path order. A run that starts or ends at a node port cannot
// move and takes its cell whether or not another edge holds it.
func (bp *borderPorts) cross(pts []Point, line map[Point]bool) ([]Point, []Crossing) {
	var out []Crossing
	for i := 0; i+1 < len(pts); i++ {
		hits := bp.hitsOn(pts[i], pts[i+1])
		if len(hits) == 0 {
			continue
		}
		vertical := pts[i].Col == pts[i+1].Col
		if i >= 1 && i+2 < len(pts) && !bp.free(hits) {
			if pos, ok := bp.freePos(pts, i, vertical, line); ok {
				pts[i], pts[i+1] = shifted(pts, i, vertical, pos)
				hits = bp.hitsOn(pts[i], pts[i+1])
			}
		}
		step := Point{sign(pts[i+1].Col - pts[i].Col), sign(pts[i+1].Row - pts[i].Row)}
		sort.SliceStable(hits, func(x, y int) bool {
			hx, hy := hits[x].at, hits[y].at
			return (hx.Col-hy.Col)*step.Col+(hx.Row-hy.Row)*step.Row < 0
		})
		for _, h := range hits {
			bp.take(h)
			out = append(out, Crossing{Subgraph: h.key.sg, At: h.at, Step: step})
		}
	}
	return pts, out
}

// freePos searches outward from run i's position for one where every side
// the run crosses is free, the run stays in its grid cell, its neighbours
// keep their directions, and no cell of it lies on another edge's line.
func (bp *borderPorts) freePos(pts []Point, i int, vertical bool, line map[Point]bool) (int, bool) {
	a := pts[i]
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
		na, nb := shifted(pts, i, vertical, pos)
		hits := bp.hitsOn(na, nb)
		if len(hits) == 0 || !bp.free(hits) {
			return false
		}
		d := Point{sign(nb.Col - na.Col), sign(nb.Row - na.Row)}
		for p := na; ; p = (Point{p.Col + d.Col, p.Row + d.Row}) {
			if line[p] {
				return false
			}
			if p == nb {
				break
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

// runIndex returns the index of the run of pts that passes through p.
func runIndex(pts []Point, p Point) int {
	for i := 0; i+1 < len(pts); i++ {
		a, b := pts[i], pts[i+1]
		if a.Col == b.Col && p.Col == a.Col && p.Row > min(a.Row, b.Row) && p.Row < max(a.Row, b.Row) {
			return i
		}
		if a.Row == b.Row && p.Row == a.Row && p.Col > min(a.Col, b.Col) && p.Col < max(a.Col, b.Col) {
			return i
		}
	}
	return -1
}

// cutAtOwnBorder trims an edge that starts or ends at a subgraph to the
// crossing of that subgraph's border: a source edge starts at the border
// cell it leaves through, a target edge ends at the cell it enters through,
// with its arrowhead just outside.
func cutAtOwnBorder(re *RoutedEdge) {
	if re.Edge.SourceIsSubgraph {
		for k, c := range re.Crossings {
			if c.Subgraph != re.Edge.Source {
				continue
			}
			if i := runIndex(re.DrawPath, c.At); i >= 0 {
				re.DrawPath = append([]Point{c.At}, re.DrawPath[i+1:]...)
				re.Crossings = re.Crossings[k+1:]
			}
			break
		}
	}
	if re.Edge.TargetIsSubgraph {
		for k := len(re.Crossings) - 1; k >= 0; k-- {
			c := re.Crossings[k]
			if c.Subgraph != re.Edge.Target {
				continue
			}
			if i := runIndex(re.DrawPath, c.At); i >= 0 {
				re.DrawPath = append(re.DrawPath[:i+1:i+1], c.At)
				re.Crossings = re.Crossings[:k]
			}
			break
		}
	}
}
