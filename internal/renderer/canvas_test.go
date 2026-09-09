package renderer

import (
	"strings"
	"testing"

	"github.com/aaronsb/mmaid-go/internal/glyph"
)

func TestNewCanvas(t *testing.T) {
	c := NewCanvas(10, 5)
	if c.Width != 10 || c.Height != 5 {
		t.Errorf("expected 10x5, got %dx%d", c.Width, c.Height)
	}
	// All spaces
	for row := range c.Height {
		for col := range c.Width {
			if ch := c.Get(row, col); ch != ' ' {
				t.Errorf("expected space at (%d,%d), got %c", row, col, ch)
			}
		}
	}
}

func TestPutAndGet(t *testing.T) {
	c := NewCanvas(10, 5)
	c.Put(2, 3, 'X', "")
	if ch := c.Get(2, 3); ch != 'X' {
		t.Errorf("expected X, got %c", ch)
	}
}

func TestPutSkipsSpaces(t *testing.T) {
	c := NewCanvas(10, 5)
	c.Put(0, 0, 'A', "")
	c.Put(0, 0, ' ', "")
	if ch := c.Get(0, 0); ch != 'A' {
		t.Errorf("space should not overwrite, got %c", ch)
	}
}

func TestPutOutOfBounds(t *testing.T) {
	c := NewCanvas(5, 5)
	c.Put(-1, 0, 'X', "")
	c.Put(0, -1, 'X', "")
	c.Put(5, 0, 'X', "")
	c.Put(0, 5, 'X', "")
	c.Arm(5, 5, glyph.N, glyph.Light, false, "")
	// Should not panic
}

func TestArmsMerge(t *testing.T) {
	c := NewCanvas(5, 5)
	c.Arm(2, 2, glyph.Horizontal, glyph.Light, false, "")
	c.Arm(2, 2, glyph.Vertical, glyph.Light, false, "")
	if ch := c.Get(2, 2); ch != '┼' {
		t.Errorf("expected ┼, got %c", ch)
	}
}

func TestPutBoxMerges(t *testing.T) {
	tests := []struct {
		a, b, want rune
	}{
		{'─', '┌', '┬'},
		{'│', '┌', '├'},
		{'┌', '┘', '┼'},
		{'─', '╮', '┬'},
		{'╭', '╯', '┼'},
	}
	for _, tt := range tests {
		c := NewCanvas(3, 3)
		c.PutBox(1, 1, tt.a, "")
		c.PutBox(1, 1, tt.b, "")
		if got := c.Get(1, 1); got != tt.want {
			t.Errorf("merge(%c, %c) = %c, want %c", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestPutBoxLiteral(t *testing.T) {
	c := NewCanvas(3, 3)
	c.PutBox(1, 1, '►', "")
	c.PutBox(1, 1, '─', "")
	if got := c.Get(1, 1); got != '►' {
		t.Errorf("literal must win, got %c", got)
	}
}

func TestLiteralWinsEitherOrder(t *testing.T) {
	c := NewCanvas(3, 3)
	c.Arm(1, 1, glyph.Horizontal, glyph.Light, false, "")
	c.Put(1, 1, 'X', "")
	if got := c.Get(1, 1); got != 'X' {
		t.Errorf("literal after arms: got %c", got)
	}
	c = NewCanvas(3, 3)
	c.Put(1, 1, 'X', "")
	c.Arm(1, 1, glyph.Horizontal, glyph.Light, false, "")
	if got := c.Get(1, 1); got != 'X' {
		t.Errorf("literal before arms: got %c", got)
	}
	if got := c.ToString(); got != "\n X" {
		t.Errorf("ToString = %q", got)
	}
}

func TestSegmentEndpointsInwardOnly(t *testing.T) {
	c := NewCanvas(6, 3)
	c.Segment(1, 1, 1, 4, glyph.Light, false, "")
	if got := c.ToString(); got != "\n ╶──╴" {
		t.Errorf("ToString = %q", got)
	}
	c = NewCanvas(3, 5)
	c.Segment(1, 1, 3, 1, glyph.Light, false, "")
	if got := c.ToString(); got != "\n ╷\n │\n ╵" {
		t.Errorf("ToString = %q", got)
	}
}

func TestSegmentSingleCell(t *testing.T) {
	c := NewCanvas(3, 1)
	c.Segment(0, 1, 0, 1, glyph.Light, false, "")
	if got := c.Get(0, 1); got != '─' {
		t.Errorf("single cell = %c, want ─", got)
	}
}

func TestSegmentsMakeCorners(t *testing.T) {
	c := NewCanvas(5, 4)
	c.Segment(0, 0, 0, 4, glyph.Light, false, "")
	c.Segment(3, 0, 3, 4, glyph.Light, false, "")
	c.Segment(0, 0, 3, 0, glyph.Light, false, "")
	c.Segment(0, 4, 3, 4, glyph.Light, false, "")
	want := "┌───┐\n│   │\n│   │\n└───┘"
	if got := c.ToString(); got != want {
		t.Errorf("ToString =\n%s\nwant\n%s", got, want)
	}
}

func TestRoundedAppliesToCornersOnly(t *testing.T) {
	c := NewCanvas(5, 3)
	c.Segment(1, 0, 1, 2, glyph.Light, true, "")
	c.Segment(1, 2, 2, 2, glyph.Light, true, "")
	if got := c.Get(1, 2); got != '╮' {
		t.Errorf("bend = %c, want ╮", got)
	}
	c.Segment(1, 2, 1, 4, glyph.Light, false, "")
	if got := c.Get(1, 2); got != '┬' {
		t.Errorf("three arms = %c, want ┬", got)
	}
}

func TestWeightMerge(t *testing.T) {
	c := NewCanvas(3, 3)
	c.Arm(1, 1, glyph.Horizontal, glyph.Dashed, false, "")
	if got := c.Get(1, 1); got != '┄' {
		t.Errorf("dashed = %c", got)
	}
	c.Arm(1, 1, glyph.Vertical, glyph.Light, false, "")
	if got := c.Get(1, 1); got != '┼' {
		t.Errorf("dashed yields to light: %c", got)
	}
	c.Arm(1, 1, glyph.N, glyph.Heavy, false, "")
	if got := c.Get(1, 1); got != '╋' {
		t.Errorf("heavy wins: %c", got)
	}
}

func TestFirstArmSetsStyle(t *testing.T) {
	c := NewCanvas(3, 3)
	c.Arm(1, 1, glyph.Horizontal, glyph.Light, false, "node")
	c.Arm(1, 1, glyph.S, glyph.Light, false, "edge")
	if s := c.GetStyle(1, 1); s != "node" {
		t.Errorf("style = %s, want node", s)
	}
	c.Put(1, 1, 'X', "label")
	if s := c.GetStyle(1, 1); s != "label" {
		t.Errorf("Put always sets style, got %s", s)
	}
}

func TestResolveASCII(t *testing.T) {
	c := NewCanvas(5, 4)
	c.SetCharSet(ASCII)
	c.Segment(0, 0, 0, 4, glyph.Light, true, "")
	c.Segment(3, 0, 3, 4, glyph.Light, true, "")
	c.Segment(0, 0, 3, 0, glyph.Light, true, "")
	c.Segment(0, 4, 3, 4, glyph.Light, true, "")
	c.Segment(0, 2, 3, 2, glyph.Dashed, true, "")
	want := "+-+-+\n| : |\n| : |\n+-+-+"
	if got := c.ToString(); got != want {
		t.Errorf("ToString =\n%s\nwant\n%s", got, want)
	}
}

func TestResolveIdempotent(t *testing.T) {
	c := NewCanvas(3, 1)
	c.Segment(0, 0, 0, 2, glyph.Light, false, "")
	first := c.ToString()
	c.Arm(0, 1, glyph.S, glyph.Light, false, "")
	if second := c.ToString(); second == first || second != "╶┬╴" {
		t.Errorf("second resolve = %q", second)
	}
}

func TestPutText(t *testing.T) {
	c := NewCanvas(20, 3)
	c.PutText(1, 2, "hello", "")
	for i, ch := range "hello" {
		if got := c.Get(1, 2+i); got != ch {
			t.Errorf("pos %d: expected %c, got %c", i, ch, got)
		}
	}
}

func TestDrawHorizontal(t *testing.T) {
	c := NewCanvas(10, 3)
	c.DrawHorizontal(1, 2, 7, glyph.Light, "")
	for col := 3; col <= 6; col++ {
		if ch := c.Get(1, col); ch != '─' {
			t.Errorf("col %d: expected ─, got %c", col, ch)
		}
	}
	c.DrawHorizontal(0, 4, 4, glyph.Light, "")
	if ch := c.Get(0, 4); ch != '─' {
		t.Errorf("single cell: expected ─, got %c", ch)
	}
}

func TestDrawVertical(t *testing.T) {
	c := NewCanvas(5, 10)
	c.DrawVertical(2, 1, 6, glyph.Light, "")
	for row := 2; row <= 5; row++ {
		if ch := c.Get(row, 2); ch != '│' {
			t.Errorf("row %d: expected │, got %c", row, ch)
		}
	}
}

func TestToString(t *testing.T) {
	c := NewCanvas(5, 3)
	c.Put(0, 0, 'A', "")
	c.Put(1, 1, 'B', "")
	s := c.ToString()
	lines := strings.Split(s, "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 non-empty lines, got %d", len(lines))
	}
	if lines[0] != "A" {
		t.Errorf("line 0: expected 'A', got %q", lines[0])
	}
	if lines[1] != " B" {
		t.Errorf("line 1: expected ' B', got %q", lines[1])
	}
}

func TestResize(t *testing.T) {
	c := NewCanvas(5, 5)
	c.Put(2, 2, 'X', "")
	c.Arm(3, 3, glyph.Horizontal, glyph.Light, false, "")
	c.Resize(10, 10)
	if c.Width != 10 || c.Height != 10 {
		t.Errorf("expected 10x10 after resize, got %dx%d", c.Width, c.Height)
	}
	if ch := c.Get(2, 2); ch != 'X' {
		t.Errorf("content lost after resize")
	}
	if ch := c.Get(3, 3); ch != '─' {
		t.Errorf("arms lost after resize")
	}
	c.Arm(9, 9, glyph.Vertical, glyph.Light, false, "")
	if ch := c.Get(9, 9); ch != '│' {
		t.Errorf("new cells take arms, got %c", ch)
	}
}

func TestResizeNoOp(t *testing.T) {
	c := NewCanvas(10, 10)
	c.Resize(5, 5) // smaller, should be no-op
	if c.Width != 10 || c.Height != 10 {
		t.Errorf("resize to smaller should be no-op")
	}
}

func TestFlipVertical(t *testing.T) {
	c := NewCanvas(3, 3)
	c.Put(0, 1, '▼', "")
	c.Segment(0, 0, 2, 0, glyph.Light, false, "")
	c.Segment(2, 0, 2, 2, glyph.Light, false, "")
	c.FlipVertical()
	if ch := c.Get(2, 1); ch != '▲' {
		t.Errorf("expected ▲ after flip, got %c", ch)
	}
	if ch := c.Get(0, 0); ch != '┌' {
		t.Errorf("corner after flip = %c, want ┌", ch)
	}
}

func TestFlipHorizontal(t *testing.T) {
	c := NewCanvas(5, 3)
	c.Put(1, 0, '►', "")
	c.Segment(0, 0, 0, 4, glyph.Light, true, "")
	c.Segment(0, 4, 2, 4, glyph.Light, true, "")
	c.FlipHorizontal()
	if ch := c.Get(1, 4); ch != '◄' {
		t.Errorf("expected ◄ after flip, got %c", ch)
	}
	if ch := c.Get(0, 0); ch != '╭' {
		t.Errorf("corner after flip = %c, want ╭", ch)
	}
}

func TestClearCell(t *testing.T) {
	c := NewCanvas(5, 5)
	c.Put(2, 2, 'X', "test")
	c.Arm(2, 2, glyph.Horizontal, glyph.Light, false, "")
	c.ClearCell(2, 2)
	if ch := c.Get(2, 2); ch != ' ' {
		t.Errorf("expected space after clear, got %c", ch)
	}
	if s := c.GetStyle(2, 2); s != "default" {
		t.Errorf("expected default style after clear, got %s", s)
	}
	if a := c.Arms(2, 2); a != 0 {
		t.Errorf("expected no arms after clear, got %04b", a)
	}
}

func TestStyle(t *testing.T) {
	c := NewCanvas(5, 5)
	c.Put(1, 1, 'A', "myStyle")
	if s := c.GetStyle(1, 1); s != "myStyle" {
		t.Errorf("expected myStyle, got %s", s)
	}
}

func TestPutTextWideRune(t *testing.T) {
	c := NewCanvas(10, 1)
	c.PutText(0, 0, "日a", "node")
	if ch := c.Get(0, 0); ch != '日' {
		t.Errorf("col 0 = %q, want 日", ch)
	}
	if ch := c.Get(0, 1); ch != Continuation {
		t.Errorf("col 1 = %q, want continuation", ch)
	}
	if ch := c.Get(0, 2); ch != 'a' {
		t.Errorf("col 2 = %q, want a", ch)
	}
	if got := c.ToString(); got != "日a" {
		t.Errorf("ToString = %q, want 日a", got)
	}
}

func TestPutOverContinuationClearsWideRune(t *testing.T) {
	c := NewCanvas(10, 1)
	c.PutText(0, 0, "日", "")
	c.Put(0, 1, 'x', "")
	if ch := c.Get(0, 0); ch != ' ' {
		t.Errorf("col 0 = %q, want space", ch)
	}
	if got := c.ToString(); got != " x" {
		t.Errorf("ToString = %q, want %q", got, " x")
	}
}

func TestPutOverWideRuneClearsContinuation(t *testing.T) {
	c := NewCanvas(10, 1)
	c.PutText(0, 0, "日", "")
	c.Put(0, 0, 'x', "")
	if ch := c.Get(0, 1); ch != ' ' {
		t.Errorf("col 1 = %q, want space", ch)
	}
	if got := c.ToString(); got != "x" {
		t.Errorf("ToString = %q, want x", got)
	}
}

func TestClearCellClearsBothHalves(t *testing.T) {
	for _, half := range []int{0, 1} {
		c := NewCanvas(10, 1)
		c.PutText(0, 0, "日本", "")
		c.ClearCell(0, half)
		c.Put(0, 4, 'x', "")
		if got := c.ToString(); got != "  本x" {
			t.Errorf("clearing column %d gave %q, want %q", half, got, "  本x")
		}
	}
	for _, half := range []int{2, 3} {
		c := NewCanvas(10, 1)
		c.PutText(0, 0, "日本", "")
		c.ClearCell(0, half)
		c.Put(0, 4, 'x', "")
		if got := c.ToString(); got != "日  x" {
			t.Errorf("clearing column %d gave %q, want %q", half, got, "日  x")
		}
	}
}

func TestPutTextSkipsZeroWidthRunes(t *testing.T) {
	c := NewCanvas(10, 1)
	c.Put(0, 5, '│', "")
	c.PutText(0, 0, "Café", "") // e followed by a combining acute
	if got := c.ToString(); got != "Cafe │" {
		t.Errorf("ToString = %q, want %q", got, "Cafe │")
	}

	c = NewCanvas(10, 1)
	c.PutText(0, 0, "éx", "")
	if got := c.ToString(); got != "ex" {
		t.Errorf("ToString = %q, want %q", got, "ex")
	}
}

func TestPutTextResizesForAWideRuneInTheLastColumn(t *testing.T) {
	c := NewCanvas(3, 1)
	c.PutText(0, 2, "日", "")
	if c.Width != 4 {
		t.Errorf("width %d, want 4", c.Width)
	}
	if ch := c.Get(0, 3); ch != Continuation {
		t.Errorf("col 3 = %q, want continuation", ch)
	}
	c.Put(0, 3, 'x', "")
	if got := c.ToString(); got != "   x" {
		t.Errorf("ToString = %q, want %q", got, "   x")
	}
}

func TestPutStyledTextWideRune(t *testing.T) {
	c := NewCanvas(10, 1)
	c.PutStyledText(0, 0, []StyledSegment{{Text: "日", Style: "node"}, {Text: "b", Style: "label"}})
	if ch := c.Get(0, 1); ch != Continuation {
		t.Errorf("col 1 = %q, want continuation", ch)
	}
	if s := c.GetStyle(0, 1); s != "node" {
		t.Errorf("continuation style %q, want node", s)
	}
	if ch := c.Get(0, 2); ch != 'b' {
		t.Errorf("col 2 = %q, want b", ch)
	}
	if s := c.GetStyle(0, 2); s != "label" {
		t.Errorf("second segment style %q, want label", s)
	}
}

func TestPutTextWideOverWide(t *testing.T) {
	c := NewCanvas(10, 1)
	c.PutText(0, 0, "日本", "")
	c.PutText(0, 1, "語", "")
	if got := c.ToString(); got != " 語" {
		t.Errorf("ToString = %q, want %q", got, " 語")
	}
	if ch := c.Get(0, 3); ch != ' ' {
		t.Errorf("col 3 = %q, want space", ch)
	}
}

func TestFlipHorizontalRepairsWideRunes(t *testing.T) {
	c := NewCanvas(4, 1)
	c.PutText(0, 0, "日", "")
	c.FlipHorizontal()
	if ch := c.Get(0, 2); ch != '日' {
		t.Errorf("col 2 = %q, want 日", ch)
	}
	if ch := c.Get(0, 3); ch != Continuation {
		t.Errorf("col 3 = %q, want continuation", ch)
	}
	if got := c.ToString(); got != "  日" {
		t.Errorf("ToString = %q, want %q", got, "  日")
	}
}

func TestWideRuneContinuationTakesNoArms(t *testing.T) {
	c := NewCanvas(6, 1)
	c.PutText(0, 2, "字", "")
	c.Segment(0, 0, 0, 5, glyph.Light, false, "")
	if a := c.Arms(0, 3); a != 0 {
		t.Errorf("continuation arms = %04b, want none", a)
	}
	if got := c.ToString(); got != "╶─字─╴" {
		t.Errorf("ToString = %q", got)
	}
}

func TestWideRuneOverArmsZeroesContinuation(t *testing.T) {
	c := NewCanvas(6, 1)
	c.Segment(0, 0, 0, 5, glyph.Light, false, "")
	c.PutText(0, 2, "字", "")
	if a := c.Arms(0, 3); a != 0 {
		t.Errorf("continuation arms = %04b, want none", a)
	}
	// Overwriting the wide rune clears its continuation, which stays blank.
	c.Put(0, 2, 'X', "")
	if got := c.ToString(); got != "╶─X ─╴" {
		t.Errorf("ToString = %q", got)
	}
}
