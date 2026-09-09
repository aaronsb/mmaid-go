package parser

import (
	"testing"

	"github.com/aaronsb/mmaid-go/internal/graph"
)

func TestParseSwimlaneLanes(t *testing.T) {
	g := ParseSwimlane(`swimlane-beta TB
    subgraph cust["Customer"]
        A[Place order]
    end
    subgraph wh["Warehouse"]
        B[Pick items]
    end
    A --> B`)
	if !g.Lanes {
		t.Error("Lanes is false")
	}
	if len(g.Subgraphs) != 2 {
		t.Fatalf("lanes = %d, want 2", len(g.Subgraphs))
	}
	for i, want := range []struct{ id, label, node string }{
		{"cust", "Customer", "A"},
		{"wh", "Warehouse", "B"},
	} {
		sg := g.Subgraphs[i]
		if sg.ID != want.id || sg.Label != want.label {
			t.Errorf("lane %d = %q/%q, want %q/%q", i, sg.ID, sg.Label, want.id, want.label)
		}
		if len(sg.NodeIDs) != 1 || sg.NodeIDs[0] != want.node {
			t.Errorf("lane %s holds %v, want [%s]", sg.ID, sg.NodeIDs, want.node)
		}
	}
	if len(g.Edges) != 1 {
		t.Errorf("edges = %d, want 1", len(g.Edges))
	}
}

func TestParseSwimlaneDirections(t *testing.T) {
	tests := []struct {
		input string
		want  graph.Direction
	}{
		{"swimlane-beta TB\n  A --> B", graph.DirTB},
		{"swimlane-beta TD\n  A --> B", graph.DirTD},
		{"swimlane-beta LR\n  A --> B", graph.DirLR},
		{"swimlane-beta BT\n  A --> B", graph.DirBT},
		{"swimlane-beta RL\n  A --> B", graph.DirRL},
		{"swimlane LR\n  A --> B", graph.DirLR},
		{"swimlane-beta\n  A --> B", graph.DirTB},
	}
	for _, tt := range tests {
		g := ParseSwimlane(tt.input)
		if g.Direction != tt.want {
			t.Errorf("%q: direction = %s, want %s", tt.input, g.Direction, tt.want)
		}
		if !g.Lanes {
			t.Errorf("%q: Lanes is false", tt.input)
		}
	}
}

func TestParseFlowchartIsNotLanes(t *testing.T) {
	if ParseFlowchart("graph TB\n  subgraph S\n    A\n  end").Lanes {
		t.Error("a flowchart came back flagged as lanes")
	}
}
