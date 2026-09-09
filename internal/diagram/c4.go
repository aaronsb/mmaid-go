package diagram

import (
	"regexp"
	"slices"
	"strings"

	"github.com/aaronsb/mmaid-go/internal/graph"
)

// ── C4 ───────────────────────────────────────────────────────────────────────
//
// C4Context, C4Container, C4Component, C4Dynamic and C4Deployment share one
// PlantUML-derived macro vocabulary, so one parser covers all five. Elements
// become nodes, boundaries and deployment nodes become subgraphs, and Rel
// becomes an edge; the flowchart engine places them.
//
// C4's own layout is statement-order driven and its direction hints
// (Rel_U/D/L/R) address a placement model this engine does not have. They are
// read only to pick between a left-to-right and a top-to-bottom graph.
//
// Skipped: `title`, `accTitle` and `accDescr` — the graph model carries no
// title; `UpdateElementStyle`, `UpdateRelStyle`, `UpdateLayoutConfig` and the
// `Lay_*` macros; `$sprite`, `$tags` and `$link` arguments; `RelIndex`'s index,
// since sequence follows statement order; the `?descr` argument of `Rel` and of
// `Deployment_Node`, which have no room beside the label the engine draws;
// `AddElementTag`/`AddRelTag` and the legend they feed.

var (
	reC4Header = regexp.MustCompile(`(?i)^C4(Context|Container|Component|Dynamic|Deployment)\b`)
	reC4Macro  = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)\s*\((.*)\)\s*(\{)?\s*$`)
	reC4Open   = regexp.MustCompile(`^\{\s*$`)
	reC4Close  = regexp.MustCompile(`^\}\s*$`)
	reC4Named  = regexp.MustCompile(`^\$(\w+)\s*=\s*(.*)$`)
	reC4Title  = regexp.MustCompile(`(?i)^(title|accTitle|accDescr)\b`)
	reC4Block  = regexp.MustCompile(`^accDescr\s*\{`)
)

// c4Kind describes how one element macro maps onto a graph node.
type c4Kind struct {
	marker  string // leading label line, empty for none
	shape   graph.NodeShape
	techArg int // positional index of the technology/type argument, -1 if none
	descArg int // positional index of the description argument
}

// c4Kinds is the element vocabulary. The `_Ext` variants are handled by
// trimming the suffix, so only the base names appear here.
var c4Kinds = map[string]c4Kind{
	"person":         {marker: "[person]", shape: graph.ShapeRectangle, techArg: -1, descArg: 2},
	"system":         {shape: graph.ShapeRectangle, techArg: -1, descArg: 2},
	"systemdb":       {shape: graph.ShapeCylinder, techArg: -1, descArg: 2},
	"systemqueue":    {shape: graph.ShapeStadium, techArg: -1, descArg: 2},
	"container":      {shape: graph.ShapeRectangle, techArg: 2, descArg: 3},
	"containerdb":    {shape: graph.ShapeCylinder, techArg: 2, descArg: 3},
	"containerqueue": {shape: graph.ShapeStadium, techArg: 2, descArg: 3},
	"component":      {shape: graph.ShapeRectangle, techArg: 2, descArg: 3},
	"componentdb":    {shape: graph.ShapeCylinder, techArg: 2, descArg: 3},
	"componentqueue": {shape: graph.ShapeStadium, techArg: 2, descArg: 3},
}

// c4Boundaries are the macros that open a `{ }` block rendered as a subgraph.
// The value is the positional index of the type/subtitle argument, -1 if none.
var c4Boundaries = map[string]int{
	"boundary":            2,
	"system_boundary":     -1,
	"container_boundary":  -1,
	"component_boundary":  -1,
	"enterprise_boundary": -1,
	"node":                2,
	"node_l":              2,
	"node_r":              2,
	"deployment_node":     2,
}

// c4Pending is a boundary macro waiting for the `{` on the next line.
type c4Pending struct {
	args    c4Args
	typeArg int
}

// c4Parser accumulates the graph while walking the boundary stack.
type c4Parser struct {
	g       *graph.Graph
	stack   []*graph.Subgraph
	pending *c4Pending
	relLR   bool // a Rel_L/Rel_R was seen
	relTB   bool // a Rel_U/Rel_D was seen
}

// ParseC4Diagram parses any of the five C4 diagram types into a *graph.Graph
// for the flowchart renderer.
func ParseC4Diagram(text string) *graph.Graph {
	p := &c4Parser{g: graph.NewGraph()}
	p.g.Direction = graph.DirTB

	explicit := false
	lines := strings.Split(text, "\n")
	for i := 0; i < len(lines); i++ {
		trimmed := strings.TrimSpace(stripLineComment(lines[i]))
		if trimmed == "" || reC4Header.MatchString(trimmed) {
			continue
		}
		// `accDescr { ... }` shares the boundary's brace shape, so its closing
		// brace would otherwise pop whichever boundary is open. This runs ahead
		// of the single-line `accDescr:` form the title check absorbs.
		if reC4Block.MatchString(trimmed) {
			i = skipBraceBlock(lines, i, trimmed)
			p.pending = nil
			continue
		}
		if reC4Title.MatchString(trimmed) {
			p.pending = nil
			continue
		}
		if m := reDirectionStmt.FindStringSubmatch(trimmed); m != nil {
			p.g.Direction = normalizeDirection(m[1])
			explicit = true
			p.pending = nil
			continue
		}
		// `Boundary(b1, "Inner")` followed by a bare `{` on the next line is a
		// grammatical form of its own, so a boundary macro without a brace is
		// held until the next line decides.
		if reC4Open.MatchString(trimmed) {
			if p.pending != nil {
				p.openBoundary(p.pending.args, p.pending.typeArg)
				p.pending = nil
			}
			continue
		}
		if reC4Close.MatchString(trimmed) {
			p.pending = nil
			p.closeBoundary()
			continue
		}
		// A boundary macro whose brace never arrives declares nothing.
		p.pending = nil
		if m := reC4Macro.FindStringSubmatch(trimmed); m != nil {
			p.macro(strings.ToLower(m[1]), parseC4Args(m[2]), m[3] == "{")
		}
	}

	if explicit {
		p.g.DirectionExplicit = true
	} else if p.relLR && !p.relTB {
		p.g.Direction = graph.DirLR
	}
	return p.g
}

// c4Args is one macro's argument list: positional values plus the `$name=value`
// forms, which may stand in for any positional argument.
type c4Args struct {
	pos   []string
	named map[string]string
}

// at returns positional argument i, or the empty string.
func (a c4Args) at(i int) string {
	if i < 0 || i >= len(a.pos) {
		return ""
	}
	return a.pos[i]
}

// pick returns the first named argument present, falling back to positional i.
func (a c4Args) pick(i int, names ...string) string {
	for _, n := range names {
		if v, ok := a.named[n]; ok {
			return v
		}
	}
	return a.at(i)
}

// parseC4Args splits a macro argument list, separating named arguments.
func parseC4Args(s string) c4Args {
	args := c4Args{named: make(map[string]string)}
	for _, raw := range splitArgs(s) {
		if m := reC4Named.FindStringSubmatch(raw); m != nil {
			args.named[strings.ToLower(m[1])] = unquote(m[2])
			continue
		}
		args.pos = append(args.pos, unquote(raw))
	}
	return args
}

// macro dispatches one parsed macro call.
func (p *c4Parser) macro(name string, args c4Args, opensBlock bool) {
	switch {
	case name == "updateelementstyle", name == "updaterelstyle",
		name == "updatelayoutconfig", strings.HasPrefix(name, "lay_"):
		return
	case name == "relindex":
		// The index is decorative here: order follows the statements.
		if len(args.pos) > 0 {
			args.pos = args.pos[1:]
		}
		p.rel(args, false, false)
		return
	case name == "birel":
		p.rel(args, true, false)
		return
	case name == "rel_back":
		p.rel(args, false, true)
		return
	case name == "rel" || strings.HasPrefix(name, "rel_"):
		switch strings.TrimPrefix(name, "rel_") {
		case "l", "left", "r", "right":
			p.relLR = true
		case "u", "up", "d", "down":
			p.relTB = true
		}
		p.rel(args, false, false)
		return
	}

	if typeArg, ok := c4Boundaries[name]; ok {
		if opensBlock {
			p.openBoundary(args, typeArg)
		} else {
			p.pending = &c4Pending{args: args, typeArg: typeArg}
		}
		return
	}

	base := strings.TrimSuffix(name, "_ext")
	kind, ok := c4Kinds[base]
	if !ok {
		return
	}
	p.element(args, kind, base != name)
}

// element adds one node, composed as marker / name / (technology) / description.
func (p *c4Parser) element(args c4Args, kind c4Kind, external bool) {
	alias := args.pick(0, "alias")
	if alias == "" {
		return
	}
	name := args.pick(1, "label")
	if name == "" {
		name = alias
	}
	if external {
		name += " (ext)"
	}

	lines := []string{}
	if kind.marker != "" {
		lines = append(lines, kind.marker)
	}
	lines = append(lines, name)
	if tech := args.pick(kind.techArg, "techn", "type"); tech != "" {
		lines = append(lines, "("+tech+")")
	}
	if descr := args.pick(kind.descArg, "descr"); descr != "" {
		lines = append(lines, descr)
	}

	p.g.AddNode(&graph.Node{ID: alias, Label: joinLabel(lines), Shape: kind.shape})
	p.place(alias)
}

// openBoundary pushes a subgraph for a boundary or deployment node.
func (p *c4Parser) openBoundary(args c4Args, typeArg int) {
	alias := args.pick(0, "alias")
	label := args.pick(1, "label")
	if label == "" {
		label = alias
	}
	if t := args.pick(typeArg, "type"); t != "" {
		label += " (" + t + ")"
	}

	sg := &graph.Subgraph{ID: alias, Label: label}
	if parent := p.current(); parent != nil {
		sg.Parent = parent
		parent.Children = append(parent.Children, sg)
	} else {
		p.g.Subgraphs = append(p.g.Subgraphs, sg)
	}
	p.stack = append(p.stack, sg)
}

func (p *c4Parser) closeBoundary() {
	if n := len(p.stack); n > 0 {
		p.stack = p.stack[:n-1]
	}
}

func (p *c4Parser) current() *graph.Subgraph {
	if n := len(p.stack); n > 0 {
		return p.stack[n-1]
	}
	return nil
}

// place files a node under the innermost open boundary.
func (p *c4Parser) place(id string) {
	sg := p.current()
	if sg == nil || slices.Contains(sg.NodeIDs, id) {
		return
	}
	sg.NodeIDs = append(sg.NodeIDs, id)
}

// rel adds one relationship. `back` flips the direction rather than drawing a
// reversed arrowhead, so the layout still reads along the arrow.
func (p *c4Parser) rel(args c4Args, bidirectional, back bool) {
	from, to := args.pick(0, "from"), args.pick(1, "to")
	if from == "" || to == "" {
		return
	}
	p.ensure(from)
	p.ensure(to)
	if back {
		from, to = to, from
	}

	edge := graph.NewEdge(from, to)
	// Edge labels are drawn on one line, so the technology follows the label
	// rather than sitting under it as it does in a node.
	label := args.pick(2, "label")
	if tech := args.pick(3, "techn"); tech != "" {
		label = strings.TrimSpace(label + " [" + tech + "]")
	}
	edge.Label = label
	if bidirectional {
		edge.HasArrowStart = true
		edge.ArrowTypeStart = graph.ArrowTypeArrow
	}
	p.g.AddEdge(edge)
}

// ensure creates a placeholder for a relationship endpoint that no element
// macro declared.
func (p *c4Parser) ensure(id string) {
	if _, ok := p.g.Nodes[id]; !ok {
		p.g.AddNode(&graph.Node{ID: id, Label: id, Shape: graph.ShapeRectangle})
	}
}
