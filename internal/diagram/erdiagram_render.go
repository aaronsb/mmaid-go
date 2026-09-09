package diagram

import (
	"github.com/aaronsb/mmaid-go/internal/glyph"
	"github.com/aaronsb/mmaid-go/internal/renderer"
	"github.com/aaronsb/mmaid-go/internal/textwidth"
)

// RenderERDiagram parses and renders a Mermaid ER diagram.
func RenderERDiagram(source string, cs renderer.CharSet) *renderer.Canvas {
	erd := parseERDiagram(source)
	if len(erd.entityOrder) == 0 {
		c := renderer.NewCanvas(35, 1)
		c.PutText(0, 0, "[er] no entities defined", "default")
		return c
	}

	isLR := erd.direction == "LR"

	// BFS layer assignment
	layers := erAssignLayers(erd)

	// Compute box sizes
	boxes := make(map[string]*erBoxInfo)
	for _, name := range erd.entityOrder {
		entity := erd.entities[name]
		box := computeERBox(entity)
		box.layer = layers[name]
		boxes[name] = box
	}

	// Group boxes by layer
	maxLayer := 0
	for _, box := range boxes {
		if box.layer > maxLayer {
			maxLayer = box.layer
		}
	}
	layerGroups := make([][]string, maxLayer+1)
	for _, name := range erd.entityOrder {
		l := boxes[name].layer
		layerGroups[l] = append(layerGroups[l], name)
	}

	// Compute natural width to inform gap scaling
	naturalW := 0
	for _, group := range layerGroups {
		layerW := 0
		for _, name := range group {
			layerW += boxes[name].width
		}
		if layerW > naturalW {
			naturalW = layerW
		}
	}

	// Position boxes
	var canvasWidth, canvasHeight int
	if isLR {
		canvasWidth, canvasHeight = positionERBoxesLR(layerGroups, boxes, naturalW)
	} else {
		canvasWidth, canvasHeight = positionERBoxesTB(layerGroups, boxes, naturalW)
	}

	// Create canvas with margin
	c := renderer.NewCanvas(canvasWidth+4, canvasHeight+4)
	c.SetCharSet(cs)

	// Draw relationships first so entity boxes paint over any crossing lines
	for _, rel := range erd.relationships {
		srcBox := boxes[rel.entity1]
		tgtBox := boxes[rel.entity2]
		if srcBox == nil || tgtBox == nil {
			continue
		}
		drawERRelationship(c, rel, srcBox, tgtBox, boxes, cs, isLR)
	}

	// Draw entity boxes on top — labels always win over lines
	for _, name := range erd.entityOrder {
		drawERBox(c, erd.entities[name], boxes[name], cs)
	}

	return c
}

// erAssignLayers assigns BFS layers to entities based on relationships.
func erAssignLayers(erd *erDiagram) map[string]int {
	layers := make(map[string]int)
	adj := make(map[string][]string)
	incoming := make(map[string]int)
	for _, name := range erd.entityOrder {
		incoming[name] = 0
	}
	for _, rel := range erd.relationships {
		adj[rel.entity1] = append(adj[rel.entity1], rel.entity2)
		incoming[rel.entity2]++
	}

	// Find roots
	var queue []string
	for _, name := range erd.entityOrder {
		if incoming[name] == 0 {
			queue = append(queue, name)
			layers[name] = 0
		}
	}
	if len(queue) == 0 && len(erd.entityOrder) > 0 {
		queue = append(queue, erd.entityOrder[0])
		layers[erd.entityOrder[0]] = 0
	}

	// BFS
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		currLayer := layers[curr]
		for _, next := range adj[curr] {
			if _, ok := layers[next]; !ok {
				layers[next] = currLayer + 1
				queue = append(queue, next)
			}
		}
	}

	// Assign unvisited to layer 0
	for _, name := range erd.entityOrder {
		if _, ok := layers[name]; !ok {
			layers[name] = 0
		}
	}

	return layers
}

// computeERBox computes the size of an entity box.
func computeERBox(entity *erEntity) *erBoxInfo {
	box := &erBoxInfo{name: entity.name}

	displayName := entity.name
	if entity.alias != "" {
		displayName = entity.alias
	}

	minWidth := textwidth.String(displayName) + 4

	for _, attr := range entity.attributes {
		attrStr := formatERAttribute(attr)
		attrWidth := textwidth.String(attrStr) + 4
		if attrWidth > minWidth {
			minWidth = attrWidth
		}
	}

	box.width = minWidth
	box.height = 3 // top border + name + bottom border
	if len(entity.attributes) > 0 {
		box.height++
		box.height += len(entity.attributes)
	}

	return box
}

// positionERBoxesTB positions boxes in top-to-bottom layout.
func positionERBoxesTB(layerGroups [][]string, boxes map[string]*erBoxInfo, naturalW int) (int, int) {
	nGaps := 0
	for _, g := range layerGroups {
		if len(g)-1 > nGaps {
			nGaps = len(g) - 1
		}
	}
	hGap := scaleGap(6, max(1, nGaps), naturalW, 4, 12)
	vGap := 4

	y := 1
	maxWidth := 0

	for _, group := range layerGroups {
		if len(group) == 0 {
			continue
		}

		totalWidth := 0
		maxHeight := 0
		for _, name := range group {
			box := boxes[name]
			totalWidth += box.width
			if box.height > maxHeight {
				maxHeight = box.height
			}
		}
		totalWidth += (len(group) - 1) * hGap

		if totalWidth > maxWidth {
			maxWidth = totalWidth
		}

		x := 1
		for colIdx, name := range group {
			box := boxes[name]
			box.x = x
			box.y = y
			box.col = colIdx
			x += box.width + hGap
		}

		y += maxHeight + vGap
	}

	return maxWidth + 2, y
}

// positionERBoxesLR positions boxes in left-to-right layout.
func positionERBoxesLR(layerGroups [][]string, boxes map[string]*erBoxInfo, naturalW int) (int, int) {
	hGap := scaleGap(8, max(1, len(layerGroups)-1), naturalW, 4, 16)
	vGap := 3

	x := 1
	maxHeight := 0

	for _, group := range layerGroups {
		if len(group) == 0 {
			continue
		}

		maxWidth := 0
		totalHeight := 0
		for _, name := range group {
			box := boxes[name]
			totalHeight += box.height
			if box.width > maxWidth {
				maxWidth = box.width
			}
		}
		totalHeight += (len(group) - 1) * vGap

		if totalHeight > maxHeight {
			maxHeight = totalHeight
		}

		y := 1
		for colIdx, name := range group {
			box := boxes[name]
			box.x = x
			box.y = y
			box.col = colIdx
			y += box.height + vGap
		}

		x += maxWidth + hGap
	}

	return x, maxHeight + 2
}

// drawERBox draws an entity box on the canvas.
func drawERBox(c *renderer.Canvas, entity *erEntity, box *erBoxInfo, cs renderer.CharSet) {
	x, y, w, h := box.x, box.y, box.width, box.height

	if x+w+2 > c.Width || y+h+2 > c.Height {
		c.Resize(x+w+2, y+h+2)
	}

	renderer.DrawRectangle(c, x, y, w, h, "", cs, "node")

	displayName := entity.name
	if entity.alias != "" {
		displayName = entity.alias
	}

	row := y + 1
	nameCol := x + (w-textwidth.String(displayName))/2
	c.PutText(row, nameCol, displayName, "node")
	row++

	if len(entity.attributes) > 0 {
		for col := x + 1; col < x+w-1; col++ {
			c.Arm(row, col, glyph.Horizontal, glyph.Light, false, "node")
		}
		c.Arm(row, x, glyph.TeeRight, glyph.Light, false, "node")
		c.Arm(row, x+w-1, glyph.TeeLeft, glyph.Light, false, "node")
		row++

		for _, attr := range entity.attributes {
			attrStr := formatERAttribute(attr)
			c.PutText(row, x+2, attrStr, "node")
			row++
		}
	}
}

// drawERRelationship draws a relationship line between two entity boxes.
func drawERRelationship(c *renderer.Canvas, rel erRelationship, src, tgt *erBoxInfo, allBoxes map[string]*erBoxInfo, cs renderer.CharSet, isLR bool) {
	w := glyph.Light
	if rel.lineStyle == ".." {
		w = glyph.Dashed
	}

	srcCY := src.y + src.height/2
	tgtCY := tgt.y + tgt.height/2

	var srcX, srcY, tgtX, tgtY int

	if isLR {
		if src.x < tgt.x {
			srcX = src.x + src.width
			srcY = srcCY
			tgtX = tgt.x
			tgtY = tgtCY
		} else {
			srcX = src.x
			srcY = srcCY
			tgtX = tgt.x + tgt.width
			tgtY = tgtCY
		}
	} else {
		if src.y < tgt.y {
			srcX, srcY, tgtX, tgtY = erTBPorts(src, tgt)
		} else {
			tgtX, tgtY, srcX, srcY = erTBPorts(tgt, src)
		}
	}

	drawERRoutedLine(c, srcX, srcY, tgtX, tgtY, w, cs, src, tgt, allBoxes)
	joinLineToBox(c, srcX, srcY, src.x, src.y, src.width, src.height, w)
	joinLineToBox(c, tgtX, tgtY, tgt.x, tgt.y, tgt.width, tgt.height, w)

	// A cardinality sits in the gap beside the line's end, never on the box.
	// At a side port it goes under the line, leaving the row above to the
	// relationship label; at a bottom port, beside the line's first cell.
	placeCard := func(text string, x, y int, box *erBoxInfo) {
		if text == "" {
			return
		}
		width := textwidth.String(text)
		col, row := x+1, y-1
		switch {
		case x == box.x+box.width:
			row = y + 1
		case x == box.x:
			row, col = y+1, x-width-1
		case y == box.y+box.height:
			row = y
		}
		if row < 0 {
			row = y + 1
		}
		if col < 0 {
			col = 0
		}
		if col+width+1 > c.Width || row+1 > c.Height {
			c.Resize(col+width+2, row+2)
		}
		c.PutText(row, col, text, "edge_label")
	}
	placeCard(cardinalityDisplay[rel.card1], srcX, srcY, src)
	placeCard(cardinalityDisplay[rel.card2], tgtX, tgtY, tgt)

	if rel.label != "" {
		midX := (srcX + tgtX) / 2
		midY := (srcY + tgtY) / 2
		labelCol := midX - textwidth.String(rel.label)/2
		labelRow := midY - 1
		if labelRow < 0 {
			labelRow = midY + 1
		}
		if labelCol < 0 {
			labelCol = 0
		}
		if labelCol+textwidth.String(rel.label)+1 > c.Width || labelRow+1 > c.Height {
			c.Resize(labelCol+textwidth.String(rel.label)+2, labelRow+2)
		}
		c.PutText(labelRow, labelCol, rel.label, "edge_label")
	}
}

// erTBPorts computes connection points for a TB-mode relationship where
// top is above bot. Returns (topX, topY, botX, botY).
func erTBPorts(top, bot *erBoxInfo) (int, int, int, int) {
	topCX := top.x + top.width/2
	topCY := top.y + top.height/2
	botCX := bot.x + bot.width/2
	botCY := bot.y + bot.height/2

	if topCX == botCX {
		return topCX, top.y + top.height, botCX, bot.y
	}
	if botCX > top.x+top.width {
		return top.x + top.width, topCY, bot.x, botCY
	}
	if botCX < top.x {
		return top.x, topCY, bot.x + bot.width, botCY
	}
	return topCX, top.y + top.height, botCX, bot.y
}

// drawERRoutedLine draws a Z-shaped line between two points, choosing a
// horizontal bend position that avoids overlapping intermediate entity boxes.
func drawERRoutedLine(c *renderer.Canvas, x1, y1, x2, y2 int, w glyph.Weight, cs renderer.CharSet, src, tgt *erBoxInfo, allBoxes map[string]*erBoxInfo) {
	maxX := max(x1, x2)
	maxY := max(y1, y2)
	if maxX+2 > c.Width || maxY+2 > c.Height {
		c.Resize(maxX+2, maxY+2)
	}

	if x1 == x2 {
		c.DrawVertical(x1, y1, y2, w, "edge")
		return
	}
	if y1 == y2 {
		c.DrawHorizontal(y1, x1, x2, w, "edge")
		return
	}

	midY := findClearMidY(y1, y2, x1, x2, src, tgt, allBoxes)

	c.DrawVertical(x1, y1, midY, w, "edge")
	c.DrawHorizontal(midY, x1, x2, w, "edge")
	c.DrawVertical(x2, midY, y2, w, "edge")

	if y1 < midY {
		if x1 < x2 {
			c.Arm(midY, x1, glyph.BottomLeft, glyph.Light, false, "edge")
			c.Arm(midY, x2, glyph.TopRight, glyph.Light, false, "edge")
		} else {
			c.Arm(midY, x1, glyph.BottomRight, glyph.Light, false, "edge")
			c.Arm(midY, x2, glyph.TopLeft, glyph.Light, false, "edge")
		}
	} else {
		if x1 < x2 {
			c.Arm(midY, x1, glyph.TopLeft, glyph.Light, false, "edge")
			c.Arm(midY, x2, glyph.BottomRight, glyph.Light, false, "edge")
		} else {
			c.Arm(midY, x1, glyph.TopRight, glyph.Light, false, "edge")
			c.Arm(midY, x2, glyph.BottomLeft, glyph.Light, false, "edge")
		}
	}
}

// findClearMidY finds a Y coordinate for the horizontal segment of a Z-route
// that doesn't pass through any entity box (other than src/tgt).
func findClearMidY(y1, y2, x1, x2 int, src, tgt *erBoxInfo, allBoxes map[string]*erBoxInfo) int {
	minY := min(y1, y2)
	maxY := max(y1, y2)
	minX := min(x1, x2)
	maxX := max(x1, x2)

	defaultMid := (y1 + y2) / 2

	var obstacles []*erBoxInfo
	for _, box := range allBoxes {
		if box == src || box == tgt {
			continue
		}
		boxLeft := box.x
		boxRight := box.x + box.width
		if boxRight < minX || boxLeft > maxX {
			continue
		}
		boxTop := box.y
		boxBottom := box.y + box.height
		if boxBottom < minY || boxTop > maxY {
			continue
		}
		obstacles = append(obstacles, box)
	}

	if len(obstacles) == 0 {
		return defaultMid
	}

	// Try the gap just above and just below each obstacle (including outside range)
	candidates := []int{defaultMid, minY - 1, maxY + 1}
	for _, box := range obstacles {
		candidates = append(candidates, box.y-1)
		candidates = append(candidates, box.y+box.height)
	}

	isClear := func(cy int) bool {
		for _, box := range obstacles {
			if cy >= box.y && cy < box.y+box.height {
				return false
			}
		}
		return true
	}

	bestY := -1
	bestDist := maxY - minY + 100

	for _, cy := range candidates {
		if !isClear(cy) {
			continue
		}
		dist := cy - defaultMid
		if dist < 0 {
			dist = -dist
		}
		if dist < bestDist {
			bestDist = dist
			bestY = cy
		}
	}

	if bestY >= 0 {
		return bestY
	}
	return minY - 1
}

// joinLineToBox runs a line end into the box border beside it. The class
// and ER routers place an end one cell past a right or bottom border, so
// that cell takes the arm back toward the box and the border cell the arm
// out, making a tee.
func joinLineToBox(c *renderer.Canvas, x, y, boxX, boxY, boxW, boxH int, w glyph.Weight) {
	switch {
	case y == boxY+boxH && x >= boxX && x < boxX+boxW:
		c.Arm(y-1, x, glyph.S, w, false, "node")
		c.Arm(y, x, glyph.N, w, false, "edge")
	case x == boxX+boxW && y >= boxY && y < boxY+boxH:
		c.Arm(y, x-1, glyph.E, w, false, "node")
		c.Arm(y, x, glyph.W, w, false, "edge")
	}
}
