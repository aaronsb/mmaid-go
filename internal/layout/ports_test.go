package layout

import (
	"testing"

	"github.com/aaronsb/mmaid-go/internal/graph"
)

func TestPreferredSides(t *testing.T) {
	cases := []struct {
		name      string
		src, tgt  GridCoord
		dir       graph.Direction
		pref, alt [2]Side
	}{
		{"LR forward", GridCoord{1, 1}, GridCoord{5, 1}, graph.DirLR, [2]Side{Right, Left}, [2]Side{Right, Left}},
		{"LR forward and down", GridCoord{1, 1}, GridCoord{5, 5}, graph.DirLR, [2]Side{Right, Left}, [2]Side{Bottom, Top}},
		{"LR back", GridCoord{5, 1}, GridCoord{1, 1}, graph.DirLR, [2]Side{Bottom, Bottom}, [2]Side{Bottom, Top}},
		{"TD forward", GridCoord{1, 1}, GridCoord{1, 5}, graph.DirTD, [2]Side{Bottom, Top}, [2]Side{Bottom, Top}},
		{"TD back", GridCoord{1, 5}, GridCoord{1, 1}, graph.DirTD, [2]Side{Right, Right}, [2]Side{Right, Left}},
	}
	for _, c := range cases {
		pref, alt := PreferredSides(c.src, c.tgt, c.dir)
		if pref != c.pref || alt != c.alt {
			t.Errorf("%s: got %v %v, want %v %v", c.name, pref, alt, c.pref, c.alt)
		}
	}
}

// fanOut is A with edges to B, C and D in LR: B level with A, C and D
// below it.
func fanOut(t *testing.T) *GridLayout {
	t.Helper()
	g := graph.NewGraph()
	g.Direction = graph.DirLR
	for _, id := range []string{"A", "B", "C", "D"} {
		g.AddNode(&graph.Node{ID: id, Label: id})
	}
	for _, tgt := range []string{"B", "C", "D"} {
		g.AddEdge(graph.NewEdge("A", tgt))
	}
	return ComputeLayout(g, 4, 2, 0)
}

func TestAssignPortsSpreadFromCentre(t *testing.T) {
	l := fanOut(t)
	other := func(id string) int {
		p := l.Placements[id]
		return p.DrawY + p.DrawHeight/2
	}
	// B is level with A and takes the centre; C sits below it.
	ports := l.AssignPorts([]PortRequest{
		{Node: "A", Side: Right, Other: other("C")},
		{Node: "A", Side: Right, Other: other("B")},
	})
	if want := []int{1, 0}; !equalInts(ports, want) {
		t.Errorf("ports %v, want %v", ports, want)
	}
	// With D as well the block would run 0..2 past the corner of a
	// three-port side, so it slides to -1..1 in row order.
	ports = l.AssignPorts([]PortRequest{
		{Node: "A", Side: Right, Other: other("D")},
		{Node: "A", Side: Right, Other: other("B")},
		{Node: "A", Side: Right, Other: other("C")},
	})
	if want := []int{1, -1, 0}; !equalInts(ports, want) {
		t.Errorf("ports %v, want %v", ports, want)
	}
}

func TestAssignPortsSlideInsideCorners(t *testing.T) {
	l := fanOut(t)
	a := l.Placements["A"]
	_, hi := l.PortRange(a, Right)
	far := a.DrawY + a.DrawHeight + 100
	// Three ends all below the centre would want 0, 1, 2; the block
	// slides so the last stays off the corner.
	ports := l.AssignPorts([]PortRequest{
		{Node: "A", Side: Right, Other: far},
		{Node: "A", Side: Right, Other: far + 1},
		{Node: "A", Side: Right, Other: far + 2},
	})
	if ports[2] != hi || ports[1] != hi-1 || ports[0] != hi-2 {
		t.Errorf("ports %v, want a block ending at %d", ports, hi)
	}
}

func TestAssignPortsOverflowTakesCentre(t *testing.T) {
	l := fanOut(t)
	a := l.Placements["A"]
	lo, hi := l.PortRange(a, Right)
	n := hi - lo + 3
	reqs := make([]PortRequest, n)
	for i := range reqs {
		reqs[i] = PortRequest{Node: "A", Side: Right, Other: a.DrawY + i}
	}
	ports := l.AssignPorts(reqs)
	centre := 0
	for _, p := range ports {
		if p < lo || p > hi {
			t.Errorf("port %d outside %d..%d", p, lo, hi)
		}
		if p == 0 {
			centre++
		}
	}
	if centre != 3 {
		t.Errorf("%d ends at the centre, want the two surplus plus one", centre)
	}
}

func TestPortCountGrowsNode(t *testing.T) {
	g := graph.NewGraph()
	g.Direction = graph.DirLR
	g.AddNode(&graph.Node{ID: "A", Label: "A"})
	for _, id := range []string{"B", "C", "D", "E", "F"} {
		g.AddNode(&graph.Node{ID: id, Label: id})
		g.AddEdge(graph.NewEdge("A", id))
	}
	l := ComputeLayout(g, 4, 2, 0)
	a := l.Placements["A"]
	if a.PortCount[Right] != 5 {
		t.Fatalf("A expects %d ports on its right, want 5", a.PortCount[Right])
	}
	if lo, hi := l.PortRange(a, Right); hi-lo+1 < 5 {
		t.Errorf("A's right side holds %d ports, want 5", hi-lo+1)
	}
}

func TestReserveDrawBlocksGridCells(t *testing.T) {
	l := fanOut(t)
	a := l.Placements["A"]
	// The gap column right of A, one row below A's centre.
	gap := a.Grid.Col + 2
	x, y := l.GridToDraw(gap, a.Grid.Row)
	if !l.IsFree(gap, a.Grid.Row, nil) {
		t.Fatal("gap cell should start free")
	}
	l.ReserveDraw(x, y+1, x+3, y+1)
	if l.IsFree(gap, a.Grid.Row, nil) {
		t.Error("gap cell should be reserved")
	}
	if !l.IsFree(gap, a.Grid.Row+1, nil) {
		t.Error("the row below should stay free")
	}
	if c, r := l.DrawToGrid(x+3, y+1); c != gap || r != a.Grid.Row {
		t.Errorf("DrawToGrid(%d,%d) = (%d,%d), want (%d,%d)", x+3, y+1, c, r, gap, a.Grid.Row)
	}
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
