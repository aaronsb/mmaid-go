package mmaid

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/aaronsb/mmaid-go/internal/textwidth"
)

// ── Helpers ──────────────────────────────────────────────────────────────────

func assertContains(t *testing.T, output, substr string) {
	t.Helper()
	if !strings.Contains(output, substr) {
		t.Errorf("output missing %q\n---\n%s\n---", substr, output)
	}
}

func assertNotContains(t *testing.T, output, substr string) {
	t.Helper()
	if strings.Contains(output, substr) {
		t.Errorf("output should not contain %q\n---\n%s\n---", substr, output)
	}
}

func assertNonEmpty(t *testing.T, output string) {
	t.Helper()
	if strings.TrimSpace(output) == "" {
		t.Error("output is empty")
	}
}

func assertReasonableDimensions(t *testing.T, output string) {
	t.Helper()
	lines := strings.Split(output, "\n")
	if len(lines) > 200 {
		t.Errorf("output has %d lines (max 200)", len(lines))
	}
	for _, line := range lines {
		if len(line) > 500 {
			t.Errorf("line width %d exceeds 500", len(line))
			break
		}
	}
}

func assertValidUnicode(t *testing.T, output string) {
	t.Helper()
	if strings.ContainsRune(output, '\ufffd') {
		t.Error("output contains Unicode replacement character")
	}
}

// ── Flowchart tests ──────────────────────────────────────────────────────────

func TestFlowchartLR(t *testing.T) {
	out := Render("graph LR\n  A --> B")
	assertContains(t, out, "A")
	assertContains(t, out, "B")
	assertContains(t, out, "►")
}

func TestFlowchartTD(t *testing.T) {
	out := Render("graph TD\n  A --> B")
	assertContains(t, out, "A")
	assertContains(t, out, "B")
	assertContains(t, out, "▼")
}

func TestFlowchartBT(t *testing.T) {
	out := Render("graph BT\n  A --> B")
	assertContains(t, out, "A")
	assertContains(t, out, "B")
	assertContains(t, out, "▲")
}

func TestFlowchartRL(t *testing.T) {
	out := Render("graph RL\n  A --> B")
	assertContains(t, out, "A")
	assertContains(t, out, "B")
	assertContains(t, out, "◄")
}

func TestFlowchartChain(t *testing.T) {
	out := Render("graph LR\n  A --> B --> C --> D --> E")
	for _, label := range []string{"A", "B", "C", "D", "E"} {
		assertContains(t, out, label)
	}
}

func TestFlowchartSingleNode(t *testing.T) {
	out := Render("graph LR\n  A")
	assertContains(t, out, "A")
	assertNotContains(t, out, "►")
}

func TestFlowchartDiamond(t *testing.T) {
	out := Render("graph LR\n  A{Decision}")
	assertContains(t, out, "Decision")
	assertContains(t, out, "╱") // chamfered diamond corners
}

func TestFlowchartRounded(t *testing.T) {
	out := Render("graph LR\n  A(Rounded)")
	assertContains(t, out, "Rounded")
	assertContains(t, out, "╭")
}

func TestFlowchartCircle(t *testing.T) {
	out := Render("graph LR\n  A((Circle))")
	assertContains(t, out, "Circle")
	assertContains(t, out, "◯")
}

func TestFlowchartEdgeLabels(t *testing.T) {
	out := Render("graph LR\n  A -->|Yes| B")
	assertContains(t, out, "A")
	assertContains(t, out, "B")
	assertContains(t, out, "Yes")
}

func TestFlowchartDottedEdge(t *testing.T) {
	out := Render("graph LR\n  A -.-> B")
	assertContains(t, out, "A")
	assertContains(t, out, "B")
	assertContains(t, out, "┄")
}

func TestFlowchartThickEdge(t *testing.T) {
	out := Render("graph LR\n  A ==> B")
	assertContains(t, out, "A")
	assertContains(t, out, "B")
	assertContains(t, out, "━")
}

func TestFlowchartSubgraphs(t *testing.T) {
	out := Render(`graph TB
    subgraph Frontend
        A[Web]
    end
    subgraph Backend
        B[API]
    end
    A --> B`)
	assertContains(t, out, "Frontend")
	assertContains(t, out, "Backend")
	assertContains(t, out, "Web")
	assertContains(t, out, "API")
}

func TestFlowchartASCII(t *testing.T) {
	out := Render("graph LR\n  A --> B", WithASCII())
	assertContains(t, out, "A")
	assertContains(t, out, "B")
	assertContains(t, out, ">")
	assertNotContains(t, out, "►")
	assertNotContains(t, out, "─")
	assertNotContains(t, out, "│")
}

func TestFlowchartBidirectional(t *testing.T) {
	out := Render("graph LR\n  A <--> B")
	assertContains(t, out, "◄")
	assertContains(t, out, "►")
}

func TestFlowchartSemicolonSyntax(t *testing.T) {
	out := Render("graph LR; A --> B; B --> C")
	assertContains(t, out, "A")
	assertContains(t, out, "B")
	assertContains(t, out, "C")
}

func TestFlowchartFrontmatterStripped(t *testing.T) {
	out := Render("---\ntitle: Test\n---\ngraph LR\n  A --> B")
	assertContains(t, out, "A")
	assertContains(t, out, "B")
}

func TestFlowchartDimensions(t *testing.T) {
	out := Render("graph TD\n  A --> B --> C\n  A --> D --> C")
	assertReasonableDimensions(t, out)
	assertValidUnicode(t, out)
}

// ── Sequence diagram tests ──────────────────────────────────────────────────

func TestSequenceBasic(t *testing.T) {
	out := Render(`sequenceDiagram
    Alice->>Bob: Hello
    Bob-->>Alice: Hi`)
	assertContains(t, out, "Alice")
	assertContains(t, out, "Bob")
	assertContains(t, out, "Hello")
	assertContains(t, out, "Hi")
	assertContains(t, out, "►")
}

func TestSequenceMultipleParticipants(t *testing.T) {
	out := Render(`sequenceDiagram
    participant A
    participant B
    participant C
    A->>B: msg1
    B->>C: msg2`)
	assertContains(t, out, "msg1")
	assertContains(t, out, "msg2")
}

func TestSequenceNotes(t *testing.T) {
	out := Render(`sequenceDiagram
    Alice->>Bob: Hello
    Note right of Alice: A note`)
	assertContains(t, out, "A note")
}

func TestSequenceLoopBlock(t *testing.T) {
	out := Render(`sequenceDiagram
    Alice->>Bob: Hello
    loop Every sec
        Bob->>Alice: Ping
    end`)
	assertContains(t, out, "loop")
}

// ── Class diagram tests ─────────────────────────────────────────────────────

func TestClassDiagramBasic(t *testing.T) {
	out := Render(`classDiagram
    Animal <|-- Duck
    Animal : +makeSound()
    Duck : +swim()`)
	assertContains(t, out, "Animal")
	assertContains(t, out, "Duck")
	assertContains(t, out, "+makeSound()")
	assertContains(t, out, "+swim()")
}

func TestClassDiagramAnnotation(t *testing.T) {
	out := Render(`classDiagram
    class Shape {
        <<interface>>
        +area() float
    }`)
	assertContains(t, out, "Shape")
	assertContains(t, out, "interface")
}

// ── ER diagram tests ────────────────────────────────────────────────────────

func TestERDiagramBasic(t *testing.T) {
	out := Render(`erDiagram
    CUSTOMER ||--o{ ORDER : places
    CUSTOMER {
        int id PK
        string name
    }`)
	assertContains(t, out, "CUSTOMER")
	assertContains(t, out, "ORDER")
	assertContains(t, out, "places")
}

// ── State diagram tests ─────────────────────────────────────────────────────

func TestStateDiagramBasic(t *testing.T) {
	out := Render(`stateDiagram-v2
    [*] --> Idle
    Idle --> Active
    Active --> [*]`)
	assertContains(t, out, "Idle")
	assertContains(t, out, "Active")
	assertContains(t, out, "●")
}

// ── Pie chart tests ─────────────────────────────────────────────────────────

func TestPieChartBasic(t *testing.T) {
	out := Render(`pie title Pets
    "Dogs" : 45
    "Cats" : 30`)
	assertContains(t, out, "Dogs")
	assertContains(t, out, "Cats")
	assertContains(t, out, "⣿") // braille solid pattern (no-theme mode)
}

func TestPieChartShowData(t *testing.T) {
	out := Render(`pie showData
    "A" : 60
    "B" : 40`)
	assertContains(t, out, "(60)")
	assertContains(t, out, "(40)")
}

// ── Git graph tests ─────────────────────────────────────────────────────────

func TestGitGraphBasic(t *testing.T) {
	out := Render(`gitGraph
    commit id: "A"
    commit id: "B"
    branch dev
    checkout dev
    commit id: "C"`)
	assertContains(t, out, "main")
	assertContains(t, out, "dev")
	assertContains(t, out, "A")
	assertContains(t, out, "B")
	assertContains(t, out, "C")
	assertContains(t, out, "●")
}

func TestGitGraphMerge(t *testing.T) {
	out := Render(`gitGraph
    commit id: "A"
    branch dev
    checkout dev
    commit id: "B"
    checkout main
    merge dev id: "C"`)
	assertContains(t, out, "C")
	// The fork is a tee on main; the merge is a corner where dev's line ends.
	assertContains(t, out, "┬")
	assertContains(t, out, "┘")
}

func TestGitGraphTags(t *testing.T) {
	out := Render(`gitGraph
    commit id: "init" tag: "v1.0"`)
	assertContains(t, out, "[v1.0]")
}

func TestGitGraphTB(t *testing.T) {
	out := Render(`gitGraph TB:
    commit id: "A"
    commit id: "B"`)
	assertContains(t, out, "A")
	assertContains(t, out, "B")
	assertNonEmpty(t, out)
}

// ── Block diagram tests ─────────────────────────────────────────────────────

func TestBlockDiagramBasic(t *testing.T) {
	out := Render(`block-beta
    columns 2
    A["Hello"] B["World"]`)
	assertContains(t, out, "Hello")
	assertContains(t, out, "World")
}

func TestBlockDiagramLinks(t *testing.T) {
	out := Render(`block-beta
    columns 3
    A["In"] B["Mid"] C["Out"]
    A-->B
    B-->C`)
	assertContains(t, out, "In")
	assertContains(t, out, "Out")
}

// ── Treemap tests ───────────────────────────────────────────────────────────

func TestTreemapBasic(t *testing.T) {
	out := Render(`treemap-beta
    "Root"
        "Leaf A": 30
        "Leaf B": 20`)
	assertContains(t, out, "Root")
	assertContains(t, out, "Leaf A")
	assertContains(t, out, "Leaf B")
	assertContains(t, out, "┄") // dashed borders for section
}

// ── Error handling tests ────────────────────────────────────────────────────

func TestRenderEmpty(t *testing.T) {
	out := Render("")
	// Should not panic
	_ = out
}

func TestRenderGarbage(t *testing.T) {
	out := Render("this is not valid mermaid")
	// Should not panic, may produce something or be empty
	_ = out
}

func TestRenderReturnsString(t *testing.T) {
	out := Render("graph LR\n  A --> B")
	if out == "" {
		t.Error("expected non-empty output")
	}
}

// ── Option tests ────────────────────────────────────────────────────────────

func TestWithPadding(t *testing.T) {
	narrow := Render("graph LR\n  A --> B", WithPadding(2, 1))
	wide := Render("graph LR\n  A --> B", WithPadding(8, 4))
	// Wider vertical padding = taller output
	narrowLines := strings.Split(narrow, "\n")
	wideLines := strings.Split(wide, "\n")
	if len(wideLines) <= len(narrowLines) {
		t.Error("wider padding should produce taller output")
	}
}

func TestWithSharpEdges(t *testing.T) {
	out := Render("graph TD\n  A --> B --> C\n  A --> D --> C", WithSharpEdges())
	// Sharp edges: routing uses ┌└ instead of ╭╰
	assertContains(t, out, "└")
	assertNotContains(t, out, "╰")
}

// ── Diagram type detection tests ────────────────────────────────────────────

func TestDetectFlowchart(t *testing.T) {
	if dt := detectDiagramType("graph LR\n  A-->B"); dt != "flowchart" {
		t.Errorf("expected flowchart, got %s", dt)
	}
	if dt := detectDiagramType("flowchart TD\n  A-->B"); dt != "flowchart" {
		t.Errorf("expected flowchart, got %s", dt)
	}
}

func TestDetectSequence(t *testing.T) {
	if dt := detectDiagramType("sequenceDiagram\n  A->>B: hi"); dt != "sequence" {
		t.Errorf("expected sequence, got %s", dt)
	}
}

func TestDetectClass(t *testing.T) {
	if dt := detectDiagramType("classDiagram\n  A <|-- B"); dt != "class" {
		t.Errorf("expected class, got %s", dt)
	}
}

func TestDetectER(t *testing.T) {
	if dt := detectDiagramType("erDiagram\n  A ||--o{ B : has"); dt != "er" {
		t.Errorf("expected er, got %s", dt)
	}
}

func TestDetectState(t *testing.T) {
	if dt := detectDiagramType("stateDiagram-v2\n  [*] --> A"); dt != "state" {
		t.Errorf("expected state, got %s", dt)
	}
}

func TestDetectPie(t *testing.T) {
	if dt := detectDiagramType("pie\n  \"A\": 50"); dt != "pie" {
		t.Errorf("expected pie, got %s", dt)
	}
}

func TestDetectGitGraph(t *testing.T) {
	if dt := detectDiagramType("gitGraph\n  commit"); dt != "gitgraph" {
		t.Errorf("expected gitgraph, got %s", dt)
	}
}

func TestDetectTreemap(t *testing.T) {
	if dt := detectDiagramType("treemap-beta\n  \"A\": 10"); dt != "treemap" {
		t.Errorf("expected treemap, got %s", dt)
	}
}

func TestDetectJourney(t *testing.T) {
	if dt := detectDiagramType("journey\n  title Day\n  section S\n    A: 3: X"); dt != "journey" {
		t.Errorf("expected journey, got %s", dt)
	}
}

func TestDetectPacket(t *testing.T) {
	if dt := detectDiagramType("packet-beta\n  0-7: \"A\""); dt != "packet" {
		t.Errorf("expected packet, got %s", dt)
	}
	if dt := detectDiagramType("packet\n  0-7: \"A\""); dt != "packet" {
		t.Errorf("expected packet (no -beta), got %s", dt)
	}
}

func TestDetectSwimlane(t *testing.T) {
	if dt := detectDiagramType("swimlane-beta TB\n  A --> B"); dt != "swimlane" {
		t.Errorf("expected swimlane, got %s", dt)
	}
	if dt := detectDiagramType("swimlane LR\n  A --> B"); dt != "swimlane" {
		t.Errorf("expected swimlane (no -beta), got %s", dt)
	}
}

func TestRenderSwimlaneDispatch(t *testing.T) {
	src := `swimlane-beta TB
    subgraph cust["Customer"]
        A[Place order]
    end
    subgraph wh["Warehouse"]
        B[Pick items]
    end
    A --> B`
	if g := Parse(src); !g.Lanes || len(g.Subgraphs) != 2 {
		t.Errorf("Parse gave lanes=%v, %d subgraphs", g.Lanes, len(g.Subgraphs))
	}
	out := Render(src)
	for _, want := range []string{"Customer", "Warehouse", "Place order", "Pick items"} {
		assertContains(t, out, want)
	}
}

func TestRenderJourneyDispatch(t *testing.T) {
	out := Render("journey\n  title Day\n  section Work\n    Tea: 5: Me")
	if !strings.Contains(out, "Tea") || !strings.Contains(out, "Work") {
		t.Errorf("journey did not dispatch/render:\n%s", out)
	}
}

func TestRenderPacketDispatch(t *testing.T) {
	out := Render("packet-beta\n  0-15: \"Src\"\n  16-31: \"Dst\"")
	if !strings.Contains(out, "Src") || !strings.Contains(out, "Dst") {
		t.Errorf("packet did not dispatch/render:\n%s", out)
	}
}

// ── Wide character tests ─────────────────────────────────────────────────────

// columnsOf returns the display columns at which target appears in line.
func columnsOf(line string, target rune) []int {
	var cols []int
	col := 0
	for _, r := range line {
		if r == target {
			cols = append(cols, col)
		}
		col += textwidth.Rune(r)
	}
	return cols
}

// lineContaining returns the first line of output that contains substr.
func lineContaining(t *testing.T, output, substr string) string {
	t.Helper()
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, substr) {
			return line
		}
	}
	t.Fatalf("no line containing %q\n---\n%s\n---", substr, output)
	return ""
}

// columnRange returns the part of line between display columns from and to.
func columnRange(line string, from, to int) string {
	var b strings.Builder
	col := 0
	for _, r := range line {
		if col >= from && col < to {
			b.WriteRune(r)
		}
		col += textwidth.Rune(r)
	}
	return b.String()
}

// runeAtColumn returns the rune occupying the given display column of line.
func runeAtColumn(line string, want int) rune {
	col := 0
	for _, r := range line {
		if col == want {
			return r
		}
		col += textwidth.Rune(r)
	}
	return ' '
}

func TestWideLabelBoxAlignment(t *testing.T) {
	out := Render("graph LR\n A[日本語テキスト] --> B[emoji 🚀 ok]")
	assertValidUnicode(t, out)

	lines := strings.Split(out, "\n")
	if len(lines) < 3 {
		t.Fatalf("expected a box, got:\n%s", out)
	}

	want := textwidth.String(strings.TrimRight(lines[0], " "))
	for i, line := range lines {
		if got := textwidth.String(strings.TrimRight(line, " ")); got != want {
			t.Errorf("line %d display width %d, want %d\n---\n%s\n---", i, got, want, out)
		}
	}

	// Every row of A's box carries a border rune in A's right-border column.
	corners := columnsOf(lines[0], '┐')
	if len(corners) < 2 {
		t.Fatalf("expected two boxes, got corners %v\n---\n%s\n---", corners, out)
	}
	rightCol := corners[0]
	for i, line := range lines {
		r := runeAtColumn(line, rightCol)
		if !strings.ContainsRune("┐│├┤┘", r) {
			t.Errorf("line %d has %q at A's right border column %d\n---\n%s\n---", i, r, rightCol, out)
		}
	}

	// The label sits centred between A's borders.
	label := lineContaining(t, out, "日本語テキスト")
	interior := columnRange(label, 1, rightCol)
	leftPad := textwidth.String(interior) - textwidth.String(strings.TrimLeft(interior, " "))
	rightPad := textwidth.String(interior) - textwidth.String(strings.TrimRight(interior, " "))
	if leftPad-rightPad > 1 || rightPad-leftPad > 1 {
		t.Errorf("label padding %d left, %d right\n---\n%s\n---", leftPad, rightPad, out)
	}
}

func TestWideParticipantLifelineCentred(t *testing.T) {
	out := Render("sequenceDiagram\n    participant 日本語ユーザー\n    participant B\n    日本語ユーザー->>B: hello")
	assertValidUnicode(t, out)

	borders := columnsOf(lineContaining(t, out, "日本語ユーザー"), '│')
	if len(borders) < 2 {
		t.Fatalf("expected a participant box, got %v\n---\n%s\n---", borders, out)
	}
	centre := (borders[0] + borders[1]) / 2

	lifelines := columnsOf(lineContaining(t, out, "┆"), '┆')
	if len(lifelines) == 0 {
		t.Fatalf("no lifeline in:\n%s", out)
	}
	if lifelines[0] < centre-1 || lifelines[0] > centre+1 {
		t.Errorf("lifeline at column %d, box centre %d\n---\n%s\n---", lifelines[0], centre, out)
	}
}

func TestNoByteLengthLabelsInLayoutRendererAndDiagram(t *testing.T) {
	pattern := regexp.MustCompile(`len\([A-Za-z_.\[\]]*([Ll]abel|[Tt]itle|[Tt]ext|[Nn]ame)\)`)
	for _, dir := range []string{"internal/layout", "internal/renderer", "internal/diagram"} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
				continue
			}
			path := filepath.Join(dir, e.Name())
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			for i, line := range strings.Split(string(src), "\n") {
				if strings.Contains(line, "// bytes, not columns") {
					continue
				}
				if m := pattern.FindString(line); m != "" {
					t.Errorf("%s:%d measures a label by bytes: %s", path, i+1, m)
				}
			}
		}
	}
}

func TestStripFrontmatter(t *testing.T) {
	input := "---\ntitle: Test\n---\ngraph LR\n  A --> B"
	result := stripFrontmatter(input)
	if strings.Contains(result, "---") {
		t.Error("frontmatter not stripped")
	}
	if !strings.Contains(result, "graph LR") {
		t.Error("content after frontmatter missing")
	}
}

func TestBOMBeforeHeaderIsIgnored(t *testing.T) {
	out := Render("\uFEFFsankey-beta\nA,B,1\n")
	if strings.Contains(out, "A,B,1") {
		t.Fatalf("a BOM made the sankey source render as a flowchart node:\n%s", out)
	}
	if detectDiagramType(stripFrontmatter("\uFEFFpie\n\"a\" : 1\n")) != "pie" {
		t.Fatal("BOM defeats detectDiagramType")
	}
}
