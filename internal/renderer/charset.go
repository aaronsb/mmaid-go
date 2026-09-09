package renderer

import "github.com/aaronsb/mmaid-go/internal/glyph"

// Fills holds the runes the chart renderers fill cells with.
type Fills struct {
	// Full, Upper and Lower draw the pie's colour circle half a cell at a
	// time.
	Full, Upper, Lower rune
	// Dark, Medium and Light draw bars and shaded regions.
	Dark, Medium, Light rune
	// Bars is the gradient the pie's bar chart cycles through.
	Bars []rune
}

// CharSet holds the tables the line layer resolves through and the runes
// drawn as literals: arrowheads, endpoint markers, shape indicators, chamfers
// and fills. CharSetFor builds one from a glyph set.
type CharSet struct {
	Tables  map[glyph.Weight]glyph.Table
	Rounded [4]rune // indexed by glyph.RoundedIndex

	ArrowRight rune
	ArrowLeft  rune
	ArrowDown  rune
	ArrowUp    rune

	CircleEndpoint rune
	CrossEndpoint  rune

	// Markers: the state diagram's start and end, the circle shape's rim,
	// and the circle and double-circle indicators.
	Dot        rune
	Bullseye   rune
	Ring       rune
	DoubleRing rune

	// Chamfers and the indicators of the shapes they cut.
	Slash         rune
	Backslash     rune
	DiagonalCross rune
	Diamond       rune
	Hexagon       rune

	Fills Fills

	// Braille says the pie may draw its dot-pattern circle.
	Braille bool

	// ASCII says every literal outside the families is drawn in ASCII too.
	ASCII bool
}

// Glyph returns the rune for an armed cell. Rounded corners exist only in
// the light stroke, which dashed borrows.
func (cs CharSet) Glyph(a glyph.Arms, w glyph.Weight, rounded bool) rune {
	if rounded && (w == glyph.Light || w == glyph.Dashed) {
		if i := glyph.RoundedIndex(a); i >= 0 {
			return cs.Rounded[i]
		}
	}
	return cs.Tables[w][a]
}

// Rune returns the sharp glyph for an arm pattern at a weight.
func (cs CharSet) Rune(a glyph.Arms, w glyph.Weight) rune {
	return cs.Tables[w][a]
}

// Round returns the rounded light glyph for a corner pattern.
func (cs CharSet) Round(a glyph.Arms) rune {
	return cs.Glyph(a, glyph.Light, true)
}

// roundedTable is the light table with its four corners rounded.
var roundedTable = func() glyph.Table {
	t := glyph.Unicode[glyph.Light]
	for i, a := range []glyph.Arms{glyph.BottomLeft, glyph.BottomRight, glyph.TopLeft, glyph.TopRight} {
		t[a] = glyph.UnicodeRounded[i]
	}
	return t
}()

// boxTable returns the table a box family draws a stroke with.
func boxTable(f glyph.Family, w glyph.Weight) glyph.Table {
	switch f {
	case glyph.ASCIIFamily:
		return glyph.ASCII[w]
	case glyph.BoxRounded:
		return roundedTable
	case glyph.BoxHeavy:
		return glyph.Unicode[glyph.Heavy]
	case glyph.BoxDouble:
		return glyph.Unicode[glyph.Double]
	}
	if w == glyph.Dashed {
		return glyph.Unicode[glyph.Dashed]
	}
	return glyph.Unicode[glyph.Light]
}

// cornersOf returns a family's four corners in glyph.RoundedIndex order.
func cornersOf(f glyph.Family) [4]rune {
	switch f {
	case glyph.BoxRounded:
		return glyph.UnicodeRounded
	case glyph.ASCIIFamily:
		return glyph.ASCIIRounded
	}
	t := boxTable(f, glyph.Light)
	return [4]rune{t[glyph.BottomLeft], t[glyph.BottomRight], t[glyph.TopLeft], t[glyph.TopRight]}
}

// Fill tables by family.
var (
	blockFills = Fills{
		Full: '█', Upper: '▀', Lower: '▄',
		Dark: '▓', Medium: '▒', Light: '░',
		Bars: []rune{'█', '▓', '░', '▒', '▞', '▚', '▖', '▗'},
	}
	// Symbols for Legacy Computing: the half-cell circle is drawn in medium
	// shade, so it dithers, and bars use the PETSCII fills.
	legacyFills = Fills{
		Full: '\U0001FB90', Upper: '\U0001FB8E', Lower: '\U0001FB8F',
		Dark: '\U0001FB97', Medium: '\U0001FB95', Light: '\U0001FB98',
		Bars: []rune{'\U0001FB97', '\U0001FB95', '\U0001FB98', '\U0001FB99', '\U0001FB96', '\U0001FB90', '\U0001FB8C', '\U0001FB8D'},
	}
	asciiFills = Fills{
		Full: '#', Upper: '"', Lower: '_',
		Dark: '#', Medium: '=', Light: '-',
		Bars: []rune{'#', '*', '+', '~', ':', '.', 'o', '='},
	}
)

func fillsOf(f glyph.Family) Fills {
	switch f {
	case glyph.LegacyFills:
		return legacyFills
	case glyph.ASCIIFamily:
		return asciiFills
	}
	return blockFills
}

// CharSetFor builds the charset the canvas consumes from a resolved set.
func CharSetFor(set glyph.Set) CharSet {
	cs := CharSet{
		Tables: map[glyph.Weight]glyph.Table{
			glyph.Light:  boxTable(set.Light, glyph.Light),
			glyph.Heavy:  boxTable(set.Heavy, glyph.Heavy),
			glyph.Double: boxTable(set.Double, glyph.Double),
			glyph.Dashed: boxTable(set.Dashed, glyph.Dashed),
		},
		Rounded: cornersOf(set.Corners),
		Fills:   fillsOf(set.Fills),
		Braille: set.Dots == glyph.Braille,
		ASCII:   set.Light == glyph.ASCIIFamily,
	}
	if set.Arrows == glyph.ASCIIFamily {
		cs.ArrowRight, cs.ArrowLeft, cs.ArrowDown, cs.ArrowUp = '>', '<', 'v', '^'
		cs.CircleEndpoint, cs.CrossEndpoint = 'o', 'x'
		cs.Dot, cs.Bullseye, cs.Ring, cs.DoubleRing = '*', '@', 'O', '@'
	} else {
		cs.ArrowRight, cs.ArrowLeft, cs.ArrowDown, cs.ArrowUp = '►', '◄', '▼', '▲'
		cs.CircleEndpoint, cs.CrossEndpoint = '○', '×'
		cs.Dot, cs.Bullseye, cs.Ring, cs.DoubleRing = '●', '◉', '◯', '◎'
	}
	if set.Chamfers == glyph.ASCIIFamily {
		cs.Slash, cs.Backslash, cs.DiagonalCross = '/', '\\', 'X'
		cs.Diamond, cs.Hexagon = '<', '{'
	} else {
		cs.Slash, cs.Backslash, cs.DiagonalCross = '╱', '╲', '╳'
		cs.Diamond, cs.Hexagon = '◇', '⎔'
	}
	return cs
}

// UNICODE is the default character set, the unicode set with nothing failed.
var UNICODE = CharSetFor(glyph.DefaultSet())

// ASCII is the seven-bit character set.
var ASCII = CharSetFor(glyph.Sets["ascii"])
