package diagram

import (
	"regexp"
	"strings"

	"github.com/aaronsb/mmaid-go/internal/glyph"
	"github.com/aaronsb/mmaid-go/internal/renderer"
)

// Skipped, against ishikawa.jison and ishikawaDb.ts at mermaid fe0e2375:
//   - `accTitle` and `accDescr`: the jison lexer has no rule for either, so
//     both are cause text upstream and here.
//
// Divergences:
//   - A cause nested below the first level becomes another rib carrying one
//     marker per level under its category. `ishikawaRenderer.ts`'s flattenTree
//     draws those recursively as sub-branches off their own bone, which needs
//     a second diagonal per level; a character grid runs out of cells long
//     before the tree does.
//
// Only a line whose first non-blank characters are `%%` is a comment: the
// lexer's TEXT rule is `[^\n]+`, so `Discount 20%% off` keeps its text.

// ishikawaNode is one line of a fishbone: the effect at the root, its
// categories below it, and their causes below those.
type ishikawaNode struct {
	text     string
	children []*ishikawaNode
}

// The header may carry the effect on the same line: the jison start rule
// allows `ISHIKAWA document` with no newline between them.
var reIshikawaHeader = regexp.MustCompile(`(?i)^ishikawa(?:-beta)?(?:\s+(.*))?$`)

// parseIshikawa parses a Mermaid ishikawa definition. The first line after the
// header is the effect; the rest are causes, nested by indentation. The
// indent of the first cause sets the base level, so an effect indented more
// than its causes still parses, as upstream's db does.
//
//	ishikawa-beta
//	    Blurry Photo
//	        Process
//	            Out of focus
func parseIshikawa(source string) *ishikawaNode {
	var root *ishikawaNode
	type stackEntry struct {
		level int
		node  *ishikawaNode
	}
	var stack []stackEntry
	base := -1

	add := func(raw int, text string) {
		if root == nil {
			root = &ishikawaNode{text: text}
			stack = []stackEntry{{0, root}}
			return
		}
		if base < 0 {
			base = raw
		}
		level := max(raw-base+1, 1)
		for len(stack) > 1 && stack[len(stack)-1].level >= level {
			stack = stack[:len(stack)-1]
		}
		node := &ishikawaNode{text: text}
		parent := stack[len(stack)-1].node
		parent.children = append(parent.children, node)
		stack = append(stack, stackEntry{level, node})
	}

	seenHeader := false
	for _, line := range strings.Split(source, "\n") {
		trimmed := strings.TrimSpace(strings.TrimRight(line, "\r"))
		if trimmed == "" || strings.HasPrefix(trimmed, "%%") {
			continue
		}
		if !seenHeader {
			if m := reIshikawaHeader.FindStringSubmatch(trimmed); m != nil {
				seenHeader = true
				if rest := strings.TrimSpace(m[1]); rest != "" {
					add(0, rest)
				}
				continue
			}
		}
		add(indentWidth(line), trimmed)
	}

	return root
}

// ishikawaCauses flattens a category's subtree into one line per cause, a
// deeper cause carrying one marker per level below the category.
func ishikawaCauses(n *ishikawaNode, depth int, marker string, out *[]string) {
	for _, ch := range n.children {
		*out = append(*out, strings.Repeat(marker, depth)+ch.text)
		ishikawaCauses(ch, depth+1, marker, out)
	}
}

// ishikawaBone is a category with the causes and geometry it draws with.
type ishikawaBone struct {
	label  string
	causes []string
	above  bool
	length int // bone cells from the spine to its far end
	attach int // the spine column the bone rises from
}

const ishikawaRib = 2 // rib cells between a cause and its bone

// ishikawaLayout turns the categories into bones and places each one in a
// column slot wide enough for its longest cause line and its own label.
// It returns the bones, the column the effect box starts at, and how far the
// bones reach above and below the spine.
func ishikawaLayout(root *ishikawaNode, marker string) (bones []ishikawaBone, boxLeft, above, below int) {
	bones = make([]ishikawaBone, 0, len(root.children))
	for i, cat := range root.children {
		var causes []string
		ishikawaCauses(cat, 0, marker, &causes)
		bones = append(bones, ishikawaBone{
			label:  cat.text,
			causes: causes,
			above:  i%2 == 0,
			length: max(len(causes)+1, 2),
		})
	}

	col := 1
	for i := range bones {
		b := &bones[i]
		need := b.length + runeLen(b.label) - 1
		for j, cause := range b.causes {
			need = max(need, j+ishikawaRib+1+runeLen(cause))
		}
		b.attach = col + need
		col = b.attach + 1
	}

	boxLeft = col + 2
	if len(bones) == 0 {
		boxLeft = 6
	}
	above, below = 1, 1
	for _, b := range bones {
		if b.above {
			above = max(above, b.length+1)
		} else {
			below = max(below, b.length+1)
		}
	}
	return bones, boxLeft, above, below
}

// ishikawaDrawBone draws one diagonal, its category label at the far end, and
// a light rib from each cause's text into the bone cell it hangs from.
func ishikawaDrawBone(c *renderer.Canvas, b ishikawaBone, spineRow, index int, cs renderer.CharSet, theme *renderer.Theme, useRegion bool) {
	labelStyle := "bold_label"
	boneStyle := "edge"
	causeStyle := "label"
	if useRegion {
		labelStyle = "_ansi:" + theme.RegionLabelStyle(index, 0)
		boneStyle = "_ansi:" + theme.RegionBorderStyle(index, 0)
		causeStyle = "_ansi:" + theme.RegionTextStyle(index, 1)
	}
	step, bone := -1, cs.Backslash
	if !b.above {
		step, bone = 1, cs.Slash
	}
	for t := 1; t <= b.length; t++ {
		c.Put(spineRow+step*t, b.attach-t, bone, boneStyle)
	}
	c.PutText(spineRow+step*(b.length+1), b.attach-b.length-runeLen(b.label)+1, b.label, labelStyle)

	for j, cause := range b.causes {
		row := spineRow + step*(j+1)
		ribEnd := b.attach - j - 2
		ribStart := ribEnd - ishikawaRib + 1
		for x := ribStart; x <= ribEnd; x++ {
			c.Arm(row, x, glyph.Horizontal, glyph.Light, false, "edge")
		}
		c.PutText(row, ribStart-runeLen(cause), cause, causeStyle)
	}
}

// ishikawaDrawEffect draws the box the spine runs into.
func ishikawaDrawEffect(c *renderer.Canvas, text string, spineRow, boxLeft, boxW int, theme *renderer.Theme, useRegion bool) {
	borderStyle := "node"
	labelStyle := "bold_label"
	if useRegion {
		borderStyle = "_ansi:" + theme.RegionBorderStyle(0, 1)
		labelStyle = "_ansi:" + theme.RegionLabelStyle(0, 1)
	}
	top, bot := spineRow-1, spineRow+1
	right := boxLeft + boxW - 1
	c.Segment(top, boxLeft, top, right, glyph.Light, false, borderStyle)
	c.Segment(bot, boxLeft, bot, right, glyph.Light, false, borderStyle)
	c.Segment(top, boxLeft, bot, boxLeft, glyph.Light, false, borderStyle)
	c.Segment(top, right, bot, right, glyph.Light, false, borderStyle)
	c.PutText(spineRow, boxLeft+(boxW-runeLen(text))/2, text, labelStyle)
	if useRegion {
		fill := "_ansi:" + theme.RegionStyle(0, 1)
		for r := top; r <= bot; r++ {
			for x := boxLeft; x <= right; x++ {
				c.SetFill(r, x, fill)
			}
		}
	}
}

// RenderIshikawa parses and renders a Mermaid ishikawa (fishbone) diagram:
// a heavy spine running into the effect box on the right, with the categories
// as diagonal bones alternating above and below it.
func RenderIshikawa(source string, cs renderer.CharSet, theme *renderer.Theme) *renderer.Canvas {
	root := parseIshikawa(source)
	if root == nil {
		c := renderer.NewCanvas(30, 1)
		c.PutText(0, 0, "[ishikawa] no effect", "default")
		return c
	}

	marker := string(cs.Dot) + " "
	bones, boxLeft, above, below := ishikawaLayout(root, marker)
	boxW := runeLen(root.text) + 4
	spineRow := above

	c := renderer.NewCanvas(boxLeft+boxW+1, spineRow+below+1)
	c.SetCharSet(cs)
	useRegion := theme != nil && theme.HasDepthColors()

	// The box goes down first so the cell the spine meets it in keeps the
	// border's weight and colour: the heavy run stops one column short and
	// reaches in with a light arm, and the border resolves to a light tee.
	ishikawaDrawEffect(c, root.text, spineRow, boxLeft, boxW, theme, useRegion)
	// The spine's tail is a dot, so its west end is closed rather than a
	// half glyph open to the canvas edge.
	c.Put(spineRow, 0, cs.Dot, "edge")
	c.Segment(spineRow, 1, spineRow, boxLeft-1, glyph.Heavy, false, "edge")
	c.Arm(spineRow, 1, glyph.W, glyph.Heavy, false, "edge")
	c.Arm(spineRow, boxLeft-1, glyph.E, glyph.Light, false, "edge")
	c.Arm(spineRow, boxLeft, glyph.W, glyph.Light, false, "edge")

	for i, b := range bones {
		ishikawaDrawBone(c, b, spineRow, i, cs, theme, useRegion)
	}
	return c
}
