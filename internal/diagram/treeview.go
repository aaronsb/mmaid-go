package diagram

import (
	"regexp"
	"strings"

	"github.com/aaronsb/mmaid-go/internal/glyph"
	"github.com/aaronsb/mmaid-go/internal/renderer"
)

// treeViewNode is one label in a treeView-beta tree. A label ending in "/" is
// a directory and renders bold; anything else is a leaf.
type treeViewNode struct {
	label    string
	desc     string
	folder   bool
	children []*treeViewNode
}

// treeViewData is the parsed `treeView-beta` diagram: a forest, because every
// node at the outermost indent is a root.
type treeViewData struct {
	title string
	roots []*treeViewNode
}

var (
	reTreeViewHeader = regexp.MustCompile(`(?i)^treeview(-beta)?$`)
	reTreeViewTitle  = regexp.MustCompile(`(?i)^title(?:\s+(.*))?$`)
	// The markers a bare label stops before, from the grammar's BARE_NAME
	// terminal.
	reTreeViewAnnotation = regexp.MustCompile(`[ \t]+(:::|icon\(|##)`)
)

// tvIndent is the column step per tree level: the width of "├──".
const tvIndent = 3

// stripTreeViewComment cuts a `%%` comment, leaving one inside a quoted label
// alone.
func stripTreeViewComment(line string) string {
	quote := rune(0)
	runes := []rune(line)
	for i := 0; i < len(runes); i++ {
		switch {
		case quote != 0:
			if runes[i] == quote {
				quote = 0
			}
		case runes[i] == '"' || runes[i] == '\'':
			quote = runes[i]
		case runes[i] == '%' && i+1 < len(runes) && runes[i+1] == '%':
			return string(runes[:i])
		}
	}
	return line
}

// splitTreeViewLabel separates a node's label from its annotations.
func splitTreeViewLabel(s string) (label, rest string) {
	if q := s[0]; q == '"' || q == '\'' {
		if end := strings.IndexByte(s[1:], q); end >= 0 {
			return s[1 : end+1], s[end+2:]
		}
		return s[1:], ""
	}
	if loc := reTreeViewAnnotation.FindStringIndex(s); loc != nil {
		return s[:loc[0]], s[loc[0]:]
	}
	return s, ""
}

// treeViewDescription returns the `## text` annotation, the only one this
// renderer draws. `:::class` and `icon(...)` are parsed off and dropped.
func treeViewDescription(rest string) string {
	rest = strings.TrimSpace(rest)
	for rest != "" {
		switch {
		case strings.HasPrefix(rest, "##"):
			return strings.TrimSpace(rest[2:])
		case strings.HasPrefix(rest, ":::"):
			rest = strings.TrimSpace(strings.TrimLeft(rest[3:], " \t"))
			if i := strings.IndexAny(rest, " \t"); i >= 0 {
				rest = strings.TrimSpace(rest[i:])
			} else {
				rest = ""
			}
		case strings.HasPrefix(rest, "icon("):
			if i := strings.IndexByte(rest, ')'); i >= 0 {
				rest = strings.TrimSpace(rest[i+1:])
			} else {
				rest = ""
			}
		default:
			return ""
		}
	}
	return ""
}

// parseTreeView parses a Mermaid treeView-beta definition.
//
//	treeView-beta
//	    my-project/
//	        src/
//	            index.js  ## entry point
func parseTreeView(source string) *treeViewData {
	td := &treeViewData{}

	type stackEntry struct {
		node   *treeViewNode
		indent int
	}
	var stack []stackEntry
	skipBlock := false

	for _, line := range strings.Split(source, "\n") {
		line = stripTreeViewComment(strings.TrimRight(line, "\r"))
		trimmed := strings.TrimSpace(line)
		if skipBlock {
			if strings.HasPrefix(trimmed, "}") {
				skipBlock = false
			}
			continue
		}
		if trimmed == "" || reTreeViewHeader.MatchString(trimmed) {
			continue
		}
		if m := reTreeViewTitle.FindStringSubmatch(trimmed); m != nil {
			td.title = strings.TrimSpace(m[1])
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "acctitle") || strings.HasPrefix(lower, "accdescr") {
			skipBlock = strings.HasSuffix(trimmed, "{")
			continue
		}

		indent := 0
	measure:
		for _, ch := range line {
			switch ch {
			case ' ':
				indent++
			case '\t':
				indent += 4
			default:
				break measure
			}
		}

		label, rest := splitTreeViewLabel(trimmed)
		label = strings.TrimSpace(label)
		if label == "" {
			continue
		}
		node := &treeViewNode{
			label:  label,
			desc:   treeViewDescription(rest),
			folder: strings.HasSuffix(label, "/"),
		}

		for len(stack) > 0 && stack[len(stack)-1].indent >= indent {
			stack = stack[:len(stack)-1]
		}
		if len(stack) == 0 {
			td.roots = append(td.roots, node)
		} else {
			parent := stack[len(stack)-1].node
			parent.children = append(parent.children, node)
		}
		stack = append(stack, stackEntry{node, indent})
	}

	return td
}

// tvPlaced is a node with the row and depth it draws at.
type tvPlaced struct {
	node  *treeViewNode
	depth int
	row   int
}

// RenderTreeView parses and renders a Mermaid treeView-beta diagram: one node
// per row, root at top, with the guides drawn as arms so they resolve to
// ├── └── │ from the charset's tables.
func RenderTreeView(source string, cs renderer.CharSet) *renderer.Canvas {
	td := parseTreeView(source)
	if len(td.roots) == 0 {
		c := renderer.NewCanvas(30, 1)
		c.PutText(0, 0, "[treeView] no nodes", "default")
		return c
	}

	row := 0
	if td.title != "" {
		row = 2
	}

	var placed []tvPlaced
	at := map[*treeViewNode]int{}
	var walk func(n *treeViewNode, depth int)
	walk = func(n *treeViewNode, depth int) {
		at[n] = row
		placed = append(placed, tvPlaced{n, depth, row})
		row++
		for _, ch := range n.children {
			walk(ch, depth+1)
		}
	}
	for _, r := range td.roots {
		walk(r, 0)
	}

	width := runeLen(td.title)
	for _, p := range placed {
		w := tvIndent*p.depth + runeLen(p.node.label)
		if p.node.desc != "" {
			w += 2 + runeLen(p.node.desc)
		}
		width = max(width, w)
	}

	c := renderer.NewCanvas(width+2, row+1)
	c.SetCharSet(cs)

	if td.title != "" {
		c.PutText(0, 0, td.title, "bold_label")
	}

	// Guides. A parent's children share one column; the run from the row
	// under the parent's label down to the last child carries the vertical,
	// and each child's row turns out of it.
	for _, p := range placed {
		kids := p.node.children
		if len(kids) == 0 {
			continue
		}
		gc := tvIndent * p.depth
		lastRow := at[kids[len(kids)-1]]
		childRow := map[int]int{} // row -> index of the child on it
		for i, k := range kids {
			childRow[at[k]] = i
		}
		for r := p.row + 1; r <= lastRow; r++ {
			arms := glyph.Vertical
			if i, ok := childRow[r]; ok {
				arms = glyph.TeeRight
				if i == len(kids)-1 {
					arms = glyph.BottomLeft
				}
			}
			c.Arm(r, gc, arms, glyph.Light, false, "edge")
		}
		for _, k := range kids {
			c.Arm(at[k], gc+1, glyph.Horizontal, glyph.Light, false, "edge")
			c.Arm(at[k], gc+2, glyph.Horizontal, glyph.Light, false, "edge")
		}
	}

	// Labels. The stem runs into the first cell of the label, so no arm ends
	// on a space.
	for _, p := range placed {
		col := tvIndent * p.depth
		style := "label"
		if p.node.folder {
			style = "bold_label"
		}
		c.PutText(p.row, col, p.node.label, style)
		if p.node.desc != "" {
			c.PutText(p.row, col+runeLen(p.node.label)+2, p.node.desc, "edge_label")
		}
	}

	return c
}
