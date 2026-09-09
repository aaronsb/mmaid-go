// Package glyph maps the four sides a line leaves a cell through to the
// box-drawing rune that draws it, and back. The renderer resolves cells from
// these tables and the lint reads glyphs through their inverse, so the two
// agree by construction.
package glyph

// Arms is a set of the four sides a line leaves a cell through.
type Arms uint8

const (
	N Arms = 1 << iota
	E
	S
	W
)

// Named arm patterns.
const (
	Horizontal  = E | W
	Vertical    = N | S
	TopLeft     = S | E
	TopRight    = S | W
	BottomLeft  = N | E
	BottomRight = N | W
	TeeRight    = N | S | E
	TeeLeft     = N | S | W
	TeeDown     = S | E | W
	TeeUp       = N | E | W
	Cross       = N | E | S | W
)

// Weight is the stroke of a line.
type Weight uint8

const (
	Light Weight = iota
	Heavy
	Double
	Dashed
)

// Heavier reports whether a is drawn over b when both meet in one cell.
// Dashed yields to any solid stroke.
func Heavier(a, b Weight) bool {
	rank := func(w Weight) int {
		switch w {
		case Dashed:
			return 0
		case Light:
			return 1
		case Heavy:
			return 2
		default:
			return 3
		}
	}
	return rank(a) > rank(b)
}

// Table holds one rune per arm pattern; index 0 is unused.
type Table [16]rune

// Unicode holds a table per weight. Single-arm cells resolve to half lines in
// light and heavy and to the full line in double and dashed, which have none.
// Dashed has no corners or tees of its own and borrows light's.
var Unicode = map[Weight]Table{
	Light: {
		' ', '╵', '╶', '└', '╷', '│', '┌', '├',
		'╴', '┘', '─', '┴', '┐', '┤', '┬', '┼',
	},
	Heavy: {
		' ', '╹', '╺', '┗', '╻', '┃', '┏', '┣',
		'╸', '┛', '━', '┻', '┓', '┫', '┳', '╋',
	},
	Double: {
		' ', '║', '═', '╚', '║', '║', '╔', '╠',
		'═', '╝', '═', '╩', '╗', '╣', '╦', '╬',
	},
	Dashed: {
		' ', '┆', '┄', '└', '┆', '┆', '┌', '├',
		'┄', '┘', '┄', '┴', '┐', '┤', '┬', '┼',
	},
}

// UnicodeRounded overrides the four light corners, indexed by RoundedIndex.
var UnicodeRounded = [4]rune{'╰', '╯', '╭', '╮'}

// ASCII keeps the stroke distinction on straight runs and draws every
// junction as '+'.
var ASCII = map[Weight]Table{
	Light:  asciiTable('-', '|'),
	Heavy:  asciiTable('=', '|'),
	Double: asciiTable('=', '|'),
	Dashed: asciiTable('.', ':'),
}

// ASCIIRounded is '+' at every corner.
var ASCIIRounded = [4]rune{'+', '+', '+', '+'}

func asciiTable(h, v rune) Table {
	var t Table
	t[0] = ' '
	for a := Arms(1); a < 16; a++ {
		switch a {
		case E, W, Horizontal:
			t[a] = h
		case N, S, Vertical:
			t[a] = v
		default:
			t[a] = '+'
		}
	}
	return t
}

// RoundedIndex returns the index into a rounded override for a two-arm
// corner, in the order N|E, N|W, S|E, S|W, and -1 for any other pattern.
func RoundedIndex(a Arms) int {
	switch a {
	case BottomLeft:
		return 0
	case BottomRight:
		return 1
	case TopLeft:
		return 2
	case TopRight:
		return 3
	}
	return -1
}

// Tails maps each arrowhead to the arm bit on its tail side. The hollow forms
// are the class and sequence diagrams' relationship arrows.
var Tails = map[rune]Arms{
	'▲': S, '▼': N, '◄': E, '►': W,
	'△': S, '▽': N, '◁': E, '▷': W,
}

// ASCIITails is the ASCII arrowheads. They are kept apart from Tails because
// '>', '<', '^' and 'v' are as often text as arrows, and the lint reads
// Unicode only.
var ASCIITails = map[rune]Arms{
	'^': S, 'v': N, '<': E, '>': W,
}

// Tail returns the arm bit on the tail side of a Unicode arrowhead.
func Tail(r rune) (Arms, bool) {
	a, ok := Tails[r]
	return a, ok
}

type entry struct {
	arms    Arms
	weight  Weight
	rounded bool
}

// inverse is built from the Unicode tables; the first entry to claim a rune
// keeps it, so light owns the corners dashed borrows, and a full line that
// also stands in for a single arm inverts to both arms.
var inverse = func() map[rune]entry {
	m := make(map[rune]entry, 64)
	for _, w := range []Weight{Light, Heavy, Double, Dashed} {
		t := Unicode[w]
		for a := Arms(15); a >= 1; a-- {
			if _, seen := m[t[a]]; !seen {
				m[t[a]] = entry{a, w, false}
			}
		}
	}
	for i, r := range UnicodeRounded {
		m[r] = entry{roundedArms[i], Light, true}
	}
	return m
}()

var roundedArms = [4]Arms{BottomLeft, BottomRight, TopLeft, TopRight}

// Of is the inverse of the Unicode tables: the arms, weight, and rounded
// flag a rune draws. ok is false for any rune outside them.
func Of(r rune) (arms Arms, w Weight, rounded bool, ok bool) {
	e, ok := inverse[r]
	return e.arms, e.weight, e.rounded, ok
}
