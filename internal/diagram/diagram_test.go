package diagram

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/aaronsb/mmaid-go/internal/graph"
	"github.com/aaronsb/mmaid-go/internal/renderer"
)

func assertCanvasContains(t *testing.T, c *renderer.Canvas, substr string) {
	t.Helper()
	out := c.ToString()
	if !strings.Contains(out, substr) {
		t.Errorf("output missing %q\n---\n%s\n---", substr, out)
	}
}

func assertCanvasNotEmpty(t *testing.T, c *renderer.Canvas) {
	t.Helper()
	out := c.ToString()
	if strings.TrimSpace(out) == "" {
		t.Error("canvas is empty")
	}
}

// ── Sequence ────────────────────────────────────────────────────────────────

func TestSequenceBasic(t *testing.T) {
	c := RenderSequence("sequenceDiagram\n  Alice->>Bob: Hello\n  Bob-->>Alice: Hi", renderer.UNICODE)
	assertCanvasContains(t, c, "Alice")
	assertCanvasContains(t, c, "Bob")
	assertCanvasContains(t, c, "Hello")
}

// ── Class Diagram ───────────────────────────────────────────────────────────

func TestClassDiagramBasic(t *testing.T) {
	c := RenderClassDiagram("classDiagram\n  class Animal {\n    +int age\n    +makeSound()\n  }", renderer.UNICODE)
	assertCanvasContains(t, c, "Animal")
	assertCanvasContains(t, c, "+int age")
}

// ── ER Diagram ──────────────────────────────────────────────────────────────

func TestERDiagramBasic(t *testing.T) {
	c := RenderERDiagram("erDiagram\n  CUSTOMER ||--o{ ORDER : places", renderer.UNICODE)
	assertCanvasContains(t, c, "CUSTOMER")
	assertCanvasContains(t, c, "ORDER")
}

// ── Pie Chart ───────────────────────────────────────────────────────────────

func TestPieChartCircle(t *testing.T) {
	c := RenderPieChart("pie\n  \"A\" : 60\n  \"B\" : 40", renderer.UNICODE, true, nil)
	assertCanvasContains(t, c, "A")
	assertCanvasContains(t, c, "B")
	assertCanvasNotEmpty(t, c)
}

func TestPieChartBraille(t *testing.T) {
	c := RenderPieChart("pie\n  \"X\" : 70\n  \"Y\" : 30", renderer.UNICODE, false, nil)
	assertCanvasContains(t, c, "X")
	assertCanvasContains(t, c, "⣿") // braille solid pattern
}

func TestPieChartASCII(t *testing.T) {
	c := RenderPieChart("pie\n  \"Go\" : 50\n  \"Rust\" : 50", renderer.ASCII, false, nil)
	assertCanvasContains(t, c, "Go")
	assertCanvasContains(t, c, "#") // ASCII fill char
}

func TestPieChartMonochromatic(t *testing.T) {
	theme := renderer.GetTheme("amber")
	c := RenderPieChart("pie\n  \"A\" : 60\n  \"B\" : 40", renderer.UNICODE, true, &theme)
	assertCanvasNotEmpty(t, c)
}

// ── Git Graph ───────────────────────────────────────────────────────────────

func TestGitGraphBasic(t *testing.T) {
	c := RenderGitGraph("gitGraph\n  commit id: \"A\"\n  commit id: \"B\"", renderer.UNICODE)
	assertCanvasContains(t, c, "A")
	assertCanvasContains(t, c, "B")
	assertCanvasContains(t, c, "●")
}

// ── Block Diagram ───────────────────────────────────────────────────────────

func TestBlockDiagramBasic(t *testing.T) {
	c := RenderBlockDiagram("block-beta\n  columns 2\n  A[\"Hello\"] B[\"World\"]", renderer.UNICODE)
	assertCanvasContains(t, c, "Hello")
	assertCanvasContains(t, c, "World")
}

// ── State Diagram ───────────────────────────────────────────────────────────

func TestStateDiagramParse(t *testing.T) {
	g := ParseStateDiagram("stateDiagram-v2\n  [*] --> Idle\n  Idle --> Done\n  Done --> [*]")
	if len(g.Nodes) < 2 {
		t.Errorf("expected at least 2 nodes, got %d", len(g.Nodes))
	}
}

// ── Gantt ───────────────────────────────────────────────────────────────────

func TestGanttBasic(t *testing.T) {
	c := RenderGantt("gantt\n  title Test\n  dateFormat YYYY-MM-DD\n  section S1\n    Task1 :a1, 2024-01-01, 7d", renderer.UNICODE, nil)
	assertCanvasContains(t, c, "Test")
	assertCanvasContains(t, c, "Task1")
}

func TestGanttNoTasks(t *testing.T) {
	c := RenderGantt("gantt\n  title Empty", renderer.UNICODE, nil)
	assertCanvasContains(t, c, "no tasks")
}

// ── Timeline ────────────────────────────────────────────────────────────────

func TestTimelineBasic(t *testing.T) {
	c := RenderTimeline("timeline\n  title History\n  2020 : Event A\n  2021 : Event B", renderer.UNICODE, nil)
	assertCanvasContains(t, c, "History")
	assertCanvasContains(t, c, "Event A")
	assertCanvasContains(t, c, "●")
}

func TestTimelineVerticalLayout(t *testing.T) {
	// Directly test vertical layout path with many events
	td := parseTimeline("timeline\n  title Computing\n  1940 : ENIAC\n  1950 : UNIVAC\n  1960 : Mainframes\n  1970 : Minicomputers : UNIX\n  1980 : PCs\n  1990 : Web\n  2000 : Cloud\n  2010 : Mobile\n  2020 : AI")
	c := renderTimelineVertical(td, renderer.UNICODE, nil)
	assertCanvasNotEmpty(t, c)
	assertCanvasContains(t, c, "Computing")
	assertCanvasContains(t, c, "ENIAC")
	assertCanvasContains(t, c, "UNIX")
	assertCanvasContains(t, c, "AI")
	// Vertical layout should have period labels stacked vertically
	out := c.ToString()
	lines := strings.Split(out, "\n")
	if len(lines) < 20 {
		t.Errorf("vertical layout should be tall, got %d lines", len(lines))
	}
}

func TestTimelineVerticalASCII(t *testing.T) {
	td := parseTimeline("timeline\n  2020 : Alpha\n  2021 : Beta\n  2022 : Release")
	c := renderTimelineVertical(td, renderer.ASCII, nil)
	assertCanvasNotEmpty(t, c)
	assertCanvasContains(t, c, "Alpha")
	assertCanvasContains(t, c, "|")
	assertCanvasContains(t, c, "o")
}

// ── Kanban ──────────────────────────────────────────────────────────────────

func TestKanbanBasic(t *testing.T) {
	c := RenderKanban("kanban\n  col1[Todo]\n    t1[Task A]\n  col2[Done]\n    t2[Task B]", renderer.UNICODE, nil)
	assertCanvasContains(t, c, "Todo")
	assertCanvasContains(t, c, "Task A")
	assertCanvasContains(t, c, "Done")
}

func TestKanbanThemed(t *testing.T) {
	theme := renderer.GetTheme("blueprint")
	c := RenderKanban("kanban\n  col1[A]\n    t1[X]\n  col2[B]\n    t2[Y]", renderer.UNICODE, &theme)
	assertCanvasNotEmpty(t, c)
}

// ── Journey ─────────────────────────────────────────────────────────────────

func TestJourneyBasic(t *testing.T) {
	src := "journey\n    title My day\n    section Work\n        Tea: 5: Me\n        Code: 1: Me, Bot"
	c := RenderJourney(src, renderer.UNICODE, nil)
	assertCanvasContains(t, c, "My day")
	assertCanvasContains(t, c, "Work")
	assertCanvasContains(t, c, "Tea")
	assertCanvasContains(t, c, "Code")
	assertCanvasContains(t, c, "Me")
	assertCanvasContains(t, c, "Bot")
	assertCanvasContains(t, c, ":D")  // score 5 face
	assertCanvasContains(t, c, ":((") // score 1 face
}

func TestJourneyScoreClamped(t *testing.T) {
	// Out-of-range and non-numeric scores fall back/clamp without panicking.
	c := RenderJourney("journey\n    section S\n        A: 99: X\n        B: nope: Y", renderer.UNICODE, nil)
	assertCanvasContains(t, c, ":D")  // 99 clamps to 5
	assertCanvasContains(t, c, ":-|") // "nope" -> default 3
}

func TestJourneyThemed(t *testing.T) {
	theme := renderer.GetTheme("blueprint")
	c := RenderJourney("journey\n    section S\n        A: 3: X", renderer.UNICODE, &theme)
	assertCanvasNotEmpty(t, c)
}

func TestJourneyEmpty(t *testing.T) {
	c := RenderJourney("journey", renderer.UNICODE, nil)
	assertCanvasContains(t, c, "no sections")
}

// ── Orientation ─────────────────────────────────────────────────────────────

func TestJourneyVerticalViaDirective(t *testing.T) {
	src := "journey\n    direction TB\n    title Day\n    section Work\n        Tea: 5: Me\n        Code: 2: Me, Bot"
	c := RenderJourney(src, renderer.UNICODE, nil)
	out := c.ToString()
	// Vertical layout is tall and narrow; horizontal is wide and short.
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) < 8 {
		t.Errorf("expected a tall vertical layout, got %d lines:\n%s", len(lines), out)
	}
	assertCanvasContains(t, c, "Tea")
	assertCanvasContains(t, c, "Code")
}

func TestJourneyHorizontalByDefault(t *testing.T) {
	// No directive, no override -> journey's natural default is horizontal.
	c := RenderJourney("journey\n    section S\n        A: 3: X\n        B: 4: Y", renderer.UNICODE, nil)
	lines := strings.Split(strings.TrimRight(c.ToString(), "\n"), "\n")
	if len(lines) > 9 {
		t.Errorf("expected a short horizontal layout, got %d lines", len(lines))
	}
}

func TestOrientationCLIOverridesDirective(t *testing.T) {
	src := "journey\n    direction TB\n    section S\n        A: 3: X"
	SetOrientationOverride("lr") // CLI says horizontal, directive says vertical
	defer SetOrientationOverride("")
	c := RenderJourney(src, renderer.UNICODE, nil)
	lines := strings.Split(strings.TrimRight(c.ToString(), "\n"), "\n")
	if len(lines) > 9 {
		t.Errorf("CLI --orientation lr should override 'direction TB'; got %d lines", len(lines))
	}
}

func TestResolveVerticalPrecedence(t *testing.T) {
	if resolveVertical("journey\n  section S", false) {
		t.Error("no directive/override should yield the default (false)")
	}
	if !resolveVertical("journey\n  direction TB\n  section S", false) {
		t.Error("'direction TB' directive should yield vertical")
	}
	if resolveVertical("journey\n  direction LR\n  section S", true) {
		t.Error("'direction LR' directive should yield horizontal even if default is vertical")
	}
	SetOrientationOverride("tb")
	defer SetOrientationOverride("")
	if !resolveVertical("journey\n  direction LR\n  section S", false) {
		t.Error("CLI override must win over the directive")
	}
}

func TestOrientationInvalidTokenIsNoop(t *testing.T) {
	if SetOrientationOverride("sideways") {
		t.Error("unrecognized token should report false")
	}
	defer SetOrientationOverride("")
	// Override not applied -> falls back to the supplied default.
	if resolveVertical("journey\n  section S", false) {
		t.Error("invalid override must not force vertical; expected default (false)")
	}
	if !resolveVertical("journey\n  section S", true) {
		t.Error("invalid override must not block the default (true)")
	}
}

func TestDirectionScopedToTopLevel(t *testing.T) {
	// `direction LR` nested in a subgraph governs that subgraph only — it
	// must not flip the whole diagram (Mermaid semantics).
	nested := "flowchart TB\n  subgraph G\n    direction LR\n    A --> B\n  end\n  C --> A"
	if !resolveVertical(nested, true) {
		t.Error("nested 'direction LR' must not override whole-diagram default")
	}
	// A top-level `direction` does govern the whole diagram.
	if resolveVertical("stateDiagram-v2\ndirection LR\n[*] --> A", true) {
		t.Error("top-level 'direction LR' should yield horizontal")
	}
	if !resolveVertical("journey\ndirection TB\nsection S", false) {
		t.Error("top-level 'direction TB' should yield vertical")
	}
}

func TestJourneyLeadingBlankAndComment(t *testing.T) {
	// Leading blank/comment lines before the header must not break parsing.
	for _, src := range []string{
		"\n\njourney\n    section S\n        Tea: 5: Me",
		"%% a note\njourney\n    section S\n        Tea: 5: Me",
	} {
		c := RenderJourney(src, renderer.UNICODE, nil)
		assertCanvasContains(t, c, "Tea")
	}
}

func TestPacketLeadingBlank(t *testing.T) {
	c := RenderPacket("\npacket-beta\n    0-7: \"A\"", renderer.UNICODE, nil)
	assertCanvasContains(t, c, "A")
}

// ── Packet ──────────────────────────────────────────────────────────────────

func TestPacketBasic(t *testing.T) {
	src := "packet-beta\n    0-15: \"Source Port\"\n    16-31: \"Destination Port\""
	c := RenderPacket(src, renderer.UNICODE, nil)
	assertCanvasContains(t, c, "Source Port")
	assertCanvasContains(t, c, "Destination Port")
	assertCanvasContains(t, c, "0")
	assertCanvasContains(t, c, "31")
}

func TestPacketAutoIncrement(t *testing.T) {
	// +N fields chain from the previous field's end bit.
	c := RenderPacket("packet-beta\n    +16: \"A\"\n    +16: \"B\"", renderer.UNICODE, nil)
	assertCanvasContains(t, c, "A")
	assertCanvasContains(t, c, "B")
	assertCanvasContains(t, c, "31") // 0..15 then 16..31
}

func TestPacketTruncationLegend(t *testing.T) {
	// A label too wide for a 1-bit field is truncated and listed in a legend.
	c := RenderPacket("packet-beta\n    0: \"VeryLongFieldName\"", renderer.UNICODE, nil)
	assertCanvasContains(t, c, "VeryLongFieldName [0]")
}

func TestPacketEmpty(t *testing.T) {
	c := RenderPacket("packet-beta", renderer.UNICODE, nil)
	assertCanvasContains(t, c, "no fields")
}

// ── Mindmap ─────────────────────────────────────────────────────────────────

func TestMindmapBasic(t *testing.T) {
	c := RenderMindmap("mindmap\n  root((Root))\n    Child1\n    Child2", renderer.UNICODE)
	assertCanvasContains(t, c, "Root")
	assertCanvasContains(t, c, "Child1")
	assertCanvasContains(t, c, "Child2")
}

// ── Quadrant ────────────────────────────────────────────────────────────────

func TestQuadrantBasic(t *testing.T) {
	c := RenderQuadrantChart("quadrantChart\n  title Test\n  x-axis A --> B\n  y-axis C --> D\n  Point1: [0.5, 0.5]", renderer.UNICODE, nil)
	assertCanvasContains(t, c, "Test")
	assertCanvasContains(t, c, "Point1")
	assertCanvasContains(t, c, "●")
}

// ── XY Chart ────────────────────────────────────────────────────────────────

func TestXYChartBasic(t *testing.T) {
	c := RenderXYChart("xychart-beta\n  title Rev\n  x-axis [a, b]\n  bar [10, 20]", renderer.UNICODE, nil)
	assertCanvasContains(t, c, "Rev")
	assertCanvasContains(t, c, "▓") // bar fill char
}

// ── Treemap ─────────────────────────────────────────────────────────────────

func TestTreemapBasic(t *testing.T) {
	c := RenderTreemap("treemap-beta\n  \"Section\"\n    \"Item\": 100", renderer.UNICODE, nil)
	assertCanvasContains(t, c, "Section")
	assertCanvasContains(t, c, "Item")
	assertCanvasContains(t, c, "100")
}

// ── Requirement Diagram ─────────────────────────────────────────────────────

const reqSrc = `requirementDiagram
    requirement checkout_req {
    id: 1
    text: Orders must be payable online.
    risk: high
    verifymethod: test
    }

    functionalRequirement payment_req {
    id: 1.1
    text: Card payments must be authorised.
    risk: high
    verifymethod: test
    }

    element checkout_service {
    type: service
    docref: docs/checkout.md
    }

    checkout_req - contains -> payment_req
    checkout_service - satisfies -> payment_req`

func TestRequirementNodesAndShapes(t *testing.T) {
	g := ParseRequirementDiagram(reqSrc)
	if len(g.Nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(g.Nodes))
	}
	if got := g.Nodes["checkout_req"].Shape; got != graph.ShapeRectangle {
		t.Errorf("requirement shape = %v, want rectangle", got)
	}
	if got := g.Nodes["checkout_service"].Shape; got != graph.ShapeRounded {
		t.Errorf("element shape = %v, want rounded", got)
	}
	if g.Direction != graph.DirTB {
		t.Errorf("direction = %v, want TB", g.Direction)
	}
}

func TestRequirementLabelLines(t *testing.T) {
	g := ParseRequirementDiagram(reqSrc)
	label := g.Nodes["checkout_req"].Label
	for _, want := range []string{"checkout_req", "id: 1", "Orders must be payable online.", "risk: high | verify: test"} {
		if !strings.Contains(label, want) {
			t.Errorf("requirement label missing %q\n%s", want, label)
		}
	}
	elem := g.Nodes["checkout_service"].Label
	for _, want := range []string{"[service]", "checkout_service", "docs/checkout.md"} {
		if !strings.Contains(elem, want) {
			t.Errorf("element label missing %q\n%s", want, elem)
		}
	}
}

func TestRequirementEdgeLabels(t *testing.T) {
	g := ParseRequirementDiagram(reqSrc)
	if len(g.Edges) != 2 {
		t.Fatalf("expected 2 edges, got %d", len(g.Edges))
	}
	if g.Edges[0].Source != "checkout_req" || g.Edges[0].Target != "payment_req" || g.Edges[0].Label != "contains" {
		t.Errorf("edge 0 = %+v", g.Edges[0])
	}
	if g.Edges[1].Label != "satisfies" {
		t.Errorf("edge 1 label = %q, want satisfies", g.Edges[1].Label)
	}
}

func TestRequirementReverseRelationAndDirection(t *testing.T) {
	g := ParseRequirementDiagram("requirementDiagram\ndirection LR\nrequirement a {\nid: 1\n}\nelement b {\ntype: sim\n}\na <- copies - b")
	if g.Direction != graph.DirLR || !g.DirectionExplicit {
		t.Errorf("direction = %v (explicit %v), want LR", g.Direction, g.DirectionExplicit)
	}
	if len(g.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(g.Edges))
	}
	if g.Edges[0].Source != "b" || g.Edges[0].Target != "a" || g.Edges[0].Label != "copies" {
		t.Errorf("reverse edge = %+v, want b -> a copies", g.Edges[0])
	}
}

// ── C4 ──────────────────────────────────────────────────────────────────────

const c4Src = `C4Context
    title Internet Banking
    Enterprise_Boundary(b0, "Bank") {
        Person(customer, "Banking Customer", "A personal account holder.")
        Person(staff, "Support Staff", "Answers customer queries.")
        System(banking, "Internet Banking", "Accounts and payments.")
    }
    System_Ext(email, "E-mail System", "Microsoft Exchange.")

    Rel(customer, banking, "Uses")
    Rel(staff, banking, "Administers")
    Rel(banking, email, "Sends mail", "SMTP")`

func TestC4NodesAndBoundary(t *testing.T) {
	g := ParseC4Diagram(c4Src)
	if len(g.Nodes) != 4 {
		t.Fatalf("expected 4 nodes, got %d", len(g.Nodes))
	}
	if len(g.Subgraphs) != 1 || g.Subgraphs[0].Label != "Bank" {
		t.Fatalf("expected one boundary labelled Bank, got %+v", g.Subgraphs)
	}
	if got := len(g.Subgraphs[0].NodeIDs); got != 3 {
		t.Errorf("boundary holds %d nodes, want 3", got)
	}
	if !strings.Contains(g.Nodes["customer"].Label, "[person]") {
		t.Errorf("person label missing marker: %q", g.Nodes["customer"].Label)
	}
	if !strings.Contains(g.Nodes["email"].Label, "E-mail System (ext)") {
		t.Errorf("external label missing (ext): %q", g.Nodes["email"].Label)
	}
}

func TestC4RelationshipLabels(t *testing.T) {
	g := ParseC4Diagram(c4Src)
	if len(g.Edges) != 3 {
		t.Fatalf("expected 3 edges, got %d", len(g.Edges))
	}
	if g.Edges[0].Label != "Uses" {
		t.Errorf("edge 0 label = %q, want Uses", g.Edges[0].Label)
	}
	if g.Edges[2].Label != "Sends mail [SMTP]" {
		t.Errorf("edge 2 label = %q, want the technology appended", g.Edges[2].Label)
	}
}

func TestC4ShapesAndTechnology(t *testing.T) {
	src := `C4Container
    Container(api, "API", "Go", "Serves requests.")
    ContainerDb(db, "Database", "Postgres")
    ContainerQueue(bus, "Events", "NATS")
    BiRel(api, db, "Reads")
    Rel_Back(bus, api, "Notifies")`
	g := ParseC4Diagram(src)
	if got := g.Nodes["db"].Shape; got != graph.ShapeCylinder {
		t.Errorf("Db shape = %v, want cylinder", got)
	}
	if got := g.Nodes["bus"].Shape; got != graph.ShapeStadium {
		t.Errorf("Queue shape = %v, want stadium", got)
	}
	label := g.Nodes["api"].Label
	for _, want := range []string{"API", "(Go)", "Serves requests."} {
		if !strings.Contains(label, want) {
			t.Errorf("container label missing %q\n%s", want, label)
		}
	}
	if !g.Edges[0].IsBidirectional() {
		t.Error("BiRel should carry arrows on both ends")
	}
	if g.Edges[1].Source != "api" || g.Edges[1].Target != "bus" {
		t.Errorf("Rel_Back = %s -> %s, want api -> bus", g.Edges[1].Source, g.Edges[1].Target)
	}
}

func TestC4DeploymentNodesAreSubgraphs(t *testing.T) {
	src := `C4Deployment
    Deployment_Node(plc, "Big Bank plc", "Data centre") {
        Deployment_Node(dn, "api host", "Ubuntu") {
            Container(api, "API", "Go")
        }
    }`
	g := ParseC4Diagram(src)
	if len(g.Subgraphs) != 1 || g.Subgraphs[0].Label != "Big Bank plc (Data centre)" {
		t.Fatalf("outer node = %+v", g.Subgraphs)
	}
	inner := g.Subgraphs[0].Children
	if len(inner) != 1 || !slices.Contains(inner[0].NodeIDs, "api") {
		t.Fatalf("inner node = %+v", inner)
	}
}

func TestC4StyleMacrosIgnored(t *testing.T) {
	src := `C4Context
    Person(a, "A")
    System(b, "B")
    Rel(a, b, "Uses")
    UpdateElementStyle(a, $fontColor="red")
    UpdateRelStyle(a, b, $textColor="blue")
    UpdateLayoutConfig($c4ShapeInRow="3")`
	g := ParseC4Diagram(src)
	if len(g.Nodes) != 2 || len(g.Edges) != 1 {
		t.Errorf("styling macros leaked: %d nodes, %d edges", len(g.Nodes), len(g.Edges))
	}
}

// ── Use Case ────────────────────────────────────────────────────────────────

const useCaseSrc = `usecase-beta
    actor Customer("Customer")
    actor Agent("Support Agent")
    systemBoundary Storefront
        Browse("Browse catalogue")
        Checkout("Place order")
        Pay("Take payment")
    end
    Customer --> Browse
    Customer --> Checkout
    Agent --> Checkout
    Checkout ..> : include Pay`

func TestUseCaseNodesAndShapes(t *testing.T) {
	g := ParseUseCaseDiagram(useCaseSrc)
	if len(g.Nodes) != 5 {
		t.Fatalf("expected 5 nodes, got %d", len(g.Nodes))
	}
	if got := g.Nodes["Customer"].Shape; got != graph.ShapeRectangle {
		t.Errorf("actor shape = %v, want rectangle", got)
	}
	if !strings.Contains(g.Nodes["Customer"].Label, "[actor]") {
		t.Errorf("actor label missing marker: %q", g.Nodes["Customer"].Label)
	}
	if got := g.Nodes["Browse"].Shape; got != graph.ShapeStadium {
		t.Errorf("use case shape = %v, want stadium", got)
	}
	if g.Nodes["Browse"].Label != "Browse catalogue" {
		t.Errorf("use case label = %q", g.Nodes["Browse"].Label)
	}
}

func TestUseCaseBoundary(t *testing.T) {
	g := ParseUseCaseDiagram(useCaseSrc)
	if len(g.Subgraphs) != 1 || g.Subgraphs[0].Label != "Storefront" {
		t.Fatalf("expected one Storefront boundary, got %+v", g.Subgraphs)
	}
	want := []string{"Browse", "Checkout", "Pay"}
	if !slices.Equal(g.Subgraphs[0].NodeIDs, want) {
		t.Errorf("boundary members = %v, want %v", g.Subgraphs[0].NodeIDs, want)
	}
}

func TestUseCaseIncludeIsDashed(t *testing.T) {
	g := ParseUseCaseDiagram(useCaseSrc)
	last := g.Edges[len(g.Edges)-1]
	if last.Source != "Checkout" || last.Target != "Pay" {
		t.Fatalf("include edge = %s -> %s", last.Source, last.Target)
	}
	if last.Style != graph.EdgeDotted || last.Label != "<<include>>" {
		t.Errorf("include edge = %+v, want a dotted <<include>>", last)
	}
}

func TestUseCaseRelationshipForms(t *testing.T) {
	src := `usecase-beta
direction LR
actor Admin
actor Person
usecase (Do thing) as Thing
Report[Generate report]
Admin --|> Person
Admin -- "runs" --> Thing
Thing ..> Report : <<extend>>
Person --o Thing
Person --x Report`
	g := ParseUseCaseDiagram(src)
	if g.Direction != graph.DirLR {
		t.Errorf("direction = %v, want LR", g.Direction)
	}
	if g.Nodes["Thing"].Label != "Do thing" {
		t.Errorf("inline use case label = %q, want Do thing", g.Nodes["Thing"].Label)
	}
	if g.Nodes["Report"].Label != "Generate report" {
		t.Errorf("bracket use case label = %q", g.Nodes["Report"].Label)
	}

	byPair := map[string]graph.Edge{}
	for _, e := range g.Edges {
		byPair[e.Source+"->"+e.Target] = e
	}
	if e, ok := byPair["Admin->Person"]; !ok || !e.HasArrowEnd {
		t.Errorf("generalization edge = %+v (found %v)", e, ok)
	}
	if e := byPair["Admin->Thing"]; e.Label != "runs" {
		t.Errorf("labelled association = %q, want runs", e.Label)
	}
	if e := byPair["Thing->Report"]; e.Style != graph.EdgeDotted || e.Label != "<<extend>>" {
		t.Errorf("extend edge = %+v", e)
	}
	if e := byPair["Person->Thing"]; e.ArrowTypeEnd != graph.ArrowTypeCircle {
		t.Errorf("--o arrow = %v, want circle", e.ArrowTypeEnd)
	}
	if e := byPair["Person->Report"]; e.ArrowTypeEnd != graph.ArrowTypeCross {
		t.Errorf("--x arrow = %v, want cross", e.ArrowTypeEnd)
	}
}

func TestUseCaseInlineAndReversed(t *testing.T) {
	g := ParseUseCaseDiagram("usecase-beta\nactor \"Main administrator\" as Admin\nAdmin --> (Reset password)\nAdmin -- (Reset password)\n(Reset password) <-- Admin")
	if g.Nodes["Admin"].Label != joinLabel([]string{"[actor]", "Main administrator"}) {
		t.Errorf("aliased actor label = %q", g.Nodes["Admin"].Label)
	}
	if _, ok := g.Nodes["Reset_password"]; !ok {
		t.Fatalf("inline use case not created: %v", g.NodeOrder)
	}
	if len(g.Edges) != 3 {
		t.Fatalf("expected 3 edges, got %d", len(g.Edges))
	}
	if g.Edges[1].HasArrowEnd {
		t.Error("a bare -- association should carry no arrowhead")
	}
	if g.Edges[2].Source != "Admin" || g.Edges[2].Target != "Reset_password" || !g.Edges[2].HasArrowEnd {
		t.Errorf("reversed association = %+v", g.Edges[2])
	}
}

// ── Review regressions ──────────────────────────────────────────────────────

// An explicit edge ID belongs to the operator, not to the source endpoint.
func TestUseCaseEdgeIDsAreNotNodes(t *testing.T) {
	g := ParseUseCaseDiagram(`usecase-beta
actor Customer
Checkout
Payment
Customer opens@-- "starts checkout" ---> Checkout
Checkout payment@..> : include Payment`)

	for _, id := range g.NodeOrder {
		if strings.Contains(id, "@") || strings.Contains(id, "opens") || strings.Contains(id, "payment") {
			t.Errorf("edge ID leaked into node %q (nodes: %v)", id, g.NodeOrder)
		}
	}
	if len(g.Nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d: %v", len(g.Nodes), g.NodeOrder)
	}
	if e := g.Edges[0]; e.Source != "Customer" || e.Target != "Checkout" || e.Label != "starts checkout" {
		t.Errorf("labelled association = %+v", e)
	}
	if e := g.Edges[1]; e.Source != "Checkout" || e.Target != "Payment" || e.Label != "<<include>>" {
		t.Errorf("include edge = %+v", e)
	}
}

// A keyword is only a keyword when a separator follows it and its case matches.
func TestUseCaseKeywordNamedNodes(t *testing.T) {
	g := ParseUseCaseDiagram(`usecase-beta
actor User
System("The System")
Title("Set title")
Note("Take note")
User --> System
User --> Title
User --> Note`)

	if len(g.Subgraphs) != 0 {
		t.Errorf("a node named System opened a boundary: %+v", g.Subgraphs)
	}
	for _, id := range []string{"System", "Title", "Note"} {
		if _, ok := g.Nodes[id]; !ok {
			t.Errorf("node %q was swallowed by a keyword regex (nodes: %v)", id, g.NodeOrder)
		}
	}
	if g.Nodes["System"] != nil && g.Nodes["System"].Label != "The System" {
		t.Errorf("System label = %q", g.Nodes["System"].Label)
	}
	if len(g.Edges) != 3 {
		t.Errorf("expected 3 edges, got %d", len(g.Edges))
	}
}

// Multi-line constructs are consumed whole rather than one node per line.
func TestUseCaseMultiLineConstructs(t *testing.T) {
	g := ParseUseCaseDiagram(`usecase-beta
accDescr {
    A customer signs in.
    Then they pay.
}
json Payload@{
  "status": "pending",
  "count": 3
}:::data
Reset("` + "`Reset`" + `
` + "`password`" + `")
actor User
User --> Reset`)

	if len(g.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d: %v", len(g.Nodes), g.NodeOrder)
	}
	if _, ok := g.Nodes["Reset"]; !ok {
		t.Fatalf("multi-line label did not resolve to one node: %v", g.NodeOrder)
	}
	if !strings.Contains(g.Nodes["Reset"].Label, labelSep) {
		t.Errorf("multi-line label lost its break: %q", g.Nodes["Reset"].Label)
	}
}

// A line that matches no statement shape is dropped, not promoted to a node.
func TestUseCaseUnknownLineIsDropped(t *testing.T) {
	g := ParseUseCaseDiagram("usecase-beta\nactor User\nthis is not a statement, at all!\n")
	if len(g.Nodes) != 1 {
		t.Errorf("unknown line fabricated a node: %v", g.NodeOrder)
	}
}

// Metadata-only statements attach to something that already exists; they never
// declare one.
func TestUseCaseMetadataOnlyStatements(t *testing.T) {
	g := ParseUseCaseDiagram(`usecase-beta
systemBoundary "Payment service"
  actor Clerk("Payment clerk")
  Authorize("Authorize payment")
end
Payment_service@{ type: package }
Clerk starts@--> Authorize
starts@{ animation: fast }`)

	if _, ok := g.Nodes["Payment_service"]; ok {
		t.Errorf("boundary metadata created a node: %v", g.NodeOrder)
	}
	if _, ok := g.Nodes["starts"]; ok {
		t.Errorf("edge metadata created a node: %v", g.NodeOrder)
	}
	if len(g.Nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d: %v", len(g.Nodes), g.NodeOrder)
	}
}

// A colon inside a quoted label is label text, not a relationship separator.
func TestUseCaseColonInsideLabel(t *testing.T) {
	g := ParseUseCaseDiagram("usecase-beta\nactor Customer\nCustomer --> Time(\"Set time: 10:00\")")
	if _, ok := g.Nodes["Time"]; !ok {
		t.Fatalf("target lost its identifier: %v", g.NodeOrder)
	}
	if got := g.Nodes["Time"].Label; got != "Set time: 10:00" {
		t.Errorf("label = %q, want the colon kept", got)
	}
	if len(g.Edges) != 1 || g.Edges[0].Label != "" {
		t.Errorf("edges = %+v, want one unlabelled association", g.Edges)
	}
}

// A C4 boundary whose brace is on the next line still opens, and its `}` closes
// it rather than the parent.
func TestC4BoundaryBraceOnNextLine(t *testing.T) {
	g := ParseC4Diagram(`C4Context
Enterprise_Boundary(b0, "Bank") {
    Boundary(b1, "Inner")
    {
      System(s1, "S1")
    }
    System(s2, "S2")
}`)

	if len(g.Subgraphs) != 1 || g.Subgraphs[0].Label != "Bank" {
		t.Fatalf("top-level boundaries = %+v", g.Subgraphs)
	}
	bank := g.Subgraphs[0]
	if len(bank.Children) != 1 || bank.Children[0].Label != "Inner" {
		t.Fatalf("Inner boundary missing: %+v", bank.Children)
	}
	if !slices.Contains(bank.Children[0].NodeIDs, "s1") {
		t.Errorf("s1 = %v, want inside Inner", bank.Children[0].NodeIDs)
	}
	if !slices.Contains(bank.NodeIDs, "s2") {
		t.Errorf("s2 = %v, want inside Bank", bank.NodeIDs)
	}
}

// An accDescr block's closing brace must not pop an open boundary.
func TestC4AccDescrBlockSkipped(t *testing.T) {
	g := ParseC4Diagram(`C4Context
accDescr {
    A bank and its systems.
}
Enterprise_Boundary(b0, "Bank") {
    System(s1, "S1")
    accDescr {
        Nested prose.
    }
    System(s2, "S2")
}`)

	if len(g.Subgraphs) != 1 {
		t.Fatalf("boundaries = %+v", g.Subgraphs)
	}
	if got := g.Subgraphs[0].NodeIDs; !slices.Equal(got, []string{"s1", "s2"}) {
		t.Errorf("Bank holds %v, want both systems", got)
	}
	if len(g.Nodes) != 2 {
		t.Errorf("accDescr prose became nodes: %v", g.NodeOrder)
	}
}

// The requirement lexer skips whitespace, so the arrow's spacing is free.
func TestRequirementRelationWhitespace(t *testing.T) {
	g := ParseRequirementDiagram(`requirementDiagram
requirement a {
id: 1
}
element b {
type: sim
}
element c {
type: sim
}
a - satisfies ->b
b- derives -> c
c   -   verifies   ->   a`)

	if len(g.Edges) != 3 {
		t.Fatalf("expected 3 edges, got %d: %+v", len(g.Edges), g.Edges)
	}
	want := []struct{ source, target, label string }{
		{"a", "b", "satisfies"},
		{"b", "c", "derives"},
		{"c", "a", "verifies"},
	}
	for i, w := range want {
		e := g.Edges[i]
		if e.Source != w.source || e.Target != w.target || e.Label != w.label {
			t.Errorf("edge %d = %s -> %s %q, want %s -> %s %q", i, e.Source, e.Target, e.Label, w.source, w.target, w.label)
		}
	}
}

// ── TreeView ────────────────────────────────────────────────────────────────

// Indentation is the whole hierarchy, and a trailing slash is what makes a
// node a directory.
func TestTreeViewIndentationAndFolders(t *testing.T) {
	td := parseTreeView(`treeView-beta
    my-project/
        src/
            index.js
        README.md`)

	if len(td.roots) != 1 {
		t.Fatalf("roots = %d, want the one outermost node", len(td.roots))
	}
	root := td.roots[0]
	if root.label != "my-project/" || !root.folder {
		t.Errorf("root = %q folder=%v", root.label, root.folder)
	}
	if len(root.children) != 2 {
		t.Fatalf("children = %d, want src/ and README.md", len(root.children))
	}
	src := root.children[0]
	if src.label != "src/" || !src.folder || len(src.children) != 1 {
		t.Errorf("src = %+v", src)
	}
	if got := src.children[0]; got.label != "index.js" || got.folder {
		t.Errorf("index.js = %q folder=%v", got.label, got.folder)
	}
	if leaf := root.children[1]; leaf.label != "README.md" || leaf.folder {
		t.Errorf("README.md = %q folder=%v", leaf.label, leaf.folder)
	}
}

// A quoted label keeps its spaces; `## text` becomes the description, and the
// annotations this renderer does not draw are parsed off rather than kept.
func TestTreeViewLabelsAndAnnotations(t *testing.T) {
	td := parseTreeView(`treeView-beta
    "my project"
        notes.md ## the running log
        theme.css :::highlight
        main.rs icon(rust)`)

	root := td.roots[0]
	if root.label != "my project" {
		t.Errorf("quoted label = %q", root.label)
	}
	want := []struct{ label, desc string }{
		{"notes.md", "the running log"},
		{"theme.css", ""},
		{"main.rs", ""},
	}
	if len(root.children) != len(want) {
		t.Fatalf("children = %d, want %d", len(root.children), len(want))
	}
	for i, w := range want {
		got := root.children[i]
		if got.label != w.label || got.desc != w.desc {
			t.Errorf("child %d = %q / %q, want %q / %q", i, got.label, got.desc, w.label, w.desc)
		}
	}
}

// Every node at the outermost indent is a root, so a treeView may be a forest.
func TestTreeViewForest(t *testing.T) {
	td := parseTreeView("treeView-beta\napps/\n    web/\nlibs/")
	if len(td.roots) != 2 {
		t.Fatalf("roots = %d, want apps/ and libs/", len(td.roots))
	}
	if td.roots[0].label != "apps/" || td.roots[1].label != "libs/" {
		t.Errorf("roots = %q, %q", td.roots[0].label, td.roots[1].label)
	}
}

func TestTreeViewRenderGuides(t *testing.T) {
	c := RenderTreeView("treeView-beta\nroot/\n    a.txt\n    b.txt", renderer.UNICODE)
	assertCanvasContains(t, c, "├──a.txt")
	assertCanvasContains(t, c, "└──b.txt")
}

// ── Event Modeling ──────────────────────────────────────────────────────────

// Entity-type aliases collapse onto one kind and `->>` records the frames a
// frame is fed from.
func TestEventModelingFramesAndSources(t *testing.T) {
	ed := parseEventModeling(`eventmodeling
    tf 01 ui CartUI
    tf 02 command AddItem
    tf 03 readmodel CartView
    tf 04 event ItemChanged ->> 02 ->> 03
    rf 05 processor Rebuild`)

	wantKind := []string{"ui", "cmd", "rmo", "evt", "pcr"}
	if len(ed.frames) != len(wantKind) {
		t.Fatalf("frames = %d, want %d", len(ed.frames), len(wantKind))
	}
	for i, k := range wantKind {
		if ed.frames[i].kind != k {
			t.Errorf("frame %s kind = %q, want %q", ed.frames[i].id, ed.frames[i].kind, k)
		}
	}
	if got := ed.frames[3].sources; len(got) != 2 || got[0] != "02" || got[1] != "03" {
		t.Errorf("sources = %v, want [02 03]", got)
	}
	if !ed.frames[4].reset {
		t.Error("rf 05 is not marked a reset frame")
	}
}

// A namespace owns one lane wherever it first appears, so a command and the
// event it produces share it.
func TestEventModelingNamespaceSharesOneLane(t *testing.T) {
	ed := parseEventModeling(`eventmodeling
    tf 01 ui CartUI
    tf 02 cmd Inventory.AddItem
    tf 03 evt Inventory.ItemAdded
    tf 04 evt Shipped`)

	lanes, laneOf := emAssignLanes(ed.frames)
	if laneOf["02"] != laneOf["03"] {
		t.Errorf("Inventory frames on lanes %d and %d, want one lane", laneOf["02"], laneOf["03"])
	}
	if laneOf["03"] == laneOf["04"] {
		t.Error("the namespaced lane and the plain event stream are the same lane")
	}
	if lanes[laneOf["02"]].label != "C/RM: Inventory" {
		t.Errorf("namespace lane label = %q", lanes[laneOf["02"]].label)
	}
	if lanes[0].label != "UI/Automation" {
		t.Errorf("first lane = %q, want the automation band on top", lanes[0].label)
	}
}

// A data block's body is not a frame.
func TestEventModelingDataBlockIsNotAFrame(t *testing.T) {
	ed := parseEventModeling(`eventmodeling
    tf 01 cmd AddItem
    tf 02 evt ItemAdded [[ItemAddedData]]

data ItemAddedData
{
  productId: 7
}`)
	if len(ed.frames) != 2 {
		t.Fatalf("frames = %d, want the two tf lines", len(ed.frames))
	}
	if ed.frames[1].name != "ItemAdded" {
		t.Errorf("name = %q, want the payload reference dropped", ed.frames[1].name)
	}
}

func TestEventModelingRendersLanesAndFrames(t *testing.T) {
	c := RenderEventModeling("eventmodeling\n  tf 01 ui CartUI\n  tf 02 cmd AddItem ->> 01", renderer.UNICODE, nil)
	assertCanvasContains(t, c, "UI/Automation")
	assertCanvasContains(t, c, "Command/Read Model")
	assertCanvasContains(t, c, "CartUI")
	assertCanvasContains(t, c, "AddItem")
}

// ── Ishikawa ────────────────────────────────────────────────────────────────

// The first line is the effect and the rest nest by indentation.
func TestIshikawaHierarchy(t *testing.T) {
	root := parseIshikawa(`ishikawa-beta
    Blurry Photo
        Process
            Out of focus
        User
            Shaky hands`)

	if root == nil || root.text != "Blurry Photo" {
		t.Fatalf("effect = %+v", root)
	}
	if len(root.children) != 2 {
		t.Fatalf("categories = %d, want Process and User", len(root.children))
	}
	if got := root.children[0]; got.text != "Process" || len(got.children) != 1 || got.children[0].text != "Out of focus" {
		t.Errorf("Process = %+v", got)
	}
	if got := root.children[1]; got.text != "User" || got.children[0].text != "Shaky hands" {
		t.Errorf("User = %+v", got)
	}
}

// The first cause sets the base level, so an effect indented more than its
// causes still parses.
func TestIshikawaEffectIndentedMoreThanCauses(t *testing.T) {
	root := parseIshikawa("ishikawa-beta\n    Problem\nCause A\n  Subcause A1\nCause B")
	if root.text != "Problem" {
		t.Fatalf("effect = %q", root.text)
	}
	if len(root.children) != 2 {
		t.Fatalf("categories = %d, want Cause A and Cause B", len(root.children))
	}
	if got := root.children[0]; len(got.children) != 1 || got.children[0].text != "Subcause A1" {
		t.Errorf("Cause A children = %+v", got.children)
	}
}

// A cause below the first level keeps its place in the list, one marker per
// level under the category.
func TestIshikawaFlattensDeepCauses(t *testing.T) {
	root := parseIshikawa("ishikawa\nEffect\n  Process\n    Slow\n      Queued\n    Manual")
	var causes []string
	ishikawaCauses(root.children[0], 0, "- ", &causes)
	want := []string{"Slow", "- Queued", "Manual"}
	if !slices.Equal(causes, want) {
		t.Errorf("causes = %v, want %v", causes, want)
	}
}

func TestIshikawaRendersSpineAndBones(t *testing.T) {
	c := RenderIshikawa("ishikawa-beta\nLate Delivery\n  Process\n    Slow handoffs\n  People", renderer.UNICODE, nil)
	assertCanvasContains(t, c, "Late Delivery")
	assertCanvasContains(t, c, "╲")
	assertCanvasContains(t, c, "╱")
	assertCanvasContains(t, c, "━")
	assertCanvasContains(t, c, "Slow handoffs")
}

// The title and accessibility terminals are case-sensitive, the two
// accessibility ones need their separator, and all three sit before the nodes
// in the entry rule — so these are three ordinary children.
func TestTreeViewKeywordNamedNodes(t *testing.T) {
	td := parseTreeView(`treeView-beta
    docs/
        accTitle.md
        Title
        notes.txt`)

	if len(td.roots) != 1 {
		t.Fatalf("roots = %d", len(td.roots))
	}
	kids := td.roots[0].children
	want := []string{"accTitle.md", "Title", "notes.txt"}
	if len(kids) != len(want) {
		t.Fatalf("children = %d, want %d", len(kids), len(want))
	}
	for i, w := range want {
		if kids[i].label != w {
			t.Errorf("child %d = %q, want %q", i, kids[i].label, w)
		}
	}
	if td.title != "" {
		t.Errorf("title = %q, want none taken from a node", td.title)
	}
}

// A real title is still read, and so is a bare `title`.
func TestTreeViewTitle(t *testing.T) {
	td := parseTreeView("treeView-beta\ntitle Project layout\nsrc/")
	if td.title != "Project layout" {
		t.Errorf("title = %q", td.title)
	}
	if len(td.roots) != 1 || td.roots[0].label != "src/" {
		t.Errorf("roots = %+v", td.roots)
	}
}

// BARE_NAME runs to end of line, so only a line that starts with %% is a
// comment.
func TestTreeViewCommentIsWholeLineOnly(t *testing.T) {
	td := parseTreeView("treeView-beta\nnotes/\n    100%%done.txt\n%% a comment\n    todo.txt")
	kids := td.roots[0].children
	if len(kids) != 2 {
		t.Fatalf("children = %d, want the two files", len(kids))
	}
	if kids[0].label != "100%%done.txt" {
		t.Errorf("label = %q, want the %%%% kept", kids[0].label)
	}
}

// The value converters count indentation one column per character, so a tab
// is one column and nests under a two-space line.
func TestTreeViewTabIsOneColumn(t *testing.T) {
	td := parseTreeView("treeView-beta\nroot/\n\tchild/\n  grand.txt")
	child := td.roots[0].children
	if len(child) != 1 || child[0].label != "child/" {
		t.Fatalf("children of root = %+v", child)
	}
	if len(child[0].children) != 1 || child[0].children[0].label != "grand.txt" {
		t.Errorf("children of child = %+v, want grand.txt nested", child[0].children)
	}
}

// A frame with no `->>` takes the nearest earlier frame in a different lane,
// which is what decidePositionRelation falls back to.
func TestEventModelingImplicitRelations(t *testing.T) {
	p := emLayout(parseEventModeling(`eventmodeling
    tf 01 ui CartUI
    tf 02 cmd AddItem
    tf 03 evt ItemAdded`))

	if len(p.boxes) != 3 {
		t.Fatalf("boxes = %d", len(p.boxes))
	}
	if len(p.boxes[0].from) != 0 {
		t.Errorf("the first frame has sources %v", p.boxes[0].from)
	}
	for i, want := range map[int]int{1: 0, 2: 1} {
		got := p.boxes[i].from
		if len(got) != 1 || got[0] != want {
			t.Errorf("box %d from = %v, want [%d]", i, got, want)
		}
	}
}

// A reset frame never receives a relation, explicit or not.
func TestEventModelingResetFrameTakesNoRelation(t *testing.T) {
	p := emLayout(parseEventModeling(`eventmodeling
    tf 01 ui CartUI
    rf 02 pcr Rebuild ->> 01`))

	if got := p.boxes[1].from; len(got) != 0 {
		t.Errorf("reset frame from = %v, want none", got)
	}
}

// extractNamespace splits on "." and takes the first part only when there are
// exactly two, so a three-part name is a plain label in its band's own lane.
func TestEventModelingThreePartNameIsNotNamespaced(t *testing.T) {
	ed := parseEventModeling("eventmodeling\n    tf 01 ui CartUI\n    tf 02 cmd Shop.Inventory.AddItem ->> 01")
	f := ed.frames[1]
	if f.ns != "" || f.name != "Shop.Inventory.AddItem" {
		t.Errorf("frame = ns %q name %q, want no namespace and the whole label", f.ns, f.name)
	}
	lanes, laneOf := emAssignLanes(ed.frames)
	if got := lanes[laneOf["02"]].label; got != "Command/Read Model" {
		t.Errorf("lane = %q, want the band's own lane", got)
	}
}

// The entity type is a closed set: a typo is a parse error upstream, and the
// line is dropped here.
func TestEventModelingUnknownEntityTypeIsDropped(t *testing.T) {
	ed := parseEventModeling("eventmodeling\n    tf 01 ui CartUI\n    tf 02 cdm AddItem\n    tf 03 evt ItemAdded")
	if len(ed.frames) != 2 {
		t.Fatalf("frames = %d, want the two well-formed ones", len(ed.frames))
	}
	if ed.frames[1].id != "03" {
		t.Errorf("second frame = %q, want 03", ed.frames[1].id)
	}
}

// The start rule allows `ISHIKAWA document` with no newline between them, so
// the rest of the header line is the effect.
func TestIshikawaEffectOnTheHeaderLine(t *testing.T) {
	root := parseIshikawa("ishikawa-beta Late Delivery\n    Process\n        Slow handoffs")
	if root.text != "Late Delivery" {
		t.Fatalf("effect = %q", root.text)
	}
	if len(root.children) != 1 || root.children[0].text != "Process" {
		t.Errorf("categories = %+v", root.children)
	}
}

// TEXT is `[^\n]+`, so only a line that starts with %% is a comment.
func TestIshikawaCommentIsWholeLineOnly(t *testing.T) {
	root := parseIshikawa("ishikawa\nEffect\n  Pricing\n    Discount 20%% off\n%% a note")
	causes := root.children[0].children
	if len(causes) != 1 {
		t.Fatalf("causes = %+v", causes)
	}
	if causes[0].text != "Discount 20%% off" {
		t.Errorf("cause = %q, want the %%%% kept", causes[0].text)
	}
}

// ── Radar ───────────────────────────────────────────────────────────────────

func TestRadarAxesAndCurves(t *testing.T) {
	c := RenderRadar("radar-beta\n  title Grades\n  axis m[\"Math\"], s[\"Science\"]\n  axis e[\"English\"]\n  curve a[\"Alice\"]{85, 90, 80}\n  max 100", renderer.UNICODE, false, nil)
	assertCanvasContains(t, c, "Grades")
	assertCanvasContains(t, c, "Math")
	assertCanvasContains(t, c, "Science")
	assertCanvasContains(t, c, "English")
	assertCanvasContains(t, c, "Alice")
	assertCanvasContains(t, c, "●")
}

func TestRadarSeveralDeclarationsOneLine(t *testing.T) {
	rc := parseRadar("radar-beta\n  axis a, b, c\n  curve one{1, 2, 3}, two{3, 2, 1}")
	if len(rc.axes) != 3 {
		t.Fatalf("axes = %d, want 3", len(rc.axes))
	}
	if len(rc.curves) != 2 {
		t.Fatalf("curves = %d, want 2", len(rc.curves))
	}
	if got := rc.curves[1].values; len(got) != 3 || got[0] != 3 {
		t.Errorf("second curve values = %v, want [3 2 1]", got)
	}
}

func TestRadarKeyedValuesFollowAxisOrder(t *testing.T) {
	rc := parseRadar("radar-beta\n  axis a, b, c\n  curve x{ c: 30, a: 10, b: 20 }")
	want := []float64{10, 20, 30}
	if got := rc.curves[0].values; len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Errorf("values = %v, want %v", got, want)
	}
}

func TestRadarOptions(t *testing.T) {
	rc := parseRadar("radar-beta\n  axis a, b, c\n  curve x{1,2,3}\n  showLegend false\n  graticule polygon\n  ticks 3\n  min 1\n  max 9")
	if rc.showLegend {
		t.Error("showLegend false was not read")
	}
	if !rc.polygon {
		t.Error("graticule polygon was not read")
	}
	if rc.ticks != 3 {
		t.Errorf("ticks = %d, want 3", rc.ticks)
	}
	lo, hi := rc.bounds()
	if lo != 1 || hi != 9 {
		t.Errorf("bounds = %v..%v, want 1..9", lo, hi)
	}
}

func TestRadarTooFewAxes(t *testing.T) {
	c := RenderRadar("radar-beta\n  axis a, b\n  curve x{1,2}", renderer.UNICODE, false, nil)
	assertCanvasContains(t, c, "three or more axes")
}

// ── Venn ────────────────────────────────────────────────────────────────────

func TestVennSetsAndUnions(t *testing.T) {
	c := RenderVenn("venn-beta\n  title Overlap\n  set Frontend\n  set Backend\n  union Frontend,Backend[\"APIs\"]", renderer.UNICODE, false, nil)
	assertCanvasContains(t, c, "Overlap")
	assertCanvasContains(t, c, "Frontend")
	assertCanvasContains(t, c, "Backend")
	assertCanvasContains(t, c, "APIs")
}

func TestVennLabelsAndSizes(t *testing.T) {
	vd := parseVenn("venn-beta\n  set A[\"Alpha\"]:20\n  set B[\"Beta\"]:12\n  union A,B[\"AB\"]:3")
	if len(vd.sets) != 2 || vd.sets[0].label != "Alpha" || vd.sets[0].size != 20 {
		t.Fatalf("sets = %+v", vd.sets)
	}
	if len(vd.unions) != 1 || vd.unions[0].mask != 0b11 || vd.unions[0].size != 3 {
		t.Fatalf("unions = %+v", vd.unions)
	}
}

func TestVennUnionOfUndeclaredSetIsDropped(t *testing.T) {
	vd := parseVenn("venn-beta\n  set A\n  union A,Ghost[\"AB\"]")
	if len(vd.unions) != 0 {
		t.Errorf("unions = %+v, want none", vd.unions)
	}
}

func TestVennTextAndStyleAreSkipped(t *testing.T) {
	vd := parseVenn("venn-beta\n  set A[\"Alpha\"]\n    text A1[\"React\"]\n  style A fill:#ff6b6b")
	if len(vd.sets) != 1 {
		t.Errorf("sets = %+v, want the one set", vd.sets)
	}
}

func TestVennNoSets(t *testing.T) {
	c := RenderVenn("venn-beta", renderer.UNICODE, false, nil)
	assertCanvasContains(t, c, "no sets")
}

// ── Wardley ─────────────────────────────────────────────────────────────────

func TestWardleyComponentsAndStages(t *testing.T) {
	c := RenderWardley("wardley-beta\n  title Tea\n  anchor Business [0.95, 0.63]\n  component Kettle [0.43, 0.35]\n  Business -> Kettle", renderer.UNICODE, nil)
	assertCanvasContains(t, c, "Tea")
	assertCanvasContains(t, c, "Business")
	assertCanvasContains(t, c, "Kettle")
	assertCanvasContains(t, c, "Genesis")
	assertCanvasContains(t, c, "Commodity")
	assertCanvasContains(t, c, "◎") // the anchor's double ring
}

func TestWardleyDecoratorsAndHyphenatedNames(t *testing.T) {
	wm := parseWardley("wardley-beta\n  component real-time processing [0.55, 0.40] (buy) (inertia)\n  component end-user [0.90, 0.95]\n  end-user -> real-time processing")
	if len(wm.nodes) != 2 {
		t.Fatalf("nodes = %+v", wm.nodes)
	}
	if wm.nodes[0].name != "real-time processing" || wm.nodes[0].sourced != "buy" || !wm.nodes[0].inertia {
		t.Errorf("node = %+v", wm.nodes[0])
	}
	if len(wm.links) != 1 || wm.links[0].from != "end-user" {
		t.Errorf("links = %+v", wm.links)
	}
}

func TestWardleyLinkFormsAndLabels(t *testing.T) {
	wm := parseWardley("wardley-beta\n  component A [0.1, 0.1]\n  component B [0.2, 0.2]\n  A --> B\n  A -.-> B\n  A +'backup'> B\n  A -> B; reads")
	if len(wm.links) != 4 {
		t.Fatalf("links = %+v", wm.links)
	}
	if wm.links[2].label != "backup" {
		t.Errorf("flow label = %q, want backup", wm.links[2].label)
	}
	if wm.links[3].label != "reads" {
		t.Errorf("annotation = %q, want reads", wm.links[3].label)
	}
}

func TestWardleyLinkToUndeclaredComponentIsDropped(t *testing.T) {
	wm := parseWardley("wardley-beta\n  component A [0.1, 0.1]\n  A -> Ghost")
	if len(wm.links) != 0 {
		t.Errorf("links = %+v, want none", wm.links)
	}
}

func TestWardleyCustomEvolutionStages(t *testing.T) {
	wm := parseWardley("wardley-beta\n  evolution Unmodelled -> Divergent -> Convergent -> Modelled\n  component A [0.1, 0.1]")
	if len(wm.stages) != 4 || wm.stages[0].name != "Unmodelled" || wm.stages[3].name != "Modelled" {
		t.Fatalf("stages = %+v", wm.stages)
	}
	wm = parseWardley("wardley-beta\n  evolution Genesis@0.2 -> Custom@0.4 -> Product@0.75 -> Commodity@1.0\n  component A [0.1, 0.1]")
	if wm.stages[0].end != 0.2 || wm.stages[2].end != 0.75 {
		t.Errorf("stage boundaries = %+v", wm.stages)
	}
}

func TestWardleyNoComponents(t *testing.T) {
	c := RenderWardley("wardley-beta\n  title Empty", renderer.UNICODE, nil)
	assertCanvasContains(t, c, "no components")
}

// ── Cynefin ─────────────────────────────────────────────────────────────────

func TestCynefinDomainsAndItems(t *testing.T) {
	c := RenderCynefin("cynefin-beta\n  title Response\n  complex\n    \"Probe it\"\n  clear\n    \"Known fix\"", renderer.UNICODE, nil)
	assertCanvasContains(t, c, "Response")
	assertCanvasContains(t, c, "Complex")
	assertCanvasContains(t, c, "Complicated")
	assertCanvasContains(t, c, "Chaotic")
	assertCanvasContains(t, c, "Clear")
	assertCanvasContains(t, c, "Confusion")
	assertCanvasContains(t, c, "Probe it")
	assertCanvasContains(t, c, "Known fix")
}

func TestCynefinEmptyFrameworkStillDrawsDomains(t *testing.T) {
	c := RenderCynefin("cynefin-beta\n  complex\n  complicated\n  clear\n  chaotic", renderer.UNICODE, nil)
	assertCanvasContains(t, c, "Complex")
	assertCanvasContains(t, c, "Chaotic")
}

func TestCynefinTransitions(t *testing.T) {
	cf := parseCynefin("cynefin-beta\n  complex --> complicated : \"Pattern identified\"\n  complex --> complex\n  chaotic --> complex")
	if len(cf.moves) != 2 {
		t.Fatalf("moves = %+v, want the two between different domains", cf.moves)
	}
	if cf.moves[0].label != "Pattern identified" {
		t.Errorf("label = %q", cf.moves[0].label)
	}
	c := RenderCynefin("cynefin-beta\n  complex --> complicated : \"Pattern identified\"", renderer.UNICODE, nil)
	assertCanvasContains(t, c, "Pattern identified")
	assertCanvasContains(t, c, "►")
}

func TestCynefinConfusionOverflowIsCounted(t *testing.T) {
	c := RenderCynefin("cynefin-beta\n  confusion\n    \"One\"\n    \"Two\"\n    \"Three\"\n    \"Four\"", renderer.UNICODE, nil)
	assertCanvasContains(t, c, "+2 more")
}

func TestCynefinItemOutsideADomainIsDropped(t *testing.T) {
	cf := parseCynefin("cynefin-beta\n  \"Homeless item\"\n  complex\n    \"Placed\"")
	if len(cf.items["complex"]) != 1 || cf.items["complex"][0] != "Placed" {
		t.Errorf("items = %+v", cf.items)
	}
}

// ── Chart family: review regressions ────────────────────────────────────────

// assertBounded fails when a canvas is larger than any terminal figure should
// be. A coordinate derived from NaN is math.MinInt64, and the rasteriser that
// receives one used to fill memory before anything could recover; a frame this
// size is the symptom that reaches a test before the process dies.
func assertBounded(t *testing.T, c *renderer.Canvas) {
	t.Helper()
	if c.Width <= 0 || c.Height <= 0 || c.Width > 1000 || c.Height > 1000 {
		t.Fatalf("frame is %dx%d, which is not a figure", c.Width, c.Height)
	}
}

func TestRadarDegenerateRangesStillRender(t *testing.T) {
	for _, tc := range []struct{ name, src string }{
		{"max below min", "radar-beta\n  axis a, b, c\n  curve x{0, 1, 2}\n  max 0"},
		{"max equal to min", "radar-beta\n  axis a, b, c\n  curve x{0, 1, 2}\n  min 5\n  max 5"},
		{"min not a number", "radar-beta\n  axis a, b, c\n  curve x{0, 1, 2}\n  min NaN"},
		{"value not finite", "radar-beta\n  axis a, b, c\n  curve x{Inf, 2, 3}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := RenderRadar(tc.src, renderer.UNICODE, false, nil)
			assertBounded(t, c)
			assertCanvasNotEmpty(t, c)
			lo, hi := parseRadar(tc.src).bounds()
			if !finite(lo) || !finite(hi) || hi <= lo {
				t.Errorf("bounds = %v..%v, want a finite range wider than nothing", lo, hi)
			}
		})
	}
}

func TestRadarKeyedCurveBeforeItsAxes(t *testing.T) {
	rc := parseRadar("radar-beta\n  curve x{ c: 30, a: 10 }\n  axis a, b, c")
	got := rc.curves[0].values
	if len(got) != 3 || got[0] != 10 || got[2] != 30 {
		t.Fatalf("values = %v, want 10 at a and 30 at c", got)
	}
	if !math.IsNaN(got[1]) {
		t.Errorf("unnamed axis = %v, want NaN so the render reads it as the low bound", got[1])
	}
}

func TestRadarLegendClearsTheAxisLabelMargin(t *testing.T) {
	// The right-hand axis label and the legend both live beyond the rim; the
	// legend has to start past the label, not on it.
	c := RenderRadar("radar-beta\n  axis speed[\"Speed\"], cost[\"Cost\"], reliability[\"Reliability\"]\n  axis support[\"Support\"], features[\"Features\"]\n  curve a[\"Vendor A\"]{85, 60, 90, 70, 75}\n  max 100", renderer.UNICODE, false, nil)
	assertCanvasContains(t, c, "Cost")
	assertCanvasContains(t, c, "Reliability")
}

func TestChartLegendBoxesAreASCIIInASCII(t *testing.T) {
	for _, tc := range []struct {
		name string
		draw func() *renderer.Canvas
	}{
		{"radar", func() *renderer.Canvas {
			return RenderRadar("radar-beta\n  axis a, b, c\n  curve one{1, 2, 3}", renderer.ASCII, false, nil)
		}},
		{"venn", func() *renderer.Canvas {
			return RenderVenn("venn-beta\n  set A\n  set B\n  union A,B[\"AB\"]", renderer.ASCII, false, nil)
		}},
		{"pie", func() *renderer.Canvas {
			return RenderPieChart("pie\n  \"A\" : 60\n  \"B\" : 40", renderer.ASCII, false, nil)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := tc.draw().ToString()
			for _, r := range out {
				if r > 127 {
					t.Fatalf("ASCII render carries %q\n---\n%s\n---", r, out)
				}
			}
		})
	}
}

func TestVennUnionJoinerFollowsTheCharset(t *testing.T) {
	vd := parseVenn("venn-beta\n  set A\n  set B\n  union A,B")
	if got := vd.unionName(0b11, marksFor(renderer.UNICODE)); got != "A ∩ B" {
		t.Errorf("unicode join = %q", got)
	}
	if got := vd.unionName(0b11, marksFor(renderer.ASCII)); got != "A n B" {
		t.Errorf("ascii join = %q", got)
	}
}

func TestVennFourthSetIsDropped(t *testing.T) {
	vd := parseVenn("venn-beta\n  set A\n  set B\n  set C\n  set D\n  union C,D[\"CD\"]")
	if len(vd.sets) != vennMaxSets {
		t.Errorf("sets = %d, want the cap of %d", len(vd.sets), vennMaxSets)
	}
	if len(vd.unions) != 0 {
		t.Errorf("unions = %+v, want none: D was never declared", vd.unions)
	}
}

func TestVennRegionLabelKeepsItsSpaces(t *testing.T) {
	// `Put` skips a space, so without clearing the cells first the fill under
	// a two-word label shows between its words and it reads as one.
	c := RenderVenn("venn-beta\n  set Desirable\n  set Feasible\n  set Viable\n  union Desirable,Feasible,Viable[\"Ship it\"]", renderer.UNICODE, false, nil)
	assertCanvasContains(t, c, "Ship it")
}

func TestWardleyLabelKeepsItsSpacesOverALeg(t *testing.T) {
	// Every dependency draws its horizontal leg on the target's row, which is
	// the target's label row.
	c := RenderWardley("wardley-beta\n  component Web App [0.75, 0.20]\n  component API Gateway [0.70, 0.90]\n  Web App -> API Gateway", renderer.UNICODE, nil)
	assertCanvasContains(t, c, "Web App")
	assertCanvasContains(t, c, "API Gateway")
}

func TestWardleyEvolveSurvivesABlockedRow(t *testing.T) {
	// B's incoming leg runs along A's row + 1, where A's evolve arrow would
	// go; the arrow moves rather than vanishing.
	src := "wardley-beta\n  component A [0.60, 0.20]\n  component B [0.55, 0.90]\n  component C [0.90, 0.40]\n  C -> B\n  evolve A 0.70"
	c := RenderWardley(src, renderer.UNICODE, nil)
	if !strings.Contains(c.ToString(), string(renderer.UNICODE.ArrowRight)) {
		t.Errorf("the evolve arrow is missing\n---\n%s\n---", c.ToString())
	}
}

func TestWardleyStageNamesDoNotMerge(t *testing.T) {
	src := "wardley-beta\n  evolution Genesis / Concept -> Custom / Emerging -> Product / Converging -> Commodity / Accepted\n  component Novel Idea [0.05, 0.20]"
	out := RenderWardley(src, renderer.UNICODE, nil).ToString()
	for _, merged := range []string{"ConceptCustom", "EmergingProduct", "ConvergingCommodity"} {
		if strings.Contains(out, merged) {
			t.Errorf("stage names merged as %q\n---\n%s\n---", merged, out)
		}
	}
}

func TestCynefinQuadrantCountsWhatDoesNotFit(t *testing.T) {
	c := RenderCynefin("cynefin-beta\n  clear\n    \"One\"\n    \"Two\"\n    \"Three\"\n    \"Four\"\n    \"Five\"", renderer.UNICODE, nil)
	assertCanvasContains(t, c, "+2 more")
}

func TestCynefinPracticeFollowsTheCharset(t *testing.T) {
	unicode := cynefinDomains[0].practice(marksFor(renderer.UNICODE))
	if unicode != "Probe · Sense · Respond — emergent practice" {
		t.Errorf("unicode practice = %q", unicode)
	}
	ascii := cynefinDomains[0].practice(marksFor(renderer.ASCII))
	if ascii != "Probe * Sense * Respond - emergent practice" {
		t.Errorf("ascii practice = %q", ascii)
	}
}

func TestTruncateMarkSaysWhereItCut(t *testing.T) {
	m := marksFor(renderer.UNICODE)
	if got := truncateMark("emergent practice", 30, m); got != "emergent practice" {
		t.Errorf("text that fits = %q, want it whole", got)
	}
	if got := truncateMark("emergent practice", 10, m); got != "emergent…" {
		t.Errorf("cut = %q, want the mark and no trailing space", got)
	}
	if got := truncateMark("emergent", 1, marksFor(renderer.ASCII)); got != "e" {
		t.Errorf("cut too narrow for the mark = %q", got)
	}
}

func TestCellPathClipsToItsRectangle(t *testing.T) {
	// The vertex is what a NaN-derived coordinate looks like by the time it
	// reaches the rasteriser.
	path := cellPath([][2]int{{0, 0}, {math.MinInt64, math.MinInt64}}, false, false, cellRect{0, 0, 9, 9})
	if len(path) > 100 {
		t.Fatalf("path is %d cells, want it clipped to the rectangle", len(path))
	}
	for _, p := range path {
		if p[0] < 0 || p[0] > 9 || p[1] < 0 || p[1] > 9 {
			t.Fatalf("cell %v is outside the rectangle", p)
		}
	}
}

// ── Sankey ──────────────────────────────────────────────────────────────────

func TestSankeyRowsAndNodeOrder(t *testing.T) {
	sd := parseSankey(`sankey-beta

%% source,target,value
Coal,Electricity,45
Gas,Electricity,30
Electricity,Homes,60`)

	if len(sd.links) != 3 {
		t.Fatalf("links = %d, want 3", len(sd.links))
	}
	want := []string{"Coal", "Electricity", "Gas", "Homes"}
	got := make([]string, len(sd.nodes))
	for i, n := range sd.nodes {
		got[i] = n.name
	}
	if !slices.Equal(got, want) {
		t.Errorf("nodes = %v, want %v", got, want)
	}
	if e := sd.nodes[sd.index["Electricity"]]; e.in != 75 || e.out != 60 || e.value() != 75 {
		t.Errorf("Electricity in=%v out=%v value=%v, want 75 60 75", e.in, e.out, e.value())
	}
}

// A quoted field carries commas, and a pair of quotes inside one is a single
// quote.
func TestSankeyQuotedFields(t *testing.T) {
	sd := parseSankey(`sankey
Pumped heat,"Heating and cooling, ""homes""",193.026`)

	if len(sd.links) != 1 {
		t.Fatalf("links = %+v", sd.links)
	}
	if got := sd.links[0].target; got != `Heating and cooling, "homes"` {
		t.Errorf("target = %q", got)
	}
	if sd.links[0].value != 193.026 {
		t.Errorf("value = %v, want 193.026", sd.links[0].value)
	}

	// Upstream trims the escaped form as well as the plain one, so a quoted
	// name and a bare one are the same node.
	sd = parseSankey("sankey\n\"A\" ,B,3\nA,C,3")
	if len(sd.nodes) != 3 {
		names := []string{}
		for _, n := range sd.nodes {
			names = append(names, n.name)
		}
		t.Errorf("nodes = %q, want A, B and C", names)
	}
}

// A row is dropped when it does not hold three fields, or when the third one
// is not a number.
func TestSankeyMalformedRowsDropped(t *testing.T) {
	sd := parseSankey("sankey\nA,B\nA,B,C,4\nA,B,many\nA,B,1")
	if len(sd.links) != 1 {
		t.Fatalf("links = %+v, want the last row only", sd.links)
	}
}

// Depth is the longest path from a source, so a node sits right of every node
// feeding it.
func TestSankeyDepthIsLongestPath(t *testing.T) {
	sd := parseSankey("sankey\nA,B,1\nB,C,1\nA,C,1")
	sankeyDepths(sd)
	for name, want := range map[string]int{"A": 0, "B": 1, "C": 2} {
		if got := sd.nodes[sd.index[name]].depth; got != want {
			t.Errorf("%s depth = %d, want %d", name, got, want)
		}
	}
}

// A link that closes a cycle is dropped: it is left out of both totals and
// out of the drawing, and the columns behind it do not spread.
func TestSankeyCycleLinksAreDropped(t *testing.T) {
	sd := parseSankey("sankey\nA,B,1\nB,A,1")
	if len(sd.links) != 2 || sd.links[0].back || !sd.links[1].back {
		t.Fatalf("links = %+v, want the second marked as the back edge", sd.links)
	}
	a, b := sd.nodes[sd.index["A"]], sd.nodes[sd.index["B"]]
	if a.in != 0 || a.out != 1 || b.in != 1 || b.out != 0 {
		t.Errorf("totals: A in=%v out=%v, B in=%v out=%v; want the back edge out of both", a.in, a.out, b.in, b.out)
	}
	p := sankeyLayout(sd)
	if len(p.cols) != 2 {
		t.Errorf("columns = %d, want 2 with none empty", len(p.cols))
	}
	for i, col := range p.cols {
		if len(col) == 0 {
			t.Errorf("column %d is empty", i)
		}
	}
	assertCanvasNotEmpty(t, RenderSankey("sankey\nA,B,1\nB,A,1", renderer.UNICODE, nil))
}

// A link from a node to itself closes the shortest cycle there is, so it does
// not inflate the node's value with rows nothing draws.
func TestSankeySelfLinkIsDropped(t *testing.T) {
	sd := parseSankey("sankey\nA,A,3\nA,B,2")
	if !sd.links[0].back || sd.links[1].back {
		t.Fatalf("links = %+v, want the self link marked", sd.links)
	}
	if a := sd.nodes[sd.index["A"]]; a.value() != 2 {
		t.Errorf("A value = %v, want 2", a.value())
	}
	p := sankeyLayout(sd)
	if len(p.cols) != 2 {
		t.Errorf("columns = %d, want 2", len(p.cols))
	}
}

func TestSankeyRendersLabelsWithValues(t *testing.T) {
	c := RenderSankey("sankey-beta\nCoal,Electricity,45\nElectricity,Homes,45", renderer.UNICODE, nil)
	assertCanvasContains(t, c, "Coal 45")
	assertCanvasContains(t, c, "Electricity 45")
	assertCanvasContains(t, c, "Homes 45")
}

func TestSankeyEmpty(t *testing.T) {
	c := RenderSankey("sankey-beta", renderer.UNICODE, nil)
	assertCanvasContains(t, c, "[sankey] no links")
}

// ── ZenUML ──────────────────────────────────────────────────────────────────

// zenMessages lists the arrows a parsed ZenUML source draws, in order, as
// "source>target:label" with a leading "~" on a dotted one.
func zenMessages(t *testing.T, source string) []string {
	t.Helper()
	var out []string
	for _, ev := range flattenEvents(parseZenUML(source).events, 0) {
		if m, ok := ev.(*message); ok {
			prefix := ""
			if m.lineType == "dotted" {
				prefix = "~"
			}
			out = append(out, prefix+m.source+">"+m.target+":"+m.label)
		}
	}
	return out
}

// A sync call is an arrow, an activation on the callee, the statements of its
// body, and the reply its `return` asks for.
func TestZenUMLSyncCallReturnsToItsCaller(t *testing.T) {
	got := zenMessages(t, `zenuml
    Client->A.method() {
      B.method() {
        return inner
      }
      return outer
    }`)
	want := []string{
		"Client>A:method()",
		"A>B:method()",
		"~B>A:inner",
		"~A>Client:outer",
	}
	if !slices.Equal(got, want) {
		t.Errorf("messages =\n%v\nwant\n%v", got, want)
	}
}

// An assignment replies with the variable's name, and only when the body did
// not reply for itself.
func TestZenUMLAssignmentReplies(t *testing.T) {
	got := zenMessages(t, "zenuml\nStarter\nresult = A.get()\nSomeType typed = A.get2()")
	want := []string{"Starter>A:get()", "~A>Starter:result", "Starter>A:get2()", "~A>Starter:typed"}
	if !slices.Equal(got, want) {
		t.Errorf("messages = %v, want %v", got, want)
	}

	got = zenMessages(t, "zenuml\nStarter\nresult = A.get() {\n  return actual\n}")
	want = []string{"Starter>A:get()", "~A>Starter:actual"}
	if !slices.Equal(got, want) {
		t.Errorf("messages = %v, want %v", got, want)
	}
}

// A call raises an activation on its callee and drops it when the body ends.
func TestZenUMLCallActivatesTheCallee(t *testing.T) {
	events := flattenEvents(parseZenUML("zenuml\nStarter\nA.method() {\n  B.method()\n}").events, 0)
	var got []string
	for _, ev := range events {
		if a, ok := ev.(*activateEvent); ok {
			state := "off"
			if a.active {
				state = "on"
			}
			got = append(got, a.participant+":"+state)
		}
	}
	want := []string{"A:on", "B:on", "B:off", "A:off"}
	if !slices.Equal(got, want) {
		t.Errorf("activations = %v, want %v", got, want)
	}
}

// The starter is the declared one, else the first statement's sender, else
// the first declared participant, else a lane of its own.
func TestZenUMLStarterResolution(t *testing.T) {
	cases := []struct{ source, want string }{
		{"zenuml\n@Starter(Bob)\nAlice->John: hi", "Bob"},
		{"zenuml\nBob\nAlice\nAlice->Bob: Hi Bob", "Alice"},
		{"zenuml\nBookService\nBookService.getBook()", "BookService"},
		{"zenuml\nA.method()", "Starter"},
	}
	for _, c := range cases {
		if got := zenStarter(zenLines(c.source)); got != c.want {
			t.Errorf("starter of %q = %q, want %q", c.source, got, c.want)
		}
	}
}

// An annotator picks the participant's shape and `as` its label.
func TestZenUMLAnnotatorsAndAliases(t *testing.T) {
	d := parseZenUML("zenuml\n@Actor Customer\n@Database Inventory\n@Lambda Fn\nA as Alice\nCustomer->A: hi")
	want := []struct{ id, label, kind string }{
		{"Customer", "Customer", "actor"},
		{"Inventory", "Inventory", "database"},
		{"Fn", "Fn", "participant"},
		{"A", "Alice", "participant"},
	}
	if len(d.participants) != len(want) {
		t.Fatalf("participants = %+v", d.participants)
	}
	for i, w := range want {
		p := d.participants[i]
		if p.id != w.id || p.label != w.label || p.kind != w.kind {
			t.Errorf("participant %d = %q/%q/%q, want %q/%q/%q", i, p.id, p.label, p.kind, w.id, w.label, w.kind)
		}
	}
}

// Each fragment keyword maps to the frame the sequence renderer draws for it.
func TestZenUMLFragmentsMapToFrames(t *testing.T) {
	d := parseZenUML(`zenuml
    Starter
    if (a) {
      A.one()
    } else if (b) {
      A.two()
    } else {
      A.three()
    }
    while (more) {
      A.four()
    }
    opt {
      A.five()
    }
    par {
      A.six()
    }
    try {
      A.seven()
    } catch (Boom) {
      A.eight()
    } finally {
      A.nine()
    }`)

	var kinds []string
	var sections []string
	for _, ev := range d.events {
		blk, ok := ev.(*block)
		if !ok {
			continue
		}
		kinds = append(kinds, blk.kind)
		for _, s := range blk.sections {
			sections = append(sections, s.label)
		}
	}
	if want := []string{"alt", "loop", "opt", "par", "critical"}; !slices.Equal(kinds, want) {
		t.Errorf("frames = %v, want %v", kinds, want)
	}
	if want := []string{"else b", "else", "catch Boom", "finally"}; !slices.Equal(sections, want) {
		t.Errorf("sections = %v, want %v", sections, want)
	}
}

// A comment above a message becomes a note over the participant it reaches.
func TestZenUMLCommentBecomesANote(t *testing.T) {
	d := parseZenUML("zenuml\nBookService\n// a comment on a message.\n// **Markdown** is supported.\nBookService.getBook()")
	n, ok := d.events[0].(*note)
	if !ok {
		t.Fatalf("first event = %T, want a note", d.events[0])
	}
	if n.position != "over" || len(n.participants) != 1 || n.participants[0] != "BookService" {
		t.Errorf("note = %+v", n)
	}
	if want := "a comment on a message.\n**Markdown** is supported."; n.text != want {
		t.Errorf("note text = %q, want %q", n.text, want)
	}
}

// A comment on a participant is not rendered.
func TestZenUMLCommentOnAParticipantIsDropped(t *testing.T) {
	d := parseZenUML("zenuml\n// a comment on a participant\nBookService\nA->B: hi")
	for _, ev := range d.events {
		if _, ok := ev.(*note); ok {
			t.Errorf("events = %+v, want no note", d.events)
		}
	}
}

// The @return annotator makes the async message that follows it a reply.
func TestZenUMLReturnAnnotator(t *testing.T) {
	got := zenMessages(t, "zenuml\n@return\nA->Client: x11\nA->Client: x12")
	want := []string{"~A>Client:x11", "A>Client:x12"}
	if !slices.Equal(got, want) {
		t.Errorf("messages = %v, want %v", got, want)
	}
}

// A title is a sequence-model field, and both syntaxes render through it.
func TestZenUMLTitle(t *testing.T) {
	c := RenderZenUML("zenuml\ntitle Demo\nAlice->John: Hello", renderer.UNICODE)
	assertCanvasContains(t, c, "Demo")
	assertCanvasContains(t, c, "Hello")
	if got := parseSequenceDiagram("sequenceDiagram\n  A->>B: x").title; got != "" {
		t.Errorf("a sequence diagram has title %q, want none", got)
	}
}

func TestZenUMLEmpty(t *testing.T) {
	assertCanvasNotEmpty(t, RenderZenUML("zenuml\nAlice->Bob: hi", renderer.UNICODE))
}

// assertSankeyBandsFitTheirBars checks the invariant the apportionment
// exists for: every band leaves and lands inside the bar it meets, and the
// bands meeting one side of a bar cover it exactly when that side carries the
// node's whole flow.
func assertSankeyBandsFitTheirBars(t *testing.T, source string) {
	t.Helper()
	sd := parseSankey(source)
	p := sankeyLayout(sd)
	fromRows := map[string]int{}
	intoRows := map[string]int{}

	for _, l := range sd.links {
		if l.back {
			continue
		}
		s, tn := p.node(l.source), p.node(l.target)
		if l.sy < s.top || l.sy+l.hs > s.top+s.height {
			t.Errorf("%s->%s leaves rows %d..%d, outside %s's bar at %d..%d",
				l.source, l.target, l.sy, l.sy+l.hs-1, l.source, s.top, s.top+s.height-1)
		}
		if l.ty < tn.top || l.ty+l.ht > tn.top+tn.height {
			t.Errorf("%s->%s lands on rows %d..%d, outside %s's bar at %d..%d",
				l.source, l.target, l.ty, l.ty+l.ht-1, l.target, tn.top, tn.top+tn.height-1)
		}
		fromRows[l.source] += l.hs
		intoRows[l.target] += l.ht
	}
	for _, n := range sd.nodes {
		if n.out == n.value() && fromRows[n.name] != n.height {
			t.Errorf("bands leaving %s cover %d rows of a %d-row bar", n.name, fromRows[n.name], n.height)
		}
		if n.in == n.value() && intoRows[n.name] != n.height {
			t.Errorf("bands meeting %s cover %d rows of a %d-row bar", n.name, intoRows[n.name], n.height)
		}
	}
}

// Five equal flows into one node round to four rows each and the bar is 18:
// apportionment spends the bar's rows, it does not hand out more than it has.
func TestSankeyBandRowsFitTheBarTheyMeet(t *testing.T) {
	assertSankeyBandsFitTheirBars(t, "sankey-beta\nP,X,0.3\nQ,X,0.3\nR,X,0.3\nS,X,0.3\nT,X,0.3")
	assertSankeyBandsFitTheirBars(t, "sankey-beta\nCoal,Electricity,45\nGas,Electricity,30\nSolar,Electricity,15\nElectricity,Homes,40\nElectricity,Industry,35\nElectricity,\"Losses, grid\",15")

	// Thirty sources of one unit each, at the width the reviewer used: more
	// links than the target bar has rows.
	var b strings.Builder
	b.WriteString("sankey-beta\n")
	for i := range 30 {
		fmt.Fprintf(&b, "S%d,Electricity,1\n", i)
	}
	src := b.String()
	SetWidthOverride(60)
	defer SetWidthOverride(0)
	assertSankeyBandsFitTheirBars(t, src)
	assertCanvasNotEmpty(t, RenderSankey(src, renderer.UNICODE, nil))
}

// A value is read the way upstream's parseFloat reads one, and a row that is
// not a finite, non-negative number is dropped.
func TestSankeyValuesMustBeFiniteAndPositive(t *testing.T) {
	for _, source := range []string{
		"sankey\nA,B,NaN\nC,D,1000",
		"sankey\nA,B,Inf\nC,D,1000",
		"sankey\nA,B,-5\nC,D,1000",
		"sankey\nA,B,1_000\nC,D,1000",
		"sankey\nA,B,0x1p4\nC,D,1000",
	} {
		sd := parseSankey(source)
		if len(sd.links) != 1 || sd.links[0].source != "C" {
			t.Errorf("%q parsed %+v, want the C,D row only", source, sd.links)
		}
	}
	// A total that overflows to infinity must not set an unbounded scale.
	c := RenderSankey("sankey\nA,B,1e308\nA,C,1e308", renderer.UNICODE, nil)
	if c.Height > 4*sankeyMaxRows {
		t.Errorf("canvas is %d rows for an overflowing total", c.Height)
	}
}

// A flow prints in at most six significant digits, and a whole number prints
// as one.
func TestSankeyValueFormat(t *testing.T) {
	cases := map[float64]string{
		45: "45", 124.729: "124.729", 0.5: "0.5",
		1000000: "1000000", 1e308: "1e+308", 1.0 / 3.0: "0.333333",
	}
	for v, want := range cases {
		if got := sankeyValue(v); got != want {
			t.Errorf("sankeyValue(%v) = %q, want %q", v, got, want)
		}
	}
}
