package diagram

import (
	"regexp"
	"slices"
	"strings"

	"github.com/aaronsb/mmaid-go/internal/graph"
)

// ── usecaseDiagram ───────────────────────────────────────────────────────────
//
// Actors, use cases and boundaries are nodes, nodes and groups, so the
// flowchart engine draws them. An actor is a rectangle led by an `[actor]`
// line, a use case is a stadium, and the UML relationships become edge styles:
// include and extend are dotted and carry their stereotype as the label,
// generalization is a plain arrow because the engine has no hollow arrowhead.

var (
	reUCHeader   = regexp.MustCompile(`(?i)^usecase(Diagram|-beta)?\s*$`)
	reUCActor    = regexp.MustCompile(`(?i)^actor\s+(.+)$`)
	reUCUseCase  = regexp.MustCompile(`(?i)^usecase\s+(.+)$`)
	reUCGroup    = regexp.MustCompile(`(?i)^(systemBoundary|package|rectangle|system)\b\s*(.*)$`)
	reUCEnd      = regexp.MustCompile(`(?i)^end\s*$`)
	reUCIgnored  = regexp.MustCompile(`(?i)^(note|json|classDef|class|style|title|accTitle|accDescr)\b`)
	reUCMetadata = regexp.MustCompile(`@\{[^}]*\}`)
	reUCStereo   = regexp.MustCompile(`<<[^>]*>>`)

	// Longest operators first: `--|>` and `-->` both open with `--`. The
	// reversed markers need a word boundary so `Foo--> B` is not read as an
	// `o--` starting inside the identifier.
	reUCRel = regexp.MustCompile(`^(.*?)\s*(\.\.>|--\|>|<--|-{2,}>|--o|--x|\bo--|\bx--|-{2,})\s*(.*)$`)

	reUCParen   = regexp.MustCompile(`^([A-Za-z0-9_]*)\s*\((.*)\)$`)
	reUCSquare  = regexp.MustCompile(`^([A-Za-z0-9_]*)\s*\[(.*)\]$`)
	reUCQuoted  = regexp.MustCompile(`^"(.*)"$`)
	reUCAsID    = regexp.MustCompile(`(?i)^(.+)\s+as\s+([A-Za-z0-9_]+)$`)
	reUCAsLabel = regexp.MustCompile(`(?i)^([A-Za-z0-9_]+)\s+as\s+(.+)$`)
	reUCIdent   = regexp.MustCompile(`^[A-Za-z0-9_]+$`)
	reUCNonID   = regexp.MustCompile(`[^A-Za-z0-9_]`)
)

// ucParser accumulates the graph while walking boundary blocks.
type ucParser struct {
	g      *graph.Graph
	stack  []*graph.Subgraph
	actors map[string]bool
}

// ParseUseCaseDiagram parses a usecaseDiagram into a *graph.Graph for the
// flowchart renderer.
func ParseUseCaseDiagram(text string) *graph.Graph {
	p := &ucParser{g: graph.NewGraph(), actors: make(map[string]bool)}
	p.g.Direction = graph.DirTB

	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(stripLineComment(raw))
		if line == "" || reUCHeader.MatchString(line) || reUCIgnored.MatchString(line) {
			continue
		}
		if m := reDirectionStmt.FindStringSubmatch(line); m != nil {
			p.g.Direction = normalizeDirection(m[1])
			p.g.DirectionExplicit = true
			continue
		}
		if reUCEnd.MatchString(line) {
			if n := len(p.stack); n > 0 {
				p.stack = p.stack[:n-1]
			}
			continue
		}
		if m := reUCRel.FindStringSubmatch(line); m != nil {
			p.relationship(m[1], m[2], m[3])
			continue
		}
		if m := reUCGroup.FindStringSubmatch(line); m != nil {
			p.openGroup(m[2])
			continue
		}
		if m := reUCActor.FindStringSubmatch(line); m != nil {
			p.declare(m[1], true)
			continue
		}
		if m := reUCUseCase.FindStringSubmatch(line); m != nil {
			p.declare(m[1], false)
			continue
		}
		p.declare(line, false)
	}

	return p.g
}

// openGroup pushes a subgraph for a systemBoundary, package, rectangle or
// system block.
func (p *ucParser) openGroup(spec string) {
	id, label, _ := parseUCSpec(spec)
	if id == "" {
		id = label
	}
	sg := &graph.Subgraph{ID: id, Label: label}
	if n := len(p.stack); n > 0 {
		sg.Parent = p.stack[n-1]
		sg.Parent.Children = append(sg.Parent.Children, sg)
	} else {
		p.g.Subgraphs = append(p.g.Subgraphs, sg)
	}
	p.stack = append(p.stack, sg)
}

// declare adds or upgrades one actor or use case and returns its ID.
func (p *ucParser) declare(spec string, isActor bool) string {
	id, label, class := parseUCSpec(spec)
	if id == "" {
		return ""
	}

	node, exists := p.g.Nodes[id]
	if !exists {
		node = &graph.Node{ID: id}
		p.g.AddNode(node)
	}
	if class != "" {
		node.StyleClass = class
	}
	// A relationship endpoint resolves to a use case; a later `actor` line
	// promotes it, which is why the kind is written on every pass.
	if isActor {
		p.actors[id] = true
	}
	if p.actors[id] {
		node.Shape = graph.ShapeRectangle
		node.Label = joinLabel([]string{"[actor]", label})
	} else {
		node.Shape = graph.ShapeStadium
		node.Label = label
	}
	p.place(id)
	return id
}

// place files a node under the innermost open boundary.
func (p *ucParser) place(id string) {
	n := len(p.stack)
	if n == 0 {
		return
	}
	sg := p.stack[n-1]
	if !slices.Contains(sg.NodeIDs, id) {
		sg.NodeIDs = append(sg.NodeIDs, id)
	}
}

// relationship parses one relationship statement. `head` and `tail` are the
// text either side of the operator.
func (p *ucParser) relationship(head, op, tail string) {
	label := ""

	// `A -- "label" --> B`: a bare dash run followed by a second operator is a
	// labelled association, not an association to a label.
	if strings.HasPrefix(op, "-") && !strings.HasSuffix(op, ">") && op != "--o" && op != "--x" {
		if m := reUCRel.FindStringSubmatch(tail); m != nil {
			label = unquote(m[1])
			op, tail = m[2], m[3]
		}
	}

	if op == "..>" {
		p.includeExtend(head, tail)
		return
	}

	// A trailing `: text` labels the association.
	if i := strings.LastIndex(tail, ":"); i >= 0 && label == "" {
		if lbl := strings.TrimSpace(tail[i+1:]); lbl != "" {
			label = strings.Trim(unquote(lbl), "<>")
			tail = tail[:i]
		}
	}

	source := p.endpoint(head)
	target := p.endpoint(tail)
	if source == "" || target == "" {
		return
	}
	if op == "<--" || op == "o--" || op == "x--" {
		source, target = target, source
	}

	edge := graph.NewEdge(source, target)
	edge.Label = label
	switch op {
	case "--o", "o--":
		edge.ArrowTypeEnd = graph.ArrowTypeCircle
	case "--x", "x--":
		edge.ArrowTypeEnd = graph.ArrowTypeCross
	case "--|>":
		// UML draws a hollow triangle; the engine has one arrowhead.
		edge.ArrowTypeEnd = graph.ArrowTypeArrow
	default:
		// A bare dash run is an association with no marker.
		if !strings.HasSuffix(op, ">") && !strings.HasPrefix(op, "<") {
			edge.HasArrowEnd = false
		}
	}
	p.g.AddEdge(edge)
}

// includeExtend handles `U ..> : include V`, `U ..> V : <<extend>>` and the
// bare `U ..> V`, which UML reads as an include.
func (p *ucParser) includeExtend(head, tail string) {
	tail = strings.TrimSpace(tail)
	kind, target := "include", tail

	if after, ok := strings.CutPrefix(tail, ":"); ok {
		fields := strings.Fields(after)
		if len(fields) > 0 {
			kind = normalizeUCStereotype(fields[0])
			target = strings.Join(fields[1:], " ")
		}
	} else if i := strings.LastIndex(tail, ":"); i >= 0 {
		kind = normalizeUCStereotype(strings.TrimSpace(tail[i+1:]))
		target = tail[:i]
	}

	source := p.endpoint(head)
	dest := p.endpoint(target)
	if source == "" || dest == "" {
		return
	}
	edge := graph.NewEdge(source, dest)
	edge.Style = graph.EdgeDotted
	edge.Label = "<<" + kind + ">>"
	p.g.AddEdge(edge)
}

// normalizeUCStereotype reduces `<<include>>`, `include` and `Include` to a
// bare keyword, defaulting to include.
func normalizeUCStereotype(s string) string {
	s = strings.ToLower(strings.Trim(strings.TrimSpace(s), "<>:"))
	if s == "extend" || s == "extends" {
		return "extend"
	}
	return "include"
}

// endpoint resolves one side of a relationship, creating an ellipse use case
// for an undeclared name. Actors are never inferred.
func (p *ucParser) endpoint(spec string) string {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return ""
	}
	if id, _, _ := parseUCSpec(spec); id != "" {
		if _, exists := p.g.Nodes[id]; exists {
			return id
		}
	}
	return p.declare(spec, false)
}

// parseUCSpec splits a declaration into an identifier, a display label and a
// `:::class` suffix. It accepts `ID`, `ID("Label")`, `ID[Label]`, `"Label"`,
// `(Label) as ID` and `"Label" as ID`.
func parseUCSpec(spec string) (id, label, class string) {
	spec = strings.TrimSpace(reUCStereo.ReplaceAllString(reUCMetadata.ReplaceAllString(spec, ""), ""))
	spec, class = splitClassSuffix(spec)
	if spec == "" {
		return "", "", class
	}

	// `(Do thing) as U`, `"Display Name" as A`, `A as "Display Name"`
	if m := reUCAsID.FindStringSubmatch(spec); m != nil {
		return m[2], ucLabel(m[1]), class
	}
	if m := reUCAsLabel.FindStringSubmatch(spec); m != nil {
		return m[1], ucLabel(m[2]), class
	}

	if m := reUCParen.FindStringSubmatch(spec); m != nil {
		return ucID(m[1], m[2]), unquote(m[2]), class
	}
	if m := reUCSquare.FindStringSubmatch(spec); m != nil {
		return ucID(m[1], m[2]), unquote(m[2]), class
	}
	if m := reUCQuoted.FindStringSubmatch(spec); m != nil {
		return ucSlug(m[1]), unquote(spec), class
	}
	if reUCIdent.MatchString(spec) {
		return spec, spec, class
	}
	return ucSlug(spec), spec, class
}

// ucLabel strips the delimiters an inline label may carry.
func ucLabel(s string) string {
	if m := reUCParen.FindStringSubmatch(s); m != nil && m[1] == "" {
		return unquote(m[2])
	}
	if m := reUCSquare.FindStringSubmatch(s); m != nil && m[1] == "" {
		return unquote(m[2])
	}
	return unquote(s)
}

// ucID prefers the written identifier and falls back to one derived from the
// label, the way Mermaid names an anonymous declaration.
func ucID(written, label string) string {
	if written != "" {
		return written
	}
	return ucSlug(unquote(label))
}

// ucSlug replaces every non-word character with an underscore.
func ucSlug(s string) string {
	return strings.Trim(reUCNonID.ReplaceAllString(strings.TrimSpace(s), "_"), "_")
}
