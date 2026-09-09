package routing

import (
	"testing"

	"github.com/aaronsb/mmaid-go/internal/graph"
	"github.com/aaronsb/mmaid-go/internal/layout"
	"github.com/aaronsb/mmaid-go/internal/parser"
)

// gapLayout is a layout with two narrow columns, one 20-cell column, and
// one box whose bottom border a vertical run in the wide column crosses.
func gapLayout() *layout.GridLayout {
	l := layout.NewGridLayout()
	for c := range 10 {
		l.ColWidths[c] = 4
	}
	l.ColWidths[2] = 20
	for r := range 10 {
		l.RowHeights[r] = 3
	}
	l.SubgraphBounds = []layout.SubgraphBounds{{
		Subgraph: &graph.Subgraph{ID: "S"}, X: 10, Y: 0, Width: 11, Height: 9,
	}}
	return l
}

func TestFreePosSkipsTakenAndBorderCells(t *testing.T) {
	bp := newBorderPorts(gapLayout())
	pts := []Point{{5, 4}, {11, 4}, {11, 12}, {20, 12}}
	hits := bp.hitsOn(pts[1], pts[2])
	if len(hits) != 1 || hits[0].At != (Point{11, 8}) || hits[0].Side != layout.Bottom {
		t.Fatalf("hits %+v", hits)
	}
	bp.take(hits)
	bp.take([]Crossing{{Subgraph: "S", Side: layout.Bottom, At: Point{12, 8}}})

	pos, ok := bp.freePos(pts, 1, true, nil, [2]bool{})
	// 10 is the box's left border line, 12 is taken, 9 is beyond the line.
	if !ok || pos != 13 {
		t.Errorf("freePos = %d, %v; want 13", pos, ok)
	}

	// A line of another edge in the way pushes the run further.
	line := map[Point]bool{{13, 6}: true}
	if pos, ok := bp.freePos(pts, 1, true, line, [2]bool{}); !ok || pos != 14 {
		t.Errorf("freePos over a line = %d, %v; want 14", pos, ok)
	}

	// A neighbouring run that ends in an arrowhead keeps two cells: with
	// the source port at x=12 the run cannot move to 13.
	near := []Point{{12, 4}, {11, 4}, {11, 12}, {20, 12}}
	if pos, ok := bp.freePos(near, 1, true, nil, [2]bool{true, true}); ok && pos == 13 {
		t.Errorf("freePos = %d next to the previous corner", pos)
	}
}

func TestCrossReportsAnchoredConflicts(t *testing.T) {
	bp := newBorderPorts(gapLayout())
	bp.take([]Crossing{{Subgraph: "S", Side: layout.Bottom, At: Point{11, 8}}})

	_, _, conflict := bp.cross([]Point{{11, 4}, {11, 12}}, nil, [2]bool{})
	if conflict != bothAnchored {
		t.Errorf("one run: conflict %d, want bothAnchored", conflict)
	}
	_, _, conflict = bp.cross([]Point{{11, 4}, {11, 12}, {20, 12}}, nil, [2]bool{})
	if conflict != sourceRun {
		t.Errorf("first run: conflict %d, want sourceRun", conflict)
	}
	_, _, conflict = bp.cross([]Point{{5, 12}, {11, 12}, {11, 4}}, nil, [2]bool{})
	if conflict != targetRun {
		t.Errorf("last run: conflict %d, want targetRun", conflict)
	}
	// A run between two turns moves; the first run, inside the box, meets
	// no border.
	pts, crossings, conflict := bp.cross([]Point{{16, 4}, {11, 4}, {11, 12}, {20, 12}}, nil, [2]bool{})
	if conflict != noConflict || len(crossings) != 1 || crossings[0].At.Col == 11 || pts[1].Col != crossings[0].At.Col {
		t.Errorf("movable run: conflict %d, crossings %+v, path %v", conflict, crossings, pts)
	}
}

// route parses and routes a flowchart.
func routeGraph(t *testing.T, src string) (*graph.Graph, *layout.GridLayout, []RoutedEdge) {
	t.Helper()
	g := parser.ParseFlowchart(src)
	l := layout.ComputeLayout(g, 4, 2, 0)
	return g, l, RouteEdgesWith(g, l, Ellipsis, nil)
}

// bordersOf indexes a layout's boxes by subgraph ID.
func bordersOf(l *layout.GridLayout) map[string]layout.SubgraphBounds {
	out := map[string]layout.SubgraphBounds{}
	for _, sb := range l.SubgraphBounds {
		out[sb.Subgraph.ID] = sb
	}
	return out
}

func onSide(sb layout.SubgraphBounds, side layout.Side, p Point) bool {
	switch side {
	case layout.Top:
		return p.Row == sb.Y && p.Col > sb.X && p.Col < sb.X+sb.Width-1
	case layout.Bottom:
		return p.Row == sb.Y+sb.Height-1 && p.Col > sb.X && p.Col < sb.X+sb.Width-1
	case layout.Left:
		return p.Col == sb.X && p.Row > sb.Y && p.Row < sb.Y+sb.Height-1
	default:
		return p.Col == sb.X+sb.Width-1 && p.Row > sb.Y && p.Row < sb.Y+sb.Height-1
	}
}

func TestSubgraphEndsAnchorOnDistinctBorderPorts(t *testing.T) {
	_, l, routed := routeGraph(t, `graph TB
    A --> S1
    subgraph S1
        B --> C
    end
    S1 --> D
    S1 --> E`)
	sb := bordersOf(l)["S1"]
	onBorder := func(p Point) bool {
		for _, side := range []layout.Side{layout.Top, layout.Bottom, layout.Left, layout.Right} {
			if onSide(sb, side, p) {
				return true
			}
		}
		return false
	}
	starts := map[Point]bool{}
	for _, re := range routed {
		p := re.DrawPath
		switch {
		case re.Edge.SourceIsSubgraph:
			if !onBorder(p[0]) {
				t.Errorf("%s->%s starts at %v, not on S1's border %+v", re.Edge.Source, re.Edge.Target, p[0], sb)
			}
			if starts[p[0]] {
				t.Errorf("two edges leave S1 through %v", p[0])
			}
			starts[p[0]] = true
		case re.Edge.TargetIsSubgraph:
			if !onSide(sb, layout.Top, p[len(p)-1]) {
				t.Errorf("%s->%s ends at %v, not on S1's top %+v", re.Edge.Source, re.Edge.Target, p[len(p)-1], sb)
			}
		}
	}
	if len(starts) != 2 {
		t.Errorf("%d distinct start ports, want 2", len(starts))
	}
}

func TestEdgeToOwnSubgraphLoopsAtTheNode(t *testing.T) {
	_, l, routed := routeGraph(t, `graph LR
    subgraph S1
        B
    end
    B --> S1
    A --> B`)
	b := l.Placements["B"]
	for _, re := range routed {
		if !re.Edge.TargetIsSubgraph {
			continue
		}
		p := re.DrawPath
		if p[0].Row != b.DrawY || p[len(p)-1].Col != b.DrawX+b.DrawWidth-1 {
			t.Errorf("B->S1 does not loop from B's top to its right side: %v, B %+v", p, b)
		}
	}
}

func TestCrossingsAreDistinctPerSide(t *testing.T) {
	for name, src := range map[string]string{
		"contested-port": `graph LR
    subgraph S0
        C
        E
    end
    subgraph S1
        A
        F
        D
    end
    subgraph S2
        direction TB
        H
        G
        B
    end
    C --> A
    B --> S1
    D --> G
    G --> H
    H --> C
    C --> F`,
		"siblings": `graph LR
    subgraph Input
        A --> B
    end
    subgraph Process
        C --> D
    end
    B --> C
    A --> D
    C --> B`,
	} {
		_, _, routed := routeGraph(t, src)
		seen := map[sideKey]map[int]int{}
		for _, re := range routed {
			for _, c := range re.Crossings {
				k := sideKey{c.Subgraph, c.Side}
				if seen[k] == nil {
					seen[k] = map[int]int{}
				}
				if prev, ok := seen[k][c.along()]; ok && prev != re.Index {
					t.Errorf("%s: edges %d and %d both cross %s's %v side at %d", name, prev, re.Index, c.Subgraph, c.Side, c.along())
				}
				seen[k][c.along()] = re.Index
			}
		}
	}
}
