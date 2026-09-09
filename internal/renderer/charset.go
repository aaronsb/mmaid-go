package renderer

import "github.com/aaronsb/mmaid-go/internal/glyph"

// CharSet holds the tables the line layer resolves through and the runes
// drawn as literals: arrowheads and endpoint markers.
type CharSet struct {
	Tables  map[glyph.Weight]glyph.Table
	Rounded [4]rune // indexed by glyph.RoundedIndex

	ArrowRight rune
	ArrowLeft  rune
	ArrowDown  rune
	ArrowUp    rune

	CircleEndpoint rune
	CrossEndpoint  rune
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

// UNICODE is the default character set using Unicode box-drawing characters.
var UNICODE = CharSet{
	Tables:     glyph.Unicode,
	Rounded:    glyph.UnicodeRounded,
	ArrowRight: '►', ArrowLeft: '◄', ArrowDown: '▼', ArrowUp: '▲',
	CircleEndpoint: '○', CrossEndpoint: '×',
}

// ASCII is a fallback character set using only ASCII characters.
var ASCII = CharSet{
	Tables:     glyph.ASCII,
	Rounded:    glyph.ASCIIRounded,
	ArrowRight: '>', ArrowLeft: '<', ArrowDown: 'v', ArrowUp: '^',
	CircleEndpoint: 'o', CrossEndpoint: 'x',
}
