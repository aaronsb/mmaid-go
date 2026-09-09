package routing

import (
	"io"
	"os"

	"github.com/aaronsb/mmaid-go/internal/graph"
	"github.com/aaronsb/mmaid-go/internal/layout"
)

// AttachDir is the side of a node an edge attaches to.
type AttachDir = layout.Side

const (
	AttachTop    = layout.Top
	AttachBottom = layout.Bottom
	AttachLeft   = layout.Left
	AttachRight  = layout.Right
)

// Ellipsis is what a label shrinks to after its first word when the whole
// label does not fit.
const Ellipsis = "…"

// RoutedEdge is an edge with its computed path in grid and drawing
// coordinates and the placement of its label.
type RoutedEdge struct {
	Edge     graph.Edge
	GridPath []Point
	DrawPath []Point
	StartDir AttachDir
	EndDir   AttachDir
	// StartPort and EndPort are the ends' offsets along their sides, in
	// draw cells from the side's centre.
	StartPort int
	EndPort   int
	Label     string
	// LabelText is what the renderer draws at (LabelRow, LabelCol): the
	// label, its first word with an ellipsis, or "" when nothing fit.
	LabelText     string
	LabelRow      int
	LabelCol      int
	Index         int
	OccupiedCells map[Point]bool
	// Crossings are the subgraph borders the draw path passes through, in
	// path order.
	Crossings []Crossing
}

// edgeEnds is what routing resolved for one edge of the graph.
type edgeEnds struct {
	src, tgt *layout.NodePlacement
	self     bool
	sides    [2]AttachDir
	ports    [2]int
}

// RouteEdges routes all edges in the graph using the computed layout. Labels
// shrink to their first word and Ellipsis when they do not fit; a label that
// fits nowhere is dropped with a warning on stderr.
func RouteEdges(g *graph.Graph, l *layout.GridLayout) []RoutedEdge {
	return RouteEdgesWith(g, l, Ellipsis, os.Stderr)
}

// RouteEdgesWith is RouteEdges with the ellipsis a shrunk label ends in and
// the writer a dropped label is reported to (nil for none).
//
// Routing takes three passes over the edges. The first chooses each edge's
// sides on the bare grid; ports are then assigned per side. The second
// routes each edge through its chosen sides, moves each subgraph border
// crossing to a free cell of the border, places its label, and reserves
// the label's cells against the edges after it. The third assigns gap lanes
// from every grid path, then converts each path again with its runs in
// their lanes and places every label again. A line is never drawn under a
// label: a label a later path runs under is placed again, or dropped.
func RouteEdgesWith(g *graph.Graph, l *layout.GridLayout, ellipsis string, warn io.Writer) []RoutedEdge {
	direction := g.Direction.Normalized()
	l.Reserved = nil

	sgBounds := make(map[string]*layout.SubgraphBounds, len(l.SubgraphBounds))
	for i := range l.SubgraphBounds {
		sb := &l.SubgraphBounds[i]
		sgBounds[sb.Subgraph.ID] = sb
	}
	border := subgraphBorderCells(l)

	// inside reports whether a node lies in a subgraph or one of its
	// descendants.
	inside := func(sgID, nodeID string) bool {
		for sg := g.FindSubgraphForNode(nodeID); sg != nil; sg = sg.Parent {
			if sg.ID == sgID {
				return true
			}
		}
		return false
	}
	virtual := make(map[string]*layout.NodePlacement)
	ends := make([]*edgeEnds, len(g.Edges))
	for i, edge := range g.Edges {
		src := resolvePlacement(edge.Source, edge.SourceIsSubgraph, l, sgBounds, virtual)
		tgt := resolvePlacement(edge.Target, edge.TargetIsSubgraph, l, sgBounds, virtual)
		if src == nil || tgt == nil {
			continue
		}
		// An edge between a node and the subgraph that holds it loops at
		// the node.
		if edge.SourceIsSubgraph && !edge.TargetIsSubgraph && inside(edge.Source, edge.Target) {
			src = tgt
		}
		if edge.TargetIsSubgraph && !edge.SourceIsSubgraph && inside(edge.Target, edge.Source) {
			tgt = src
		}
		ends[i] = &edgeEnds{src: src, tgt: tgt, self: src == tgt && !src.Block}
	}
	placement := func(node string) *layout.NodePlacement {
		if p, ok := virtual[node]; ok {
			return p
		}
		return l.Placements[node]
	}

	// Pass 1: sides.
	soft := make(map[Point]Axis)
	free := func(c, r int) bool { return l.IsFree(c, r, nil) }
	for _, e := range ends {
		if e == nil {
			continue
		}
		if e.self {
			e.sides = [2]AttachDir{AttachTop, AttachRight}
			Occupy(soft, selfPath(e.src))
			continue
		}
		path, sides := choosePath(e.src, e.tgt, direction, free, &Obstacles{Soft: soft, Border: border})
		e.sides = sides
		Occupy(soft, path)
	}

	// Ports.
	var reqs []layout.PortRequest
	var reqEnd []*edgeEnds
	var reqWhich []int
	for _, e := range ends {
		if e == nil {
			continue
		}
		reqs = append(reqs, layout.PortRequest{Node: e.src.NodeID, Side: e.sides[0], Other: centreAlong(e.tgt, e.sides[0])})
		reqEnd, reqWhich = append(reqEnd, e), append(reqWhich, 0)
		reqs = append(reqs, layout.PortRequest{Node: e.tgt.NodeID, Side: e.sides[1], Other: centreAlong(e.src, e.sides[1])})
		reqEnd, reqWhich = append(reqEnd, e), append(reqWhich, 1)
	}
	for i, port := range l.AssignPortsWith(reqs, placement) {
		reqEnd[i].ports[reqWhich[i]] = port
	}

	// finish converts edge i's grid path to its draw path and places its
	// label, appending to routed.
	apr := aprons(g, ends, direction)
	paths := make([][]Point, len(g.Edges))
	arrows := make([][2]bool, len(g.Edges))
	for i, edge := range g.Edges {
		arrows[i] = [2]bool{edge.HasArrowStart, edge.HasArrowEnd}
	}
	var routed []RoutedEdge
	finish := func(i int, space *labelSpace, borders *borderPorts, lanes []int) {
		e, edge, path := ends[i], g.Edges[i], paths[i]
		var srcSG, tgtSG *layout.SubgraphBounds
		if e.src.Block {
			srcSG = sgBounds[edge.Source]
		}
		if e.tgt.Block {
			tgtSG = sgBounds[edge.Target]
		}
		dp, crossings := borders.route(e, ends, l, path, srcSG, tgtSG, space.lines, arrows[i], lanes)

		re := RoutedEdge{
			Edge:          edge,
			GridPath:      SimplifyPath(path),
			DrawPath:      dp,
			Crossings:     crossings,
			StartDir:      e.sides[0],
			EndDir:        e.sides[1],
			StartPort:     e.ports[0],
			EndPort:       e.ports[1],
			Label:         edge.Label,
			Index:         i,
			OccupiedCells: make(map[Point]bool, len(path)),
		}
		for _, p := range path {
			re.OccupiedCells[p] = true
		}
		displaced := space.addLines(re.DrawPath)
		space.place(&re, ellipsis)
		routed = append(routed, re)
		for _, idx := range displaced {
			for j := range routed {
				if routed[j].Index == idx {
					space.place(&routed[j], ellipsis)
				}
			}
		}
	}

	// Pass 2: paths, with each label placed so the edges after it route
	// around it. These draw paths and labels are provisional.
	soft = make(map[Point]Axis)
	space := newLabelSpace(g, l, apr, nil)
	borders := newBorderPorts(l)
	for i := range g.Edges {
		e := ends[i]
		if e == nil {
			continue
		}
		if e.self {
			paths[i] = selfPath(e.src)
		} else {
			paths[i] = routeThrough(e, ends, l, direction, &Obstacles{Soft: soft, Border: border})
		}
		Occupy(soft, paths[i])
		finish(i, space, borders, nil)
	}

	// Pass 3: gap lanes, then the draw paths and labels again.
	l.Reserved = nil
	space = newLabelSpace(g, l, apr, warn)
	lanes := gapLanes(l, ends, paths, arrows, space.blocked)
	borders = newBorderPorts(l)
	routed = nil
	for i := range g.Edges {
		if ends[i] == nil {
			continue
		}
		finish(i, space, borders, lanes[i])
	}

	return routed
}

// aprons returns the grid cells a node side's ports step into, for every
// side an edge may use.
func aprons(g *graph.Graph, ends []*edgeEnds, direction graph.Direction) map[layout.GridCoord]bool {
	out := make(map[layout.GridCoord]bool)
	add := func(p *layout.NodePlacement, side AttachDir) {
		out[side.AttachCell(p.Attach(side))] = true
	}
	for i, e := range ends {
		if e == nil {
			continue
		}
		if e.self {
			add(e.src, AttachTop)
			add(e.src, AttachRight)
			continue
		}
		pref, alt := layout.PreferredSides(e.src.Grid, e.tgt.Grid, direction)
		for _, pair := range [][2]AttachDir{pref, alt} {
			if !g.Edges[i].SourceIsSubgraph {
				add(e.src, pair[0])
			}
			if !g.Edges[i].TargetIsSubgraph {
				add(e.tgt, pair[1])
			}
		}
	}
	return out
}

// centreAlong returns a placement's centre along the axis a side runs
// along, in draw cells.
func centreAlong(p *layout.NodePlacement, side AttachDir) int {
	if side.Vertical() {
		return p.DrawY + p.DrawHeight/2
	}
	return p.DrawX + p.DrawWidth/2
}

// choosePath routes an edge through its preferred and alternative side
// pairs and returns the cheaper path with the pair it used. With no path
// either way it returns a direct line through the preferred pair.
func choosePath(
	src, tgt *layout.NodePlacement,
	direction graph.Direction,
	free func(c, r int) bool,
	obs *Obstacles,
) ([]Point, [2]AttachDir) {
	pref, alt := layout.PreferredSides(src.Grid, tgt.Grid, direction)
	pathPref, costPref := findBetween(src, tgt, pref, free, obs)
	pathAlt, costAlt := findBetween(src, tgt, alt, free, obs)

	switch {
	case pathPref != nil && (pathAlt == nil || costPref <= costAlt):
		return pathPref, pref
	case pathAlt != nil:
		return pathAlt, alt
	default:
		return []Point{attachPoint(src, pref[0]), attachPoint(tgt, pref[1])}, pref
	}
}

// routeThrough routes an edge through the sides pass 1 chose, with the
// labels placed so far as hard obstacles. If those sides no longer connect
// it tries the other pair, taking the free port nearest each side's centre.
// With no label-free path either way it routes through the labels, which
// then move.
func routeThrough(e *edgeEnds, ends []*edgeEnds, l *layout.GridLayout, direction graph.Direction, obs *Obstacles) []Point {
	free := func(c, r int) bool { return l.IsFree(c, r, nil) }
	if path, _ := findBetween(e.src, e.tgt, e.sides, free, obs); path != nil {
		return path
	}
	pref, alt := layout.PreferredSides(e.src.Grid, e.tgt.Grid, direction)
	other := alt
	if e.sides == alt {
		other = pref
	}
	if other != e.sides {
		if path, _ := findBetween(e.src, e.tgt, other, free, obs); path != nil {
			e.sides = other
			e.ports = [2]int{freePort(l, ends, e, 0), freePort(l, ends, e, 1)}
			return path
		}
	}
	ignoreLabels := func(c, r int) bool {
		_, node := l.GridOccupied[layout.GridCoord{Col: c, Row: r}]
		return c >= 0 && r >= 0 && !node
	}
	if path, _ := findBetween(e.src, e.tgt, e.sides, ignoreLabels, obs); path != nil {
		return path
	}
	return []Point{attachPoint(e.src, e.sides[0]), attachPoint(e.tgt, e.sides[1])}
}

// freePort returns the port nearest the centre of end which of e's side
// that no other edge end on that side holds; the centre when the side is
// full.
func freePort(l *layout.GridLayout, ends []*edgeEnds, e *edgeEnds, which int) int {
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
	for d := 0; d <= max(-lo, hi); d++ {
		for _, off := range []int{-d, d} {
			if off >= lo && off <= hi && !taken[off] {
				return off
			}
		}
	}
	return 0
}

func findBetween(src, tgt *layout.NodePlacement, sides [2]AttachDir, free func(c, r int) bool, obs *Obstacles) ([]Point, float64) {
	s := attachPoint(src, sides[0])
	t := attachPoint(tgt, sides[1])
	return FindPath(s.Col, s.Row, t.Col, t.Row, free, obs)
}

// attachPoint returns the grid cell an edge attaches to a placement
// through on a side.
func attachPoint(p *layout.NodePlacement, side AttachDir) Point {
	gc := p.Attach(side)
	return Point{gc.Col, gc.Row}
}

// selfPath is the grid loop a self-referencing edge draws: out of the top,
// over to the right, and back into the right side.
func selfPath(src *layout.NodePlacement) []Point {
	gc := src.Grid
	return []Point{
		{gc.Col, gc.Row - 1},
		{gc.Col, gc.Row - 2},
		{gc.Col + 2, gc.Row - 2},
		{gc.Col + 2, gc.Row},
		{gc.Col + 1, gc.Row},
	}
}

// drawPath converts a grid path to draw coordinates: the points of
// drawPoints, each run moved to its gap lane, and straight runs joined.
// lanes holds an offset per run of the points, or is nil.
func drawPath(l *layout.GridLayout, path []Point, sides [2]AttachDir, ports [2]int, lanes []int) []Point {
	pts := drawPoints(l, path, sides, ports)
	for i, off := range lanes {
		if off == 0 || i+1 >= len(pts) {
			continue
		}
		vertical := pts[i].Col == pts[i+1].Col
		pts[i], pts[i+1] = shifted(pts, i, vertical, cross(pts[i], vertical)+off)
	}
	return straighten(pts)
}

// drawPoints converts a grid path to one draw point per turn. Each end sits
// at its port on the node border; the first and last runs carry the port's
// offset to their first turn, and a path with no turn jogs between its two
// ports on the centre line of its first gap cell.
func drawPoints(l *layout.GridLayout, path []Point, sides [2]AttachDir, ports [2]int) []Point {
	simp := SimplifyPath(path)
	pts := make([]Point, len(simp))
	for i, p := range simp {
		x, y := l.GridToDrawCenter(p.Col, p.Row)
		pts[i] = Point{x, y}
	}
	if len(pts) < 2 {
		return pts
	}
	m := len(pts) - 1
	shift(&pts[0], sides[0], ports[0])
	shift(&pts[m], sides[1], ports[1])

	if m == 1 {
		var corr Point
		if len(path) >= 3 {
			x, y := l.GridToDrawCenter(path[1].Col, path[1].Row)
			corr = Point{x, y}
		} else {
			corr = Point{(pts[0].Col + pts[1].Col) / 2, (pts[0].Row + pts[1].Row) / 2}
		}
		var j1, j2 Point
		if path[1].Row == path[0].Row {
			j1, j2 = Point{corr.Col, pts[0].Row}, Point{corr.Col, pts[1].Row}
		} else {
			j1, j2 = Point{pts[0].Col, corr.Row}, Point{pts[1].Col, corr.Row}
		}
		return []Point{pts[0], j1, j2, pts[1]}
	}
	if simp[1].Row == simp[0].Row {
		pts[1].Row = pts[0].Row
	} else {
		pts[1].Col = pts[0].Col
	}
	if simp[m].Row == simp[m-1].Row {
		pts[m-1].Row = pts[m].Row
	} else {
		pts[m-1].Col = pts[m].Col
	}
	return pts
}

// straighten drops repeated points and points inside a straight run. Unlike
// SimplifyPath it compares directions, since draw runs step by more than
// one cell.
func straighten(pts []Point) []Point {
	var out []Point
	for _, p := range pts {
		n := len(out)
		if n > 0 && p == out[n-1] {
			continue
		}
		if n > 1 {
			a, b := out[n-2], out[n-1]
			if sign(b.Col-a.Col) == sign(p.Col-b.Col) && sign(b.Row-a.Row) == sign(p.Row-b.Row) {
				out[n-1] = p
				continue
			}
		}
		out = append(out, p)
	}
	return out
}

// shift moves a draw point along its side by a port offset.
func shift(p *Point, side AttachDir, port int) {
	if side.Vertical() {
		p.Row += port
	} else {
		p.Col += port
	}
}

// subgraphBorderCells returns the grid cells a subgraph border runs
// through.
func subgraphBorderCells(l *layout.GridLayout) map[Point]bool {
	if len(l.SubgraphBounds) == 0 {
		return nil
	}
	cells := make(map[Point]bool)
	mark := func(x, y int) {
		c, r := l.DrawToGrid(x, y)
		cells[Point{c, r}] = true
	}
	for _, sb := range l.SubgraphBounds {
		if sb.Width <= 0 || sb.Height <= 0 {
			continue
		}
		x1, y1 := sb.X+sb.Width-1, sb.Y+sb.Height-1
		for x := sb.X; x <= x1; x++ {
			mark(x, sb.Y)
			mark(x, y1)
		}
		for y := sb.Y; y <= y1; y++ {
			mark(sb.X, y)
			mark(x1, y)
		}
	}
	return cells
}

// resolvePlacement resolves a node or subgraph ID to a placement. A
// subgraph gets one placement covering its block, with its box as the draw
// rectangle, shared by every edge that ends at it so that their ports are
// assigned together.
func resolvePlacement(
	nodeID string,
	isSubgraph bool,
	l *layout.GridLayout,
	sgBounds map[string]*layout.SubgraphBounds,
	virtual map[string]*layout.NodePlacement,
) *layout.NodePlacement {
	if !isSubgraph {
		return l.Placements[nodeID]
	}
	if p, ok := virtual[nodeID]; ok {
		return p
	}
	sb, ok := sgBounds[nodeID]
	if !ok {
		return nil
	}
	lo, hi, ok := l.Block(sb.Subgraph)
	if !ok {
		return nil
	}
	p := &layout.NodePlacement{
		NodeID:     nodeID,
		Grid:       layout.GridCoord{Col: (lo.Col + hi.Col) / 2, Row: (lo.Row + hi.Row) / 2},
		Min:        lo,
		Max:        hi,
		Block:      true,
		DrawX:      sb.X,
		DrawY:      sb.Y,
		DrawWidth:  sb.Width,
		DrawHeight: sb.Height,
	}
	virtual[nodeID] = p
	return p
}
