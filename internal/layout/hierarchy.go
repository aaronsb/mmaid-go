package layout

import (
	"slices"
	"sort"

	"github.com/aaronsb/mmaid-go/internal/graph"
)

// posRect is a block of positions on the layout grid, inclusive at both
// ends. Position p is the node at grid p*Stride+1.
type posRect struct {
	c0, r0, c1, r1 int
}

// block is a laid-out scope: the positions of its direct nodes and the
// offsets of its child blocks, relative to its own origin, with its extent.
type block struct {
	nodes    map[string]GridCoord
	children []childBlock
	w, h     int
}

type childBlock struct {
	sg *graph.Subgraph
	at GridCoord
	b  *block
}

// hierarchy walks the subgraph tree. Each subgraph collapses to one
// compound vertex in its parent's scope; edges between a member and the
// rest of the parent rewrite to the compound. The parent is layered and
// ordered as a flat graph; each subgraph is laid out in its own space with
// its own direction and its members take a run of consecutive layers in the
// parent's sequence starting at the compound's layer.
type hierarchy struct {
	g     *graph.Graph
	owner map[string]*graph.Subgraph
}

// localEdge is an edge between two vertices of one scope.
type localEdge struct {
	src, tgt string
	minLen   int
}

// localGraph is the vertices of one scope in definition order with the
// edges between them.
type localGraph struct {
	order []string
	edges []localEdge
}

// compoundKey names a subgraph's compound vertex; the prefix keeps it apart
// from any node ID.
func compoundKey(sg *graph.Subgraph) string {
	return "\x00sg:" + sg.ID
}

// layoutHierarchy places every node and returns each subgraph's block of
// positions.
func layoutHierarchy(g *graph.Graph) (map[string]GridCoord, map[*graph.Subgraph]posRect) {
	h := &hierarchy{g: g, owner: make(map[string]*graph.Subgraph, len(g.NodeOrder))}
	for _, nid := range g.NodeOrder {
		h.owner[nid] = g.FindSubgraphForNode(nid)
	}
	root := h.layoutScope(nil, g.Direction)

	positions := make(map[string]GridCoord, len(g.NodeOrder))
	rects := make(map[*graph.Subgraph]posRect)
	flattenBlock(root, GridCoord{}, positions, rects)
	return positions, rects
}

// flattenBlock records the position of every node a block holds and the rect
// of every block nested in it, each relative to origin.
func flattenBlock(b *block, origin GridCoord, positions map[string]GridCoord, rects map[*graph.Subgraph]posRect) {
	for nid, p := range b.nodes {
		positions[nid] = GridCoord{origin.Col + p.Col, origin.Row + p.Row}
	}
	for _, cb := range b.children {
		at := GridCoord{origin.Col + cb.at.Col, origin.Row + cb.at.Row}
		rects[cb.sg] = posRect{at.Col, at.Row, at.Col + cb.b.w - 1, at.Row + cb.b.h - 1}
		flattenBlock(cb.b, at, positions, rects)
	}
}

// rep maps an edge endpoint to its vertex in scope: the node itself when it
// is a direct member, the compound of the child subgraph that holds it, or
// nothing when it lies outside the scope. A subgraph endpoint maps to the
// child compound that holds the subgraph.
func (h *hierarchy) rep(scope *graph.Subgraph, id string, isSubgraph bool) (string, bool) {
	var sg *graph.Subgraph
	if isSubgraph {
		sg = h.g.FindSubgraphByID(id)
		if sg == nil || sg == scope {
			return "", false
		}
	} else {
		if _, ok := h.g.Nodes[id]; !ok {
			return "", false
		}
		sg = h.owner[id]
		if sg == scope {
			return id, true
		}
		if sg == nil {
			return "", false
		}
	}
	for ; sg != nil; sg = sg.Parent {
		if sg.Parent == scope {
			return compoundKey(sg), true
		}
	}
	return "", false
}

// local builds the scope's graph: its direct nodes and one compound per
// child subgraph that holds any node, in the order their first node appears
// in the graph, with the edges between them. An edge inside one child or to
// a subgraph with no node is dropped; a plain self-edge stays.
func (h *hierarchy) local(scope *graph.Subgraph) (*localGraph, map[string]*graph.Subgraph) {
	lg := &localGraph{}
	compounds := make(map[string]*graph.Subgraph)
	seen := make(map[string]bool)
	for _, nid := range h.g.NodeOrder {
		v, ok := h.rep(scope, nid, false)
		if !ok || seen[v] {
			continue
		}
		seen[v] = true
		lg.order = append(lg.order, v)
		if v != nid {
			sg := h.owner[nid]
			for sg.Parent != scope {
				sg = sg.Parent
			}
			compounds[v] = sg
		}
	}
	for _, e := range h.g.Edges {
		s, ok1 := h.rep(scope, e.Source, e.SourceIsSubgraph)
		t, ok2 := h.rep(scope, e.Target, e.TargetIsSubgraph)
		if !ok1 || !ok2 || !seen[s] || !seen[t] {
			continue
		}
		if s == t && compounds[s] != nil {
			continue
		}
		lg.edges = append(lg.edges, localEdge{s, t, e.MinLength})
	}
	return lg, compounds
}

// layoutScope lays out one scope in direction dir and returns its block.
func (h *hierarchy) layoutScope(scope *graph.Subgraph, dir graph.Direction) *block {
	lg, compounds := h.local(scope)
	b := &block{nodes: make(map[string]GridCoord)}
	if len(lg.order) == 0 {
		return b
	}

	blocks := make(map[string]*block, len(compounds))
	for v, sg := range compounds {
		d := dir
		if sg.Direction != nil {
			d = *sg.Direction
		}
		blocks[v] = h.layoutScope(sg, d)
	}

	vertical := dir.Normalized().IsVertical()
	span := func(cb *block) int {
		if vertical {
			return cb.h
		}
		return cb.w
	}
	across := func(cb *block) int {
		if vertical {
			return cb.w
		}
		return cb.h
	}
	place := func(flow, cross int) GridCoord {
		if vertical {
			return GridCoord{Col: cross, Row: flow}
		}
		return GridCoord{Col: flow, Row: cross}
	}

	layers := assignLayers(lg)
	flow, crossMax := 0, 0
	for _, layer := range orderLayers(lg, layers) {
		depth, cross := 1, 0
		for _, v := range layer {
			if cb, ok := blocks[v]; ok {
				b.children = append(b.children, childBlock{sg: compounds[v], at: place(flow, cross), b: cb})
				cross += across(cb)
				depth = max(depth, span(cb))
				continue
			}
			b.nodes[v] = place(flow, cross)
			cross++
		}
		crossMax = max(crossMax, cross)
		flow += depth
	}
	if vertical {
		b.w, b.h = crossMax, flow
	} else {
		b.w, b.h = flow, crossMax
	}

	// The canvas flips the whole diagram for a reversed graph direction,
	// so a block is reversed in its own space only where its direction and
	// the graph's disagree on its axis.
	if scope != nil {
		if vertical && (dir == graph.DirBT) != (h.g.Direction == graph.DirBT) {
			b.flip(true)
		}
		if !vertical && (dir == graph.DirRL) != (h.g.Direction == graph.DirRL) {
			b.flip(false)
		}
	}
	return b
}

// flip reverses the block's rows, or its columns, in its own space; child
// blocks move but keep their own arrangement.
func (b *block) flip(rows bool) {
	for nid, p := range b.nodes {
		if rows {
			p.Row = b.h - 1 - p.Row
		} else {
			p.Col = b.w - 1 - p.Col
		}
		b.nodes[nid] = p
	}
	for i, cb := range b.children {
		if rows {
			b.children[i].at.Row = b.h - cb.at.Row - cb.b.h
		} else {
			b.children[i].at.Col = b.w - cb.at.Col - cb.b.w
		}
	}
}

// roots returns the vertices with no incoming edge, in order; the first
// vertex when there is none.
func (lg *localGraph) roots() []string {
	targets := make(map[string]bool, len(lg.edges))
	for _, e := range lg.edges {
		targets[e.tgt] = true
	}
	var roots []string
	for _, v := range lg.order {
		if !targets[v] {
			roots = append(roots, v)
		}
	}
	if len(roots) == 0 && len(lg.order) > 0 {
		return []string{lg.order[0]}
	}
	return roots
}

// children returns the targets of a vertex's outgoing edges, in edge order,
// without duplicates or the vertex itself.
func (lg *localGraph) children(v string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, e := range lg.edges {
		if e.src == v && e.tgt != v && !seen[e.tgt] {
			seen[e.tgt] = true
			out = append(out, e.tgt)
		}
	}
	return out
}

// assignLayers assigns each vertex a layer by longest path from a root over
// the BFS tree edges, so back-edges do not lengthen the diagram.
func assignLayers(lg *localGraph) map[string]int {
	layers := make(map[string]int)
	roots := lg.roots()
	for _, root := range roots {
		layers[root] = 0
	}

	// BFS discovers each vertex at its shallowest depth, so an edge into a
	// vertex already reached by a shorter path is a cross-edge.
	treeEdges := make(map[edgeKey]bool)
	visited := make(map[string]bool)
	var queue []string
	bfs := func() {
		for len(queue) > 0 {
			v := queue[0]
			queue = queue[1:]
			for _, c := range lg.children(v) {
				if !visited[c] {
					visited[c] = true
					treeEdges[edgeKey{v, c}] = true
					queue = append(queue, c)
				}
			}
		}
	}
	for _, root := range roots {
		if !visited[root] {
			visited[root] = true
			queue = append(queue, root)
		}
	}
	bfs()
	for _, v := range lg.order {
		if !visited[v] {
			visited[v] = true
			queue = append(queue, v)
			bfs()
		}
	}

	minLen := make(map[edgeKey]int)
	for _, e := range lg.edges {
		k := edgeKey{e.src, e.tgt}
		if cur, ok := minLen[k]; !ok || e.minLen > cur {
			minLen[k] = e.minLen
		}
	}

	changed := true
	for iter := 0; changed && iter < len(lg.order)*2; iter++ {
		changed = false
		for ek := range treeEdges {
			srcLayer, ok := layers[ek.src]
			if !ok {
				continue
			}
			ml := 1
			if v, ok := minLen[ek]; ok {
				ml = v
			}
			if tgtLayer, ok := layers[ek.tgt]; !ok || tgtLayer < srcLayer+ml {
				layers[ek.tgt] = srcLayer + ml
				changed = true
			}
		}
	}

	for _, v := range lg.order {
		if _, ok := layers[v]; !ok {
			layers[v] = 0
		}
	}
	return layers
}

// countCrossings counts the edge crossings between adjacent layers.
func countCrossings(lg *localGraph, layerLists [][]string) int {
	total := 0
	for li := 1; li < len(layerLists); li++ {
		prevPos := make(map[string]int)
		for i, v := range layerLists[li-1] {
			prevPos[v] = i
		}
		curPos := make(map[string]int)
		for i, v := range layerLists[li] {
			curPos[v] = i
		}
		type pair struct{ u, v int }
		var between []pair
		for _, e := range lg.edges {
			u, ok1 := prevPos[e.src]
			v, ok2 := curPos[e.tgt]
			if ok1 && ok2 {
				between = append(between, pair{u, v})
			}
		}
		for i := 0; i < len(between); i++ {
			for j := i + 1; j < len(between); j++ {
				if (between[i].u-between[j].u)*(between[i].v-between[j].v) < 0 {
					total++
				}
			}
		}
	}
	return total
}

// orderLayers orders the vertices within each layer by the barycenter
// heuristic, keeping the ordering with the fewest crossings.
func orderLayers(lg *localGraph, layers map[string]int) [][]string {
	maxLayer := 0
	for _, l := range layers {
		maxLayer = max(maxLayer, l)
	}
	layerLists := make([][]string, maxLayer+1)
	for i := range layerLists {
		layerLists[i] = []string{}
	}
	for _, v := range lg.order {
		l := layers[v]
		layerLists[l] = append(layerLists[l], v)
	}

	bestCrossings := countCrossings(lg, layerLists)
	bestOrdering := copyLayers(layerLists)
	noImprovement := 0

	for pass := 0; pass < 8; pass++ {
		for li := 1; li < len(layerLists); li++ {
			prevPositions := make(map[string]int)
			for i, v := range layerLists[li-1] {
				prevPositions[v] = i
			}
			barycenters := make(map[string]float64)
			for _, v := range layerLists[li] {
				sum, n := 0, 0
				for _, e := range lg.edges {
					if e.tgt == v {
						if pos, ok := prevPositions[e.src]; ok {
							sum += pos
							n++
						}
					}
				}
				if n > 0 {
					barycenters[v] = float64(sum) / float64(n)
				} else {
					barycenters[v] = float64(slices.Index(layerLists[li], v))
				}
			}
			sort.SliceStable(layerLists[li], func(i, j int) bool {
				return barycenters[layerLists[li][i]] < barycenters[layerLists[li][j]]
			})
		}

		crossings := countCrossings(lg, layerLists)
		if crossings < bestCrossings {
			bestCrossings = crossings
			bestOrdering = copyLayers(layerLists)
			noImprovement = 0
		} else {
			noImprovement++
		}
		if noImprovement >= 4 || bestCrossings == 0 {
			break
		}
	}
	return bestOrdering
}

// placeNodes records each node's grid cell and reserves its 3x3 block.
func placeNodes(layout *GridLayout, positions map[string]GridCoord) {
	for nid, p := range positions {
		gc := GridCoord{Col: p.Col*Stride + 1, Row: p.Row*Stride + 1}
		layout.Placements[nid] = &NodePlacement{NodeID: nid, Grid: gc, Min: gc, Max: gc}
		for dc := -1; dc <= 1; dc++ {
			for dr := -1; dr <= 1; dr++ {
				layout.GridOccupied[GridCoord{gc.Col + dc, gc.Row + dr}] = nid
			}
		}
	}
}
