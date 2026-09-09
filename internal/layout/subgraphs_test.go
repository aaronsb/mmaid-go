package layout

import (
	"testing"

	"github.com/aaronsb/mmaid-go/internal/graph"
	"github.com/aaronsb/mmaid-go/internal/parser"
)

// overlaps reports whether two boxes share any cell.
func overlaps(a, b SubgraphBounds) bool {
	return a.X < b.X+b.Width && b.X < a.X+a.Width &&
		a.Y < b.Y+b.Height && b.Y < a.Y+a.Height
}

// contains reports whether outer holds every cell of inner.
func contains(outer, inner SubgraphBounds) bool {
	return inner.X >= outer.X && inner.Y >= outer.Y &&
		inner.X+inner.Width <= outer.X+outer.Width &&
		inner.Y+inner.Height <= outer.Y+outer.Height
}

func boundsByID(l *GridLayout) map[string]SubgraphBounds {
	out := make(map[string]SubgraphBounds)
	for _, sb := range l.SubgraphBounds {
		out[sb.Subgraph.ID] = sb
	}
	return out
}

func TestSiblingBoundsDoNotOverlap(t *testing.T) {
	g := parser.ParseFlowchart(`graph LR
    subgraph S1
        A --> B
    end
    subgraph S2
        C --> D
    end
    A --> C
    B --> D`)
	l := ComputeLayout(g, 4, 2, 0)
	b := boundsByID(l)
	s1, s2 := b["S1"], b["S2"]
	if s1.Width == 0 || s2.Width == 0 {
		t.Fatalf("missing bounds: %+v", l.SubgraphBounds)
	}
	if overlaps(s1, s2) {
		t.Errorf("S1 %+v overlaps S2 %+v", s1, s2)
	}
	for _, nid := range []string{"A", "B"} {
		p := l.Placements[nid]
		if p.DrawX < s1.X || p.DrawX+p.DrawWidth > s1.X+s1.Width {
			t.Errorf("%s at x=%d..%d is outside S1 %+v", nid, p.DrawX, p.DrawX+p.DrawWidth, s1)
		}
	}
}

func TestNestedBoundsAreContained(t *testing.T) {
	g := parser.ParseFlowchart(`graph TB
    Start --> A
    subgraph Outer
        A --> B
        subgraph Inner
            direction LR
            C --> D
        end
        B --> C
        D --> E
    end
    E --> Stop`)
	l := ComputeLayout(g, 4, 2, 0)
	b := boundsByID(l)
	outer, inner := b["Outer"], b["Inner"]
	if outer.Width == 0 || inner.Width == 0 {
		t.Fatalf("missing bounds: %+v", l.SubgraphBounds)
	}
	if !contains(outer, inner) {
		t.Errorf("Inner %+v is not inside Outer %+v", inner, outer)
	}
	c, d := l.Placements["C"], l.Placements["D"]
	if c.DrawY != d.DrawY || c.DrawX >= d.DrawX {
		t.Errorf("Inner is LR: C (%d,%d) should be left of D (%d,%d) on one row", c.DrawX, c.DrawY, d.DrawX, d.DrawY)
	}
	for _, nid := range []string{"Start", "Stop"} {
		p := l.Placements[nid]
		if p.DrawY >= outer.Y && p.DrawY < outer.Y+outer.Height {
			t.Errorf("%s at y=%d lies inside Outer %+v", nid, p.DrawY, outer)
		}
	}
}

func TestSiblingBlocksDoNotInterleave(t *testing.T) {
	g := parser.ParseFlowchart(`graph LR
    subgraph Input
        A --> B
    end
    subgraph Process
        C --> D
    end
    subgraph Output
        E --> F
    end
    B --> C
    D --> E
    A --> D
    C --> F`)
	l := ComputeLayout(g, 4, 2, 0)
	b := boundsByID(l)
	ids := []string{"Input", "Process", "Output"}
	for i := range ids {
		for j := i + 1; j < len(ids); j++ {
			if overlaps(b[ids[i]], b[ids[j]]) {
				t.Errorf("%s %+v overlaps %s %+v", ids[i], b[ids[i]], ids[j], b[ids[j]])
			}
		}
	}
	if !(b["Input"].X < b["Process"].X && b["Process"].X < b["Output"].X) {
		t.Errorf("siblings out of chain order: %+v", b)
	}
}

func TestWideLabelDoesNotOverlapSibling(t *testing.T) {
	g := parser.ParseFlowchart(`graph TB
    subgraph "A very very very long subgraph label that is wider than its block"
        A
    end
    subgraph S2
        B
    end
    A --> C
    B --> C`)
	l := ComputeLayout(g, 4, 2, 0)
	var wide, s2 SubgraphBounds
	for _, sb := range l.SubgraphBounds {
		if sb.Subgraph.ID == "S2" {
			s2 = sb
		} else {
			wide = sb
		}
	}
	if overlaps(wide, s2) {
		t.Errorf("wide-label box %+v overlaps S2 %+v", wide, s2)
	}
	b := l.Placements["B"]
	if b.DrawX < wide.X+wide.Width {
		t.Errorf("B at x=%d lies under the wide box ending at %d", b.DrawX, wide.X+wide.Width)
	}
}

func TestInnerDirectionBTReversesTheBlock(t *testing.T) {
	g := parser.ParseFlowchart(`graph TB
    Start --> A
    subgraph S
        direction BT
        A --> B
    end
    B --> Stop`)
	l := ComputeLayout(g, 4, 2, 0)
	a, b := l.Placements["A"], l.Placements["B"]
	if b.DrawY >= a.DrawY {
		t.Errorf("BT inside TB: B (y=%d) should be above A (y=%d)", b.DrawY, a.DrawY)
	}
	start, stop := l.Placements["Start"], l.Placements["Stop"]
	if start.DrawY >= b.DrawY || stop.DrawY <= a.DrawY {
		t.Errorf("the graph stays TB around the block: Start y=%d, Stop y=%d", start.DrawY, stop.DrawY)
	}

	g = parser.ParseFlowchart(`graph BT
    Start --> A
    subgraph S
        direction BT
        A --> B
    end`)
	l = ComputeLayout(g, 4, 2, 0)
	a, b = l.Placements["A"], l.Placements["B"]
	if b.DrawY <= a.DrawY {
		t.Errorf("BT inside BT flips with the canvas: B (y=%d) should be laid out below A (y=%d)", b.DrawY, a.DrawY)
	}
}

func TestEmptySubgraphAddsNoLayer(t *testing.T) {
	with := parser.ParseFlowchart(`graph TB
    A --> S1
    subgraph S1
    end
    S1 --> B
    A --> B`)
	without := parser.ParseFlowchart(`graph TB
    A --> B`)
	lw, lo := ComputeLayout(with, 4, 2, 0), ComputeLayout(without, 4, 2, 0)
	if lw.Placements["B"].Grid != lo.Placements["B"].Grid {
		t.Errorf("B at %+v with the empty subgraph, %+v without", lw.Placements["B"].Grid, lo.Placements["B"].Grid)
	}
	if len(lw.SubgraphBounds) != 0 {
		t.Errorf("an empty subgraph has bounds: %+v", lw.SubgraphBounds)
	}
}

func TestRepWithSubgraphEndpoints(t *testing.T) {
	g := parser.ParseFlowchart(`graph TB
    A --> Outer
    subgraph Outer
        B
        subgraph Inner
            C
        end
    end
    Inner --> D
    B --> Inner
    Outer --> A`)
	h := &hierarchy{g: g, owner: make(map[string]*graph.Subgraph)}
	for _, nid := range g.NodeOrder {
		h.owner[nid] = g.FindSubgraphForNode(nid)
	}
	outer := g.FindSubgraphByID("Outer")
	inner := g.FindSubgraphByID("Inner")

	cases := []struct {
		scope      *graph.Subgraph
		id         string
		isSubgraph bool
		want       string
		ok         bool
	}{
		{nil, "A", false, "A", true},
		{nil, "B", false, compoundKey(outer), true},
		{nil, "C", false, compoundKey(outer), true},
		{nil, "Outer", true, compoundKey(outer), true},
		{nil, "Inner", true, compoundKey(outer), true},
		{outer, "B", false, "B", true},
		{outer, "C", false, compoundKey(inner), true},
		{outer, "Inner", true, compoundKey(inner), true},
		{outer, "Outer", true, "", false},
		{outer, "A", false, "", false},
		{inner, "C", false, "C", true},
		{inner, "Inner", true, "", false},
		{inner, "Outer", true, "", false},
		{inner, "B", false, "", false},
	}
	for _, c := range cases {
		scope := "root"
		if c.scope != nil {
			scope = c.scope.ID
		}
		got, ok := h.rep(c.scope, c.id, c.isSubgraph)
		if got != c.want || ok != c.ok {
			t.Errorf("rep(%s, %s, subgraph=%v) = %q, %v; want %q, %v", scope, c.id, c.isSubgraph, got, ok, c.want, c.ok)
		}
	}

	lg, _ := h.local(nil)
	if len(lg.order) != 3 {
		t.Errorf("root order %v, want A, D and Outer's compound", lg.order)
	}
	for _, e := range lg.edges {
		if e.src == e.tgt {
			t.Errorf("root keeps an edge inside a compound: %+v", e)
		}
	}
}

func TestGapCellsAndGapSize(t *testing.T) {
	closing, opening := gapCells([]borderRun{{opening: false}, {opening: false}}, 10, false)
	if !closing[1] || !closing[3] || len(closing) != 2 || len(opening) != 0 {
		t.Errorf("two closing sides: closing %v opening %v", closing, opening)
	}
	closing, opening = gapCells([]borderRun{{opening: true}, {opening: true}}, 20, true)
	if !opening[16] || !opening[12] || len(opening) != 2 || len(closing) != 0 {
		t.Errorf("two opening tops in 20 rows: closing %v opening %v", closing, opening)
	}
	_, opening = gapCells([]borderRun{{opening: true}}, 8, false)
	if !opening[6] {
		t.Errorf("one opening left side in 8 columns: %v", opening)
	}

	// The corridor at size/2 lies outside every box with a straight cell
	// between it and each border.
	cases := []struct {
		runs []borderRun
		min  int
		rows bool
		want int
	}{
		{[]borderRun{{opening: false}}, 3, true, 6},
		{[]borderRun{{opening: true}}, 3, true, 11},
		{[]borderRun{{opening: false}, {opening: true}}, 3, true, 11},
		{[]borderRun{{opening: true}}, 4, false, 7},
		{[]borderRun{{opening: false}, {opening: true}}, 4, false, 7},
		{[]borderRun{{opening: false}}, 20, false, 20},
	}
	for _, c := range cases {
		got := gapSize(c.runs, c.min, c.rows)
		if got != c.want {
			t.Errorf("gapSize(%v, %d, rows=%v) = %d, want %d", c.runs, c.min, c.rows, got, c.want)
		}
		closing, opening := gapCells(c.runs, got, c.rows)
		mid := got / 2
		for b := range closing {
			if mid-b < 2 {
				t.Errorf("size %d: corridor %d within one cell of closing border %d", got, mid, b)
			}
		}
		for b := range opening {
			if b-mid < 2 {
				t.Errorf("size %d: corridor %d within one cell of opening border %d", got, mid, b)
			}
		}
	}
}

func TestBoundsAndGapsAgreeOnNestedTops(t *testing.T) {
	g := parser.ParseFlowchart(`graph TB
    Start --> A
    subgraph Outer
        subgraph Inner
            A
        end
    end`)
	l := ComputeLayout(g, 4, 2, 0)
	b := boundsByID(l)
	outer, inner := b["Outer"], b["Inner"]
	a := l.Placements["A"]
	if inner.Y != a.DrawY-sideDepth(true) || outer.Y != inner.Y-sideDepth(true) {
		t.Errorf("tops stack by sideDepth: A y=%d, Inner y=%d, Outer y=%d", a.DrawY, inner.Y, outer.Y)
	}
	start := l.Placements["Start"]
	_, corridor := l.GridToDrawCenter(start.Grid.Col, start.Grid.Row+2)
	if corridor <= start.DrawY+start.DrawHeight || outer.Y-corridor < 2 {
		t.Errorf("the gap's corridor at y=%d must lie between Start's bottom %d and two cells above Outer's top %d", corridor, start.DrawY+start.DrawHeight-1, outer.Y)
	}
}
