package diagram

import (
	"regexp"
	"strings"

	"github.com/aaronsb/mmaid-go/internal/glyph"
	"github.com/aaronsb/mmaid-go/internal/renderer"
)

// Skipped, against treeView.langium and its value converter at mermaid
// fe0e2375:
//   - `icon(...)`: the canvas has no icon pack to draw from.
//   - `:::class`: there is no stylesheet behind a terminal frame.
//   - `accTitle` and `accDescr`.
//   - The box-drawing preprocessor, which takes a tree someone has already
//     drawn and reads the structure back out of its guides.
//
// Only a line whose first non-blank characters are `%%` is a comment: the
// grammar's BARE_NAME and DESC_ANNOTATION both run to end of line, so
// `100%%done.txt` keeps its name.

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
	// TITLE, ACC_TITLE and ACC_DESCR are case-sensitive terminals, and the
	// two accessibility ones need their separator, so `accTitle.md` and
	// `Title` are ordinary labels.
	reTreeViewTitle    = regexp.MustCompile(`^title(?:[ \t](.*))?$`)
	reTreeViewAccTitle = regexp.MustCompile(`^accTitle[ \t]*:`)
	reTreeViewAccDescr = regexp.MustCompile(`^accDescr[ \t]*[:{]`)
	// The markers a bare label stops before, from the grammar's BARE_NAME
	// terminal.
	reTreeViewAnnotation = regexp.MustCompile(`[ \t]+(:::|icon\(|##)`)
)

// tvIndent is the column step per tree level: the width of "├──".
const tvIndent = 3

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

// indentWidth counts leading whitespace one column per character, as the
// grammar's value converters do.
func indentWidth(line string) int {
	n := 0
	for _, ch := range line {
		if ch != ' ' && ch != '\t' {
			break
		}
		n++
	}
	return n
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
		line = strings.TrimRight(line, "\r")
		trimmed := strings.TrimSpace(line)
		if skipBlock {
			if strings.HasPrefix(trimmed, "}") {
				skipBlock = false
			}
			continue
		}
		if trimmed == "" || strings.HasPrefix(trimmed, "%%") || reTreeViewHeader.MatchString(trimmed) {
			continue
		}
		// TitleAndAccessibilities sits before the nodes in the entry rule, so
		// once a node has been read these keywords are labels again.
		if len(stack) == 0 {
			if m := reTreeViewTitle.FindStringSubmatch(trimmed); m != nil {
				td.title = strings.TrimSpace(m[1])
				continue
			}
			if reTreeViewAccTitle.MatchString(trimmed) {
				continue
			}
			if reTreeViewAccDescr.MatchString(trimmed) {
				skipBlock = strings.HasSuffix(trimmed, "{")
				continue
			}
		}

		indent := indentWidth(line)
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
