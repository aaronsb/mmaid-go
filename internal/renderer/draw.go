package renderer

import (
	"strings"

	"github.com/aaronsb/mmaid-go/internal/glyph"
	"github.com/aaronsb/mmaid-go/internal/graph"
	"github.com/aaronsb/mmaid-go/internal/layout"
	"github.com/aaronsb/mmaid-go/internal/routing"
	"github.com/aaronsb/mmaid-go/internal/textwidth"
)

// graphFillPercent is the fraction of terminal width used for graph layouts.
// See terminal_scale.go GraphFillRatio for the rationale.
const graphFillPercent = 75

// RenderGraph renders a graph to a string.
func RenderGraph(g *graph.Graph, cs CharSet, paddingX, paddingY int, roundedEdges bool) string {
	canvas := RenderGraphCanvas(g, cs, paddingX, paddingY, roundedEdges, 0)
	if canvas == nil {
		return ""
	}
	return canvas.ToString()
}

// RenderGraphCanvas renders a graph to a Canvas.
// maxWidth, if > 0, hints the target canvas width for gap scaling.
func RenderGraphCanvas(g *graph.Graph, cs CharSet, paddingX, paddingY int, roundedEdges bool, maxWidth int) *Canvas {
	if len(g.NodeOrder) == 0 {
		return nil
	}

	// Handle BT/RL by rendering as TB/LR then flipping
	needFlipV := g.Direction == graph.DirBT
	needFlipH := g.Direction == graph.DirRL

	// Compute layout (Normalized direction is used internally)
	layoutMaxW := 0
	if maxWidth > 0 {
		layoutMaxW = maxWidth * graphFillPercent / 100
	}
	l := layout.ComputeLayout(g, paddingX, paddingY, layoutMaxW)

	// Auto-flip LR→TD if the layout overflows and direction wasn't explicit
	if maxWidth > 0 && l.CanvasWidth+4 > maxWidth && !g.DirectionExplicit &&
		g.Direction.IsHorizontal() {
		g.Direction = graph.DirTB
		needFlipV = false
		needFlipH = false
		l = layout.ComputeLayout(g, paddingX, paddingY, layoutMaxW)
	}

	// Route edges
	ellipsis := routing.Ellipsis
	if useASCII {
		ellipsis = "..."
	}
	routed := routing.RouteEdgesWith(g, l, ellipsis)

	// The canvas covers every layout column and row, so an edge routed
	// through the gap past the last node has cells to draw in, plus a
	// small margin.
	extentW, extentH := l.Extent()
	canvas := NewCanvas(max(l.CanvasWidth, extentW)+4, max(l.CanvasHeight, extentH)+4)
	canvas.SetCharSet(cs)

	// Draw subgraph borders (background layer)
	drawSubgraphBorders(canvas, l)

	// Draw nodes
	drawNodes(canvas, g, l, cs)

	// Draw edges (lines, arrows, labels)
	drawEdges(canvas, g, l, routed, cs, roundedEdges)

	// Draw subgraph labels (on top of borders)
	drawSubgraphLabels(canvas, l, cs)

	// Draw notes
	drawNotes(canvas, g, l, cs)

	// Flip if needed
	if needFlipV {
		canvas.FlipVertical()
	}
	if needFlipH {
		canvas.FlipHorizontal()
	}

	return canvas
}

// drawSubgraphBorders draws the borders around subgraph regions.
func drawSubgraphBorders(canvas *Canvas, l *layout.GridLayout) {
	for _, sb := range l.SubgraphBounds {
		x, y := sb.X, sb.Y
		w, h := sb.Width, sb.Height
		if w <= 0 || h <= 0 {
			continue
		}
		if x < 0 {
			x = 0
		}
		if y < 0 {
			y = 0
		}

		drawBorder(canvas, x, y, w, h, false, "subgraph")

		// Fill interior with background layer so themed renders
		// show a colored region. Content drawn on top keeps its fg.
		for r := y; r < y+h; r++ {
			endC := x + w
			if endC > canvas.Width {
				endC = canvas.Width
			}
			for c := x; c < endC; c++ {
				canvas.SetFill(r, c, "subgraph_fill")
			}
		}
	}
}

// drawSubgraphLabels draws labels inside subgraph borders.
func drawSubgraphLabels(canvas *Canvas, l *layout.GridLayout, cs CharSet) {
	for _, sb := range l.SubgraphBounds {
		x, y := sb.X, sb.Y
		w, h := sb.Width, sb.Height
		if w <= 0 || h <= 0 {
			continue
		}
		if x < 0 {
			x = 0
		}
		if y < 0 {
			y = 0
		}

		label := sb.Subgraph.Label
		if label == "" {
			label = sb.Subgraph.ID
		}
		if label != "" {
			_ = h // bounds already checked above
			canvas.PutText(y+1, x+2, label, "subgraph_label")
		}
	}
}

// drawNodes draws all nodes using the ShapeRenderers map.
func drawNodes(canvas *Canvas, g *graph.Graph, l *layout.GridLayout, cs CharSet) {
	for _, nid := range g.NodeOrder {
		node, ok := g.Nodes[nid]
		if !ok {
			continue
		}
		p, ok := l.Placements[nid]
		if !ok {
			continue
		}

		// Resolve style key: inline style > :::className > classDef default > "node"
		style := resolveNodeStyle(g, node)

		renderer, ok := ShapeRenderers[node.Shape]
		if !ok {
			renderer = DrawRectangle
		}

		// Text written while the link is current belongs to it, which is the
		// node's label and nothing else: borders and edges are not text.
		canvas.SetCurrentLink(node.Link)

		renderer(canvas, p.DrawX, p.DrawY, p.DrawWidth, p.DrawHeight, node.Label, cs, style)

		// If node has label segments, overwrite label with styled text
		if len(node.LabelSegments) > 0 {
			segments := labelSegmentsToStyled(node.LabelSegments)
			// Center the styled text in the node
			totalLen := 0
			for _, seg := range segments {
				totalLen += textwidth.String(seg.Text)
			}
			col := p.DrawX + (p.DrawWidth-totalLen)/2
			row := p.DrawY + p.DrawHeight/2
			canvas.PutStyledText(row, col, segments)
		}

		canvas.SetCurrentLink("")
	}
}

// resolveNodeStyle determines the style key for a node.
func resolveNodeStyle(g *graph.Graph, node *graph.Node) string {
	// Check for inline style
	if _, ok := g.NodeStyles[node.ID]; ok {
		return node.ID
	}
	// Check for :::className
	if node.StyleClass != "" {
		if _, ok := g.ClassDefs[node.StyleClass]; ok {
			return node.StyleClass
		}
	}
	// Check for classDef default
	if _, ok := g.ClassDefs["default"]; ok {
		return "default"
	}
	return "node"
}

// labelSegmentsToStyled converts graph.LabelSegment to StyledSegment.
func labelSegmentsToStyled(segments []graph.LabelSegment) []StyledSegment {
	styled := make([]StyledSegment, len(segments))
	for i, seg := range segments {
		style := "label"
		if seg.Bold && seg.Italic {
			style = "label_bold_italic"
		} else if seg.Bold {
			style = "label_bold"
		} else if seg.Italic {
			style = "label_italic"
		}
		styled[i] = StyledSegment{Text: seg.Text, Style: style}
	}
	return styled
}

// drawEdges draws all routed edges: lines, corners, arrows, T-junctions, and labels.
func drawEdges(canvas *Canvas, g *graph.Graph, l *layout.GridLayout, routed []routing.RoutedEdge, cs CharSet, roundedEdges bool) {
	// Pass 1a: Draw line segments
	for _, re := range routed {
		arrowEnd := re.Edge.HasArrowEnd && !isMarker(g.Nodes[re.Edge.Target])
		drawEdgeLines(canvas, re, roundedEdges, arrowEnd)
	}

	// Pass 1b: Draw arrows and source tees
	for _, re := range routed {
		drawEdgeEndpoints(canvas, re, g, l, cs)
	}

	// Pass 2: Draw edge labels
	for _, re := range routed {
		drawEdgeLabel(canvas, re)
	}
}

// edgeWeight returns the stroke for an edge style; ok is false for an
// invisible edge, which draws nothing.
func edgeWeight(style graph.EdgeStyle) (glyph.Weight, bool) {
	switch style {
	case graph.EdgeDotted:
		return glyph.Dashed, true
	case graph.EdgeThick:
		return glyph.Heavy, true
	case graph.EdgeInvisible:
		return glyph.Light, false
	default:
		return glyph.Light, true
	}
}

// sign returns -1, 0, or 1 for the sign of x.
func sign(x int) int {
	if x > 0 {
		return 1
	}
	if x < 0 {
		return -1
	}
	return 0
}

// drawEdgeLines draws a routed edge's path as segments (Pass 1a). Bends are
// cells where two segments meet. With arrowEnd, the last segment ends at the
// arrowhead's cell, one short of the node border, so the edge never writes
// an arm into the target node.
func drawEdgeLines(canvas *Canvas, re routing.RoutedEdge, roundedEdges, arrowEnd bool) {
	path := re.DrawPath
	if len(path) < 2 {
		return
	}

	edge := re.Edge
	w, ok := edgeWeight(edge.Style)
	if !ok {
		return
	}
	nSegs := len(path) - 1

	for i := 0; i < nSegs; i++ {
		x1, y1 := path[i].Col, path[i].Row
		x2, y2 := path[i+1].Col, path[i+1].Row
		dx, dy := sign(x2-x1), sign(y2-y1)

		if i == 0 && edge.HasArrowStart {
			x1, y1 = x1+dx, y1+dy
		}
		if i == nSegs-1 && arrowEnd {
			x2, y2 = x2-dx, y2-dy
		}

		canvas.Segment(y1, x1, y2, x2, w, roundedEdges, "edge")
	}
}

// drawEdgeEndpoints draws arrow heads and the source tee for a routed edge (Pass 1b).
func drawEdgeEndpoints(canvas *Canvas, re routing.RoutedEdge, g *graph.Graph, l *layout.GridLayout, cs CharSet) {
	path := re.DrawPath
	if len(path) < 2 {
		return
	}

	edge := re.Edge

	// Draw arrow at end (target)
	if edge.HasArrowEnd && len(path) >= 2 {
		from := path[len(path)-2]
		to := path[len(path)-1]
		// An end-state marker takes the line itself rather than an arrowhead.
		if isMarker(g.Nodes[edge.Target]) {
			markerThrough(canvas, to, from, edge.Style)
		} else {
			drawArrowHead(canvas, from, to, cs, edge.Style, edge.ArrowTypeEnd)
		}
	}

	// Draw arrow at start (source) - for bidirectional edges
	if edge.HasArrowStart && len(path) >= 2 {
		from := path[1]
		to := path[0]
		drawArrowHead(canvas, from, to, cs, edge.Style, edge.ArrowTypeStart)
	}

	// The source attach cell: a tee on a box border, a line running into a
	// state marker. An arrowhead at the start owns that end instead.
	if len(path) >= 2 && !edge.HasArrowStart {
		if isMarker(g.Nodes[edge.Source]) {
			markerThrough(canvas, path[0], path[1], edge.Style)
		} else {
			drawBoxStart(canvas, path[0], path[1], edge.Style)
		}
	}
}

// isMarker reports whether a node draws as a single start or end marker
// instead of a bordered shape.
func isMarker(n *graph.Node) bool {
	return n != nil && (n.Shape == graph.ShapeStartState || n.Shape == graph.ShapeEndState)
}

// markerThrough writes both along-axis arms into the attach cell of a marker
// node, so the line reads as running into the marker.
func markerThrough(canvas *Canvas, attach, next routing.Point, style graph.EdgeStyle) {
	w, ok := edgeWeight(style)
	if !ok {
		return
	}
	a := glyph.Vertical
	if next.Row == attach.Row {
		a = glyph.Horizontal
	}
	canvas.Arm(attach.Row, attach.Col, a, w, false, "edge")
}

// drawArrowHead draws an arrow head at the end of an edge path.
// The arrow is placed one cell BACK from to_point (in the gap, not on border).
func drawArrowHead(canvas *Canvas, from, to routing.Point, cs CharSet, style graph.EdgeStyle, arrowType graph.ArrowType) {
	if style == graph.EdgeInvisible {
		return
	}

	dx := sign(to.Col - from.Col)
	dy := sign(to.Row - from.Row)

	// Arrow position is one cell back from to_point
	arrowCol := to.Col - dx
	arrowRow := to.Row - dy

	var ch rune
	switch arrowType {
	case graph.ArrowTypeCircle:
		ch = cs.CircleEndpoint
	case graph.ArrowTypeCross:
		ch = cs.CrossEndpoint
	default:
		// Directional arrow
		if dx > 0 {
			ch = cs.ArrowRight
		} else if dx < 0 {
			ch = cs.ArrowLeft
		} else if dy > 0 {
			ch = cs.ArrowDown
		} else if dy < 0 {
			ch = cs.ArrowUp
		} else {
			return
		}
	}

	canvas.Put(arrowRow, arrowCol, ch, "edge")
}

// drawBoxStart writes the arm an edge leaves a node border through. The
// border's own arms make it a tee, and the cell keeps the node's style.
func drawBoxStart(canvas *Canvas, edgePoint, nextPoint routing.Point, style graph.EdgeStyle) {
	w, ok := edgeWeight(style)
	if !ok {
		return
	}
	dx := sign(nextPoint.Col - edgePoint.Col)
	dy := sign(nextPoint.Row - edgePoint.Row)

	var a glyph.Arms
	switch {
	case dx > 0:
		a = glyph.E
	case dx < 0:
		a = glyph.W
	case dy > 0:
		a = glyph.S
	case dy < 0:
		a = glyph.N
	default:
		return
	}
	canvas.Arm(edgePoint.Row, edgePoint.Col, a, w, false, "edge")
}

// drawEdgeLabel writes the label at the placement routing recorded.
func drawEdgeLabel(canvas *Canvas, re routing.RoutedEdge) {
	if re.LabelText == "" {
		return
	}
	end := re.LabelCol + textwidth.String(re.LabelText)
	if end >= canvas.Width || re.LabelRow >= canvas.Height {
		canvas.Resize(end+2, re.LabelRow+2)
	}
	canvas.PutText(re.LabelRow, re.LabelCol, re.LabelText, "edge_label")
}

// drawNotes draws notes attached to nodes.
func drawNotes(canvas *Canvas, g *graph.Graph, l *layout.GridLayout, cs CharSet) {
	for _, note := range g.Notes {
		p, ok := l.Placements[note.Target]
		if !ok {
			continue
		}

		lines := strings.Split(note.Text, "\n")
		noteWidth := 4 // minimum: 2 border + 2 padding
		for _, line := range lines {
			if w := textwidth.String(line) + 4; w > noteWidth {
				noteWidth = w
			}
		}
		noteHeight := len(lines) + 2

		var noteX int
		if note.Position == "rightof" {
			noteX = p.DrawX + p.DrawWidth + 2
		} else {
			noteX = p.DrawX - noteWidth - 2
			if noteX < 0 {
				noteX = 0
			}
		}
		noteY := p.DrawY + (p.DrawHeight-noteHeight)/2

		// Resize canvas if needed
		requiredW := noteX + noteWidth + 2
		requiredH := noteY + noteHeight + 2
		if requiredW > canvas.Width || requiredH > canvas.Height {
			canvas.Resize(requiredW, requiredH)
		}

		DrawRectangle(canvas, noteX, noteY, noteWidth, noteHeight, note.Text, cs, "node")
	}
}
