package cells

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// listedDiffs is how many differing cells a listing prints.
const listedDiffs = 10

// Default tolerance thresholds.
const (
	DefaultMinIdentical = 98.0
	DefaultMaxDistance  = 2.0
)

// Diff is one cell that differs between the reference and the rendered frame.
type Diff struct {
	Row, Col int
	Expected Cell
	Actual   Cell
}

// Report says how close a rendered frame is to its reference.
type Report struct {
	// Identical is the percentage of cells with the same glyph and colours.
	Identical float64
	// Glyph is the percentage of cells with the same glyph.
	Glyph float64
	// Distance is the mean six-channel colour distance over all cells.
	Distance float64
	// Diffs lists every differing cell in row-major order.
	Diffs []Diff
}

// Compare matches two frames of the same size cell by cell.
func Compare(expected, actual *Frame) (Report, error) {
	if expected.W != actual.W || expected.H != actual.H {
		return Report{}, fmt.Errorf("reference is %dx%d but the frame is %dx%d",
			expected.W, expected.H, actual.W, actual.H)
	}
	n := len(expected.Cells)
	identical, glyph, distance := 0, 0, uint64(0)
	var diffs []Diff
	for i, e := range expected.Cells {
		a := actual.Cells[i]
		if e == a {
			identical++
			glyph++
			continue
		}
		if e.Cp == a.Cp {
			glyph++
		}
		distance += uint64(channelDistance(e.Fg, a.Fg) + channelDistance(e.Bg, a.Bg))
		diffs = append(diffs, Diff{Row: i / expected.W, Col: i % expected.W, Expected: e, Actual: a})
	}
	pct := func(k int) float64 {
		if n == 0 {
			return 100
		}
		return float64(k) * 100 / float64(n)
	}
	mean := 0.0
	if n != 0 {
		mean = float64(distance) / float64(n)
	}
	return Report{Identical: pct(identical), Glyph: pct(glyph), Distance: mean, Diffs: diffs}, nil
}

func channelDistance(a, b [3]uint8) int {
	d := 0
	for i := range a {
		if a[i] > b[i] {
			d += int(a[i] - b[i])
		} else {
			d += int(b[i] - a[i])
		}
	}
	return d
}

// Summary is the one-line score of a comparison.
func (r Report) Summary() string {
	return fmt.Sprintf("identical %.3f%%  glyph %.3f%%  distance %.4f  (%d cells differ)",
		r.Identical, r.Glyph, r.Distance, len(r.Diffs))
}

// Listing prints the first differing cells, one per line.
func (r Report) Listing() string {
	var b strings.Builder
	for i, d := range r.Diffs {
		if i == listedDiffs {
			fmt.Fprintf(&b, "      ... and %d more\n", len(r.Diffs)-listedDiffs)
			break
		}
		fmt.Fprintf(&b, "      (%d, %d): expected %s  actual %s\n",
			d.Col, d.Row, showCell(d.Expected), showCell(d.Actual))
	}
	return b.String()
}

func showCell(c Cell) string {
	glyph := "'" + string(c.Cp) + "'"
	if c.Cp == ' ' {
		glyph = "' '"
	}
	return fmt.Sprintf("U+%04X %s fg(%d,%d,%d) bg(%d,%d,%d)",
		c.Cp, glyph, c.Fg[0], c.Fg[1], c.Fg[2], c.Bg[0], c.Bg[1], c.Bg[2])
}

// Tolerance is how far a frame may drift from its reference.
type Tolerance struct {
	MinIdentical float64
	MaxDistance  float64
	Strict       bool
}

// FromEnv reads GOLDEN_MIN_IDENTICAL, GOLDEN_MAX_DISTANCE, and GOLDEN_STRICT.
func FromEnv() Tolerance {
	num := func(key string, def float64) float64 {
		if v, err := strconv.ParseFloat(os.Getenv(key), 64); err == nil {
			return v
		}
		return def
	}
	strict := os.Getenv("GOLDEN_STRICT")
	return Tolerance{
		MinIdentical: num("GOLDEN_MIN_IDENTICAL", DefaultMinIdentical),
		MaxDistance:  num("GOLDEN_MAX_DISTANCE", DefaultMaxDistance),
		Strict:       strict != "" && strict != "0",
	}
}

// Passes reports whether a report clears the tolerance. Glyphs must all match
// even outside strict mode.
func (t Tolerance) Passes(r Report) bool {
	if t.Strict {
		return len(r.Diffs) == 0
	}
	return r.Glyph == 100 && r.Identical >= t.MinIdentical && r.Distance <= t.MaxDistance
}

// Describe states the tolerance in words.
func (t Tolerance) Describe() string {
	if t.Strict {
		return "strict: every cell identical"
	}
	return fmt.Sprintf("every glyph, identical >= %.1f%% and distance <= %.2f",
		t.MinIdentical, t.MaxDistance)
}
