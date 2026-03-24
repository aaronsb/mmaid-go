package diagram

import (
	"regexp"
	"strings"

	"github.com/aaronsb/mmaid-go/internal/renderer"
)

// erAttribute represents an attribute of an entity.
type erAttribute struct {
	attrType string
	name     string
	keys     []string // e.g. "PK", "FK", "UK"
	comment  string
}

// erEntity represents an entity in the ER diagram.
type erEntity struct {
	name       string
	alias      string
	attributes []erAttribute
}

// erRelationship represents a relationship between two entities.
type erRelationship struct {
	entity1   string
	entity2   string
	card1     string // cardinality on entity1 side: "||", "|o", "}|", "}o"
	card2     string // cardinality on entity2 side: "||", "o|", "|{", "o{"
	lineStyle string // "--" or ".."
	label     string
}

// erDiagram represents a parsed ER diagram.
type erDiagram struct {
	entities      map[string]*erEntity
	entityOrder   []string
	relationships []erRelationship
	direction     string // "TB" or "LR"
}

// cardinalityDisplay maps cardinality markers to display text.
var cardinalityDisplay = map[string]string{
	"||": "1",
	"|o": "0..1",
	"o|": "0..1",
	"}|": "1..*",
	"|{": "1..*",
	"}o": "0..*",
	"o{": "0..*",
}

// Regex patterns for ER diagram parsing.
var (
	reERDiagramHeader = regexp.MustCompile(`(?i)^\s*erDiagram\s*$`)
	reERDirection     = regexp.MustCompile(`(?i)^\s*direction\s+(LR|RL|TB|BT|TD)\s*$`)
	reEREntityOpen    = regexp.MustCompile(`^\s*(\w+)\s*\{\s*$`)
	reEREntityClose   = regexp.MustCompile(`^\s*\}\s*$`)
	// Attribute line: type name PK,FK "comment"
	reERAttribute = regexp.MustCompile(`^\s*(\w+)\s+(\w+)\s*(?:((?:PK|FK|UK)(?:\s*,\s*(?:PK|FK|UK))*))?\s*(?:"([^"]*)")?\s*$`)
	// Relationship: Entity1 cardinality1--cardinality2 Entity2 : "label"
	// Cardinality markers: ||, |o, }|, }o on left; ||, o|, |{, o{ on right
	reERRelationship = regexp.MustCompile(
		`^\s*(\w+)\s+` +                               // entity1
			`(\|\||[|o}\s]{0,1}\||\}[|o]|\|o)` +       // card1
			`(--|\.\.)\s*` +                            // line style
			`(\|\||\|[{o]|o[|{]|o\|)` +               // card2
			`\s+(\w+)` +                               // entity2
			`(?:\s*:\s*"?([^"]*)"?)?\s*$`,             // optional label
	)
	// Word-based relationship: Entity1 zero or one to one or more Entity2 : "label"
	reERRelationshipWords = regexp.MustCompile(
		`(?i)^\s*(\w+)\s+` +
			`(zero or one|one or more|zero or more|only one|one to one|one to many|many to one|many to many)` +
			`\s+(?:to\s+)?` +
			`(zero or one|one or more|zero or more|only one|one to one|one to many|many to one|many to many)?\s*` +
			`(\w+)` +
			`(?:\s*:\s*"?([^"]*)"?)?\s*$`,
	)
	// Entity alias: EntityName ["Alias"]
	reEREntityAlias = regexp.MustCompile(`^\s*(\w+)\s+"([^"]+)"\s*$`)
)

// wordToCardinality maps word-based cardinality to marker format.
var wordToCardinality = map[string]string{
	"zero or one":  "|o",
	"only one":     "||",
	"one or more":  "}|",
	"zero or more": "}o",
}

// parseERDiagram parses ER diagram source into an erDiagram model.
func parseERDiagram(source string) *erDiagram {
	erd := &erDiagram{
		entities:  make(map[string]*erEntity),
		direction: "TB",
	}

	lines := strings.Split(source, "\n")
	i := 0
	for i < len(lines) {
		trimmed := strings.TrimSpace(lines[i])

		// Skip empty lines, comments, header
		if trimmed == "" || strings.HasPrefix(trimmed, "%%") || reERDiagramHeader.MatchString(trimmed) {
			i++
			continue
		}

		// Direction
		if m := reERDirection.FindStringSubmatch(trimmed); m != nil {
			dir := strings.ToUpper(m[1])
			if dir == "TD" {
				dir = "TB"
			}
			erd.direction = dir
			i++
			continue
		}

		// Entity with body: EntityName {
		if m := reEREntityOpen.FindStringSubmatch(trimmed); m != nil {
			entityName := m[1]
			entity := erd.ensureEntity(entityName)
			i++
			// Read attributes until closing brace
			for i < len(lines) {
				attrLine := strings.TrimSpace(lines[i])
				if reEREntityClose.MatchString(attrLine) {
					i++
					break
				}
				if attrLine == "" {
					i++
					continue
				}
				attr := parseERAttribute(attrLine)
				if attr != nil {
					entity.attributes = append(entity.attributes, *attr)
				}
				i++
			}
			continue
		}

		// Entity alias: EntityName "Display Name"
		if m := reEREntityAlias.FindStringSubmatch(trimmed); m != nil {
			entityName := m[1]
			alias := m[2]
			entity := erd.ensureEntity(entityName)
			entity.alias = alias
			i++
			continue
		}

		// Relationship with markers
		if m := reERRelationship.FindStringSubmatch(trimmed); m != nil {
			rel := erRelationship{
				entity1:   m[1],
				card1:     m[2],
				lineStyle: m[3],
				card2:     m[4],
				entity2:   m[5],
			}
			if len(m) > 6 {
				rel.label = strings.TrimSpace(m[6])
			}
			erd.ensureEntity(rel.entity1)
			erd.ensureEntity(rel.entity2)
			erd.relationships = append(erd.relationships, rel)
			i++
			continue
		}

		// Word-based relationship
		if m := reERRelationshipWords.FindStringSubmatch(trimmed); m != nil {
			card1Str := strings.ToLower(m[2])
			card2Str := strings.ToLower(m[3])
			if card2Str == "" {
				card2Str = card1Str
			}
			c1 := wordToCardinality[card1Str]
			c2 := wordToCardinality[card2Str]
			if c1 == "" {
				c1 = "||"
			}
			if c2 == "" {
				c2 = "||"
			}
			rel := erRelationship{
				entity1:   m[1],
				card1:     c1,
				lineStyle: "--",
				card2:     c2,
				entity2:   m[4],
			}
			if len(m) > 5 {
				rel.label = strings.TrimSpace(m[5])
			}
			erd.ensureEntity(rel.entity1)
			erd.ensureEntity(rel.entity2)
			erd.relationships = append(erd.relationships, rel)
			i++
			continue
		}

		i++
	}

	return erd
}

// ensureEntity creates an entity entry if it doesn't exist and returns it.
func (erd *erDiagram) ensureEntity(name string) *erEntity {
	if e, ok := erd.entities[name]; ok {
		return e
	}
	e := &erEntity{name: name}
	erd.entities[name] = e
	erd.entityOrder = append(erd.entityOrder, name)
	return e
}

// parseERAttribute parses a single entity attribute line.
func parseERAttribute(text string) *erAttribute {
	if m := reERAttribute.FindStringSubmatch(text); m != nil {
		attr := &erAttribute{
			attrType: m[1],
			name:     m[2],
		}
		if m[3] != "" {
			// Parse comma-separated keys
			keyStr := strings.ReplaceAll(m[3], " ", "")
			attr.keys = strings.Split(keyStr, ",")
		}
		if m[4] != "" {
			attr.comment = m[4]
		}
		return attr
	}
	return nil
}

// erBoxInfo holds computed layout info for an entity box.
type erBoxInfo struct {
	name   string
	x, y   int
	width  int
	height int
	layer  int
	col    int
}

// RenderERDiagram parses and renders a Mermaid ER diagram.
func RenderERDiagram(source string, useASCII bool) *renderer.Canvas {
	erd := parseERDiagram(source)
	if len(erd.entityOrder) == 0 {
		c := renderer.NewCanvas(35, 1)
		c.PutText(0, 0, "[er] no entities defined", "default")
		return c
	}

	cs := renderer.UNICODE
	if useASCII {
		cs = renderer.ASCII
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

	// Draw entity boxes
	for _, name := range erd.entityOrder {
		drawERBox(c, erd.entities[name], boxes[name], cs)
	}

	// Draw relationships
	for _, rel := range erd.relationships {
		srcBox := boxes[rel.entity1]
		tgtBox := boxes[rel.entity2]
		if srcBox == nil || tgtBox == nil {
			continue
		}
		drawERRelationship(c, rel, srcBox, tgtBox, cs, isLR)
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

	// Display name
	displayName := entity.name
	if entity.alias != "" {
		displayName = entity.alias
	}

	minWidth := len(displayName) + 4

	// Account for attributes
	for _, attr := range entity.attributes {
		attrStr := formatERAttribute(attr)
		attrWidth := len(attrStr) + 4
		if attrWidth > minWidth {
			minWidth = attrWidth
		}
	}

	box.width = minWidth

	// Height: border(1) + name(1) + divider(0-1) + attributes + border(1)
	box.height = 3 // top border + name + bottom border
	if len(entity.attributes) > 0 {
		box.height++                          // divider
		box.height += len(entity.attributes) // attribute lines
	}

	return box
}

// formatERAttribute formats an attribute for display.
func formatERAttribute(attr erAttribute) string {
	var b strings.Builder
	b.WriteString(attr.attrType)
	b.WriteString(" ")
	b.WriteString(attr.name)
	if len(attr.keys) > 0 {
		b.WriteString(" ")
		b.WriteString(strings.Join(attr.keys, ","))
	}
	if attr.comment != "" {
		b.WriteString(" \"")
		b.WriteString(attr.comment)
		b.WriteString("\"")
	}
	return b.String()
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

	// Ensure canvas is big enough
	if x+w+2 > c.Width || y+h+2 > c.Height {
		c.Resize(x+w+2, y+h+2)
	}

	// Draw border
	renderer.DrawRectangle(c, x, y, w, h, "", cs, "node")

	// Display name
	displayName := entity.name
	if entity.alias != "" {
		displayName = entity.alias
	}

	// Entity name (centered)
	row := y + 1
	nameCol := x + (w-len(displayName))/2
	c.PutText(row, nameCol, displayName, "node")
	row++

	// Divider and attributes
	if len(entity.attributes) > 0 {
		// Draw divider
		for col := x + 1; col < x+w-1; col++ {
			c.Put(row, col, cs.Horizontal, true, "node")
		}
		c.Put(row, x, cs.TeeRight, true, "node")
		c.Put(row, x+w-1, cs.TeeLeft, true, "node")
		row++

		// Attributes
		for _, attr := range entity.attributes {
			attrStr := formatERAttribute(attr)
			c.PutText(row, x+2, attrStr, "node")
			row++
		}
	}
}

// drawERRelationship draws a relationship line between two entity boxes.
func drawERRelationship(c *renderer.Canvas, rel erRelationship, src, tgt *erBoxInfo, cs renderer.CharSet, isLR bool) {
	// Determine line characters
	lineH := cs.LineHorizontal
	lineV := cs.LineVertical
	if rel.lineStyle == ".." {
		lineH = cs.LineDottedH
		lineV = cs.LineDottedV
	}

	// Compute connection points
	srcCX := src.x + src.width/2
	srcCY := src.y + src.height/2
	tgtCX := tgt.x + tgt.width/2
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
			srcX = srcCX
			srcY = src.y + src.height
			tgtX = tgtCX
			tgtY = tgt.y
		} else {
			srcX = srcCX
			srcY = src.y
			tgtX = tgtCX
			tgtY = tgt.y + tgt.height
		}
	}

	// Draw the routed line
	drawRoutedLine(c, srcX, srcY, tgtX, tgtY, lineH, lineV, cs)

	// Draw cardinality text near connection points
	card1Text := cardinalityDisplay[rel.card1]
	card2Text := cardinalityDisplay[rel.card2]

	if card1Text != "" {
		cardCol := srcX + 1
		cardRow := srcY - 1
		if cardRow < 0 {
			cardRow = srcY + 1
		}
		if cardCol+len(card1Text)+1 > c.Width || cardRow+1 > c.Height {
			c.Resize(cardCol+len(card1Text)+2, cardRow+2)
		}
		c.PutText(cardRow, cardCol, card1Text, "edge_label")
	}

	if card2Text != "" {
		cardCol := tgtX + 1
		cardRow := tgtY - 1
		if cardRow < 0 {
			cardRow = tgtY + 1
		}
		if cardCol+len(card2Text)+1 > c.Width || cardRow+1 > c.Height {
			c.Resize(cardCol+len(card2Text)+2, cardRow+2)
		}
		c.PutText(cardRow, cardCol, card2Text, "edge_label")
	}

	// Draw label at midpoint
	if rel.label != "" {
		midX := (srcX + tgtX) / 2
		midY := (srcY + tgtY) / 2
		labelCol := midX - len(rel.label)/2
		labelRow := midY - 1
		if labelRow < 0 {
			labelRow = midY + 1
		}
		if labelCol < 0 {
			labelCol = 0
		}
		if labelCol+len(rel.label)+1 > c.Width || labelRow+1 > c.Height {
			c.Resize(labelCol+len(rel.label)+2, labelRow+2)
		}
		c.PutText(labelRow, labelCol, rel.label, "edge_label")
	}
}
