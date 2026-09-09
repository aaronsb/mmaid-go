package parser

import (
	"io"
	"strings"
	"testing"

	"github.com/aaronsb/mmaid-go/internal/graph"
)

// parseArch parses a source with the header and indentation the fixtures use
// and returns the graph with whatever went to the warning writer.
func parseArch(t *testing.T, body string) (*graph.Graph, string) {
	t.Helper()
	var warn strings.Builder
	g := parseArchitecture("architecture-beta\n"+body, &warn)
	return g, warn.String()
}

func posOf(t *testing.T, g *graph.Graph, id string) graph.GridCoord {
	t.Helper()
	p, ok := g.Positions[id]
	if !ok {
		t.Fatalf("%s has no position", id)
	}
	return p
}

// TestArchitectureHintDirections walks one hint of each direction from the
// first declared node.
func TestArchitectureHintDirections(t *testing.T) {
	tests := []struct {
		hint string
		want graph.GridCoord
	}{
		{"a:R -- L:b", graph.GridCoord{Col: 1, Row: 0}},
		{"a:L -- R:b", graph.GridCoord{Col: -1, Row: 0}},
		{"a:B -- T:b", graph.GridCoord{Col: 0, Row: 1}},
		{"a:T -- B:b", graph.GridCoord{Col: 0, Row: -1}},
	}
	for _, tt := range tests {
		g, warn := parseArch(t, "service a\nservice b\n"+tt.hint+"\n")
		if warn != "" {
			t.Errorf("%s: %s", tt.hint, warn)
		}
		if got := posOf(t, g, "a"); got != (graph.GridCoord{}) {
			t.Errorf("%s: a at %v, want the origin", tt.hint, got)
		}
		if got := posOf(t, g, "b"); got != tt.want {
			t.Errorf("%s: b at %v, want %v", tt.hint, got, tt.want)
		}
	}
}

// TestArchitectureBendPlacesDiagonally checks a hint pair across the two
// axes, and that it resolves the same way whichever end the walk reaches
// first.
func TestArchitectureBendPlacesDiagonally(t *testing.T) {
	g, _ := parseArch(t, "service a\nservice b\na:R -- B:b\n")
	if got, want := posOf(t, g, "b"), (graph.GridCoord{Col: 1, Row: -1}); got != want {
		t.Errorf("b at %v, want %v", got, want)
	}
	rev, _ := parseArch(t, "service b\nservice a\na:R -- B:b\n")
	if got, want := posOf(t, rev, "a"), (graph.GridCoord{Col: -1, Row: 1}); got != want {
		t.Errorf("walked from b: a at %v, want %v", got, want)
	}
}

// TestArchitectureChain walks three hints out from one node.
func TestArchitectureChain(t *testing.T) {
	g, warn := parseArch(t, `service a
service b
service c
service d
a:R -- L:b
b:B -- T:c
c:R -- L:d
`)
	if warn != "" {
		t.Errorf("unexpected report: %s", warn)
	}
	want := map[string]graph.GridCoord{
		"a": {Col: 0, Row: 0},
		"b": {Col: 1, Row: 0},
		"c": {Col: 1, Row: 1},
		"d": {Col: 2, Row: 1},
	}
	for id, w := range want {
		if got := posOf(t, g, id); got != w {
			t.Errorf("%s at %v, want %v", id, got, w)
		}
	}
}

// TestArchitectureConflictKeepsTheFirstHint checks that the second hint for
// a node is reported and the first stands.
func TestArchitectureConflictKeepsTheFirstHint(t *testing.T) {
	g, warn := parseArch(t, `service a
service b
service c
a:R -- L:b
a:B -- T:c
c:R -- L:b
`)
	if got, want := posOf(t, g, "b"), (graph.GridCoord{Col: 1, Row: 0}); got != want {
		t.Errorf("b at %v, want %v from the first hint", got, want)
	}
	if !strings.Contains(warn, "b is at column 1 row 0") {
		t.Errorf("the second hint for b was not reported: %q", warn)
	}
}

// TestArchitectureCollisionTakesTheNextColumn checks that two nodes resolving
// to one cell are reported and separated.
func TestArchitectureCollisionTakesTheNextColumn(t *testing.T) {
	g, warn := parseArch(t, `service a
service b
service c
a:R -- L:b
a:R -- L:c
`)
	if got, want := posOf(t, g, "b"), (graph.GridCoord{Col: 1, Row: 0}); got != want {
		t.Errorf("b at %v, want %v", got, want)
	}
	if got, want := posOf(t, g, "c"), (graph.GridCoord{Col: 2, Row: 0}); got != want {
		t.Errorf("c at %v, want %v: the next free column", got, want)
	}
	if !strings.Contains(warn, "b and c both resolve to column 1 row 0") {
		t.Errorf("the collision was not reported: %q", warn)
	}
}

// TestArchitectureUnreachedNodesTakeARowBelow checks where a node no hint
// reaches lands.
func TestArchitectureUnreachedNodesTakeARowBelow(t *testing.T) {
	g, _ := parseArch(t, `service a
service b
service loose
service other
a:B -- T:b
`)
	if got, want := posOf(t, g, "loose"), (graph.GridCoord{Col: 0, Row: 2}); got != want {
		t.Errorf("loose at %v, want %v", got, want)
	}
	if got, want := posOf(t, g, "other"), (graph.GridCoord{Col: 1, Row: 2}); got != want {
		t.Errorf("other at %v, want %v", got, want)
	}
}

// TestArchitectureGroupBorderEdge checks that a {group} end attaches to the
// group holding the service, and that the hint still positions the service.
func TestArchitectureGroupBorderEdge(t *testing.T) {
	g, warn := parseArch(t, `group one(cloud)[One]
group two(cloud)[Two]
service server(server)[Server] in one
service subnet(server)[Subnet] in two
server{group}:R --> L:subnet{group}
`)
	if warn != "" {
		t.Errorf("unexpected report: %s", warn)
	}
	if len(g.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(g.Edges))
	}
	e := g.Edges[0]
	if e.Source != "one" || !e.SourceIsSubgraph {
		t.Errorf("source is %q (subgraph %v), want the group one", e.Source, e.SourceIsSubgraph)
	}
	if e.Target != "two" || !e.TargetIsSubgraph {
		t.Errorf("target is %q (subgraph %v), want the group two", e.Target, e.TargetIsSubgraph)
	}
	if got, want := posOf(t, g, "subnet"), (graph.GridCoord{Col: 1, Row: 0}); got != want {
		t.Errorf("subnet at %v, want %v: the hint places the service", got, want)
	}
}

// TestArchitectureDeclarations checks the icon prefix, the group tree, the
// junction shape and the arrowheads.
func TestArchitectureDeclarations(t *testing.T) {
	g, _ := parseArch(t, `group outer(cloud)[Outer]
group inner(database)[Inner] in outer
service db(database)[My Database] in inner
service plain
junction j in inner
db:R <--> L:j
`)
	if got, want := g.Nodes["db"].Label, `[database]\nMy Database`; got != want {
		t.Errorf("db label %q, want %q", got, want)
	}
	if got, want := g.Nodes["plain"].Label, "plain"; got != want {
		t.Errorf("plain label %q, want %q", got, want)
	}
	if g.Nodes["j"].Shape != graph.ShapeJunction {
		t.Errorf("j is shape %v, want a junction", g.Nodes["j"].Shape)
	}
	if len(g.Subgraphs) != 1 || g.Subgraphs[0].ID != "outer" {
		t.Fatalf("expected one top-level group outer, got %v", g.Subgraphs)
	}
	outer := g.Subgraphs[0]
	if got, want := outer.Label, "[cloud] Outer"; got != want {
		t.Errorf("outer label %q, want %q", got, want)
	}
	if len(outer.Children) != 1 || outer.Children[0].ID != "inner" {
		t.Fatalf("expected inner nested in outer, got %v", outer.Children)
	}
	if got := outer.Children[0].NodeIDs; len(got) != 2 || got[0] != "db" || got[1] != "j" {
		t.Errorf("inner holds %v, want db and j", got)
	}
	e := g.Edges[0]
	if !e.HasArrowStart || !e.HasArrowEnd {
		t.Errorf("<--> gave arrows %v/%v, want both", e.HasArrowStart, e.HasArrowEnd)
	}
}

// TestArchitectureEdgeTitleAndPlainLine checks the `-[Label]-` title form and
// that a plain `--` carries no arrowhead.
func TestArchitectureEdgeTitleAndPlainLine(t *testing.T) {
	g, _ := parseArch(t, "service a\nservice b\na:R -[reads]- L:b\n")
	e := g.Edges[0]
	if e.Label != "reads" {
		t.Errorf("label %q, want reads", e.Label)
	}
	if e.HasArrowStart || e.HasArrowEnd {
		t.Errorf("-- gave arrows %v/%v, want none", e.HasArrowStart, e.HasArrowEnd)
	}
}

// TestArchitectureEdgesAreEmittedLeftToRight checks the orientation the
// engine's side chooser needs: a hint that places the far node left of the
// near one emits the edge from the far node, arrowheads with it.
func TestArchitectureEdgesAreEmittedLeftToRight(t *testing.T) {
	g, _ := parseArch(t, "service a\nservice b\na:L --> R:b\n")
	e := g.Edges[0]
	if e.Source != "b" || e.Target != "a" {
		t.Errorf("edge is %s -> %s, want b -> a", e.Source, e.Target)
	}
	if !e.HasArrowStart || e.HasArrowEnd {
		t.Errorf("arrows %v/%v, want the head still at b", e.HasArrowStart, e.HasArrowEnd)
	}
}

// TestArchitectureSkippedStatements checks that the statements this parser
// does not read declare nothing.
func TestArchitectureSkippedStatements(t *testing.T) {
	g := parseArchitecture(`architecture-beta
    title A title
    accTitle: An accessible title
    service a
    service b
    a:R -- L:b
    align row a b
`, io.Discard)
	if len(g.Nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d: %v", len(g.Nodes), g.NodeOrder)
	}
	if len(g.Edges) != 1 {
		t.Errorf("expected 1 edge, got %d", len(g.Edges))
	}
}
