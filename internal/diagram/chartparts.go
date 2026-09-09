package diagram

import (
	"fmt"
	"os"
	"strings"

	"github.com/aaronsb/mmaid-go/internal/glyph"
	"github.com/aaronsb/mmaid-go/internal/renderer"
	"github.com/aaronsb/mmaid-go/internal/textwidth"
)

// warnf tells the user on stderr what a renderer did with input it could not
// draw as written. The canvas is the answer to the question that was asked;
// stderr is where it says the question was changed.
func warnf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "mmaid: "+format+"\n", args...)
}

// Parts the charts share: the legend box the pie, the radar and the venn all
// draw, the punctuation a chart writes between words, and the two placement
// rules — clear the cells under a label, and move a label that would land on
// something already drawn.

// drawLegendBox draws a legend's border and fills its interior. The border
// goes through Segment, so it resolves from the canvas charset and merges with
// anything else in those cells (ADR-400); a literal `┌` would survive
// `--glyphs ascii` unchanged.
func drawLegendBox(c *renderer.Canvas, top, left, height, width int, style string) {
	bottom, right := top+height-1, left+width-1
	c.Segment(top, left, top, right, glyph.Light, false, style)
	c.Segment(bottom, left, bottom, right, glyph.Light, false, style)
	c.Segment(top, left, bottom, left, glyph.Light, false, style)
	c.Segment(top, right, bottom, right, glyph.Light, false, style)
	for row := top; row <= bottom; row++ {
		for col := left; col <= right; col++ {
			c.SetFill(row, col, "subgraph_fill")
		}
	}
}

// chartMarks is the punctuation a chart writes between words. Each falls back
// to its ASCII stand-in when the glyph set has no Unicode to draw it with.
type chartMarks struct {
	join     string // between the names of two sets
	separate string // between the steps of a decision model
	dash     string // between a name and the gloss that follows it
	ellipsis string // where text was cut
}

func marksFor(cs renderer.CharSet) chartMarks {
	if cs.ASCII {
		return chartMarks{join: "n", separate: "*", dash: "-", ellipsis: "..."}
	}
	return chartMarks{join: "∩", separate: "·", dash: "—", ellipsis: "…"}
}

// truncateMark cuts text to a width, marking the cut. Text that fits is
// returned whole; a width too small for the mark alone cuts without one.
func truncateMark(s string, width int, m chartMarks) string {
	if width <= 0 {
		return ""
	}
	if textwidth.String(s) <= width {
		return s
	}
	mw := textwidth.String(m.ellipsis)
	if width <= mw {
		return textwidth.Truncate(s, width)
	}
	return strings.TrimRight(textwidth.Truncate(s, width-mw), " ") + m.ellipsis
}

// clearSpan blanks the cells a label is about to be written over. `Put` skips
// spaces, so without this a label's own spaces let the line or the fill under
// it show through and the label reads as one word.
//
// Only the span itself is cleared. The cells on either side keep their arms,
// which then meet the label's first and last glyph — text, which satisfies
// ADR-101's rule 1 — so a run of line the label interrupts stays lint-clean.
func clearSpan(c *renderer.Canvas, row, col, width int) {
	for i := range width {
		c.ClearCell(row, col+i)
	}
}

// spanFree reports whether a run of cells is empty: no glyph and no arms.
func spanFree(c *renderer.Canvas, row, col, width int) bool {
	return spanIs(c, row, col, width, func(g rune, arms glyph.Arms) bool {
		return g == ' ' && arms == 0
	})
}

// spanClearOfText reports whether a run of cells holds no text. A line may be
// written over — the label interrupting it still meets its arms with a glyph
// — but another label or a marker may not, because one of the two would be
// unreadable and neither would say which.
func spanClearOfText(c *renderer.Canvas, row, col, width int) bool {
	return spanIs(c, row, col, width, func(g rune, _ glyph.Arms) bool {
		if g == ' ' {
			return true
		}
		_, _, _, isLine := glyph.Of(g)
		return isLine
	})
}

func spanIs(c *renderer.Canvas, row, col, width int, ok func(rune, glyph.Arms) bool) bool {
	if row < 0 || row >= c.Height {
		return false
	}
	for i := range width {
		if col+i < 0 || col+i >= c.Width {
			return false
		}
		if !ok(c.Get(row, col+i), c.Arms(row, col+i)) {
			return false
		}
	}
	return true
}

// placeText writes text at the first row from `row` down to `row+slack` that
// `free` accepts, clearing the cells under it first. Where every one of those
// rows is taken it writes on `row`, cut to the run of cells `free` accepts
// there. It reports the row used.
func placeText(c *renderer.Canvas, row, col, slack int, text, style string, m chartMarks,
	free func(*renderer.Canvas, int, int, int) bool) int {
	w := textwidth.String(text)
	for r := row; r <= row+slack; r++ {
		if free(c, r, col, w) {
			clearSpan(c, r, col, w)
			c.PutText(r, col, text, style)
			return r
		}
	}
	room := 0
	for col+room < c.Width && free(c, row, col+room, 1) {
		room++
	}
	if cut := truncateMark(text, room, m); cut != "" {
		clearSpan(c, row, col, textwidth.String(cut))
		c.PutText(row, col, cut, style)
	}
	return row
}
