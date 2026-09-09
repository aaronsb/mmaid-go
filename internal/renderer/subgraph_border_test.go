package renderer

import (
	"os"
	"testing"

	"github.com/aaronsb/mmaid-go/internal/glyph"
	"github.com/aaronsb/mmaid-go/internal/layout"
	"github.com/aaronsb/mmaid-go/internal/parser"
)

// TestSubgraphBorderCrossingsAreTees renders the subgraph-cross fixture and
// checks that no subgraph border cell resolves to a four-arm glyph: an edge
// crosses a border at a tee, never at a cross.
func TestSubgraphBorderCrossingsAreTees(t *testing.T) {
	src, err := os.ReadFile("../../testdata/fixtures/subgraph-cross.mmd")
	if err != nil {
		t.Fatal(err)
	}
	g := parser.ParseFlowchart(string(src))
	const paddingX, paddingY = 4, 2
	l := layout.ComputeLayout(g, paddingX, paddingY, 0)
	canvas := RenderGraphCanvas(g, UNICODE, paddingX, paddingY, true, 0)
	if canvas == nil {
		t.Fatal("no canvas")
	}
	if len(l.SubgraphBounds) == 0 {
		t.Fatal("no subgraph bounds")
	}

	crossings, checked := 0, 0
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
				checked++
				arms := canvas.Arms(row, col)
				if arms == glyph.N|glyph.S|glyph.E|glyph.W {
					t.Errorf("subgraph %s border cell (%d,%d) is a four-arm glyph", sb.Subgraph.ID, row, col)
				}
				var border glyph.Arms
				if onH {
					border = glyph.Horizontal
				} else {
					border = glyph.Vertical
				}
				if !(onH && onV) && arms&^border != 0 {
					crossings++
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no border cells checked")
	}
	if crossings == 0 {
		t.Fatal("no edge crosses a subgraph border")
	}
}
