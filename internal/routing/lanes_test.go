package routing

import (
	"os"
	"testing"

	"github.com/aaronsb/mmaid-go/internal/layout"
	"github.com/aaronsb/mmaid-go/internal/parser"
)

// runCell is one draw cell of one edge with the axis the edge runs through
// it along.
type runCell struct {
	edge string
	axis Axis
}

// cellsByEdge returns every draw cell of every routed edge with the axis
// of the run through it; a corner carries both.
func cellsByEdge(routed []RoutedEdge) map[Point][]runCell {
	out := map[Point][]runCell{}
	for _, re := range routed {
		name := re.Edge.Source + "->" + re.Edge.Target
		p := re.DrawPath
		for i := 1; i < len(p); i++ {
			a, b := p[i-1], p[i]
			d := Point{sign(b.Col - a.Col), sign(b.Row - a.Row)}
			for c := a; ; c = (Point{c.Col + d.Col, c.Row + d.Row}) {
				out[c] = append(out[c], runCell{name, axisOf(d)})
				if c == b {
					break
				}
			}
		}
	}
	return out
}

// TestGapLanesSeparateParallelRuns routes flowchart-cross at the golden's
// layout width. No cell is shared by two edges running through it along
// the same axis: two edges meet only where one crosses the other.
func TestGapLanesSeparateParallelRuns(t *testing.T) {
	src, err := os.ReadFile("../../testdata/fixtures/flowchart-cross.mmd")
	if err != nil {
		t.Fatal(err)
	}
	g := parser.ParseFlowchart(string(src))
	l := layout.ComputeLayout(g, 4, 2, 120*75/100)
	routed := RouteEdgesWith(g, l, Ellipsis, nil)
	cells := cellsByEdge(routed)

	for p, rcs := range cells {
		for i, a := range rcs {
			for _, b := range rcs[:i] {
				if a.edge != b.edge && a.axis&b.axis != 0 {
					t.Errorf("%v: %s and %s share the cell along one axis", p, a.edge, b.edge)
				}
			}
		}
	}

	// The corridors the issue names: D's top apron row and the gap row
	// under A and B, which E->A runs the length of.
	horizontal := func(p Point) []string {
		var out []string
		for _, rc := range cells[p] {
			if rc.axis == AxisH {
				out = append(out, rc.edge)
			}
		}
		return out
	}
	for col := 28; col <= 42; col++ {
		if h := horizontal(Point{col, 14}); len(h) > 1 {
			t.Errorf("(%d,14): %v run together", col, h)
		}
	}
	for col := 0; col < 120; col++ {
		if h := horizontal(Point{col, 6}); len(h) > 1 {
			t.Errorf("(%d,6): %v run together", col, h)
		}
	}
}
