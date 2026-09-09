package layout

import (
	"fmt"
	"strings"

	"github.com/aaronsb/mmaid-go/internal/graph"
)

// laneVertex is one vertex of the lane scope: a node its lane holds
// directly, or the compound of a subgraph nested in a lane.
type laneVertex struct {
	id   string
	sg   *graph.Subgraph // non-nil when id names a compound
	lane *graph.Subgraph // nil for a node no lane declared
}

// laneRep maps an edge endpoint to its vertex in the lane scope: the node
// itself when a lane holds it directly or no lane holds it, and the compound
// of the nested subgraph a lane holds that encloses it otherwise. A lane
// itself is not a vertex, and neither is an unknown endpoint.
func (h *hierarchy) laneRep(id string, isSubgraph bool) (laneVertex, bool) {
	var sg *graph.Subgraph
	if isSubgraph {
		sg = h.g.FindSubgraphByID(id)
		if sg == nil || sg.Parent == nil {
			return laneVertex{}, false
		}
	} else {
		if _, ok := h.g.Nodes[id]; !ok {
			return laneVertex{}, false
		}
		sg = h.owner[id]
		if sg == nil {
			return laneVertex{id: id}, true
		}
		if sg.Parent == nil {
			return laneVertex{id: id, lane: sg}, true
		}
	}
	for ; sg.Parent != nil; sg = sg.Parent {
		if sg.Parent.Parent == nil {
			return laneVertex{id: compoundKey(sg), sg: sg, lane: sg.Parent}, true
		}
	}
	return laneVertex{}, false
}

// layoutLanes lays the whole graph out as one scope: layering and ordering
// run in flow order over every vertex, so a node's layer is its position
// along the flow axis whatever lane holds it, and each vertex's cross-axis
// position is constrained to its lane's band. Lanes are the top-level
// subgraphs in declaration order, each band as wide as its widest layer
// needs; within a lane at one layer the vertices keep the barycenter order.
// A subgraph nested in a lane collapses to a compound vertex ADR-103's
// recursion lays out inside the band. It returns the placed positions, the
// block of every lane and nested subgraph, the lanes whose boxes are
// computed from the top, and the nodes no lane declared.
func layoutLanes(g *graph.Graph) (map[string]GridCoord, map[*graph.Subgraph]posRect, []*graph.Subgraph, []string) {
	h := &hierarchy{g: g, owner: make(map[string]*graph.Subgraph, len(g.NodeOrder))}
	for _, nid := range g.NodeOrder {
		h.owner[nid] = g.FindSubgraphForNode(nid)
	}

	lg := &localGraph{}
	vertices := make(map[string]laneVertex, len(g.NodeOrder))
	var unlaned []string
	for _, nid := range g.NodeOrder {
		lv, ok := h.laneRep(nid, false)
		if !ok {
			continue
		}
		if lv.lane == nil {
			unlaned = append(unlaned, nid)
		}
		if _, seen := vertices[lv.id]; seen {
			continue
		}
		vertices[lv.id] = lv
		lg.order = append(lg.order, lv.id)
	}
	for _, e := range g.Edges {
		s, ok1 := h.laneRep(e.Source, e.SourceIsSubgraph)
		t, ok2 := h.laneRep(e.Target, e.TargetIsSubgraph)
		if !ok1 || !ok2 {
			continue
		}
		if _, ok := vertices[s.id]; !ok {
			continue
		}
		if _, ok := vertices[t.id]; !ok {
			continue
		}
		if s.id == t.id && s.sg != nil {
			continue
		}
		lg.edges = append(lg.edges, localEdge{s.id, t.id, e.MinLength})
	}

	// The lanes in declaration order, with a trailing unlabelled one for
	// what no lane declared.
	lanes := make([]*graph.Subgraph, 0, len(g.Subgraphs)+1)
	lanes = append(lanes, g.Subgraphs...)
	var rest *graph.Subgraph
	if len(unlaned) > 0 {
		rest = &graph.Subgraph{NodeIDs: unlaned}
		lanes = append(lanes, rest)
	}
	index := make(map[*graph.Subgraph]int, len(lanes))
	for i, lane := range lanes {
		index[lane] = i
	}
	laneOf := func(id string) int {
		if lv := vertices[id]; lv.lane != nil {
			return index[lv.lane]
		}
		return index[rest]
	}

	blocks := make(map[string]*block)
	for _, lv := range vertices {
		if lv.sg == nil {
			continue
		}
		d := g.Direction
		if lv.sg.Direction != nil {
			d = *lv.sg.Direction
		}
		blocks[lv.id] = h.layoutScope(lv.sg, d)
	}

	vertical := g.Direction.Normalized().IsVertical()
	span := func(id string) int {
		b, ok := blocks[id]
		switch {
		case !ok:
			return 1
		case vertical:
			return b.h
		default:
			return b.w
		}
	}
	across := func(id string) int {
		b, ok := blocks[id]
		switch {
		case !ok:
			return 1
		case vertical:
			return b.w
		default:
			return b.h
		}
	}
	place := func(flow, cross int) GridCoord {
		if vertical {
			return GridCoord{Col: cross, Row: flow}
		}
		return GridCoord{Col: flow, Row: cross}
	}

	ordered := orderLayers(lg, assignLayers(lg))

	bands := make([]int, len(lanes))
	for _, layer := range ordered {
		need := make([]int, len(lanes))
		for _, id := range layer {
			need[laneOf(id)] += across(id)
		}
		for i, n := range need {
			bands[i] = max(bands[i], n)
		}
	}
	offsets := make([]int, len(lanes))
	cross := 0
	for i, w := range bands {
		offsets[i] = cross
		cross += w
	}

	root := &block{nodes: make(map[string]GridCoord)}
	flow := 0
	for _, layer := range ordered {
		depth := 1
		cursor := make([]int, len(lanes))
		for _, id := range layer {
			li := laneOf(id)
			at := place(flow, offsets[li]+cursor[li])
			cursor[li] += across(id)
			if b, ok := blocks[id]; ok {
				root.children = append(root.children, childBlock{sg: vertices[id].sg, at: at, b: b})
				depth = max(depth, span(id))
				continue
			}
			root.nodes[id] = at
		}
		flow += depth
	}
	if vertical {
		root.w, root.h = cross, flow
	} else {
		root.w, root.h = flow, cross
	}

	positions := make(map[string]GridCoord, len(g.NodeOrder))
	rects := make(map[*graph.Subgraph]posRect)
	flattenBlock(root, GridCoord{}, positions, rects)

	// A lane spans every layer on the flow axis and its band on the cross
	// axis. A lane with no vertex has no band and so no box.
	for i, lane := range lanes {
		if bands[i] == 0 {
			continue
		}
		lo, hi := offsets[i], offsets[i]+bands[i]-1
		if vertical {
			rects[lane] = posRect{lo, 0, hi, flow - 1}
		} else {
			rects[lane] = posRect{0, lo, flow - 1, hi}
		}
	}
	return positions, rects, lanes, unlaned
}

// unlanedWarning is what the layout reports for nodes no lane declared.
func unlanedWarning(ids []string) string {
	return fmt.Sprintf("mmaid: swimlane: %s outside every lane; placed in a trailing lane of their own",
		strings.Join(ids, ", "))
}

// rootSubgraphs returns the subgraphs whose boxes are computed from the top:
// on the lane path the lanes, the trailing one included; g.Subgraphs
// otherwise.
func (l *GridLayout) rootSubgraphs(g *graph.Graph) []*graph.Subgraph {
	if g.Lanes {
		return l.laneRoots
	}
	return g.Subgraphs
}
