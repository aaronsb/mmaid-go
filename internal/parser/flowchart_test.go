package parser

import (
	"testing"

	"github.com/aaronsb/mmaid-go/internal/graph"
)

func TestParseSimpleLR(t *testing.T) {
	g := ParseFlowchart("graph LR\n  A --> B")
	if g.Direction != graph.DirLR {
		t.Errorf("expected LR, got %s", g.Direction)
	}
	if len(g.Nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(g.Nodes))
	}
	if len(g.Edges) != 1 {
		t.Errorf("expected 1 edge, got %d", len(g.Edges))
	}
}

func TestParseDirections(t *testing.T) {
	tests := []struct {
		input string
		want  graph.Direction
	}{
		{"graph TB\n  A --> B", graph.DirTB},
		{"graph TD\n  A --> B", graph.DirTD},
		{"graph LR\n  A --> B", graph.DirLR},
		{"graph BT\n  A --> B", graph.DirBT},
		{"graph RL\n  A --> B", graph.DirRL},
		{"flowchart LR\n  A --> B", graph.DirLR},
	}
	for _, tt := range tests {
		g := ParseFlowchart(tt.input)
		if g.Direction != tt.want {
			t.Errorf("input %q: expected %s, got %s", tt.input, tt.want, g.Direction)
		}
	}
}

func TestParseNodeShapes(t *testing.T) {
	tests := []struct {
		input string
		shape graph.NodeShape
	}{
		{"graph LR\n  A[rect]", graph.ShapeRectangle},
		{"graph LR\n  A(round)", graph.ShapeRounded},
		{"graph LR\n  A{diamond}", graph.ShapeDiamond},
		{"graph LR\n  A((circle))", graph.ShapeCircle},
		{"graph LR\n  A([stadium])", graph.ShapeStadium},
		{"graph LR\n  A[[sub]]", graph.ShapeSubroutine},
		{"graph LR\n  A{{hex}}", graph.ShapeHexagon},
		{"graph LR\n  A[(cyl)]", graph.ShapeCylinder},
	}
	for _, tt := range tests {
		g := ParseFlowchart(tt.input)
		if n, ok := g.Nodes["A"]; !ok {
			t.Errorf("input %q: node A not found", tt.input)
		} else if n.Shape != tt.shape {
			t.Errorf("input %q: expected shape %d, got %d", tt.input, tt.shape, n.Shape)
		}
	}
}

func TestParseEdgeStyles(t *testing.T) {
	tests := []struct {
		input string
		style graph.EdgeStyle
	}{
		{"graph LR\n  A --> B", graph.EdgeSolid},
		{"graph LR\n  A -.-> B", graph.EdgeDotted},
		{"graph LR\n  A ==> B", graph.EdgeThick},
	}
	for _, tt := range tests {
		g := ParseFlowchart(tt.input)
		if len(g.Edges) != 1 {
			t.Errorf("input %q: expected 1 edge, got %d", tt.input, len(g.Edges))
			continue
		}
		if g.Edges[0].Style != tt.style {
			t.Errorf("input %q: expected style %d, got %d", tt.input, tt.style, g.Edges[0].Style)
		}
	}
}

func TestParseEdgeLabel(t *testing.T) {
	g := ParseFlowchart("graph LR\n  A -->|yes| B")
	if len(g.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(g.Edges))
	}
	if g.Edges[0].Label != "yes" {
		t.Errorf("expected label 'yes', got %q", g.Edges[0].Label)
	}
}

func TestParseChainedArrows(t *testing.T) {
	g := ParseFlowchart("graph LR\n  A --> B --> C")
	if len(g.Nodes) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(g.Nodes))
	}
	if len(g.Edges) != 2 {
		t.Errorf("expected 2 edges, got %d", len(g.Edges))
	}
}

func TestParseAmpersand(t *testing.T) {
	g := ParseFlowchart("graph LR\n  A & B --> C")
	if len(g.Nodes) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(g.Nodes))
	}
	if len(g.Edges) != 2 {
		t.Errorf("expected 2 edges (A->C, B->C), got %d", len(g.Edges))
	}
}

func TestParseSubgraph(t *testing.T) {
	g := ParseFlowchart(`graph TB
    subgraph sg1 [My Group]
        A --> B
    end`)
	if len(g.Subgraphs) != 1 {
		t.Fatalf("expected 1 subgraph, got %d", len(g.Subgraphs))
	}
	if g.Subgraphs[0].Label != "My Group" {
		t.Errorf("expected label 'My Group', got %q", g.Subgraphs[0].Label)
	}
	if len(g.Subgraphs[0].NodeIDs) != 2 {
		t.Errorf("expected 2 nodes in subgraph, got %d", len(g.Subgraphs[0].NodeIDs))
	}
}

func TestParseClassDef(t *testing.T) {
	g := ParseFlowchart("graph LR\n  classDef red fill:#f00,color:#fff\n  A:::red --> B")
	if _, ok := g.ClassDefs["red"]; !ok {
		t.Error("classDef 'red' not found")
	}
	if n, ok := g.Nodes["A"]; !ok {
		t.Error("node A not found")
	} else if n.StyleClass != "red" {
		t.Errorf("expected style class 'red', got %q", n.StyleClass)
	}
}

func TestParseSemicolons(t *testing.T) {
	g := ParseFlowchart("graph LR; A --> B; B --> C; C --> D")
	if len(g.Nodes) != 4 {
		t.Errorf("expected 4 nodes, got %d", len(g.Nodes))
	}
	if len(g.Edges) != 3 {
		t.Errorf("expected 3 edges, got %d", len(g.Edges))
	}
}

func TestParseComments(t *testing.T) {
	g := ParseFlowchart("graph LR\n  A --> B %% this is a comment\n  B --> C")
	if len(g.Edges) != 2 {
		t.Errorf("expected 2 edges (comment stripped), got %d", len(g.Edges))
	}
}

func TestParseBidirectional(t *testing.T) {
	g := ParseFlowchart("graph LR\n  A <--> B")
	if len(g.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(g.Edges))
	}
	e := g.Edges[0]
	if !e.HasArrowStart || !e.HasArrowEnd {
		t.Error("expected bidirectional edge")
	}
}

func TestParseNodeLabel(t *testing.T) {
	g := ParseFlowchart("graph LR\n  A[Hello World]")
	if n, ok := g.Nodes["A"]; !ok {
		t.Error("node A not found")
	} else if n.Label != "Hello World" {
		t.Errorf("expected label 'Hello World', got %q", n.Label)
	}
}

func TestParseLinkStyle(t *testing.T) {
	g := ParseFlowchart("graph LR\n  A --> B\n  linkStyle 0 stroke:#ff0")
	if _, ok := g.LinkStyles[0]; !ok {
		t.Error("linkStyle 0 not found")
	}
}

func TestParseClick(t *testing.T) {
	g := ParseFlowchart(`graph LR
  A[One] --> B[Two]
  C[Three]
  D[Four]
  click A "https://example.com/a"
  click B href "https://example.com/b" _blank
  click C callback "a tooltip"
`)
	for id, want := range map[string]string{
		"A": "https://example.com/a",
		"B": "https://example.com/b",
		"C": "",
		"D": "",
	} {
		node, ok := g.Nodes[id]
		if !ok {
			t.Fatalf("node %s not found", id)
		}
		if node.Link != want {
			t.Errorf("node %s link = %q, want %q", id, node.Link, want)
		}
	}
}

// TestParseClickRejectsUnsafeURL keeps a diagram from steering the terminal
// through the OSC 8 sequence a link is emitted in.
func TestParseClickRejectsUnsafeURL(t *testing.T) {
	unsafe := map[string]string{
		"escape":     "https://x/\x1b]0;PWNED\x07\x1b]52;c;cGduZWQ=\x07",
		"bel":        "https://x/\x07",
		"newline":    "https://x/\nclick",
		"delete":     "https://x/\x7f",
		"javascript": "javascript:alert(1)",
		"data":       "data:text/html,<script>alert(1)</script>",
		"schemeless": "example.com/docs",
	}
	for name, url := range unsafe {
		t.Run(name, func(t *testing.T) {
			g := ParseFlowchart("graph LR\n  A[One]\n  click A \"" + url + "\"")
			if link := g.Nodes["A"].Link; link != "" {
				t.Errorf("link = %q, want none", link)
			}
		})
	}

	for _, url := range []string{
		"https://example.com/a",
		"http://example.com/a",
		"mailto:someone@example.com",
		"file:///tmp/notes.md",
		"HTTPS://EXAMPLE.COM/A",
	} {
		g := ParseFlowchart("graph LR\n  A[One]\n  click A \"" + url + "\"")
		if g.Nodes["A"].Link != url {
			t.Errorf("link for %q = %q, want it kept", url, g.Nodes["A"].Link)
		}
	}
}

func TestSanitizeLabel_StripsControlChars(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"normal text", "normal text"},
		{"has\ttab", "has tab"},
		{"has\x1b[31mANSI\x1b[0m", "hasANSI"},
		{"null\x00byte", "nullbyte"},
		{"line\nbreak", "linebreak"},    // real newline stripped by stripControlChars
		{`line\nbreak`, "line break"},   // literal \n replaced by sanitizeLabel
		{"html<br>break", "html break"}, // <br> → space
	}
	for _, tt := range tests {
		got := sanitizeLabel(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeLabel(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestStripControlChars(t *testing.T) {
	// Full ANSI escape sequences should be stripped entirely.
	input := "before\x1b[31mred\x1b[0mafter"
	got := stripControlChars(input)
	want := "beforeredafter"
	if got != want {
		t.Errorf("stripControlChars(%q) = %q, want %q", input, got, want)
	}
}

// A subgraph id needs no space before its bracketed label; that is how
// the swimlane docs declare a lane.
func TestParseSubgraphBracketWithoutSpace(t *testing.T) {
	for _, src := range []string{`graph TB
    subgraph cust["Customer"]
        A
    end`, `graph TB
    subgraph cust ["Customer"]
        A
    end`} {
		sg := ParseFlowchart(src).Subgraphs[0]
		if sg.ID != "cust" || sg.Label != "Customer" {
			t.Errorf("%q: got %q/%q, want cust/Customer", src, sg.ID, sg.Label)
		}
	}
}
