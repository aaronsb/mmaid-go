package diagram

import (
	"regexp"
	"strings"

	"github.com/aaronsb/mmaid-go/internal/graph"
)

// ── requirementDiagram ───────────────────────────────────────────────────────
//
// SysML requirements and elements are two box kinds joined by named
// relationships, which is a flowchart. The parser flattens each block into a
// multi-line node label and hands the graph to the flowchart engine.
//
// A requirement box reads
//
//	test_req
//	id: 1
//	the test text.
//	risk: high | verify: test
//
// and an element box leads with its type, the way the other ported diagrams
// mark a node kind on its first line.

var (
	reReqHeader = regexp.MustCompile(`(?i)^requirementDiagram\s*$`)
	reReqBlock  = regexp.MustCompile(`(?i)^(requirement|functionalRequirement|interfaceRequirement|performanceRequirement|physicalRequirement|designConstraint|element)\s+(.+?)\s*\{\s*$`)
	reReqField  = regexp.MustCompile(`(?i)^(id|text|risk|verifymethod|type|docref)\s*:\s*(.*)$`)
	reReqClose  = regexp.MustCompile(`^\}\s*$`)
	// {source} - <type> -> {destination}
	reReqRelFwd = regexp.MustCompile(`^(.+?)\s+-\s*(\w+)\s*->\s+(.+?)$`)
	// {destination} <- <type> - {source}
	reReqRelBack = regexp.MustCompile(`^(.+?)\s+<-\s*(\w+)\s*-\s+(.+?)$`)
	reReqStyling = regexp.MustCompile(`(?i)^(style|classDef|class)\s`)
)

// requirementKinds are the SysML requirement types; anything else opening a
// block is an element.
var requirementKinds = map[string]bool{
	"requirement":            true,
	"functionalrequirement":  true,
	"interfacerequirement":   true,
	"performancerequirement": true,
	"physicalrequirement":    true,
	"designconstraint":       true,
}

// ParseRequirementDiagram parses a requirementDiagram into a *graph.Graph for
// the flowchart renderer.
func ParseRequirementDiagram(text string) *graph.Graph {
	g := graph.NewGraph()
	g.Direction = graph.DirTB

	lines := strings.Split(text, "\n")
	for i := 0; i < len(lines); i++ {
		line := stripLineComment(lines[i])
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || reReqHeader.MatchString(trimmed) || reReqStyling.MatchString(trimmed) {
			continue
		}

		if m := reDirectionStmt.FindStringSubmatch(trimmed); m != nil {
			g.Direction = normalizeDirection(m[1])
			g.DirectionExplicit = true
			continue
		}

		if m := reReqBlock.FindStringSubmatch(trimmed); m != nil {
			kind := strings.ToLower(m[1])
			name, class := splitClassSuffix(unquote(strings.TrimSpace(m[2])))
			fields, next := readRequirementFields(lines, i+1)
			i = next
			addRequirementNode(g, kind, name, class, fields)
			continue
		}

		if reReqClose.MatchString(trimmed) {
			continue
		}

		// A bare `name:::class` styling statement.
		if !strings.Contains(trimmed, "->") && !strings.Contains(trimmed, "<-") && strings.Contains(trimmed, ":::") {
			continue
		}

		if m := reReqRelBack.FindStringSubmatch(trimmed); m != nil {
			addRequirementEdge(g, unquote(strings.TrimSpace(m[3])), unquote(strings.TrimSpace(m[1])), strings.ToLower(m[2]))
			continue
		}
		if m := reReqRelFwd.FindStringSubmatch(trimmed); m != nil {
			addRequirementEdge(g, unquote(strings.TrimSpace(m[1])), unquote(strings.TrimSpace(m[3])), strings.ToLower(m[2]))
			continue
		}
	}

	return g
}

// readRequirementFields collects the `key: value` lines of a block and returns
// the index of its closing brace.
func readRequirementFields(lines []string, start int) (map[string]string, int) {
	fields := make(map[string]string)
	i := start
	for ; i < len(lines); i++ {
		trimmed := strings.TrimSpace(stripLineComment(lines[i]))
		if trimmed == "" {
			continue
		}
		if reReqClose.MatchString(trimmed) {
			return fields, i
		}
		if m := reReqField.FindStringSubmatch(trimmed); m != nil {
			fields[strings.ToLower(m[1])] = unquote(strings.TrimSpace(m[2]))
		}
	}
	return fields, i - 1
}

// addRequirementNode turns one block into a node. Requirements are rectangles
// led by their name; elements are rounded boxes led by their type.
func addRequirementNode(g *graph.Graph, kind, name, class string, fields map[string]string) {
	var lines []string
	shape := graph.ShapeRectangle

	if requirementKinds[kind] {
		lines = append(lines, name)
		if id := fields["id"]; id != "" {
			lines = append(lines, "id: "+id)
		}
		if text := fields["text"]; text != "" {
			lines = append(lines, text)
		}
		if tail := riskVerifyLine(fields); tail != "" {
			lines = append(lines, tail)
		}
	} else {
		shape = graph.ShapeRounded
		if t := fields["type"]; t != "" {
			lines = append(lines, "["+t+"]")
		} else {
			lines = append(lines, "[element]")
		}
		lines = append(lines, name)
		if ref := fields["docref"]; ref != "" {
			lines = append(lines, ref)
		}
	}

	g.AddNode(&graph.Node{
		ID:         name,
		Label:      joinLabel(lines),
		Shape:      shape,
		StyleClass: class,
	})
}

// riskVerifyLine renders the risk and verification method on one closing line.
func riskVerifyLine(fields map[string]string) string {
	risk, verify := fields["risk"], fields["verifymethod"]
	switch {
	case risk != "" && verify != "":
		return "risk: " + risk + " | verify: " + verify
	case risk != "":
		return "risk: " + risk
	case verify != "":
		return "verify: " + verify
	}
	return ""
}

// addRequirementEdge joins two blocks, creating either end if the source
// declared the relationship first.
func addRequirementEdge(g *graph.Graph, source, target, relType string) {
	for _, id := range []string{source, target} {
		if _, ok := g.Nodes[id]; !ok {
			g.AddNode(&graph.Node{ID: id, Label: id, Shape: graph.ShapeRectangle})
		}
	}
	edge := graph.NewEdge(source, target)
	edge.Label = relType
	g.AddEdge(edge)
}
