package routing

import (
	"bytes"
	"strings"
	"testing"

	"github.com/aaronsb/mmaid-go/internal/graph"
	"github.com/aaronsb/mmaid-go/internal/layout"
	"github.com/aaronsb/mmaid-go/internal/parser"
	"github.com/aaronsb/mmaid-go/internal/textwidth"
)

func TestStepCosts(t *testing.T) {
	obs := &Obstacles{
		Soft:   map[Point]Axis{{1, 0}: AxisH, {2, 0}: AxisH | AxisV},
		Border: map[Point]bool{{3, 0}: true, {1, 0}: true},
	}
	cases := []struct {
		p      Point
		a      Axis
		waived bool
		want   float64
	}{
		{Point{1, 0}, AxisH, false, CostStep + CostShared + CostBorder},
		{Point{1, 0}, AxisV, false, CostStep + CostCrossing + CostBorder},
		{Point{1, 0}, AxisH, true, CostStep + CostBorder},
		{Point{2, 0}, AxisV, false, CostStep + CostShared},
		{Point{3, 0}, AxisH, false, CostStep + CostBorder},
		{Point{4, 0}, AxisH, false, CostStep},
	}
	for _, c := range cases {
		if got := obs.stepCost(c.p, c.a, c.waived); got != c.want {
			t.Errorf("stepCost(%v, %d, %v) = %v, want %v", c.p, c.a, c.waived, got, c.want)
		}
	}
}

func TestFindPathLeavesSharedCorridor(t *testing.T) {
	free := func(col, row int) bool { return col >= 0 && row >= 0 && row <= 2 }
	soft := make(map[Point]Axis)
	Occupy(soft, []Point{{0, 0}, {1, 0}, {2, 0}, {3, 0}, {4, 0}, {5, 0}, {6, 0}})
	path, _ := FindPath(0, 0, 6, 0, free, &Obstacles{Soft: soft})
	if path == nil {
		t.Fatal("no path")
	}
	for _, p := range path[2 : len(path)-2] {
		if p.Row == 0 {
			t.Fatalf("path %v runs down the occupied row", path)
		}
	}
}

func TestFindPathApronIsFree(t *testing.T) {
	free := func(col, row int) bool { return col >= 0 && row >= 0 }
	soft := make(map[Point]Axis)
	Occupy(soft, []Point{{0, 0}, {1, 0}, {2, 0}})
	_, cost := FindPath(0, 0, 2, 0, free, &Obstacles{Soft: soft})
	if cost != 2*CostStep {
		t.Errorf("cost %v, want %v: the apron and the end are free of the shared charge", cost, 2*CostStep)
	}
}

func TestOccupyAxes(t *testing.T) {
	soft := make(map[Point]Axis)
	Occupy(soft, []Point{{0, 0}, {1, 0}, {1, 1}})
	want := map[Point]Axis{{0, 0}: AxisH, {1, 0}: AxisH | AxisV, {1, 1}: AxisV}
	for p, a := range want {
		if soft[p] != a {
			t.Errorf("%v: axis %d, want %d", p, soft[p], a)
		}
	}
}

// twoCells is a layout of two 3x3 node blocks side by side with a gap
// column between: node columns 1 and 5, node row 1 with a second node row
// at 5.
func twoCells() *layout.GridLayout {
	l := layout.NewGridLayout()
	for c, w := range map[int]int{0: 1, 1: 5, 2: 1, 3: 4, 4: 1, 5: 5, 6: 1} {
		l.ColWidths[c] = w
	}
	for r, h := range map[int]int{0: 1, 1: 3, 2: 1, 3: 3, 4: 1, 5: 3, 6: 1} {
		l.RowHeights[r] = h
	}
	return l
}

func TestDrawPathPorts(t *testing.T) {
	l := twoCells()
	sides := [2]AttachDir{AttachRight, AttachLeft}
	cases := []struct {
		name  string
		path  []Point
		ports [2]int
		want  []Point
	}{
		{"straight, centre ports", []Point{{2, 1}, {3, 1}, {4, 1}}, [2]int{0, 0},
			[]Point{{6, 2}, {11, 2}}},
		{"straight, jog on the gap centre line", []Point{{2, 1}, {3, 1}, {4, 1}}, [2]int{1, -1},
			[]Point{{6, 3}, {9, 3}, {9, 1}, {11, 1}}},
		{"one turn, offset rides the run", []Point{{2, 1}, {3, 1}, {3, 2}, {3, 3}, {3, 4}, {3, 5}, {4, 5}}, [2]int{1, 0},
			[]Point{{6, 3}, {9, 3}, {9, 10}, {11, 10}}},
	}
	for _, c := range cases {
		got := drawPath(l, c.path, sides, c.ports)
		if len(got) != len(c.want) {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%s: %v, want %v", c.name, got, c.want)
				break
			}
		}
	}
}

// labelledSources are graphs whose labels sit in the cells a node side's
// ports step into.
var labelledSources = []string{
	"graph LR\n A -->|label one| B\n A -->|label two| C\n A --> D\n",
	"graph TD\n A -->|label one| B\n A -->|label two| C\n A --> D\n",
	"graph TD\n Idle -->|start| Run\n Run -->|stop| Idle\n",
	"graph LR\n A[Request] --> B{Auth?}\n B -->|Yes| C[Process]\n B -->|No| D[Reject]\n C --> E[Response]\n",
	"graph TD\n A -->|one| B\n A -->|two| C\n A -->|three| D\n B -->|four| E\n C -->|five| E\n D -->|six| E\n",
}

func route(t *testing.T, src string) (*graph.Graph, *layout.GridLayout, []RoutedEdge, string) {
	t.Helper()
	g := parser.ParseFlowchart(src)
	l := layout.ComputeLayout(g, 4, 2, 0)
	var warn bytes.Buffer
	routed := RouteEdgesWith(g, l, Ellipsis, &warn)
	return g, l, routed, warn.String()
}

// assertDistinctEnds fails when two edges leave or arrive through one draw
// cell, or one arrives where another leaves.
func assertDistinctEnds(t *testing.T, src string, routed []RoutedEdge) {
	t.Helper()
	starts := map[Point]string{}
	ends := map[Point]string{}
	for _, re := range routed {
		p := re.DrawPath
		s, e := p[0], p[len(p)-1]
		name := re.Edge.Source + "->" + re.Edge.Target
		if other, ok := starts[s]; ok {
			t.Errorf("%q: %s and %s leave through the same cell %v", src, name, other, s)
		}
		if other, ok := ends[e]; ok {
			t.Errorf("%q: %s and %s arrive through the same cell %v", src, name, other, e)
		}
		if other, ok := starts[e]; ok {
			t.Errorf("%q: %s arrives through the cell %s leaves by: %v", src, name, other, e)
		}
		starts[s] = name
		ends[e] = name
	}
}

// lineCells returns every draw cell of every routed edge, by edge name.
func lineCells(routed []RoutedEdge) map[Point]string {
	out := map[Point]string{}
	for _, re := range routed {
		p := re.DrawPath
		for i := 1; i < len(p); i++ {
			a, b := p[i-1], p[i]
			dx, dy := sign(b.Col-a.Col), sign(b.Row-a.Row)
			for c := a; ; c = (Point{c.Col + dx, c.Row + dy}) {
				out[c] = re.Edge.Source + "->" + re.Edge.Target
				if c == b {
					break
				}
			}
		}
	}
	return out
}

// assertNoSharedStub fails when two edges leave one side and run together
// for the first cell out of the port.
func assertNoSharedStub(t *testing.T, src string, routed []RoutedEdge) {
	t.Helper()
	seen := map[Point]string{}
	for _, re := range routed {
		p := re.DrawPath
		stub := stepToward(p[0], p[1])
		name := re.Edge.Source + "->" + re.Edge.Target
		if other, ok := seen[stub]; ok {
			t.Errorf("%q: %s and %s share the stub cell %v", src, name, other, stub)
		}
		seen[stub] = name
	}
}

func TestPortsSeparateEdgesOnASide(t *testing.T) {
	src := "graph LR\n A --> B\n A --> C\n A --> D\n B --> D\n D --> A\n"
	_, _, routed, _ := route(t, src)
	assertDistinctEnds(t, src, routed)
	assertNoSharedStub(t, src, routed)
}

func TestLabelsDoNotSteerPorts(t *testing.T) {
	for _, src := range labelledSources {
		_, _, routed, warn := route(t, src)
		assertDistinctEnds(t, src, routed)
		assertNoSharedStub(t, src, routed)
		if warn != "" {
			t.Errorf("%q: %s", src, warn)
		}
	}
}

func TestLabelsStayOffNodesLinesAndEachOther(t *testing.T) {
	for _, src := range labelledSources {
		_, l, routed, _ := route(t, src)
		lines := lineCells(routed)
		taken := map[Point]string{}
		for _, re := range routed {
			if re.Label == "" {
				continue
			}
			if re.LabelText == "" {
				t.Errorf("%q: label %q was dropped", src, re.Label)
				continue
			}
			for i := range textwidth.String(re.LabelText) {
				p := Point{re.LabelCol + i, re.LabelRow}
				for _, np := range l.Placements {
					if p.Col >= np.DrawX && p.Col < np.DrawX+np.DrawWidth && p.Row >= np.DrawY && p.Row < np.DrawY+np.DrawHeight {
						t.Errorf("%q: label %q cell %v is inside node %s", src, re.LabelText, p, np.NodeID)
					}
				}
				if edge, ok := lines[p]; ok {
					t.Errorf("%q: label %q cell %v is under the line of %s", src, re.LabelText, p, edge)
				}
				if other, ok := taken[p]; ok {
					t.Errorf("%q: label %q overlaps %q at %v", src, re.LabelText, other, p)
				}
				taken[p] = re.LabelText
			}
		}
	}
}

func TestLabelReservesGridCellsButNotAprons(t *testing.T) {
	// The label beside A->B sits in the gap cell A's right ports step
	// into, which stays free so it does not steer later edges.
	_, l, routed, _ := route(t, "graph LR\n A -->|label| B\n")
	if re := routed[0]; re.LabelText != "label" {
		t.Fatalf("label %q", re.LabelText)
	}
	a := l.Placements["A"]
	if !l.IsFree(a.Grid.Col+2, a.Grid.Row, nil) {
		t.Error("A's right apron is reserved")
	}

	// The back-edge's label sits beside its run through the gap row under
	// B, in cells no port steps into, which later edges must route around.
	_, l, routed, _ = route(t, "graph LR\n A --> B\n B --> C\n C -->|label| A\n")
	// A side's apron is the cell two steps out from the node's centre
	// through a side an edge attaches to.
	aprons := map[Point]bool{}
	for _, re := range routed {
		src, tgt := l.Placements[re.Edge.Source].Grid, l.Placements[re.Edge.Target].Grid
		a := re.StartDir.AttachCell(re.StartDir.AttachCell(src))
		b := re.EndDir.AttachCell(re.EndDir.AttachCell(tgt))
		aprons[Point{a.Col, a.Row}] = true
		aprons[Point{b.Col, b.Row}] = true
	}
	apron := func(c, r int) bool { return aprons[Point{c, r}] }
	reserved := 0
	for _, re := range routed {
		if re.Label == "" {
			continue
		}
		if re.LabelText != "label" {
			t.Fatalf("label %q", re.LabelText)
		}
		for i := range textwidth.String(re.LabelText) {
			c, r := l.DrawToGrid(re.LabelCol+i, re.LabelRow)
			switch {
			case apron(c, r) && !l.IsFree(c, r, nil):
				t.Errorf("apron (%d,%d) under the label is reserved", c, r)
			case !apron(c, r) && l.IsFree(c, r, nil):
				t.Errorf("grid cell (%d,%d) under the label is still free", c, r)
			case !apron(c, r):
				reserved++
			}
		}
	}
	if reserved == 0 {
		t.Error("the label reserved nothing")
	}
}

func TestDroppedLabelWarnsOnce(t *testing.T) {
	g := parser.ParseFlowchart("graph LR\n A -->|label| B\n")
	l := layout.ComputeLayout(g, 4, 2, 0)
	var warn bytes.Buffer
	space := newLabelSpace(g, l, nil, &warn)
	// Every cell is a line: nothing fits.
	for y := -5; y < 40; y++ {
		for x := -5; x < 120; x++ {
			space.lines[Point{x, y}] = true
		}
	}
	re := RoutedEdge{Edge: g.Edges[0], Label: "label", DrawPath: []Point{{10, 2}, {20, 2}}}
	space.place(&re, Ellipsis)
	space.place(&re, Ellipsis)
	if re.LabelText != "" {
		t.Fatalf("label placed at %d,%d", re.LabelRow, re.LabelCol)
	}
	if got := warn.String(); strings.Count(got, "mmaid: edge label \"label\" dropped") != 1 {
		t.Errorf("warning %q, want it once", got)
	}
}
