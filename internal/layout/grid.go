// grid.go implements the grid-based layout algorithm for flowchart diagrams.
//
// Layout algorithm:
//  1. layoutHierarchy - Layer and order each subgraph scope recursively, with
//     child subgraphs as compound vertices, and place every node
//  2. countPorts - Edge ends expected per node side
//  3. computeSizes - Column widths and row heights from node content, word wrapping
//  4. normalizeSizes - Per-layer normalization, capped at MaxNormalized*
//  5. expandGapsForSubgraphs - Gap cells sized for the borders that run through them
//  6. computeDrawCoords - Convert grid to draw coordinates
//  7. computeSubgraphBounds - Boxes from each subgraph's block of positions
//  8. adjustForNegativeBounds - Shift everything if subgraph bounds go negative
//  9. Compute canvas size from max extents
package layout

import (
	"slices"
	"sort"
	"strings"

	"github.com/aaronsb/mmaid-go/internal/graph"
	"github.com/aaronsb/mmaid-go/internal/textwidth"
)

const (
	// Stride is the grid distance between node centers.
	Stride = 4

	// MaxLabelWidth is the character count before word wrapping.
	MaxLabelWidth = 20
	// MaxNormalizedWidth caps per-layer column normalization.
	MaxNormalizedWidth = 25
	// MaxNormalizedHeight caps per-layer row normalization.
	MaxNormalizedHeight = 7

	// SGBorderPad is padding between content and subgraph border.
	SGBorderPad = 2
	// SGLabelHeight is space for subgraph label + border line.
	SGLabelHeight = 2
	// SGGapPerLevel is the gap per nesting level.
	SGGapPerLevel = SGBorderPad + SGLabelHeight + 1

	// GapWidth and GapHeight are the draw size of a gap column and row. A
	// gap holds a port's stub, the corridor centre line an edge turns or
	// jogs on, and the arrowhead cell before the far border, so neither
	// can go below 3.
	GapWidth  = 4
	GapHeight = 3

	// SGGapMin is the least size of any gap in a graph with subgraphs: a
	// stub or arrowhead cell at each end, the corridor, and a free cell on
	// each side of it for a crossing run to move to.
	SGGapMin = 5
)

// GridCoord represents a position on the logical grid.
type GridCoord struct {
	Col int
	Row int
}

// NodePlacement stores the grid and drawing coordinates of a placed node.
type NodePlacement struct {
	NodeID string
	Grid   GridCoord
	// Min and Max are the first and last node cells the placement covers:
	// Grid for a node, the block's corners for a subgraph an edge ends at,
	// which Block marks.
	Min, Max   GridCoord
	Block      bool
	DrawX      int
	DrawY      int
	DrawWidth  int
	DrawHeight int
	// PortCount is the number of edge ends expected on each side, indexed
	// by Side; node sizing keeps room for that many ports.
	PortCount [4]int
}

// SubgraphBounds stores the drawing bounds of a subgraph.
type SubgraphBounds struct {
	Subgraph *graph.Subgraph
	X        int
	Y        int
	Width    int
	Height   int
}

// GridLayout is the result of the layout process.
type GridLayout struct {
	Placements   map[string]*NodePlacement
	ColWidths    map[int]int
	RowHeights   map[int]int
	GridOccupied map[GridCoord]string
	// Reserved holds cells edge labels have claimed; IsFree treats them as
	// occupied.
	Reserved       map[GridCoord]bool
	CanvasWidth    int
	CanvasHeight   int
	SubgraphBounds []SubgraphBounds
	OffsetX        int
	OffsetY        int
	// blocks holds each subgraph's block of positions.
	blocks map[*graph.Subgraph]posRect
}

// NewGridLayout returns a GridLayout initialized with empty maps.
func NewGridLayout() *GridLayout {
	return &GridLayout{
		Placements:   make(map[string]*NodePlacement),
		ColWidths:    make(map[int]int),
		RowHeights:   make(map[int]int),
		GridOccupied: make(map[GridCoord]string),
	}
}

// IsFree reports whether a grid cell is neither in a node's 3x3 block nor
// reserved by an edge label.
func (l *GridLayout) IsFree(col, row int, exclude map[string]bool) bool {
	if col < 0 || row < 0 {
		return false
	}
	key := GridCoord{col, row}
	if l.Reserved[key] {
		return false
	}
	occupant, ok := l.GridOccupied[key]
	if !ok {
		return true
	}
	if exclude != nil && exclude[occupant] {
		return true
	}
	return false
}

// Extent returns the draw width and height of every column and row the
// layout defines, gap columns past the last node included.
func (l *GridLayout) Extent() (w, h int) {
	w, h = l.OffsetX, l.OffsetY
	for _, cw := range l.ColWidths {
		w += cw
	}
	for _, rh := range l.RowHeights {
		h += rh
	}
	return w, h
}

// GridToDraw converts grid coordinates to drawing (character) coordinates.
func (l *GridLayout) GridToDraw(col, row int) (int, int) {
	x := l.OffsetX
	for c := range col {
		if w, ok := l.ColWidths[c]; ok {
			x += w
		} else {
			x++
		}
	}
	y := l.OffsetY
	for r := range row {
		if h, ok := l.RowHeights[r]; ok {
			y += h
		} else {
			y++
		}
	}
	return x, y
}

// GridToDrawCenter converts grid coordinates to the center of the cell.
func (l *GridLayout) GridToDrawCenter(col, row int) (int, int) {
	x, y := l.GridToDraw(col, row)
	w := 1
	if cw, ok := l.ColWidths[col]; ok {
		w = cw
	}
	h := 1
	if rh, ok := l.RowHeights[row]; ok {
		h = rh
	}
	return x + w/2, y + h/2
}

// ComputeLayout computes the grid layout for a graph.
// maxWidth, if > 0, hints the target canvas width — gap columns will be
// scaled proportionally to fill or compress to this width.
func ComputeLayout(g *graph.Graph, paddingX, paddingY, maxWidth int) *GridLayout {
	layout := NewGridLayout()

	if len(g.NodeOrder) == 0 {
		return layout
	}

	// Step 1: Layer and order every scope, place the nodes
	positions, blocks := layoutHierarchy(g)
	placeNodes(layout, positions)
	layout.blocks = blocks

	// Step 1b: Count the edge ends each node side expects
	countPorts(g, layout)

	// Step 2: Compute column widths and row heights (with word wrapping)
	computeSizes(g, layout, paddingX, paddingY)

	// Step 2b: Normalize sizes (per-layer, capped)
	normalizeSizes(g, layout)

	// Step 3: Expand gaps for subgraph borders and labels
	expandGapsForSubgraphs(g, layout)

	// Step 4: Compute drawing coordinates
	computeDrawCoords(layout)

	// Step 4b: Scale node columns to fill target width (before subgraph bounds)
	if maxWidth > 0 {
		scaleNodeColumns(layout, maxWidth)
		// Recompute draw coords after scaling
		computeDrawCoords(layout)
	}

	// Step 5: Compute subgraph bounds
	computeSubgraphBounds(g, layout)

	// Step 6: Adjust for negative subgraph bounds
	adjustForNegativeBounds(layout)

	// Step 7: Compute canvas size
	computeCanvasSize(layout)

	return layout
}

// computeCanvasSize sets CanvasWidth/CanvasHeight from placements and subgraph bounds.
func computeCanvasSize(layout *GridLayout) {
	maxX := 0
	maxY := 0
	for _, p := range layout.Placements {
		if p.DrawX+p.DrawWidth > maxX {
			maxX = p.DrawX + p.DrawWidth
		}
		if p.DrawY+p.DrawHeight > maxY {
			maxY = p.DrawY + p.DrawHeight
		}
	}
	for _, sb := range layout.SubgraphBounds {
		if sb.X+sb.Width > maxX {
			maxX = sb.X + sb.Width
		}
		if sb.Y+sb.Height > maxY {
			maxY = sb.Y + sb.Height
		}
	}
	layout.CanvasWidth = maxX
	layout.CanvasHeight = maxY
}

// scaleNodeColumns proportionally scales node columns (center column of each
// node's 3×3 block) to fill targetWidth. Gap columns stay fixed so edges
// remain correctly attached at node boundaries.
func scaleNodeColumns(layout *GridLayout, targetWidth int) {
	// Compute current canvas width from column widths
	currentW := 0
	for _, w := range layout.ColWidths {
		currentW += w
	}

	slack := targetWidth - currentW
	if slack <= 0 {
		return // don't compress — content-driven width is the minimum
	}

	// Identify node columns
	nodeCols := map[int]bool{}
	for _, p := range layout.Placements {
		nodeCols[p.Grid.Col] = true
	}

	var scalable []int
	for c := range layout.ColWidths {
		if nodeCols[c] {
			scalable = append(scalable, c)
		}
	}
	if len(scalable) == 0 {
		return
	}
	// ColWidths is a map, so sort before handing out the remainder: without
	// this the extra columns land somewhere different on every run.
	slices.Sort(scalable)

	// Distribute slack evenly across node columns
	n := len(scalable)
	for i, c := range scalable {
		share := slack / n
		if i < slack%n {
			share++
		}
		layout.ColWidths[c] += share
	}
}

// edgeKey is a source-target pair used as a map key.
type edgeKey struct{ src, tgt string }

// WordWrap splits text at word boundaries, keeping lines under maxWidth.
func WordWrap(text string, maxWidth int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{text}
	}

	var lines []string
	currentLine := words[0]

	for _, word := range words[1:] {
		if textwidth.String(currentLine)+1+textwidth.String(word) <= maxWidth {
			currentLine += " " + word
		} else {
			lines = append(lines, currentLine)
			currentLine = word
		}
	}

	lines = append(lines, currentLine)
	return lines
}

// computeSizes computes column widths and row heights based on node content.
func computeSizes(
	g *graph.Graph,
	layout *GridLayout,
	paddingX, paddingY int,
) {
	for nid, placement := range layout.Placements {
		node := g.Nodes[nid]
		label := node.Label

		var lines []string
		if strings.Contains(label, "\\n") {
			lines = strings.Split(label, "\\n")
		} else {
			lines = []string{label}
		}

		// Word-wrap lines that exceed max width
		var wrappedLines []string
		for _, line := range lines {
			if textwidth.String(line) <= MaxLabelWidth {
				wrappedLines = append(wrappedLines, line)
			} else {
				wrappedLines = append(wrappedLines, WordWrap(line, MaxLabelWidth)...)
			}
		}

		// Update the node's label with wrapped text
		if len(wrappedLines) > 1 && !slices.Equal(wrappedLines, lines) {
			node.Label = strings.Join(wrappedLines, "\\n")
		}

		textWidth := 0
		for _, l := range wrappedLines {
			if w := textwidth.String(l); w > textWidth {
				textWidth = w
			}
		}
		textHeight := len(wrappedLines)

		contentWidth := textWidth + paddingX   // padding on each side
		contentHeight := textHeight + paddingY // padding top/bottom

		// Start/end state markers are single-character — use minimal sizing
		// so they sit tight against the connecting edges.
		isMarker := node.Shape == graph.ShapeStartState || node.Shape == graph.ShapeEndState
		if isMarker {
			contentWidth = 1
			contentHeight = 1
		}

		// Ensure minimum sizes, and a cell per port on the busiest side
		// (but not for point markers)
		if !isMarker {
			contentWidth = max(contentWidth, 3, placement.PortCount[Top], placement.PortCount[Bottom])
			contentHeight = max(contentHeight, 3, placement.PortCount[Left], placement.PortCount[Right])
		}

		col := placement.Grid.Col
		row := placement.Grid.Row

		// Center column gets the content width
		if cur, ok := layout.ColWidths[col]; ok {
			if contentWidth > cur {
				layout.ColWidths[col] = contentWidth
			}
		} else {
			layout.ColWidths[col] = contentWidth
		}

		// Center row gets the content height
		if cur, ok := layout.RowHeights[row]; ok {
			if contentHeight > cur {
				layout.RowHeights[row] = contentHeight
			}
		} else {
			layout.RowHeights[row] = contentHeight
		}
	}

	// Border cells (around nodes) get width 1
	allCols := make(map[int]bool)
	allRows := make(map[int]bool)
	for _, placement := range layout.Placements {
		c, r := placement.Grid.Col, placement.Grid.Row
		for dc := -1; dc <= 1; dc++ {
			allCols[c+dc] = true
		}
		for dr := -1; dr <= 1; dr++ {
			allRows[r+dr] = true
		}
	}

	for c := range allCols {
		if _, ok := layout.ColWidths[c]; !ok {
			layout.ColWidths[c] = 1
		}
	}
	for r := range allRows {
		if _, ok := layout.RowHeights[r]; !ok {
			layout.RowHeights[r] = 1
		}
	}

	// Gap cells between nodes
	maxCol := 0
	maxRow := 0
	for c := range allCols {
		if c > maxCol {
			maxCol = c
		}
	}
	for r := range allRows {
		if r > maxRow {
			maxRow = r
		}
	}
	for c := 0; c < maxCol+2; c++ {
		if _, ok := layout.ColWidths[c]; !ok {
			layout.ColWidths[c] = GapWidth
		}
	}
	for r := 0; r < maxRow+2; r++ {
		if _, ok := layout.RowHeights[r]; !ok {
			layout.RowHeights[r] = GapHeight
		}
	}

	// Expand gaps to fit edge labels
	expandGapsForEdgeLabels(g, layout)
}

// expandGapsForEdgeLabels expands gap cells between nodes to fit edge labels.
func expandGapsForEdgeLabels(g *graph.Graph, layout *GridLayout) {
	direction := g.Direction.Normalized()
	isHorizontal := direction.IsHorizontal()

	for _, edge := range g.Edges {
		if edge.Label == "" {
			continue
		}
		labelLen := textwidth.String(edge.Label)

		srcP := layout.Placements[edge.Source]
		tgtP := layout.Placements[edge.Target]
		if srcP == nil || tgtP == nil {
			continue
		}

		if isHorizontal {
			// Edges run horizontally -- label needs gap column width
			c1 := min(srcP.Grid.Col, tgtP.Grid.Col)
			c2 := max(srcP.Grid.Col, tgtP.Grid.Col)
			gapStart := c1 + 2
			gapEnd := c2 - 2
			if gapStart > gapEnd {
				continue
			}
			// Need: gap_width + 1 >= label_len + 2 -> gap_width >= label_len + 1
			needed := labelLen + 1
			cur := 4
			if v, ok := layout.ColWidths[gapStart]; ok {
				cur = v
			}
			if needed > cur {
				layout.ColWidths[gapStart] = needed
			}
		} else {
			// Edges run vertically -- label placed beside the line (x+1)
			r1 := min(srcP.Grid.Row, tgtP.Grid.Row)
			r2 := max(srcP.Grid.Row, tgtP.Grid.Row)
			gapStart := r1 + 2
			gapEnd := r2 - 2
			if gapStart > gapEnd {
				continue
			}
			// Need enough vertical space: at least 2 rows for the label
			cur := 3
			if v, ok := layout.RowHeights[gapStart]; ok {
				cur = v
			}
			if cur < 3 {
				layout.RowHeights[gapStart] = 3
			}

			// Also ensure the gap column beside the edge is wide enough
			// for the label text.
			srcCol := srcP.Grid.Col
			tgtCol := tgtP.Grid.Col
			gapCols := make(map[int]bool)
			if tgtCol >= srcCol {
				gapCols[srcCol+2] = true // gap to the right of source
			}
			if tgtCol <= srcCol {
				gapCols[srcCol-2] = true // gap to the left of source
			}
			// For edges crossing multiple columns, also expand intermediate gaps
			cMin := min(srcCol, tgtCol)
			cMax := max(srcCol, tgtCol)
			for c := cMin + 2; c < cMax; c += Stride {
				gapCols[c] = true
			}
			for gapCol := range gapCols {
				if gapCol >= 0 {
					if cur, ok := layout.ColWidths[gapCol]; ok {
						if labelLen+1 > cur {
							layout.ColWidths[gapCol] = labelLen + 1
						}
					}
				}
			}
		}
	}

	// For vertical flow: when multiple labeled edges leave the same source,
	// ensure the gap row is tall enough for all labels with spacing.
	if !isHorizontal {
		labeledPerSrc := make(map[string]int)
		for _, edge := range g.Edges {
			if edge.Label != "" {
				labeledPerSrc[edge.Source]++
			}
		}
		for srcID, count := range labeledPerSrc {
			if count < 2 {
				continue
			}
			srcP := layout.Placements[srcID]
			if srcP == nil {
				continue
			}
			gapRow := srcP.Grid.Row + 2
			needed := count*2 + 1 // 2 rows per label + spacing
			cur := 3
			if v, ok := layout.RowHeights[gapRow]; ok {
				cur = v
			}
			if needed > cur {
				layout.RowHeights[gapRow] = needed
			}
		}
	}
}

// normalizeSizes normalizes node dimensions within the same layer, capped at a maximum.
// Nodes at the same flow level (same layer) are normalized to the same
// perpendicular dimension so side-by-side nodes look consistent.
func normalizeSizes(g *graph.Graph, layout *GridLayout) {
	direction := g.Direction.Normalized()

	// Group placements by layer
	layerGroups := make(map[int][]*NodePlacement)
	for _, p := range layout.Placements {
		var layerKey int
		if direction.IsVertical() {
			layerKey = p.Grid.Row // same row = same layer in TD
		} else {
			layerKey = p.Grid.Col // same col = same layer in LR
		}
		layerGroups[layerKey] = append(layerGroups[layerKey], p)
	}

	for _, placements := range layerGroups {
		if len(placements) < 2 {
			continue // single node in layer, nothing to normalize
		}

		if direction.IsVertical() {
			// TD: normalize column widths within same layer
			cols := make(map[int]bool)
			for _, p := range placements {
				cols[p.Grid.Col] = true
			}
			maxW := 0
			for c := range cols {
				w := 1
				if v, ok := layout.ColWidths[c]; ok {
					w = v
				}
				if w > maxW {
					maxW = w
				}
			}
			target := maxW
			if target > MaxNormalizedWidth {
				target = MaxNormalizedWidth
			}
			for c := range cols {
				cur := 1
				if v, ok := layout.ColWidths[c]; ok {
					cur = v
				}
				if target > cur {
					layout.ColWidths[c] = target
				}
			}
		} else {
			// LR: normalize row heights within same layer
			rows := make(map[int]bool)
			for _, p := range placements {
				rows[p.Grid.Row] = true
			}
			maxH := 0
			for r := range rows {
				h := 1
				if v, ok := layout.RowHeights[r]; ok {
					h = v
				}
				if h > maxH {
					maxH = h
				}
			}
			target := maxH
			if target > MaxNormalizedHeight {
				target = MaxNormalizedHeight
			}
			for r := range rows {
				cur := 1
				if v, ok := layout.RowHeights[r]; ok {
					cur = v
				}
				if target > cur {
					layout.RowHeights[r] = target
				}
			}
		}
	}
}

// computeDrawCoords converts grid positions to drawing coordinates.
func computeDrawCoords(layout *GridLayout) {
	for _, placement := range layout.Placements {
		gc := placement.Grid
		// Top-left of the 3x3 block
		x, y := layout.GridToDraw(gc.Col-1, gc.Row-1)
		w := 0
		for dc := -1; dc <= 1; dc++ {
			if cw, ok := layout.ColWidths[gc.Col+dc]; ok {
				w += cw
			} else {
				w += 1
			}
		}
		h := 0
		for dr := -1; dr <= 1; dr++ {
			if rh, ok := layout.RowHeights[gc.Row+dr]; ok {
				h += rh
			} else {
				h += 1
			}
		}
		placement.DrawX = x
		placement.DrawY = y
		placement.DrawWidth = w
		placement.DrawHeight = h
	}
}

// adjustForNegativeBounds shifts all coordinates if subgraph bounds extend into negative space.
func adjustForNegativeBounds(layout *GridLayout) {
	if len(layout.SubgraphBounds) == 0 {
		return
	}

	minX := 0
	minY := 0
	for _, sb := range layout.SubgraphBounds {
		if sb.X < minX {
			minX = sb.X
		}
		if sb.Y < minY {
			minY = sb.Y
		}
	}

	if minX >= 0 && minY >= 0 {
		return
	}

	dx := 0
	if minX < 0 {
		dx = -minX + 1
	}
	dy := 0
	if minY < 0 {
		dy = -minY + 1
	}

	for _, p := range layout.Placements {
		p.DrawX += dx
		p.DrawY += dy
	}

	for i := range layout.SubgraphBounds {
		layout.SubgraphBounds[i].X += dx
		layout.SubgraphBounds[i].Y += dy
	}

	layout.OffsetX += dx
	layout.OffsetY += dy
}

// gatherAllNodes recursively gathers all node IDs from a subgraph and its children.
func gatherAllNodes(sg *graph.Subgraph, result map[string]bool) {
	for _, nid := range sg.NodeIDs {
		result[nid] = true
	}
	for _, child := range sg.Children {
		gatherAllNodes(child, result)
	}
}

// --- Helper functions ---

// copyLayers creates a deep copy of layer lists.
func copyLayers(layers [][]string) [][]string {
	result := make([][]string, len(layers))
	for i, layer := range layers {
		result[i] = make([]string, len(layer))
		copy(result[i], layer)
	}
	return result
}

// sortedKeys returns the sorted keys of a map.
func sortedKeys[V any](m map[int]V) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	return keys
}
