package diagram

import (
	"os"
	"testing"

	"github.com/aaronsb/mmaid-go/internal/glyph"
	"github.com/aaronsb/mmaid-go/internal/graph"
	"github.com/aaronsb/mmaid-go/internal/layout"
	"github.com/aaronsb/mmaid-go/internal/renderer"
)

// TestBoundaryCrossingsAreTees renders the C4 and use case fixtures, whose
// boundaries are subgraphs, and checks that no boundary border cell
// resolves to a four-arm glyph.
func TestBoundaryCrossingsAreTees(t *testing.T) {
	cases := map[string]func(string) *graph.Graph{
		"../../testdata/fixtures/c4.mmd":      ParseC4Diagram,
		"../../testdata/fixtures/usecase.mmd": ParseUseCaseDiagram,
	}
	for path, parse := range cases {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		g := parse(string(src))
		const paddingX, paddingY, width = 4, 2, 120
		canvas := renderer.RenderGraphCanvas(g, renderer.UNICODE, paddingX, paddingY, true, width)
		l := layout.ComputeLayout(g, paddingX, paddingY, width*75/100)
		if canvas == nil || len(l.SubgraphBounds) == 0 {
			t.Fatalf("%s: nothing to check", path)
		}
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
						t.Errorf("%s: %s border cell (%d,%d) is a four-arm glyph", path, sb.Subgraph.ID, row, col)
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
		if crossings == 0 {
			t.Errorf("%s: no edge crosses a boundary", path)
		}
	}
}
