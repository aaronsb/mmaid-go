package diagram

import (
	"strings"
	"testing"

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
	c := RenderSequence("sequenceDiagram\n  Alice->>Bob: Hello\n  Bob-->>Alice: Hi", false)
	assertCanvasContains(t, c, "Alice")
	assertCanvasContains(t, c, "Bob")
	assertCanvasContains(t, c, "Hello")
}

// ── Class Diagram ───────────────────────────────────────────────────────────

func TestClassDiagramBasic(t *testing.T) {
	c := RenderClassDiagram("classDiagram\n  class Animal {\n    +int age\n    +makeSound()\n  }", false)
	assertCanvasContains(t, c, "Animal")
	assertCanvasContains(t, c, "+int age")
}

// ── ER Diagram ──────────────────────────────────────────────────────────────

func TestERDiagramBasic(t *testing.T) {
	c := RenderERDiagram("erDiagram\n  CUSTOMER ||--o{ ORDER : places", false)
	assertCanvasContains(t, c, "CUSTOMER")
	assertCanvasContains(t, c, "ORDER")
}

// ── Pie Chart ───────────────────────────────────────────────────────────────

func TestPieChartCircle(t *testing.T) {
	c := RenderPieChart("pie\n  \"A\" : 60\n  \"B\" : 40", false, true, nil)
	assertCanvasContains(t, c, "A")
	assertCanvasContains(t, c, "B")
	assertCanvasNotEmpty(t, c)
}

func TestPieChartBraille(t *testing.T) {
	c := RenderPieChart("pie\n  \"X\" : 70\n  \"Y\" : 30", false, false, nil)
	assertCanvasContains(t, c, "X")
	assertCanvasContains(t, c, "⣿") // braille solid pattern
}

func TestPieChartASCII(t *testing.T) {
	c := RenderPieChart("pie\n  \"Go\" : 50\n  \"Rust\" : 50", true, false, nil)
	assertCanvasContains(t, c, "Go")
	assertCanvasContains(t, c, "#") // ASCII fill char
}

func TestPieChartMonochromatic(t *testing.T) {
	theme := renderer.GetTheme("amber")
	c := RenderPieChart("pie\n  \"A\" : 60\n  \"B\" : 40", false, true, &theme)
	assertCanvasNotEmpty(t, c)
}

// ── Git Graph ───────────────────────────────────────────────────────────────

func TestGitGraphBasic(t *testing.T) {
	c := RenderGitGraph("gitGraph\n  commit id: \"A\"\n  commit id: \"B\"", false)
	assertCanvasContains(t, c, "A")
	assertCanvasContains(t, c, "B")
	assertCanvasContains(t, c, "●")
}

// ── Block Diagram ───────────────────────────────────────────────────────────

func TestBlockDiagramBasic(t *testing.T) {
	c := RenderBlockDiagram("block-beta\n  columns 2\n  A[\"Hello\"] B[\"World\"]", false)
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
	c := RenderGantt("gantt\n  title Test\n  dateFormat YYYY-MM-DD\n  section S1\n    Task1 :a1, 2024-01-01, 7d", false, nil)
	assertCanvasContains(t, c, "Test")
	assertCanvasContains(t, c, "Task1")
}

func TestGanttNoTasks(t *testing.T) {
	c := RenderGantt("gantt\n  title Empty", false, nil)
	assertCanvasContains(t, c, "no tasks")
}

// ── Timeline ────────────────────────────────────────────────────────────────

func TestTimelineBasic(t *testing.T) {
	c := RenderTimeline("timeline\n  title History\n  2020 : Event A\n  2021 : Event B", false, nil)
	assertCanvasContains(t, c, "History")
	assertCanvasContains(t, c, "Event A")
	assertCanvasContains(t, c, "●")
}

func TestTimelineVerticalLayout(t *testing.T) {
	// Directly test vertical layout path with many events
	td := parseTimeline("timeline\n  title Computing\n  1940 : ENIAC\n  1950 : UNIVAC\n  1960 : Mainframes\n  1970 : Minicomputers : UNIX\n  1980 : PCs\n  1990 : Web\n  2000 : Cloud\n  2010 : Mobile\n  2020 : AI")
	c := renderTimelineVertical(td, false, nil)
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
	c := renderTimelineVertical(td, true, nil)
	assertCanvasNotEmpty(t, c)
	assertCanvasContains(t, c, "Alpha")
	assertCanvasContains(t, c, "|")
	assertCanvasContains(t, c, "o")
}

// ── Kanban ──────────────────────────────────────────────────────────────────

func TestKanbanBasic(t *testing.T) {
	c := RenderKanban("kanban\n  col1[Todo]\n    t1[Task A]\n  col2[Done]\n    t2[Task B]", false, nil)
	assertCanvasContains(t, c, "Todo")
	assertCanvasContains(t, c, "Task A")
	assertCanvasContains(t, c, "Done")
}

func TestKanbanThemed(t *testing.T) {
	theme := renderer.GetTheme("blueprint")
	c := RenderKanban("kanban\n  col1[A]\n    t1[X]\n  col2[B]\n    t2[Y]", false, &theme)
	assertCanvasNotEmpty(t, c)
}

// ── Mindmap ─────────────────────────────────────────────────────────────────

func TestMindmapBasic(t *testing.T) {
	c := RenderMindmap("mindmap\n  root((Root))\n    Child1\n    Child2", false)
	assertCanvasContains(t, c, "Root")
	assertCanvasContains(t, c, "Child1")
	assertCanvasContains(t, c, "Child2")
}

// ── Quadrant ────────────────────────────────────────────────────────────────

func TestQuadrantBasic(t *testing.T) {
	c := RenderQuadrantChart("quadrantChart\n  title Test\n  x-axis A --> B\n  y-axis C --> D\n  Point1: [0.5, 0.5]", false, nil)
	assertCanvasContains(t, c, "Test")
	assertCanvasContains(t, c, "Point1")
	assertCanvasContains(t, c, "●")
}

// ── XY Chart ────────────────────────────────────────────────────────────────

func TestXYChartBasic(t *testing.T) {
	c := RenderXYChart("xychart-beta\n  title Rev\n  x-axis [a, b]\n  bar [10, 20]", false, nil)
	assertCanvasContains(t, c, "Rev")
	assertCanvasContains(t, c, "▓") // bar fill char
}

// ── Treemap ─────────────────────────────────────────────────────────────────

func TestTreemapBasic(t *testing.T) {
	c := RenderTreemap("treemap-beta\n  \"Section\"\n    \"Item\": 100", false, nil)
	assertCanvasContains(t, c, "Section")
	assertCanvasContains(t, c, "Item")
	assertCanvasContains(t, c, "100")
}
