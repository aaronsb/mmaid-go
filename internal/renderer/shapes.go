package renderer

import (
	"strings"

	"github.com/aaronsb/mmaid-go/internal/glyph"
	"github.com/aaronsb/mmaid-go/internal/graph"
	"github.com/aaronsb/mmaid-go/internal/textwidth"
)

// drawLabel centers multi-line text inside a shape region.
// Lines are split on "\n" or the literal two-character sequence "\\n".
func drawLabel(c *Canvas, x, y, width, height int, label string, style string) {
	labelStyle := ""
	if style != "" {
		labelStyle = "label"
	}

	var lines []string
	if strings.Contains(label, "\n") {
		lines = strings.Split(label, "\n")
	} else if strings.Contains(label, `\n`) {
		lines = strings.Split(label, `\n`)
	} else {
		lines = []string{label}
	}

	startRow := y + (height-len(lines))/2
	for i, line := range lines {
		row := startRow + i
		col := x + (width-textwidth.String(line))/2
		if row >= 0 && row < c.Height {
			c.PutText(row, col, line, labelStyle)
		}
	}
}

// fillInterior sets the style on all interior cells of a box (for background-color themes).
func fillInterior(c *Canvas, x, y, width, height int, style string) {
	for row := y + 1; row < y+height-1; row++ {
		for col := x + 1; col < x+width-1; col++ {
			c.SetStyle(row, col, style)
		}
	}
}

// drawBorder draws a rectangle as four segments; the corners come from the
// arms that meet there.
func drawBorder(c *Canvas, x, y, width, height int, rounded bool, style string) {
	x2, y2 := x+width-1, y+height-1
	c.Segment(y, x, y, x2, glyph.Light, rounded, style)
	c.Segment(y2, x, y2, x2, glyph.Light, rounded, style)
	c.Segment(y, x, y2, x, glyph.Light, rounded, style)
	c.Segment(y, x2, y2, x2, glyph.Light, rounded, style)
}

// drawCorners writes four literal runes over the corners of a border.
func drawCorners(c *Canvas, x, y, width, height int, tl, tr, bl, br rune, style string) {
	c.Put(y, x, tl, style)
	c.Put(y, x+width-1, tr, style)
	c.Put(y+height-1, x, bl, style)
	c.Put(y+height-1, x+width-1, br, style)
}

// shapeIndicator places a small shape-type symbol inside the upper-left corner.
func shapeIndicator(c *Canvas, x, y int, indicator rune, style string) {
	c.Put(y+1, x+1, indicator, style)
}

// DrawRectangle draws a standard box with corners, horizontal/vertical borders,
// and a centered label.
func DrawRectangle(c *Canvas, x, y, width, height int, label string, cs CharSet, style string) {
	drawBorder(c, x, y, width, height, false, style)
	fillInterior(c, x, y, width, height, style)
	drawLabel(c, x, y, width, height, label, style)
}

// DrawRounded draws a box with rounded corners and a centered label.
func DrawRounded(c *Canvas, x, y, width, height int, label string, cs CharSet, style string) {
	drawBorder(c, x, y, width, height, true, style)
	fillInterior(c, x, y, width, height, style)
	drawLabel(c, x, y, width, height, label, style)
	shapeIndicator(c, x, y, '◦', style) // rounded
}

// DrawStadium draws a stadium shape: rounded top/bottom with parentheses on sides.
func DrawStadium(c *Canvas, x, y, width, height int, label string, cs CharSet, style string) {
	drawBorder(c, x, y, width, height, true, style)
	for row := y + 1; row < y+height-1; row++ {
		c.Put(row, x, '(', style)
		c.Put(row, x+width-1, ')', style)
	}

	fillInterior(c, x, y, width, height, style)
	drawLabel(c, x, y, width, height, label, style)
	shapeIndicator(c, x, y, '⊂', style) // stadium
}

// DrawSubroutine draws a rectangle with inner vertical lines at x+1 and
// x+width-2, tee'd into the top and bottom borders.
func DrawSubroutine(c *Canvas, x, y, width, height int, label string, cs CharSet, style string) {
	drawBorder(c, x, y, width, height, false, style)
	c.Segment(y, x+1, y+height-1, x+1, glyph.Light, false, style)
	c.Segment(y, x+width-2, y+height-1, x+width-2, glyph.Light, false, style)

	fillInterior(c, x, y, width, height, style)
	drawLabel(c, x, y, width, height, label, style)
	shapeIndicator(c, x, y+1, '‖', style) // subroutine (offset since inner borders at x+1)
}

// DrawDiamond draws a diamond shape with chamfered corners.
func DrawDiamond(c *Canvas, x, y, width, height int, label string, cs CharSet, style string) {
	drawBorder(c, x, y, width, height, false, style)
	drawCorners(c, x, y, width, height, cs.Slash, cs.Backslash, cs.Backslash, cs.Slash, style)

	fillInterior(c, x, y, width, height, style)
	drawLabel(c, x, y, width, height, label, style)
	shapeIndicator(c, x, y, cs.Diamond, style)
}

// DrawHexagon draws a hexagon shape with / \ top corners and \ / bottom corners.
func DrawHexagon(c *Canvas, x, y, width, height int, label string, cs CharSet, style string) {
	drawBorder(c, x, y, width, height, false, style)
	drawCorners(c, x, y, width, height, '/', '\\', '\\', '/', style)

	fillInterior(c, x, y, width, height, style)
	drawLabel(c, x, y, width, height, label, style)
	shapeIndicator(c, x, y, cs.Hexagon, style)
}

// DrawCircle draws a rounded box with circle markers at the top and bottom center.
func DrawCircle(c *Canvas, x, y, width, height int, label string, cs CharSet, style string) {
	cx := x + width/2

	drawBorder(c, x, y, width, height, true, style)
	c.Put(y, cx, cs.Ring, style)
	c.Put(y+height-1, cx, cs.Ring, style)

	fillInterior(c, x, y, width, height, style)
	drawLabel(c, x, y, width, height, label, style)
	shapeIndicator(c, x, y, cs.CircleEndpoint, style)
}

// DrawDoubleCircle draws a rounded box with an inner border.
func DrawDoubleCircle(c *Canvas, x, y, width, height int, label string, cs CharSet, style string) {
	drawBorder(c, x, y, width, height, true, style)
	if width > 2 && height > 2 {
		drawBorder(c, x+1, y+1, width-2, height-2, true, style)
	}

	fillInterior(c, x, y, width, height, style)
	drawLabel(c, x, y, width, height, label, style)
	shapeIndicator(c, x+1, y+1, cs.DoubleRing, style) // offset for the inner border
}

// DrawAsymmetric draws a flag shape: > on the left side, straight right side.
func DrawAsymmetric(c *Canvas, x, y, width, height int, label string, cs CharSet, style string) {
	cy := y + height/2

	drawBorder(c, x, y, width, height, false, style)

	// Left side: \ above center, > at center, / below center
	for row := y; row < y+height; row++ {
		if row < cy {
			c.Put(row, x, '\\', style)
		} else if row == cy {
			c.Put(row, x, '>', style)
		} else {
			c.Put(row, x, '/', style)
		}
	}

	fillInterior(c, x, y, width, height, style)
	drawLabel(c, x, y, width, height, label, style)
}

// DrawCylinder draws a cylinder: a rounded box whose top ellipse closes on
// the second row.
func DrawCylinder(c *Canvas, x, y, width, height int, label string, cs CharSet, style string) {
	drawBorder(c, x, y, width, height, true, style)
	if height > 2 {
		c.Segment(y+1, x, y+1, x+width-1, glyph.Light, true, style)
	}

	fillInterior(c, x, y, width, height, style)
	drawLabel(c, x, y, width, height, label, style)
}

// DrawTrapezoid draws a trapezoid: / top-left, \ top-right, \ bottom-left, / bottom-right.
func DrawTrapezoid(c *Canvas, x, y, width, height int, label string, cs CharSet, style string) {
	drawBorder(c, x, y, width, height, false, style)
	drawCorners(c, x, y, width, height, '/', '\\', '\\', '/', style)

	fillInterior(c, x, y, width, height, style)
	drawLabel(c, x, y, width, height, label, style)
}

// DrawTrapezoidAlt draws an inverted trapezoid: \ top-left, / top-right,
// / bottom-left, \ bottom-right.
func DrawTrapezoidAlt(c *Canvas, x, y, width, height int, label string, cs CharSet, style string) {
	drawBorder(c, x, y, width, height, false, style)
	drawCorners(c, x, y, width, height, '\\', '/', '/', '\\', style)

	fillInterior(c, x, y, width, height, style)
	drawLabel(c, x, y, width, height, label, style)
}

// DrawParallelogram draws a parallelogram with / on all four corners.
func DrawParallelogram(c *Canvas, x, y, width, height int, label string, cs CharSet, style string) {
	drawBorder(c, x, y, width, height, false, style)
	drawCorners(c, x, y, width, height, '/', '/', '/', '/', style)

	fillInterior(c, x, y, width, height, style)
	drawLabel(c, x, y, width, height, label, style)
}

// DrawParallelogramAlt draws a parallelogram with \ on all four corners.
func DrawParallelogramAlt(c *Canvas, x, y, width, height int, label string, cs CharSet, style string) {
	drawBorder(c, x, y, width, height, false, style)
	drawCorners(c, x, y, width, height, '\\', '\\', '\\', '\\', style)

	fillInterior(c, x, y, width, height, style)
	drawLabel(c, x, y, width, height, label, style)
}

// DrawStartState draws a filled circle marker at the center of the region.
func DrawStartState(c *Canvas, x, y, width, height int, label string, cs CharSet, style string) {
	c.Put(y+height/2, x+width/2, cs.Dot, style)
}

// DrawEndState draws a bullseye marker at the center of the region.
func DrawEndState(c *Canvas, x, y, width, height int, label string, cs CharSet, style string) {
	c.Put(y+height/2, x+width/2, cs.Bullseye, style)
}

// DrawForkJoin fills the entire area with thick horizontal lines.
func DrawForkJoin(c *Canvas, x, y, width, height int, label string, cs CharSet, style string) {
	ch := cs.Rune(glyph.Horizontal, glyph.Heavy)
	for row := y; row < y+height; row++ {
		for col := x; col < x+width; col++ {
			c.Put(row, col, ch, style)
		}
	}
}

// DrawJunction draws nothing. An architecture junction is the one cell its
// edges meet in, and the arms they leave there resolve to a tee or a cross.
func DrawJunction(c *Canvas, x, y, width, height int, label string, cs CharSet, style string) {
}

// ShapeRenderers maps each NodeShape constant to its renderer function.
var ShapeRenderers = map[graph.NodeShape]func(*Canvas, int, int, int, int, string, CharSet, string){
	graph.ShapeRectangle:        DrawRectangle,
	graph.ShapeRounded:          DrawRounded,
	graph.ShapeStadium:          DrawStadium,
	graph.ShapeSubroutine:       DrawSubroutine,
	graph.ShapeDiamond:          DrawDiamond,
	graph.ShapeHexagon:          DrawHexagon,
	graph.ShapeCircle:           DrawCircle,
	graph.ShapeDoubleCircle:     DrawDoubleCircle,
	graph.ShapeAsymmetric:       DrawAsymmetric,
	graph.ShapeCylinder:         DrawCylinder,
	graph.ShapeParallelogram:    DrawParallelogram,
	graph.ShapeParallelogramAlt: DrawParallelogramAlt,
	graph.ShapeTrapezoid:        DrawTrapezoid,
	graph.ShapeTrapezoidAlt:     DrawTrapezoidAlt,
	graph.ShapeStartState:       DrawStartState,
	graph.ShapeEndState:         DrawEndState,
	graph.ShapeForkJoin:         DrawForkJoin,
	graph.ShapeJunction:         DrawJunction,
}
