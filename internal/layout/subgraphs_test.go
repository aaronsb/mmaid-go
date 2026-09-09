package layout

import (
	"testing"

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
