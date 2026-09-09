package parser

// ── architecture-beta ────────────────────────────────────────────────────────
//
// Groups become subgraphs, services and junctions become nodes, and the
// direction hints become the graph's explicit positions (ADR-104), which the
// layout takes in place of layering, ordering and placement.
//
// Read: `group id(icon)[Label] in parent`, `service id(icon)[Label] in parent`
// with the quoted-text icon form, `junction id in parent`, and edges
// `a:R -- L:b` with the arrowheads `-->`, `<--`, `<-->`, the title form
// `a:R -[Label]- L:b` of the grammar, and the `{group}` suffix on either end,
// which attaches that end to the border of the group holding the service.
//
// Icons have no terminal form, so the icon name is kept as a label prefix in
// square brackets: a line of its own above a service's label, inline before a
// group's, which is drawn on one line in the border.
//
// Skipped: `align row|column`, which exists to spread the siblings upstream's
// force-directed layout collapses onto one point — a collision here is
// reported and moved to the next free column instead; `title`, `accTitle` and
// `accDescr`, which the graph model has nowhere to put; the `architecture`
// config block (`randomize`, `seed`, `nodeSeparation`,
// `idealEdgeLengthMultiplier`, `edgeElasticity`, `numIter`), every field of
// which tunes fcose; icon packs, since the prefix is the icon's name.
//
// Upstream's resolver: `architectureDb.ts` `getDataStructures` builds a
// direction-pair adjacency list and walks it breadth-first from an arbitrary
// node, one spatial map per connected component, with no collision check —
// mermaid-js/mermaid at fe0e237. It reads the far node's letter as a
// direction on the X axis and as its opposite on the Y axis, so a bend
// resolves differently depending on which end the walk reaches first. Here
// both letters name the side of their own node the edge leaves through, so a
// bend resolves the same either way.

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/aaronsb/mmaid-go/internal/graph"
)

// archLabelSep is the line break the layout and the renderer split node
// labels on: the literal two characters, not a newline.
const archLabelSep = `\n`

var (
	reArchHeader = regexp.MustCompile(`(?i)^architecture-beta\b`)
	reArchGroup  = regexp.MustCompile(
		`^group\s+([\w][-\w]*)\s*(\([\w:.-]+\))?\s*(\[[^\]]*\])?\s*(?:in\s+([\w][-\w]*))?\s*$`)
	reArchService = regexp.MustCompile(
		`^service\s+([\w][-\w]*)\s*(\([\w:.-]+\)|"[^"]*")?\s*(\[[^\]]*\])?\s*(?:in\s+([\w][-\w]*))?\s*$`)
	reArchJunction = regexp.MustCompile(
		`^junction\s+([\w][-\w]*)\s*(?:in\s+([\w][-\w]*))?\s*$`)
	// An edge is an id, an optional {group}, a side letter, and the mirror
	// of that on the other side of the line, which carries the arrowheads
	// inside the dashes and an optional title between them.
	reArchEdge = regexp.MustCompile(
		`^([\w][-\w]*)(\{group\})?\s*:\s*([LRTB])\s*(<)?(?:--|-(\[[^\]]*\])-)(>)?\s*([LRTB])\s*:\s*([\w][-\w]*)(\{group\})?\s*$`)
	reArchSkip = regexp.MustCompile(`(?i)^(align|title|accTitle|accDescr)\b`)
)

// archDecl is one declared group, service or junction.
type archDecl struct {
	id       string
	icon     string
	label    string
	parent   string
	group    bool
	junction bool
}

// archEdge is one declared edge: the two ends, the side of each the line
// leaves through, and whether that end attaches to the border of the group
// holding it.
type archEdge struct {
	lhs, rhs           string
	lhsSide, rhsSide   byte
	lhsGroup, rhsGroup bool
	arrowLhs, arrowRhs bool
	label              string
}

// ParseArchitecture parses an architecture-beta diagram into a graph with
// explicit positions for the flowchart engine.
func ParseArchitecture(text string) *graph.Graph {
	return parseArchitecture(text, os.Stderr)
}

// parseArchitecture is ParseArchitecture with the writer conflicts are
// reported to.
func parseArchitecture(text string, warn io.Writer) *graph.Graph {
	var decls []archDecl
	var edges []archEdge

	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(stripComments(raw))
		if line == "" || reArchHeader.MatchString(line) || reArchSkip.MatchString(line) {
			continue
		}
		switch {
		case strings.HasPrefix(line, "group "):
			if m := reArchGroup.FindStringSubmatch(line); m != nil {
				decls = append(decls, archDecl{
					id: m[1], icon: archIcon(m[2]), label: archTitle(m[3]),
					parent: m[4], group: true,
				})
			}
		case strings.HasPrefix(line, "service "):
			if m := reArchService.FindStringSubmatch(line); m != nil {
				decls = append(decls, archDecl{
					id: m[1], icon: archIcon(m[2]), label: archTitle(m[3]), parent: m[4],
				})
			}
		case strings.HasPrefix(line, "junction "):
			if m := reArchJunction.FindStringSubmatch(line); m != nil {
				decls = append(decls, archDecl{id: m[1], parent: m[2], junction: true})
			}
		default:
			if m := reArchEdge.FindStringSubmatch(line); m != nil {
				edges = append(edges, archEdge{
					lhs: m[1], lhsGroup: m[2] != "", lhsSide: m[3][0],
					arrowLhs: m[4] != "", label: archTitle(m[5]), arrowRhs: m[6] != "",
					rhsSide: m[7][0], rhs: m[8], rhsGroup: m[9] != "",
				})
			}
		}
	}

	g := graph.NewGraph()
	g.Direction = graph.DirLR

	// Groups first, so a service's `in` resolves whatever order the
	// declarations came in.
	groups := make(map[string]*graph.Subgraph, len(decls))
	for _, d := range decls {
		if d.group {
			groups[d.id] = &graph.Subgraph{ID: d.id, Label: archGroupLabel(d)}
		}
	}
	for _, d := range decls {
		if !d.group {
			continue
		}
		sg := groups[d.id]
		if parent, ok := groups[d.parent]; ok && parent != sg {
			sg.Parent = parent
			parent.Children = append(parent.Children, sg)
			continue
		}
		g.Subgraphs = append(g.Subgraphs, sg)
	}

	for _, d := range decls {
		if d.group {
			continue
		}
		shape := graph.ShapeRectangle
		label := archNodeLabel(d)
		if d.junction {
			shape, label = graph.ShapeJunction, d.id
		}
		g.AddNode(&graph.Node{ID: d.id, Label: label, Shape: shape})
		if sg, ok := groups[d.parent]; ok {
			sg.NodeIDs = append(sg.NodeIDs, d.id)
		}
	}

	// An end an edge never declared is a service with no icon or label.
	for _, e := range edges {
		for _, id := range []string{e.lhs, e.rhs} {
			if _, ok := g.Nodes[id]; !ok {
				g.AddNode(&graph.Node{ID: id, Label: id})
			}
		}
	}

	positions := archPositions(g.NodeOrder, edges, warn)

	// The group of a `{group}` end, which is where the edge attaches.
	holder := func(id string) (string, bool) {
		if sg := g.FindSubgraphForNode(id); sg != nil {
			return sg.ID, true
		}
		return id, false
	}
	for _, e := range edges {
		// The engine reads an edge whose target lies left of its source as
		// a back edge and sends it around the bottom, which a positioned
		// graph has no use for: the ends are emitted left to right and the
		// arrowheads travel with them.
		if positions[e.rhs].Col < positions[e.lhs].Col {
			e = archEdge{
				lhs: e.rhs, rhs: e.lhs,
				lhsSide: e.rhsSide, rhsSide: e.lhsSide,
				lhsGroup: e.rhsGroup, rhsGroup: e.lhsGroup,
				arrowLhs: e.arrowRhs, arrowRhs: e.arrowLhs,
				label: e.label,
			}
		}
		src, srcIsGroup := e.lhs, false
		tgt, tgtIsGroup := e.rhs, false
		if e.lhsGroup {
			src, srcIsGroup = holder(e.lhs)
		}
		if e.rhsGroup {
			tgt, tgtIsGroup = holder(e.rhs)
		}
		g.AddEdge(graph.Edge{
			Source:           src,
			Target:           tgt,
			Label:            e.label,
			Style:            graph.EdgeSolid,
			HasArrowStart:    e.arrowLhs,
			HasArrowEnd:      e.arrowRhs,
			ArrowTypeStart:   graph.ArrowTypeArrow,
			ArrowTypeEnd:     graph.ArrowTypeArrow,
			MinLength:        1,
			SourceIsSubgraph: srcIsGroup,
			TargetIsSubgraph: tgtIsGroup,
		})
	}

	g.Positions = positions
	return g
}

// archStep is one hint: the node it places and where, relative to the node
// the hint is read from.
type archStep struct {
	to         string
	dCol, dRow int
}

// archPositions resolves the direction hints into one cell per node.
//
// Each end's letter names the side of its own node the line leaves through,
// so the near end's letter says which way the far node lies and the far
// end's letter says which way the near node lies from it. A hint across the
// axes places the far node diagonally, which the router draws as an elbow.
//
// The walk is breadth-first from the first declared node and first-fit: a
// node already placed keeps its cell and the later hint is reported, and a
// cell another node holds sends this one to the next free column of the row.
// Nodes the walk never reaches take a row below the rest.
func archPositions(order []string, edges []archEdge, warn io.Writer) map[string]graph.GridCoord {
	if len(order) == 0 {
		return nil
	}
	adj := make(map[string][]archStep, len(order))
	for _, e := range edges {
		dc, dr := archDelta(e.lhsSide, e.rhsSide)
		adj[e.lhs] = append(adj[e.lhs], archStep{e.rhs, dc, dr})
		adj[e.rhs] = append(adj[e.rhs], archStep{e.lhs, -dc, -dr})
	}

	at := map[string]graph.GridCoord{order[0]: {}}
	held := map[graph.GridCoord]string{{}: order[0]}
	maxRow := 0
	for queue := []string{order[0]}; len(queue) > 0; queue = queue[1:] {
		from := queue[0]
		for _, step := range adj[from] {
			want := graph.GridCoord{
				Col: at[from].Col + step.dCol,
				Row: at[from].Row + step.dRow,
			}
			if cur, placed := at[step.to]; placed {
				if cur != want {
					archWarn(warn, "%s is at column %d row %d; the hint from %s asks for column %d row %d",
						step.to, cur.Col, cur.Row, from, want.Col, want.Row)
				}
				continue
			}
			if other, taken := held[want]; taken {
				for held[want] != "" {
					want.Col++
				}
				archWarn(warn, "%s and %s both resolve to column %d row %d; %s moved to column %d",
					other, step.to, at[from].Col+step.dCol, want.Row, step.to, want.Col)
			}
			at[step.to] = want
			held[want] = step.to
			maxRow = max(maxRow, want.Row)
			queue = append(queue, step.to)
		}
	}

	col := 0
	for _, id := range order {
		if _, placed := at[id]; placed {
			continue
		}
		at[id] = graph.GridCoord{Col: col, Row: maxRow + 1}
		col++
	}
	return at
}

// archDelta is the cell offset from the node whose side is near to the node
// whose side is far. A letter on the column axis moves the far node along
// it; the far node's own letter points back, so its L puts it to the right.
func archDelta(near, far byte) (dCol, dRow int) {
	switch near {
	case 'L':
		dCol = -1
	case 'R':
		dCol = 1
	case 'T':
		dRow = -1
	default:
		dRow = 1
	}
	if archIsColumn(near) == archIsColumn(far) {
		return dCol, dRow
	}
	switch far {
	case 'L':
		dCol = 1
	case 'R':
		dCol = -1
	case 'T':
		dRow = 1
	default:
		dRow = -1
	}
	return dCol, dRow
}

// archIsColumn reports whether a side letter names a side on the column
// axis.
func archIsColumn(side byte) bool { return side == 'L' || side == 'R' }

// archWarn reports one conflict.
func archWarn(warn io.Writer, format string, args ...any) {
	if warn == nil {
		return
	}
	fmt.Fprintf(warn, "mmaid: architecture: "+format+"\n", args...)
}

// archIcon returns the icon name from `(name)` or from the quoted text form.
func archIcon(field string) string {
	field = strings.TrimSpace(field)
	if len(field) < 2 {
		return ""
	}
	return strings.Trim(field[1:len(field)-1], `"`)
}

// archTitle returns the label text from `[Label]`, unquoted.
func archTitle(field string) string {
	if len(field) < 2 {
		return ""
	}
	return strings.Trim(strings.TrimSpace(field[1:len(field)-1]), `"'`)
}

// archNodeLabel is a service's icon over its label, or its id when it has
// neither.
func archNodeLabel(d archDecl) string {
	var lines []string
	if d.icon != "" {
		lines = append(lines, "["+d.icon+"]")
	}
	if d.label != "" {
		lines = append(lines, d.label)
	}
	if len(lines) == 0 {
		return d.id
	}
	return strings.Join(lines, archLabelSep)
}

// archGroupLabel is a group's icon before its label; a border draws one
// line.
func archGroupLabel(d archDecl) string {
	label := d.label
	if label == "" {
		label = d.id
	}
	if d.icon != "" {
		return "[" + d.icon + "] " + label
	}
	return label
}
