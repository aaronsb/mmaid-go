package diagram

import (
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
