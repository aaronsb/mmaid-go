package renderer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aaronsb/mmaid-go/internal/glyph"
	"github.com/aaronsb/mmaid-go/internal/graph"
	"github.com/aaronsb/mmaid-go/internal/layout"
	"github.com/aaronsb/mmaid-go/internal/parser"
)

// CheckSubgraphBorders renders a graph and reports every subgraph border
// cell that resolves to a four-arm glyph; an edge crosses a border at a
// tee, never at a cross. It returns the number of crossings seen.
func CheckSubgraphBorders(t *testing.T, name string, g *graph.Graph) int {
	t.Helper()
	// The golden harness renders at width 120; the canvas may flip an
	// overflowing LR graph, so the layout is computed after it with the
	// same width hint.
	const paddingX, paddingY, width = 4, 2, 120
	canvas := RenderGraphCanvas(g, UNICODE, paddingX, paddingY, true, width)
	if canvas == nil {
		t.Fatalf("%s: no canvas", name)
	}
	l := layout.ComputeLayout(g, paddingX, paddingY, width*graphFillPercent/100)
	if len(l.SubgraphBounds) == 0 {
		t.Fatalf("%s: no subgraph bounds", name)
	}
	failed := false
	defer func() {
		if failed {
			t.Logf("%s:\n%s", name, canvas.ToString())
		}
	}()

	crossings := 0
	for _, sb := range l.SubgraphBounds {
		x0, y0 := sb.X, sb.Y
		x1, y1 := sb.X+sb.Width-1, sb.Y+sb.Height-1
		for row := y0; row <= y1; row++ {
			for col := x0; col <= x1; col++ {
				onV := col == x0 || col == x1
				onH := row == y0 || row == y1
				if !onV && !onH {
					continue
				}
				arms := canvas.Arms(row, col)
				if arms == glyph.N|glyph.S|glyph.E|glyph.W {
					failed = true
					t.Errorf("%s: subgraph %s border cell (%d,%d) is a four-arm glyph", name, sb.Subgraph.ID, row, col)
				}
				border := glyph.Vertical
				if onH {
					border = glyph.Horizontal
				}
				if !(onH && onV) && arms&^border != 0 {
					crossings++
				}
			}
		}
	}
	return crossings
}

// TestSubgraphBorderCrossingsAreTees runs the border check over every
// subgraph fixture and over graphs whose edges contest one border cell.
func TestSubgraphBorderCrossingsAreTees(t *testing.T) {
	paths, err := filepath.Glob("../../testdata/fixtures/subgraph-*.mmd")
	if err != nil || len(paths) == 0 {
		t.Fatalf("no subgraph fixtures: %v", err)
	}
	total := 0
	for _, p := range paths {
		src, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		total += CheckSubgraphBorders(t, filepath.Base(p), parser.ParseFlowchart(string(src)))
	}
	if total == 0 {
		t.Fatal("no edge crosses a subgraph border")
	}

	scenarios := map[string]string{
		// Two port-anchored runs meet one cell of S0's right border from
		// opposite sides.
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
		// Two edges leave one subgraph and must take distinct border ports.
		"subgraph-source": `graph TB
    A --> S1
    subgraph S1
        B --> C
    end
    S1 --> D
    S1 --> S2
    subgraph S2
        E
    end`,
	}
	for name, src := range scenarios {
		if CheckSubgraphBorders(t, name, parser.ParseFlowchart(src)) == 0 {
			t.Errorf("%s: no edge crosses a subgraph border", name)
		}
	}
}
