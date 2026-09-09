package routing

import (
	"testing"

	"github.com/aaronsb/mmaid-go/internal/layout"
	"github.com/aaronsb/mmaid-go/internal/parser"
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

func TestPortsSeparateEdgesOnASide(t *testing.T) {
	g := parser.ParseFlowchart("graph LR\n A --> B\n A --> C\n A --> D\n B --> D\n D --> A\n")
	l := layout.ComputeLayout(g, 4, 2, 0)
	routed := RouteEdges(g, l)
	starts := map[Point]string{}
	ends := map[Point]string{}
	for _, re := range routed {
		p := re.DrawPath
		s, e := p[0], p[len(p)-1]
		if other, ok := starts[s]; ok {
			t.Errorf("%s->%s and %s leave through the same cell %v", re.Edge.Source, re.Edge.Target, other, s)
		}
		if other, ok := ends[e]; ok {
			t.Errorf("%s->%s and %s arrive through the same cell %v", re.Edge.Source, re.Edge.Target, other, e)
		}
		if _, ok := starts[e]; ok {
			t.Errorf("%s->%s arrives through a cell another edge leaves by: %v", re.Edge.Source, re.Edge.Target, e)
		}
		starts[s] = re.Edge.Source + "->" + re.Edge.Target
		ends[e] = re.Edge.Source + "->" + re.Edge.Target
	}
}

func TestLabelsStayOffNodesAndEachOther(t *testing.T) {
	sources := []string{
		"graph TD\n Idle -->|start| Run\n Run -->|stop| Idle\n",
		"graph LR\n A[Request] --> B{Auth?}\n B -->|Yes| C[Process]\n B -->|No| D[Reject]\n C --> E[Response]\n",
		"graph TD\n A -->|one| B\n A -->|two| C\n A -->|three| D\n B -->|four| E\n C -->|five| E\n D -->|six| E\n",
	}
	for _, src := range sources {
		g := parser.ParseFlowchart(src)
		l := layout.ComputeLayout(g, 4, 2, 0)
		taken := map[Point]string{}
		for _, re := range RouteEdges(g, l) {
			if re.Label == "" {
				continue
			}
			if re.LabelText == "" {
				t.Errorf("%q: label %q was dropped", src, re.Label)
				continue
			}
			for i := range len(re.LabelText) {
				p := Point{re.LabelCol + i, re.LabelRow}
				for _, np := range l.Placements {
					if p.Col >= np.DrawX && p.Col < np.DrawX+np.DrawWidth && p.Row >= np.DrawY && p.Row < np.DrawY+np.DrawHeight {
						t.Errorf("%q: label %q cell %v is inside node %s", src, re.LabelText, p, np.NodeID)
					}
				}
				if other, ok := taken[p]; ok {
					t.Errorf("%q: label %q overlaps %q at %v", src, re.LabelText, other, p)
				}
				taken[p] = re.LabelText
			}
		}
	}
}

func TestLabelReservesGridCells(t *testing.T) {
	g := parser.ParseFlowchart("graph LR\n A -->|label| B\n")
	l := layout.ComputeLayout(g, 4, 2, 0)
	routed := RouteEdges(g, l)
	re := routed[0]
	if re.LabelText != "label" {
		t.Fatalf("label %q", re.LabelText)
	}
	c, r := l.DrawToGrid(re.LabelCol, re.LabelRow)
	if l.IsFree(c, r, nil) {
		t.Errorf("grid cell (%d,%d) under the label is still free", c, r)
	}
}
