package diagram

import (
	"regexp"
	"slices"
	"strings"

	"github.com/aaronsb/mmaid-go/internal/graph"
)

// ── usecase-beta ─────────────────────────────────────────────────────────────
//
// Actors, use cases and boundaries are nodes, nodes and groups, so the
// flowchart engine draws them. An actor is a rectangle led by an `[actor]`
// line, a use case is a stadium, and the UML relationships become edge styles:
// include and extend are dotted and carry their stereotype as the label,
// generalization is a plain arrow because the engine has no hollow arrowhead.
//
// `usecaseDiagram` and a bare `usecase` are accepted as aliases for the
// documented `usecase-beta` header.
//
// Skipped: notes (`note for X "..."`); `json` tables; `classDef`, `class` and
// `style`; `title`, `accTitle` and `accDescr`; `@{ ... }` metadata, so actor
// type/icon/business variants, edge animation and boundary `type: package` all
// render as the default shape; `<<stereotypes>>` on a declaration; explicit
// edge IDs, which are parsed and discarded rather than made styleable; extra
// dashes requesting a longer edge; Mermaid entity codes such as `#quot;`.
// A line that matches no statement shape is dropped, not turned into a node.

var (
	reUCHeader   = regexp.MustCompile(`(?i)^usecase(Diagram|-beta)?\s*$`)
	reUCActor    = regexp.MustCompile(`^actor(?:\s+(.*))?$`)
	reUCUseCase  = regexp.MustCompile(`^usecase(?:\s+(.*))?$`)
	reUCEnd      = regexp.MustCompile(`^end\s*$`)
	reUCMetadata = regexp.MustCompile(`@\{[^}]*\}`)
	reUCStereo   = regexp.MustCompile(`<<[^>]*>>`)

	// Mermaid's usecase keywords are lowercase and case-sensitive, and a
	// keyword only opens a statement when a separator follows it — otherwise
	// a node legitimately named `System` or `Title` is swallowed.
	reUCGroup   = regexp.MustCompile(`^systemBoundary(?:\s+(.*))?$`)
	reUCIgnored = regexp.MustCompile(`^(note|json|classDef|class|style|title|accTitle|accDescr)(?:\s|:|$)`)

	// Block-structured statements whose body spans lines. Their closing brace
	// must not be mistaken for anything else.
	reUCBlockOpen = regexp.MustCompile(`^(?:accDescr\s*\{|json\b.*@\{)`)

	// A statement that carries nothing but metadata targets an existing
	// element, an edge, or nothing at all; it never declares one.
	reUCMetaOnly = regexp.MustCompile(`^([A-Za-z0-9_]+)@\{[^}]*\}(?::::[A-Za-z0-9_-]+)?$`)

	// Longest operators first: `--|>` and `-->` both open with `--`. The
	// reversed markers need a word boundary so `Foo--> B` is not read as an
	// `o--` starting inside the identifier. The optional `id@` before the
	// operator is Mermaid's explicit edge ID.
	reUCRel = regexp.MustCompile(`^(.*?)(?:\s+([A-Za-z0-9_]+)@)?\s*(\.\.>|--\|>|<--|-{2,}>|--o|--x|\bo--|\bx--|-{2,})\s*(.*)$`)

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

// ParseUseCaseDiagram parses a usecase-beta diagram into a *graph.Graph for the
// flowchart renderer.
func ParseUseCaseDiagram(text string) *graph.Graph {
	p := &ucParser{g: graph.NewGraph(), actors: make(map[string]bool)}
	p.g.Direction = graph.DirTB

	lines := strings.Split(text, "\n")
	for i := 0; i < len(lines); i++ {
		// A quoted or backtick label may hold a physical line break, so a
		// statement runs on until its quoting closes.
		stmt := lines[i]
		for !quotingBalanced(stmt) && i+1 < len(lines) {
			i++
			stmt += labelSep + strings.TrimSpace(lines[i])
		}

		line := strings.TrimSpace(stripLineComment(stmt))
		if line == "" || reUCHeader.MatchString(line) {
			continue
		}
		if reUCBlockOpen.MatchString(line) {
			i = skipBraceBlock(lines, i, line)
			continue
		}
		if reUCIgnored.MatchString(line) {
			continue
		}
		// `Payment_service@{ type: package }` and `opens@{ animate: false }`
		// attach metadata to a boundary or an edge. Metadata this parser does
		// not read has nothing to attach, and a metadata statement never
		// declares an element, so either way the line adds nothing.
		if reUCMetaOnly.MatchString(line) {
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
		if m := ucRelation(line); m != nil {
			p.relationship(m[0], m[1], m[2], m[3])
			continue
		}
		if m := reUCGroup.FindStringSubmatch(line); m != nil {
			p.openGroup(m[1])
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
		// Only a line shaped like a declaration becomes one; anything else is
		// grammar this parser does not know, and inventing a node from it
		// fabricates diagram content.
		if looksLikeUCDeclaration(line) {
			p.declare(line, false)
		}
	}

	return p.g
}

// ucRelation matches a relationship statement, returning head, edge ID,
// operator and tail. An operator inside quotes or brackets belongs to a label,
// so it does not make the line a relationship.
func ucRelation(line string) []string {
	idx := reUCRel.FindStringSubmatchIndex(line)
	if idx == nil {
		return nil
	}
	if !outsideLabel(line, idx[6]) {
		return nil
	}
	groups := make([]string, 4)
	for g := 1; g <= 4; g++ {
		if s, e := idx[2*g], idx[2*g+1]; s >= 0 {
			groups[g-1] = line[s:e]
		}
	}
	return groups
}

// outsideLabel reports whether byte offset pos sits outside every quoted span
// and bracketed group.
func outsideLabel(s string, pos int) bool {
	inQuote, depth := false, 0
	for i := 0; i < pos && i < len(s); i++ {
		switch s[i] {
		case '"':
			inQuote = !inQuote
		case '(', '[', '{':
			if !inQuote {
				depth++
			}
		case ')', ']', '}':
			if !inQuote && depth > 0 {
				depth--
			}
		}
	}
	return !inQuote && depth == 0
}

// quotingBalanced reports whether a statement's double quotes and backticks
// both close.
func quotingBalanced(s string) bool {
	return strings.Count(s, `"`)%2 == 0 && strings.Count(s, "`")%2 == 0
}

// skipBraceBlock returns the index of the line closing a brace-delimited block
// that opened on line `start`, whose text is `first`.
func skipBraceBlock(lines []string, start int, first string) int {
	depth := braceDepth(first)
	if depth <= 0 {
		return start
	}
	for i := start + 1; i < len(lines); i++ {
		if depth += braceDepth(lines[i]); depth <= 0 {
			return i
		}
	}
	return len(lines) - 1
}

// braceDepth is the net brace count of a line, ignoring braces inside quotes.
func braceDepth(s string) int {
	depth, inQuote := 0, false
	for _, r := range s {
		switch {
		case r == '"':
			inQuote = !inQuote
		case inQuote:
		case r == '{':
			depth++
		case r == '}':
			depth--
		}
	}
	return depth
}

// looksLikeUCDeclaration reports whether a bare line has the shape of an actor
// or use case declaration.
func looksLikeUCDeclaration(line string) bool {
	spec := strings.TrimSpace(reUCStereo.ReplaceAllString(reUCMetadata.ReplaceAllString(line, ""), ""))
	spec, _ = splitClassSuffix(spec)
	if spec == "" {
		return false
	}
	return reUCParen.MatchString(spec) || reUCSquare.MatchString(spec) ||
		reUCQuoted.MatchString(spec) || reUCIdent.MatchString(spec) ||
		reUCAsID.MatchString(spec) || reUCAsLabel.MatchString(spec)
}

// openGroup pushes a subgraph for a systemBoundary block.
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
// text either side of the operator; `edgeID` is Mermaid's optional `id@`
// handle for `class`, `style` and animation, none of which this parser reads,
// so it is consumed here only to keep it out of the source endpoint.
func (p *ucParser) relationship(head, edgeID, op, tail string) {
	label := ""

	// `A -- "label" --> B`: a bare dash run followed by a second operator is a
	// labelled association, not an association to a label.
	if strings.HasPrefix(op, "-") && !strings.HasSuffix(op, ">") && op != "--o" && op != "--x" {
		if m := ucRelation(tail); m != nil {
			label = unquote(m[0])
			op, tail = m[2], m[3]
		}
	}

	if op == "..>" {
		p.includeExtend(head, tail)
		return
	}

	// A trailing ` : text` labels the association. The separator is the first
	// one outside quotes and brackets, so `Time("Set time: 10:00")` keeps its
	// colon.
	if label == "" {
		if before, after, ok := cutOutsideLabel(tail, " : "); ok && after != "" {
			label = strings.Trim(unquote(after), "<>")
			tail = before
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
	} else if before, after, ok := cutOutsideLabel(tail, " : "); ok {
		kind = normalizeUCStereotype(after)
		target = before
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

// cutOutsideLabel splits on the first occurrence of sep that lies outside
// quotes, parentheses and brackets.
func cutOutsideLabel(s, sep string) (before, after string, found bool) {
	for i := 0; i+len(sep) <= len(s); i++ {
		if s[i:i+len(sep)] == sep && outsideLabel(s, i) {
			return strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+len(sep):]), true
		}
	}
	return s, "", false
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
