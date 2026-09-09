package layout

import (
	"sort"

	"github.com/aaronsb/mmaid-go/internal/graph"
)

// Side is a side of a node an edge attaches to.
type Side int

const (
	Top Side = iota
	Bottom
	Left
	Right
)

// Vertical reports whether the side runs vertically, so its ports spread
// along rows.
func (s Side) Vertical() bool {
	return s == Left || s == Right
}

// AttachCell returns the grid cell on a node's border for a side.
func (s Side) AttachCell(gc GridCoord) GridCoord {
	switch s {
	case Top:
		return GridCoord{Col: gc.Col, Row: gc.Row - 1}
	case Bottom:
		return GridCoord{Col: gc.Col, Row: gc.Row + 1}
	case Left:
		return GridCoord{Col: gc.Col - 1, Row: gc.Row}
	default:
		return GridCoord{Col: gc.Col + 1, Row: gc.Row}
	}
}

// Attach returns the grid cell an edge attaches to a placement through on
// a side: the border cell of a node, or the gap cell just outside a
// subgraph's block at the middle of that side.
func (p *NodePlacement) Attach(side Side) GridCoord {
	mid := GridCoord{Col: (p.Min.Col + p.Max.Col) / 2, Row: (p.Min.Row + p.Max.Row) / 2}
	mid.Col -= (mid.Col - 1) % Stride
	mid.Row -= (mid.Row - 1) % Stride
	out := 1
	switch {
	case p.Block:
		out = 2
	case p.Point:
		// A one-cell node is its own attach cell: the arms of the edges
		// that reach it resolve there to a tee or a cross.
		return mid
	}
	switch side {
	case Top:
		return GridCoord{Col: mid.Col, Row: p.Min.Row - out}
	case Bottom:
		return GridCoord{Col: mid.Col, Row: p.Max.Row + out}
	case Left:
		return GridCoord{Col: p.Min.Col - out, Row: mid.Row}
	default:
		return GridCoord{Col: p.Max.Col + out, Row: mid.Row}
	}
}

// PreferredSides returns the pair of sides an edge from src to tgt prefers to
// leave and enter through, and the alternative pair, from their relative
// grid positions and the flow direction.
func PreferredSides(src, tgt GridCoord, direction graph.Direction) (preferred, alt [2]Side) {
	sc, sr := src.Col, src.Row
	tc, tr := tgt.Col, tgt.Row

	if direction.IsHorizontal() {
		switch {
		case tc > sc:
			preferred = [2]Side{Right, Left}
		case tc < sc:
			// Back-edge: leave from the bottom to stay clear of the
			// back-edges entering at the top.
			return [2]Side{Bottom, Bottom}, [2]Side{Bottom, Top}
		case tr > sr:
			preferred = [2]Side{Bottom, Top}
		default:
			preferred = [2]Side{Top, Bottom}
		}
		switch {
		case tr > sr:
			alt = [2]Side{Bottom, Top}
		case tr < sr:
			alt = [2]Side{Top, Bottom}
		default:
			alt = preferred
		}
		return preferred, alt
	}

	switch {
	case tr > sr:
		preferred = [2]Side{Bottom, Top}
	case tr < sr:
		// Back-edge: leave from the right to stay clear of the back-edges
		// entering at the left.
		return [2]Side{Right, Right}, [2]Side{Right, Left}
	case tc > sc:
		preferred = [2]Side{Right, Left}
	default:
		preferred = [2]Side{Left, Right}
	}
	switch {
	case tc > sc:
		alt = [2]Side{Right, Left}
	case tc < sc:
		alt = [2]Side{Left, Right}
	default:
		alt = preferred
	}
	return preferred, alt
}

// PortRange returns the offsets, in draw cells from the side's centre, at
// which a port may sit: every border cell between the corners.
func (l *GridLayout) PortRange(p *NodePlacement, side Side) (lo, hi int) {
	ac := p.Attach(side)
	cx, cy := l.GridToDrawCenter(ac.Col, ac.Row)
	if side.Vertical() {
		return p.DrawY + 1 - cy, p.DrawY + p.DrawHeight - 2 - cy
	}
	return p.DrawX + 1 - cx, p.DrawX + p.DrawWidth - 2 - cx
}

// LaneRange returns the offsets, in draw cells from the centre line of a
// grid column (vertical) or row, at which a run along it stays inside it.
func (l *GridLayout) LaneRange(idx int, vertical bool) (lo, hi int) {
	sizes := l.RowHeights
	if vertical {
		sizes = l.ColWidths
	}
	size := 1
	if s, ok := sizes[idx]; ok {
		size = s
	}
	return -(size / 2), size - 1 - size/2
}

// PortRequest is one edge end on a node side. Other is the other endpoint's
// centre along the side's axis, in draw cells.
type PortRequest struct {
	Node  string
	Side  Side
	Other int
}

// AssignPorts returns a port offset for each request. The requests on one
// side are sorted by Other; the one nearest the side's centre takes the
// centre port and the rest spread outward in sorted order. A block that
// runs past a corner slides back inside; when the side has fewer ports than
// requests, the requests past the corners take the centre.
func (l *GridLayout) AssignPorts(reqs []PortRequest) []int {
	return l.AssignPortsWith(reqs, func(node string) *NodePlacement { return l.Placements[node] })
}

// AssignPortsWith is AssignPorts with the placement each request's node
// resolves through, so a subgraph an edge ends at can offer ports along
// its border; a nil placement takes the centre.
func (l *GridLayout) AssignPortsWith(reqs []PortRequest, placement func(node string) *NodePlacement) []int {
	type key struct {
		node string
		side Side
	}
	groups := make(map[key][]int)
	var order []key
	for i, r := range reqs {
		k := key{r.Node, r.Side}
		if _, ok := groups[k]; !ok {
			order = append(order, k)
		}
		groups[k] = append(groups[k], i)
	}

	out := make([]int, len(reqs))
	for _, k := range order {
		idx := groups[k]
		p := placement(k.node)
		if p == nil || len(idx) == 1 {
			continue
		}
		sort.SliceStable(idx, func(a, b int) bool {
			return reqs[idx[a]].Other < reqs[idx[b]].Other
		})

		ac := p.Attach(k.side)
		cx, cy := l.GridToDrawCenter(ac.Col, ac.Row)
		centre := cx
		if k.side.Vertical() {
			centre = cy
		}
		nearest := 0
		for i, ri := range idx {
			if abs(reqs[ri].Other-centre) < abs(reqs[idx[nearest]].Other-centre) {
				nearest = i
			}
		}

		lo, hi := l.PortRange(p, k.side)
		n := len(idx)
		shift := 0
		if n <= hi-lo+1 {
			if -nearest < lo {
				shift = lo + nearest
			} else if n-1-nearest > hi {
				shift = hi - (n - 1 - nearest)
			}
		}
		for i, ri := range idx {
			off := i - nearest + shift
			if off < lo || off > hi {
				off = 0
			}
			out[ri] = off
		}
	}
	return out
}

// Reserve marks a grid cell as an obstacle for later edges.
func (l *GridLayout) Reserve(col, row int) {
	if l.Reserved == nil {
		l.Reserved = make(map[GridCoord]bool)
	}
	l.Reserved[GridCoord{Col: col, Row: row}] = true
}

// ReserveDraw reserves every grid cell whose draw rectangle meets the draw
// rectangle (x0, y0)-(x1, y1), inclusive.
func (l *GridLayout) ReserveDraw(x0, y0, x1, y1 int) {
	c0, r0 := l.DrawToGrid(x0, y0)
	c1, r1 := l.DrawToGrid(x1, y1)
	for c := c0; c <= c1; c++ {
		for r := r0; r <= r1; r++ {
			l.Reserve(c, r)
		}
	}
}

// DrawToGrid returns the grid cell containing a draw coordinate. Coordinates
// before the layout's offset map to -1.
func (l *GridLayout) DrawToGrid(x, y int) (col, row int) {
	return l.axisToGrid(x-l.OffsetX, l.ColWidths), l.axisToGrid(y-l.OffsetY, l.RowHeights)
}

func (l *GridLayout) axisToGrid(v int, sizes map[int]int) int {
	if v < 0 {
		return -1
	}
	acc := 0
	for i := 0; ; i++ {
		size := 1
		if s, ok := sizes[i]; ok {
			size = s
		}
		if v < acc+size {
			return i
		}
		acc += size
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// countPorts records, per node and side, the edge ends the preferred sides
// would put there. Routing may choose the alternative side for an edge, so
// the count is the estimate node sizing works from. A subgraph end stands
// in for one of its nodes and takes no port itself.
func countPorts(g *graph.Graph, layout *GridLayout) {
	direction := g.Direction.Normalized()
	stand := func(id string, isSubgraph bool) *NodePlacement {
		if !isSubgraph {
			return layout.Placements[id]
		}
		if sg := g.FindSubgraphByID(id); sg != nil {
			for nid := range gatherIDs(sg) {
				if p, ok := layout.Placements[nid]; ok {
					return p
				}
			}
		}
		return nil
	}
	for _, e := range g.Edges {
		src := stand(e.Source, e.SourceIsSubgraph)
		tgt := stand(e.Target, e.TargetIsSubgraph)
		if src == nil || tgt == nil {
			continue
		}
		if e.IsSelfReference() && !e.SourceIsSubgraph {
			src.PortCount[Top]++
			src.PortCount[Right]++
			continue
		}
		pref, _ := PreferredSides(src.Grid, tgt.Grid, direction)
		if !e.SourceIsSubgraph {
			src.PortCount[pref[0]]++
		}
		if !e.TargetIsSubgraph {
			tgt.PortCount[pref[1]]++
		}
	}
}

// gatherIDs returns every node in a subgraph and its children.
func gatherIDs(sg *graph.Subgraph) map[string]bool {
	out := make(map[string]bool)
	gatherAllNodes(sg, out)
	return out
}
