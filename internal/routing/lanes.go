package routing

import (
	"sort"

	"github.com/aaronsb/mmaid-go/internal/layout"
)

// A gap row or column is one grid cell across, so two edges routed through
// it along the same axis lie on its centre line for as long as they share
// it. Gap lanes spread them: every run between two turns takes an offset
// from its gap's centre line that no run it overlaps takes, assigned the
// way ports are assigned along a node side.

// laneRun is one straight run of an edge's draw points.
type laneRun struct {
	edge, i  int
	vertical bool
	// movable runs lie between two turns; the first and last runs are
	// anchored at ports and keep their place.
	movable bool
	key     int
	idx     int
	done    bool
}

// gapKey names one gap column or row.
type gapKey struct {
	vertical bool
	idx      int
}

// gapLanes returns, per edge and per run of its draw points, the offset
// the run takes from its gap's centre line. Anchored runs and runs
// outside a gap take 0. blocked holds the cells no line may cross: node
// rectangles, subgraph borders and titles.
func gapLanes(l *layout.GridLayout, ends []*edgeEnds, paths [][]Point, arrows [][2]bool, blocked map[Point]bool) [][]int {
	occRows, occCols := make(map[int]bool), make(map[int]bool)
	for gc := range l.GridOccupied {
		occRows[gc.Row] = true
		occCols[gc.Col] = true
	}
	pts := make([][]Point, len(paths))
	lanes := make([][]int, len(paths))
	arrowAt := make(map[Point]bool)
	for i, e := range ends {
		if e == nil || len(paths[i]) < 2 {
			continue
		}
		pts[i] = drawPoints(l, paths[i], e.sides, e.ports)
		if len(pts[i]) < 2 {
			continue
		}
		lanes[i] = make([]int, len(pts[i])-1)
		for _, p := range arrowCellsOf(pts[i], arrows[i]) {
			arrowAt[p] = true
		}
	}

	groups := make(map[gapKey][]*laneRun)
	var order []gapKey
	for i := range pts {
		for r := 0; r+1 < len(pts[i]); r++ {
			a, b := pts[i][r], pts[i][r+1]
			if a == b {
				continue
			}
			run := &laneRun{edge: i, i: r, vertical: a.Col == b.Col, movable: r >= 1 && r+2 < len(pts[i])}
			var k gapKey
			if run.vertical {
				c, _ := l.DrawToGrid(a.Col, a.Row)
				if occCols[c] {
					continue
				}
				k = gapKey{true, c}
			} else {
				_, r := l.DrawToGrid(a.Col, a.Row)
				if occRows[r] {
					continue
				}
				k = gapKey{false, r}
			}
			if _, ok := groups[k]; !ok {
				order = append(order, k)
			}
			groups[k] = append(groups[k], run)
		}
	}
	sort.Slice(order, func(a, b int) bool {
		if order[a].vertical != order[b].vertical {
			return !order[a].vertical
		}
		return order[a].idx < order[b].idx
	})
	g := &laneGroups{l: l, pts: pts, lanes: lanes, arrows: arrows, arrowAt: arrowAt, blocked: blocked}
	for _, k := range order {
		g.assign(k, groups[k])
	}
	return lanes
}

// laneGroups is the state lane assignment reads and writes: the draw
// points move as lanes are applied, so every extent and position is read
// from them live.
type laneGroups struct {
	l       *layout.GridLayout
	pts     [][]Point
	lanes   [][]int
	arrows  [][2]bool
	arrowAt map[Point]bool
	blocked map[Point]bool
}

func cross(p Point, vertical bool) int {
	if vertical {
		return p.Col
	}
	return p.Row
}

func along(p Point, vertical bool) int {
	if vertical {
		return p.Row
	}
	return p.Col
}

// extent returns a run's span along its axis, inclusive.
func (g *laneGroups) extent(r *laneRun) (lo, hi int) {
	p := g.pts[r.edge]
	a, b := along(p[r.i], r.vertical), along(p[r.i+1], r.vertical)
	return min(a, b), max(a, b)
}

// pos returns a run's position across its axis.
func (g *laneGroups) pos(r *laneRun) int {
	return cross(g.pts[r.edge][r.i], r.vertical)
}

func (g *laneGroups) overlap(a, b *laneRun) bool {
	alo, ahi := g.extent(a)
	blo, bhi := g.extent(b)
	return alo <= bhi && blo <= ahi
}

// assign gives the movable runs of one gap their lanes. The runs are
// sorted by where their ends turn, each takes the lane after every sorted
// run it overlaps, and each connected cluster is centred on the gap's
// centre line. A lane a run cannot take falls back to the nearest lane
// from the centre outward that it can, and to the centre when there is
// none.
func (g *laneGroups) assign(k gapKey, runs []*laneRun) {
	lo, hi := g.l.LaneRange(k.idx, k.vertical)
	if lo == hi {
		return
	}
	var movable []*laneRun
	for _, r := range runs {
		if r.movable {
			r.key = g.key(r)
			movable = append(movable, r)
		}
	}
	sort.SliceStable(movable, func(a, b int) bool { return movable[a].key < movable[b].key })

	// Lane indices, then clusters of overlapping runs and their depth.
	parent := make([]int, len(movable))
	for i := range parent {
		parent[i] = i
	}
	root := func(i int) int {
		for parent[i] != i {
			parent[i] = parent[parent[i]]
			i = parent[i]
		}
		return i
	}
	for a, r := range movable {
		for b, q := range movable[:a] {
			if g.overlap(r, q) {
				r.idx = max(r.idx, q.idx+1)
				parent[root(a)] = root(b)
			}
		}
	}
	depth := make(map[int]int)
	for i, r := range movable {
		depth[root(i)] = max(depth[root(i)], r.idx+1)
	}

	for i, r := range movable {
		want := r.idx - depth[root(i)]/2
		chosen := 0
		for _, off := range append([]int{want}, centreOut(lo, hi)...) {
			if off >= lo && off <= hi && g.valid(r, off) && !g.taken(runs, r, off) {
				chosen = off
				break
			}
		}
		g.apply(r, chosen)
	}
}

// centreOut lists the offsets lo..hi from the centre outward.
func centreOut(lo, hi int) []int {
	out := []int{0}
	for d := 1; -d >= lo || d <= hi; d++ {
		if -d >= lo {
			out = append(out, -d)
		}
		if d <= hi {
			out = append(out, d)
		}
	}
	return out
}

// key orders the movable runs of a gap from its near side (up, or left)
// to its far side: runs whose ends both turn toward the near side first,
// mixed runs next, runs whose ends both turn toward the far side last.
// Among two cups the narrower sits nearer; among two caps the narrower
// sits farther; among two staircases the one whose near-side end lies
// inside the other's extent sits nearer. Each rule keeps a run's turn out
// of the run beside it.
func (g *laneGroups) key(r *laneRun) int {
	p := g.pts[r.edge]
	nat := cross(p[r.i], r.vertical)
	prev, next := cross(p[r.i-1], r.vertical), cross(p[r.i+2], r.vertical)
	score := 0
	for _, c := range []int{prev, next} {
		switch {
		case c < nat:
			score--
		case c > nat:
			score++
		}
	}
	lo, hi := g.extent(r)
	var tie int
	switch {
	case score < 0:
		tie = hi - lo
	case score > 0:
		tie = lo - hi
	default:
		near, far := along(p[r.i], r.vertical), along(p[r.i+1], r.vertical)
		if prev > nat {
			near, far = far, near
		}
		if near < far {
			tie = -near
		} else {
			tie = near
		}
	}
	return score<<20 + tie
}

// valid reports whether a run may lie at offset off from its centre line:
// its neighbouring runs keep their directions and at least one cell, two
// when the neighbour ends in an arrowhead; it crosses no arrowhead and no
// blocked cell; and it stays on its side of every subgraph border.
func (g *laneGroups) valid(r *laneRun, off int) bool {
	p := g.pts[r.edge]
	nat := g.pos(r)
	pos := nat + off
	prev, next := cross(p[r.i-1], r.vertical), cross(p[r.i+2], r.vertical)
	minPrev, minNext := 1, 1
	if r.i == 1 && g.arrows[r.edge][0] {
		minPrev = 2
	}
	if r.i+2 == len(p)-1 && g.arrows[r.edge][1] {
		minNext = 2
	}
	if sign(pos-prev) != sign(nat-prev) || sign(next-pos) != sign(next-nat) {
		return false
	}
	if abs(pos-prev) < minPrev || abs(next-pos) < minNext {
		return false
	}
	lo, hi := g.extent(r)
	for v := lo; v <= hi; v++ {
		c := Point{v, pos}
		if r.vertical {
			c = Point{pos, v}
		}
		if g.arrowAt[c] || g.blocked[c] {
			return false
		}
	}
	a, b := shifted(p, r.i, r.vertical, pos)
	return sameSide(g.l, a, b, r.vertical, pos, nat)
}

// taken reports whether a run at offset off would lie on another run of
// its gap that overlaps it: an anchored run, or one already placed.
func (g *laneGroups) taken(runs []*laneRun, r *laneRun, off int) bool {
	pos := g.pos(r) + off
	for _, q := range runs {
		if q == r || (q.movable && !q.done) || q.edge == r.edge {
			continue
		}
		if g.pos(q) == pos && g.overlap(r, q) {
			return true
		}
	}
	return false
}

// apply moves a run to its lane.
func (g *laneGroups) apply(r *laneRun, off int) {
	p := g.pts[r.edge]
	p[r.i], p[r.i+1] = shifted(p, r.i, r.vertical, g.pos(r)+off)
	g.lanes[r.edge][r.i] = off
	r.done = true
}

// arrowCellsOf returns the cells a path's arrowheads occupy: one step in
// from each end that has one.
func arrowCellsOf(p []Point, arrows [2]bool) []Point {
	n := len(p)
	if n < 2 {
		return nil
	}
	var out []Point
	if arrows[1] {
		out = append(out, stepToward(p[n-1], p[n-2]))
	}
	if arrows[0] {
		out = append(out, stepToward(p[0], p[1]))
	}
	return out
}
