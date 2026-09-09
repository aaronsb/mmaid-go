package cells

import (
	"bytes"
	"strings"
	"testing"
)

// ── Interpreter ──────────────────────────────────────────────────────────────

func TestInterpretPlainText(t *testing.T) {
	f, err := Interpret("ab\nlonger")
	if err != nil {
		t.Fatal(err)
	}
	if f.W != 6 || f.H != 2 {
		t.Fatalf("got %dx%d, want 6x2", f.W, f.H)
	}
	if got := f.At(0, 0); got != (Cell{Cp: 'a', Fg: DefaultFg, Bg: DefaultBg}) {
		t.Errorf("(0,0) = %+v", got)
	}
	if got := f.At(0, 3); got != Blank() {
		t.Errorf("padding cell = %+v", got)
	}
}

func TestInterpretNamedColorsAndAttributes(t *testing.T) {
	tests := []struct {
		name string
		ansi string
		fg   [3]uint8
		bg   [3]uint8
	}{
		{"named fg", "\033[31mx", [3]uint8{205, 0, 0}, DefaultBg},
		{"named bg", "\033[44mx", DefaultFg, [3]uint8{0, 0, 238}},
		{"bright fg", "\033[92mx", [3]uint8{0, 255, 0}, DefaultBg},
		{"bright bg", "\033[105mx", DefaultFg, [3]uint8{255, 0, 255}},
		{"bold promotes a named fg", "\033[1m\033[31mx", [3]uint8{255, 0, 0}, DefaultBg},
		{"bold leaves truecolor alone", "\033[1m\033[38;2;10;20;30mx", [3]uint8{10, 20, 30}, DefaultBg},
		{"dim scales to 60 percent", "\033[2m\033[38;2;100;200;255mx", [3]uint8{60, 120, 153}, DefaultBg},
		{"dim applies after bold", "\033[1;2;31mx", [3]uint8{153, 0, 0}, DefaultBg},
		{"22 clears bold and dim", "\033[1;2;31m\033[22mx", [3]uint8{205, 0, 0}, DefaultBg},
		{"italic is ignored", "\033[3m\033[23mx", DefaultFg, DefaultBg},
		{"39 and 49 reset", "\033[31;44m\033[39;49mx", DefaultFg, DefaultBg},
		{"0 resets all", "\033[1;31;44m\033[0mx", DefaultFg, DefaultBg},
		{"256 cube", "\033[38;5;196mx", [3]uint8{255, 0, 0}, DefaultBg},
		{"256 greyscale", "\033[48;5;232mx", DefaultFg, [3]uint8{8, 8, 8}},
		{"256 named", "\033[38;5;3mx", [3]uint8{205, 205, 0}, DefaultBg},
		{"truecolor bg", "\033[48;2;1;2;3mx", DefaultFg, [3]uint8{1, 2, 3}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f, err := Interpret(tc.ansi)
			if err != nil {
				t.Fatal(err)
			}
			got := f.At(0, 0)
			if got.Fg != tc.fg || got.Bg != tc.bg {
				t.Errorf("fg %v bg %v, want fg %v bg %v", got.Fg, got.Bg, tc.fg, tc.bg)
			}
		})
	}
}

func TestInterpretSkipsOSC(t *testing.T) {
	for _, ansi := range []string{
		"\033]8;;https://example.com\007x\033]8;;\007",
		"\033]8;;https://example.com\033\\x\033]8;;\033\\",
	} {
		f, err := Interpret(ansi)
		if err != nil {
			t.Fatal(err)
		}
		if f.W != 1 || f.At(0, 0).Cp != 'x' {
			t.Errorf("%q gave %dx%d starting %q", ansi, f.W, f.H, string(f.At(0, 0).Cp))
		}
	}
}

func TestInterpretRejectsOtherEscapes(t *testing.T) {
	_, err := Interpret("ab\033[2Jcd")
	if err == nil {
		t.Fatal("clear-screen was accepted")
	}
	if !strings.Contains(err.Error(), "byte 2") || !strings.Contains(err.Error(), `[2J`) {
		t.Errorf("error names neither the offset nor the sequence: %v", err)
	}
	if _, err := Interpret("\033Ax"); err == nil {
		t.Error("a bare escape was accepted")
	}
	if _, err := Interpret("\033[999mx"); err == nil {
		t.Error("an unknown SGR was accepted")
	}
}

// ── The .cells format ────────────────────────────────────────────────────────

func TestCellsRoundTrip(t *testing.T) {
	f := NewFrame(7, 3)
	for i := range f.Cells {
		f.Cells[i] = Cell{
			Cp: rune(0x1FB00 + i),
			Fg: [3]uint8{uint8(i), 200, 3},
			Bg: [3]uint8{9, uint8(i * 10), 250},
		}
	}
	var buf bytes.Buffer
	if err := Write(&buf, f); err != nil {
		t.Fatal(err)
	}
	if lines := strings.Count(buf.String(), "\n"); lines != 1+21 {
		t.Errorf("wrote %d lines, want 22", lines)
	}
	back, err := Read(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if back.W != 7 || back.H != 3 {
		t.Fatalf("got %dx%d, want 7x3", back.W, back.H)
	}
	for i, c := range back.Cells {
		if c != f.Cells[i] {
			t.Fatalf("cell %d: %+v, want %+v", i, c, f.Cells[i])
		}
	}
	if _, err := Read(strings.NewReader("2 2\n32 1 1 1 1 1 1\n")); err == nil {
		t.Error("a truncated file was accepted")
	}
}

// ── Comparator ───────────────────────────────────────────────────────────────

const (
	testW = 120
	testH = 40
)

// figureAt is a blank testW by testH frame with a three-cell figure at
// (20, x0).
func figureAt(x0 int) *Frame {
	f := &Frame{W: testW, H: testH, Cells: make([]Cell, testW*testH)}
	for i := range f.Cells {
		f.Cells[i] = Cell{Cp: ' ', Fg: [3]uint8{0, 0, 0}, Bg: [3]uint8{40, 60, 30}}
	}
	for i, ch := range []rune{'(', '@', ')'} {
		f.Set(20, x0+i, Cell{Cp: ch, Fg: [3]uint8{240, 214, 176}, Bg: [3]uint8{52, 74, 150}})
	}
	return f
}

func TestCompareScoresIdenticalFramesPerfectly(t *testing.T) {
	a := figureAt(50)
	r, err := Compare(a, a)
	if err != nil {
		t.Fatal(err)
	}
	if r.Identical != 100 || r.Glyph != 100 || r.Distance != 0 || len(r.Diffs) != 0 {
		t.Fatalf("%s", r.Summary())
	}
	if !(Tolerance{MinIdentical: 100, MaxDistance: 0, Strict: true}).Passes(r) {
		t.Error("an identical frame failed strict tolerance")
	}
	if _, err := Compare(a, NewFrame(testW, testH-1)); err == nil {
		t.Error("a size mismatch was accepted")
	}
}

func TestCompareScoresOneChangedCell(t *testing.T) {
	a := figureAt(50)
	b := figureAt(50)
	// Only the background of one cell moves, by 12 on one channel.
	b.Set(7, 3, Cell{Cp: ' ', Fg: [3]uint8{0, 0, 0}, Bg: [3]uint8{52, 60, 30}})
	r, err := Compare(a, b)
	if err != nil {
		t.Fatal(err)
	}
	cells := float64(testW * testH)
	if want := 4799 * 100 / cells; abs(r.Identical-want) > 1e-9 {
		t.Errorf("identical %v, want %v", r.Identical, want)
	}
	if r.Glyph != 100 {
		t.Errorf("glyph %v, want 100", r.Glyph)
	}
	if want := 12 / cells; abs(r.Distance-want) > 1e-12 {
		t.Errorf("distance %v, want %v", r.Distance, want)
	}
	if len(r.Diffs) != 1 || r.Diffs[0].Row != 7 || r.Diffs[0].Col != 3 {
		t.Fatalf("diffs %+v", r.Diffs)
	}
	def := Tolerance{MinIdentical: DefaultMinIdentical, MaxDistance: DefaultMaxDistance}
	if !def.Passes(r) {
		t.Error("one colour tweak should be within the default tolerance")
	}
	strict := def
	strict.Strict = true
	if strict.Passes(r) {
		t.Error("one colour tweak should fail strict tolerance")
	}

	// A glyph change in the same cell counts against the glyph score and
	// fails the default tolerance.
	b.Set(7, 3, Cell{Cp: '#', Fg: [3]uint8{0, 0, 0}, Bg: [3]uint8{40, 60, 30}})
	r, err = Compare(a, b)
	if err != nil {
		t.Fatal(err)
	}
	if want := 4799 * 100 / cells; abs(r.Glyph-want) > 1e-9 {
		t.Errorf("glyph %v, want %v", r.Glyph, want)
	}
	if r.Distance != 0 {
		t.Errorf("distance %v, want 0 with the colours back", r.Distance)
	}
	if def.Passes(r) {
		t.Error("a glyph change should fail the default tolerance")
	}
}

func TestCompareReportsAShiftedFigure(t *testing.T) {
	r, err := Compare(figureAt(50), figureAt(51))
	if err != nil {
		t.Fatal(err)
	}
	// Three cells of the figure and the one it vacated.
	var where [][2]int
	for _, d := range r.Diffs {
		where = append(where, [2]int{d.Col, d.Row})
	}
	want := [][2]int{{50, 20}, {51, 20}, {52, 20}, {53, 20}}
	if len(where) != len(want) {
		t.Fatalf("%d diffs, want %d", len(where), len(want))
	}
	for i := range want {
		if where[i] != want[i] {
			t.Fatalf("diff %d at %v, want %v", i, where[i], want[i])
		}
	}
	if r.Diffs[0].Expected.Cp != '(' || r.Diffs[0].Actual.Cp != ' ' {
		t.Error("the vacated cell is not reported")
	}
	if r.Diffs[1].Expected.Cp != '@' || r.Diffs[1].Actual.Cp != '(' {
		t.Error("the figure did not move right")
	}
	listing := r.Listing()
	const line = "(50, 20): expected U+0028 '(' fg(240,214,176) bg(52,74,150)  actual U+0020 ' ' fg(0,0,0) bg(40,60,30)"
	if !strings.Contains(listing, line) {
		t.Errorf("listing is missing %q:\n%s", line, listing)
	}
	if n := strings.Count(listing, "\n"); n != 4 {
		t.Errorf("listing has %d lines, want 4", n)
	}
	if !strings.HasPrefix(r.Summary(), "identical 99.917%  glyph 99.917%") {
		t.Errorf("summary %q", r.Summary())
	}
}

func TestListingTruncatesAtTen(t *testing.T) {
	a := NewFrame(20, 1)
	b := NewFrame(20, 1)
	for c := 0; c < 15; c++ {
		b.Set(0, c, Cell{Cp: '#', Fg: DefaultFg, Bg: DefaultBg})
	}
	r, err := Compare(a, b)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.Listing(), "... and 5 more") {
		t.Errorf("listing does not truncate:\n%s", r.Listing())
	}
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

// ── Lint ─────────────────────────────────────────────────────────────────────

// frameOf builds a frame from rows of text, padded to the widest row.
func frameOf(rows ...string) *Frame {
	lines := make([][]rune, len(rows))
	w := 0
	for i, r := range rows {
		lines[i] = []rune(r)
		if len(lines[i]) > w {
			w = len(lines[i])
		}
	}
	f := NewFrame(w, len(rows))
	for row, l := range lines {
		for col, r := range l {
			f.Set(row, col, Cell{Cp: r, Fg: DefaultFg, Bg: DefaultBg})
		}
	}
	return f
}

// findRule reports whether any finding cites rule n at (row, col).
func findRule(fs []Finding, rule, row, col int) bool {
	for _, f := range fs {
		if f.Rule == rule && f.Row == row && f.Col == col {
			return true
		}
	}
	return false
}

func TestLintFlagsAnArrowheadWithNoTail(t *testing.T) {
	// ├──╮►│ : the corner turns away from the arrowhead it precedes.
	f := frameOf(
		"     │",
		"├──╮►│",
		"     │",
	)
	found := Lint(f)
	if !findRule(found, 2, 1, 4) {
		t.Errorf("rule 2 did not flag the arrowhead at (1,4):\n%s", show(found))
	}
}

func TestLintFlagsACornerInAHorizontalRun(t *testing.T) {
	// ─╭─ : the left arm of the run meets a corner with no arm back.
	found := Lint(frameOf("─╭─"))
	if !findRule(found, 1, 0, 0) {
		t.Errorf("rule 1 did not flag the run at (0,0):\n%s", show(found))
	}
}

func TestLintFlagsAnArmMeetingAnArrowheadFromTheWrongSide(t *testing.T) {
	// ▼ over ┴ : the edge bleeds one cell past its arrowhead.
	f := frameOf(
		" │ ",
		" ▼ ",
		"─┴─",
	)
	found := Lint(f)
	if !findRule(found, 3, 2, 1) {
		t.Errorf("rule 3 did not flag the tee at (2,1):\n%s", show(found))
	}
}

func TestLintPassesACleanBoxWithAnEdgeAndArrow(t *testing.T) {
	f := frameOf(
		"┌───┐   ┌───┐",
		"│ A ├──►│ B │",
		"└───┘   └───┘",
	)
	if found := Lint(f); len(found) != 0 {
		t.Errorf("clean frame produced findings:\n%s", show(found))
	}
}

func TestLintTreatsASCIIArrowheadsAsTextWithoutAnArm(t *testing.T) {
	// "over" contains a 'v' with nothing above it.
	f := frameOf(
		"+---+      ",
		"|over|     ",
		"+---+      ",
	)
	for _, fd := range Lint(f) {
		if fd.Glyph == 'v' {
			t.Errorf("a 'v' in label text was read as an arrowhead: %s", fd)
		}
	}
}

func TestLintReadsASCIIArrowheadsFedByAnArm(t *testing.T) {
	f := frameOf(
		"+---+",
		"| A |",
		"+-+-+",
		"  |  ",
		"  v  ",
		"+---+",
		"| B |",
		"+---+",
	)
	if found := Lint(f); len(found) != 0 {
		t.Errorf("a fed ASCII arrowhead produced findings:\n%s", show(found))
	}
}

// An ASCII rule carries an arm only where a neighbour feeds it, so text that
// happens to contain one of these glyphs is not a dangling arm.

func TestLintReadsAnASCIIRuleInTextAsText(t *testing.T) {
	if found := Lint(frameOf("a.b")); len(found) != 0 {
		t.Errorf("a period in a label produced findings:\n%s", show(found))
	}
}

func TestLintPassesAnASCIIBox(t *testing.T) {
	f := frameOf(
		"+--+",
		"|  |",
		"+--+",
	)
	if found := Lint(f); len(found) != 0 {
		t.Errorf("an ASCII box produced findings:\n%s", show(found))
	}
}

func TestLintPassesAnASCIIEdgeOffABoxSide(t *testing.T) {
	f := frameOf(
		"+--+",
		"|  +--->",
		"+--+",
	)
	if found := Lint(f); len(found) != 0 {
		t.Errorf("an ASCII edge produced findings:\n%s", show(found))
	}
}

func show(fs []Finding) string {
	var b strings.Builder
	for _, f := range fs {
		b.WriteString("  " + f.String() + "\n")
	}
	return b.String()
}
