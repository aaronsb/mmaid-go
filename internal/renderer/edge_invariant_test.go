package renderer

import (
	"os"
	"testing"

	"github.com/aaronsb/mmaid-go/internal/glyph"
	"github.com/aaronsb/mmaid-go/internal/layout"
	"github.com/aaronsb/mmaid-go/internal/parser"
	"github.com/aaronsb/mmaid-go/internal/routing"
)

// TestEdgeInvariant renders the flowchart-cross fixture and checks that no
// node border cell other than an attach cell carries an arm the border did
// not draw itself. An edge may add an arm at its source attach cell and, when
// it ends without an arrowhead, at its target attach cell.
func TestEdgeInvariant(t *testing.T) {
	src, err := os.ReadFile("../../testdata/fixtures/flowchart-cross.mmd")
	if err != nil {
		t.Fatal(err)
	}
	g := parser.ParseFlowchart(string(src))
	const paddingX, paddingY = 4, 2
	l := layout.ComputeLayout(g, paddingX, paddingY, 0)
	routed := routing.RouteEdges(g, l)
	canvas := RenderGraphCanvas(g, false, paddingX, paddingY, true, 0)
	if canvas == nil {
		t.Fatal("no canvas")
	}

	attach := map[routing.Point]bool{}
	for _, re := range routed {
		p := re.DrawPath
		if len(p) < 2 {
			continue
		}
		attach[p[0]] = true
		if !re.Edge.HasArrowEnd {
			attach[p[len(p)-1]] = true
		}
	}

	checked := 0
	for _, nid := range g.NodeOrder {
		p := l.Placements[nid]
		x, y, w, h := p.DrawX, p.DrawY, p.DrawWidth, p.DrawHeight
		x2, y2 := x+w-1, y+h-1
		for row := y; row <= y2; row++ {
			for col := x; col <= x2; col++ {
				onV := col == x || col == x2
				onH := row == y || row == y2
				if !onV && !onH {
					continue
				}
				if attach[routing.Point{Col: col, Row: row}] {
					continue
				}
				var want glyph.Arms
				if onH {
					want |= glyph.Horizontal
				}
				if onV {
					want |= glyph.Vertical
				}
				switch {
				case row == y && col == x:
					want = glyph.TopLeft
				case row == y && col == x2:
					want = glyph.TopRight
				case row == y2 && col == x:
					want = glyph.BottomLeft
				case row == y2 && col == x2:
					want = glyph.BottomRight
				}
				if got := canvas.Arms(row, col); got != want {
					t.Errorf("node %s border cell (%d,%d): arms %04b, want %04b", nid, row, col, got, want)
				}
				checked++
			}
		}
	}
	if checked == 0 {
		t.Fatal("no border cells checked")
	}
	if len(attach) == 0 {
		t.Fatal("no attach cells")
	}
}
