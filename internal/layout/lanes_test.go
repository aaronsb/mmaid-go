package layout

import (
	"strings"
	"testing"

	"github.com/aaronsb/mmaid-go/internal/parser"
)

// The same graph in both flow directions: two lanes the flow crosses twice.
const (
	laneSourceTB = `swimlane-beta TB
    subgraph cust["Customer"]
        A[Place order]
        D[Receive goods]
    end
    subgraph wh["Warehouse"]
        B[Pick items]
        C[Ship]
    end
    A --> B --> C --> D`

	laneSourceLR = `swimlane-beta LR
    subgraph cust["Customer"]
        A[Place order]
        D[Receive goods]
    end
    subgraph wh["Warehouse"]
        B[Pick items]
        C[Ship]
    end
    A --> B --> C --> D`
)

// laneOfNode names the lane holding each node of the sources above.
var laneOfNode = map[string]string{"A": "cust", "D": "cust", "B": "wh", "C": "wh"}

func laneLayouts(t *testing.T) map[string]*GridLayout {
	t.Helper()
	return map[string]*GridLayout{
		"TB": ComputeLayout(parser.ParseSwimlane(laneSourceTB), 4, 2, 0),
		"LR": ComputeLayout(parser.ParseSwimlane(laneSourceLR), 4, 2, 0),
	}
}

func TestLaneHoldsItsNodes(t *testing.T) {
	for dir, l := range laneLayouts(t) {
		b := boundsByID(l)
		if len(b) != 2 {
			t.Fatalf("%s: lane boxes = %d, want 2", dir, len(b))
		}
		for nid, laneID := range laneOfNode {
			lane, ok := b[laneID]
			if !ok {
				t.Fatalf("%s: no box for lane %s", dir, laneID)
			}
			p := l.Placements[nid]
			if p == nil {
				t.Fatalf("%s: %s is unplaced", dir, nid)
			}
			node := SubgraphBounds{X: p.DrawX, Y: p.DrawY, Width: p.DrawWidth, Height: p.DrawHeight}
			if !contains(lane, node) {
				t.Errorf("%s: %s at %+v is outside lane %s %+v", dir, nid, node, laneID, lane)
			}
		}
	}
}

func TestLaneBoundsSpanTheFlowAxis(t *testing.T) {
	// Every lane covers every layer, so on the flow axis all the boxes
	// share one range and it holds every node.
	for dir, l := range laneLayouts(t) {
		vertical := dir == "TB"
		b := boundsByID(l)
		lo, hi := 0, 0
		for i, sb := range []SubgraphBounds{b["cust"], b["wh"]} {
			s, e := sb.X, sb.X+sb.Width
			if vertical {
				s, e = sb.Y, sb.Y+sb.Height
			}
			if i == 0 {
				lo, hi = s, e
				continue
			}
			if s != lo || e != hi {
				t.Errorf("%s: lane flow range %d..%d, want %d..%d", dir, s, e, lo, hi)
			}
		}
		for nid := range laneOfNode {
			p := l.Placements[nid]
			s, e := p.DrawX, p.DrawX+p.DrawWidth
			if vertical {
				s, e = p.DrawY, p.DrawY+p.DrawHeight
			}
			if s < lo || e > hi {
				t.Errorf("%s: %s spans %d..%d on the flow axis, outside %d..%d", dir, nid, s, e, lo, hi)
			}
		}
	}
}

func TestLanesDoNotOverlap(t *testing.T) {
	for dir, l := range laneLayouts(t) {
		b := boundsByID(l)
		if overlaps(b["cust"], b["wh"]) {
			t.Errorf("%s: cust %+v overlaps wh %+v", dir, b["cust"], b["wh"])
		}
	}
}

// The bands are ordered by declaration, and the flow axis ignores them: a
// node's layer is its position along the flow whatever lane holds it.
func TestLaneBandsFollowDeclarationOrder(t *testing.T) {
	for dir, l := range laneLayouts(t) {
		b := boundsByID(l)
		cust, wh := b["cust"], b["wh"]
		if dir == "TB" {
			if cust.X >= wh.X {
				t.Errorf("TB: cust at x=%d is not left of wh at x=%d", cust.X, wh.X)
			}
		} else if cust.Y >= wh.Y {
			t.Errorf("LR: cust at y=%d is not above wh at y=%d", cust.Y, wh.Y)
		}
		flow := func(nid string) int {
			p := l.Placements[nid]
			if dir == "TB" {
				return p.Grid.Row
			}
			return p.Grid.Col
		}
		for _, pair := range [][2]string{{"A", "B"}, {"B", "C"}, {"C", "D"}} {
			if flow(pair[0]) >= flow(pair[1]) {
				t.Errorf("%s: %s is not before %s on the flow axis", dir, pair[0], pair[1])
			}
		}
	}
}

// A node no lane declared goes to a trailing lane of its own, reported.
func TestNodeOutsideEveryLane(t *testing.T) {
	g := parser.ParseSwimlane(`swimlane-beta TB
    subgraph cust["Customer"]
        A[Place order]
    end
    B[Loose]
    A --> B`)
	l := ComputeLayout(g, 4, 2, 0)
	if len(l.Warnings) != 1 || !strings.Contains(l.Warnings[0], "B") {
		t.Errorf("warnings = %v, want one naming B", l.Warnings)
	}
	if len(l.SubgraphBounds) != 2 {
		t.Fatalf("lane boxes = %d, want 2", len(l.SubgraphBounds))
	}
	var rest *SubgraphBounds
	for i := range l.SubgraphBounds {
		if l.SubgraphBounds[i].Subgraph.ID == "" {
			rest = &l.SubgraphBounds[i]
		}
	}
	if rest == nil {
		t.Fatalf("no trailing lane in %+v", l.SubgraphBounds)
	}
	if rest.Subgraph.Label != "" {
		t.Errorf("the trailing lane is labelled %q", rest.Subgraph.Label)
	}
	p := l.Placements["B"]
	node := SubgraphBounds{X: p.DrawX, Y: p.DrawY, Width: p.DrawWidth, Height: p.DrawHeight}
	if !contains(*rest, node) {
		t.Errorf("B at %+v is outside the trailing lane %+v", node, *rest)
	}
	if overlaps(*rest, boundsByID(l)["cust"]) {
		t.Errorf("the trailing lane %+v overlaps cust", *rest)
	}
}

// A subgraph nested in a lane is laid out by ADR-103's recursion inside the
// band, so its box lies in its lane's.
func TestNestedSubgraphStaysInItsLane(t *testing.T) {
	g := parser.ParseSwimlane(`swimlane-beta TB
    subgraph one["One"]
        subgraph inner["Inner"]
            A --> B
        end
    end
    subgraph two["Two"]
        C --> D
    end
    B --> C`)
	l := ComputeLayout(g, 4, 2, 0)
	b := boundsByID(l)
	if !contains(b["one"], b["inner"]) {
		t.Errorf("inner %+v is not inside one %+v", b["inner"], b["one"])
	}
	if overlaps(b["inner"], b["two"]) {
		t.Errorf("inner %+v overlaps two %+v", b["inner"], b["two"])
	}
}
