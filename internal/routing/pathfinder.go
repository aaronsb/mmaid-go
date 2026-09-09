// Package routing provides A* pathfinding and edge routing for grid-based layouts.
package routing

import "container/heap"

// Point represents a grid coordinate (col, row).
type Point struct {
	Col, Row int
}

// dirs defines the 4-directional movement: up, down, left, right.
var dirs = [4]Point{{0, -1}, {0, 1}, {-1, 0}, {1, 0}}

// heuristic computes Manhattan distance with +1 corner penalty when not
// axis-aligned.
func heuristic(c1, r1, c2, r2 int) float64 {
	dx := abs(c1 - c2)
	dy := abs(r1 - r2)
	if dx == 0 || dy == 0 {
		return float64(dx + dy)
	}
	return float64(dx + dy + 1) // corner penalty
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// aStarNode is a node in the A* open set.
type aStarNode struct {
	fCost  float64
	gCost  float64
	col    int
	row    int
	parent *aStarNode
	index  int // index in the heap
}

// nodeHeap implements heap.Interface for A* nodes, ordered by fCost.
type nodeHeap []*aStarNode

func (h nodeHeap) Len() int            { return len(h) }
func (h nodeHeap) Less(i, j int) bool  { return h[i].fCost < h[j].fCost }
func (h nodeHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i]; h[i].index = i; h[j].index = j }
func (h *nodeHeap) Push(x interface{}) { n := x.(*aStarNode); n.index = len(*h); *h = append(*h, n) }
func (h *nodeHeap) Pop() interface{} {
	old := *h
	n := len(old)
	node := old[n-1]
	old[n-1] = nil // avoid memory leak
	node.index = -1
	*h = old[:n-1]
	return node
}

const defaultMaxIterations = 5000

// Step costs. A cell an earlier edge runs through along the same axis is a
// shared corridor; crossing it perpendicular is cheaper; a corner is
// cheaper still. Entering a cell on a subgraph border pays for the border.
const (
	CostStep     = 1.0
	CostShared   = 6.0
	CostCrossing = 3.0
	CostBorder   = 4.0
	CostCorner   = 0.5
)

// Axis is a set of the orientations an edge runs through a cell along.
type Axis uint8

const (
	AxisH Axis = 1 << iota
	AxisV
)

// axisOf returns the axis of a unit step.
func axisOf(d Point) Axis {
	if d.Row == 0 {
		return AxisH
	}
	return AxisV
}

// Obstacles is what earlier edges and the layout put in the way of a path.
// Soft maps a cell to the axes earlier edges run through it along; Border
// marks cells on a subgraph border. Either map may be nil.
type Obstacles struct {
	Soft   map[Point]Axis
	Border map[Point]bool
}

// stepCost is the cost of entering cell p along axis a; waived skips the
// charge for earlier edges through p.
func (o *Obstacles) stepCost(p Point, a Axis, waived bool) float64 {
	cost := CostStep
	if o == nil {
		return cost
	}
	if occ := o.Soft[p]; occ != 0 && !waived {
		if occ&a != 0 {
			cost += CostShared
		} else {
			cost += CostCrossing
		}
	}
	if o.Border[p] {
		cost += CostBorder
	}
	return cost
}

// FindPath finds a path from (startCol, startRow) to (endCol, endRow) using
// A* and returns it with its cost.
//
// isFree reports whether a grid cell is open to a path; the start and end
// cells are allowed regardless. The cells next to the start and end are
// where a node side's ports fan out, so earlier edges through them cost
// nothing: the first step out of the start, the step into the end, and the
// step into any free neighbour of the end. Returns nil when no path exists.
func FindPath(startCol, startRow, endCol, endRow int, isFree func(col, row int) bool, obs *Obstacles) ([]Point, float64) {
	if startCol == endCol && startRow == endRow {
		return []Point{{startCol, startRow}}, 0
	}

	start := Point{startCol, startRow}
	end := Point{endCol, endRow}
	endApron := make(map[Point]bool, 4)
	for _, d := range dirs {
		if p := (Point{endCol + d.Col, endRow + d.Row}); isFree(p.Col, p.Row) {
			endApron[p] = true
		}
	}
	waived := func(from, to Point) bool {
		return to == end || endApron[to] || from == start
	}

	startNode := &aStarNode{
		fCost: heuristic(startCol, startRow, endCol, endRow),
		gCost: 0,
		col:   startCol,
		row:   startRow,
	}

	openSet := &nodeHeap{startNode}
	heap.Init(openSet)

	closed := make(map[Point]bool)
	bestG := make(map[Point]float64)
	bestG[Point{startCol, startRow}] = 0

	iterations := 0
	for openSet.Len() > 0 && iterations < defaultMaxIterations {
		iterations++
		current := heap.Pop(openSet).(*aStarNode)

		if current.col == endCol && current.row == endRow {
			return reconstruct(current), current.gCost
		}

		key := Point{current.col, current.row}
		if closed[key] {
			continue
		}
		closed[key] = true

		for _, d := range dirs {
			nc, nr := current.col+d.Col, current.row+d.Row
			nkey := Point{nc, nr}

			if closed[nkey] {
				continue
			}

			// Allow start and end even if "occupied"
			isEndpoint := nc == endCol && nr == endRow
			if !isEndpoint && !isFree(nc, nr) {
				continue
			}

			stepCost := obs.stepCost(nkey, axisOf(d), waived(key, nkey))

			// Corner penalty: if direction changes from parent's direction
			if current.parent != nil {
				prevDC := current.col - current.parent.col
				prevDR := current.row - current.parent.row
				if d.Col != prevDC || d.Row != prevDR {
					stepCost += CostCorner
				}
			}

			newG := current.gCost + stepCost

			if prev, ok := bestG[nkey]; ok && prev <= newG {
				continue
			}
			bestG[nkey] = newG

			h := heuristic(nc, nr, endCol, endRow)
			neighbor := &aStarNode{
				fCost:  newG + h,
				gCost:  newG,
				col:    nc,
				row:    nr,
				parent: current,
			}
			heap.Push(openSet, neighbor)
		}
	}

	return nil, 0 // no path found
}

// Occupy records the axes a path runs through each of its cells along: the
// first cell gets the axis it leaves by, the last the axis it arrives by,
// and a corner both.
func Occupy(soft map[Point]Axis, path []Point) {
	for i, p := range path {
		if i > 0 {
			soft[p] |= axisOf(Point{p.Col - path[i-1].Col, p.Row - path[i-1].Row})
		}
		if i+1 < len(path) {
			soft[p] |= axisOf(Point{path[i+1].Col - p.Col, path[i+1].Row - p.Row})
		}
	}
}

// reconstruct walks parent pointers to build the path from start to end.
func reconstruct(node *aStarNode) []Point {
	var path []Point
	for n := node; n != nil; n = n.parent {
		path = append(path, Point{n.col, n.row})
	}
	// Reverse
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return path
}

// SimplifyPath removes collinear intermediate points, keeping only corners.
func SimplifyPath(path []Point) []Point {
	if len(path) <= 2 {
		return path
	}

	result := []Point{path[0]}
	for i := 1; i < len(path)-1; i++ {
		prev := path[i-1]
		curr := path[i]
		nxt := path[i+1]
		// Direction from prev to curr
		d1c := curr.Col - prev.Col
		d1r := curr.Row - prev.Row
		// Direction from curr to next
		d2c := nxt.Col - curr.Col
		d2r := nxt.Row - curr.Row
		if d1c != d2c || d1r != d2r {
			result = append(result, curr)
		}
	}
	result = append(result, path[len(path)-1])
	return result
}
