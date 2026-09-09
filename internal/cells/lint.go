package cells

import "fmt"

// Finding is one structural defect in a frame's glyph grid.
type Finding struct {
	Row, Col int
	Glyph    rune
	Rule     int
	Msg      string
}

// String renders a finding as "row:col glyph rule N: msg".
func (f Finding) String() string {
	return fmt.Sprintf("%d:%d %c rule %d: %s", f.Row, f.Col, f.Glyph, f.Rule, f.Msg)
}

// A direction and the arm bit that points that way.
type dir int

const (
	north dir = iota
	south
	east
	west
)

var dirs = [4]dir{north, south, east, west}

const (
	armN = 1 << iota
	armS
	armE
	armW
)

func (d dir) bit() int {
	return [4]int{armN, armS, armE, armW}[d]
}

func (d dir) opposite() dir {
	return [4]dir{south, north, west, east}[d]
}

func (d dir) name() string {
	return [4]string{"north", "south", "east", "west"}[d]
}

func (d dir) step(row, col int) (int, int) {
	switch d {
	case north:
		return row - 1, col
	case south:
		return row + 1, col
	case east:
		return row, col + 1
	default:
		return row, col - 1
	}
}

// boxArms maps each box-drawing glyph mmaid draws to the arms it extends.
// ASCII rules are absent: '-', '|', '+', '.' and ':' are as often text as
// they are lines, so the lint reads them as text and leaves ASCII output to
// the goldens.
var boxArms = map[rune]int{
	// Light.
	'─': armE | armW, '│': armN | armS,
	'┌': armS | armE, '┐': armS | armW, '└': armN | armE, '┘': armN | armW,
	'├': armN | armS | armE, '┤': armN | armS | armW,
	'┬': armS | armE | armW, '┴': armN | armE | armW,
	'┼': armN | armS | armE | armW,
	// Heavy.
	'━': armE | armW, '┃': armN | armS, '╋': armN | armS | armE | armW,
	// Dotted.
	'┄': armE | armW, '┆': armN | armS,
	// Rounded.
	'╭': armS | armE, '╮': armS | armW, '╰': armN | armE, '╯': armN | armW,
	// Double.
	'═': armE | armW, '║': armN | armS,
	'╔': armS | armE, '╗': armS | armW, '╚': armN | armE, '╝': armN | armW,
}

// arrowTails maps each arrowhead to the side its tail is on.
var arrowTails = map[rune]dir{'▲': south, '▼': north, '◄': east, '►': west}

// glyphAt returns the glyph at (row, col); outside the frame it is a space.
func glyphAt(f *Frame, row, col int) rune {
	if row < 0 || row >= f.H || col < 0 || col >= f.W {
		return ' '
	}
	return f.At(row, col).Cp
}

// fedFrom reports whether the neighbour on side d of (row, col) extends an arm
// back toward it.
func fedFrom(f *Frame, row, col int, d dir) bool {
	nr, nc := d.step(row, col)
	return boxArms[glyphAt(f, nr, nc)]&d.opposite().bit() != 0
}

// Lint walks the glyph grid and reports every arm that does not meet
// something, every arrowhead with no arm feeding its tail, and every arm that
// meets an arrowhead from the wrong side.
func Lint(f *Frame) []Finding {
	var out []Finding
	for row := 0; row < f.H; row++ {
		for col := 0; col < f.W; col++ {
			g := glyphAt(f, row, col)

			for _, d := range dirs {
				if boxArms[g]&d.bit() == 0 {
					continue
				}
				nr, nc := d.step(row, col)
				ng := glyphAt(f, nr, nc)
				if tail, isArrow := arrowTails[ng]; isArrow {
					if tail != d.opposite() {
						out = append(out, Finding{row, col, g, 3, fmt.Sprintf(
							"%s arm meets arrowhead %c on its %s side, not its tail",
							d.name(), ng, d.opposite().name())})
					}
					continue
				}
				if narms, isBox := boxArms[ng]; isBox {
					if narms&d.opposite().bit() == 0 {
						out = append(out, Finding{row, col, g, 1, fmt.Sprintf(
							"%s arm meets %c, which has no %s arm back",
							d.name(), ng, d.opposite().name())})
					}
					continue
				}
				if ng == ' ' {
					out = append(out, Finding{row, col, g, 1, fmt.Sprintf(
						"%s arm meets nothing", d.name())})
				}
			}

			if tail, isArrow := arrowTails[g]; isArrow && !fedFrom(f, row, col, tail) {
				out = append(out, Finding{row, col, g, 2, fmt.Sprintf(
					"no arm feeds arrowhead %c from the %s", g, tail.name())})
			}
		}
	}
	return out
}
