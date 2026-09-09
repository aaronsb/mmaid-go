package renderer

import (
	"fmt"
	"strings"

	"github.com/aaronsb/mmaid-go/internal/glyph"
	"github.com/aaronsb/mmaid-go/internal/textwidth"
)

// Sample is one line of the glyph sheet: a family, a short figure drawn in
// the family the set binds to its role, and the same figure in the family
// the tester's reference column shows.
type Sample struct {
	Family    glyph.Family
	Text      string
	Reference string
	Width     int // display width of Text under the current ambiguous-width setting
	Narrow    int // display width with ambiguous runes one column wide
	Wide      int // display width with ambiguous runes two columns wide
}

// figures holds one figure per family. The box families draw one figure,
// the fill families another, so a role rebound to a family of the same kind
// shows the same shape in the other family's runes.
var figures = map[glyph.Family]string{
	glyph.BoxLight:    "┌─┬─┐├─┼─┤└─┴─┘",
	glyph.BoxRounded:  "╭─┬─╮├─┼─┤╰─┴─╯",
	glyph.BoxHeavy:    "┏━┳━┓┣━╋━┫┗━┻━┛",
	glyph.BoxDouble:   "╔═╦═╗╠═╬═╣╚═╩═╝",
	glyph.Diagonals:   "╱╲╳ ◇⎔",
	glyph.Arrows:      "►◄▲▼ ○●◉×",
	glyph.Blocks:      "█▀▄▓▒░",
	glyph.Braille:     "⣀⣤⣶⣿",
	glyph.Sextants:    "\U0001FB02\U0001FB0E\U0001FB39\U0001FB2D",
	glyph.Octants:     "\U0001CD00\U0001CD03\U0001CD09\U0001CD18\U0001CDE5",
	glyph.LegacyFills: "\U0001FB90\U0001FB8E\U0001FB8F\U0001FB97\U0001FB95\U0001FB98",
	glyph.ASCIIFamily: "+-+|+ ><^v #=-",
}

// asciiFigures draws each family's figure in the seven-bit set.
var asciiFigures = map[glyph.Family]string{
	glyph.BoxLight:    "+-+-++-+-++-+-+",
	glyph.BoxRounded:  "+-+-++-+-++-+-+",
	glyph.BoxHeavy:    "+=+=++=+=++=+=+",
	glyph.BoxDouble:   "+=+=++=+=++=+=+",
	glyph.Diagonals:   "/\\X <{",
	glyph.Arrows:      "><^v o*@x",
	glyph.Blocks:      "#\"_#=-",
	glyph.Braille:     "_=+#",
	glyph.Sextants:    "-=#_",
	glyph.Octants:     "..,,#",
	glyph.LegacyFills: "#\"_#=-",
	glyph.ASCIIFamily: "+-+|+ ><^v #=-",
}

// figure draws role's figure in the runes of bound.
func figure(role, bound glyph.Family) string {
	if bound == glyph.ASCIIFamily {
		return asciiFigures[role]
	}
	return figures[bound]
}

// GlyphSamples returns one sample per family in ADR order. Text is the
// figure as the set draws it, so a failed family shows its fallback.
// Reference is the figure in box-light for the box families that fall back
// to it and in ASCII for every other.
func GlyphSamples(set glyph.Set) []Sample {
	out := make([]Sample, 0, len(glyph.Families))
	for _, f := range glyph.Families {
		text := figure(f, set.Binding(f))
		ref := asciiFigures[f]
		if glyph.Fallback[f] == glyph.BoxLight {
			ref = figures[glyph.BoxLight]
		}
		out = append(out, Sample{
			Family:    f,
			Text:      text,
			Reference: ref,
			Width:     textwidth.String(text),
			Narrow:    textwidth.StringWith(text, false),
			Wide:      textwidth.StringWith(text, true),
		})
	}
	return out
}

// FormatSamples lays the sheet out as `N  family  sample  reference`, one
// line per sample, the columns padded by display width.
func FormatSamples(samples []Sample) string {
	nameW, textW := 0, 0
	for _, s := range samples {
		nameW = max(nameW, len(s.Family))
		textW = max(textW, s.Width)
	}
	var b strings.Builder
	for i, s := range samples {
		fmt.Fprintf(&b, "%2d  %-*s  %s%s  %s\n",
			i+1, nameW, s.Family, s.Text, strings.Repeat(" ", textW-s.Width), s.Reference)
	}
	return strings.TrimRight(b.String(), "\n")
}
