package renderer

import (
	"testing"

	"github.com/aaronsb/mmaid-go/internal/glyph"
)

func TestUnicodeCharSetIsTheGlyphTables(t *testing.T) {
	for _, w := range []glyph.Weight{glyph.Light, glyph.Heavy, glyph.Double, glyph.Dashed} {
		if UNICODE.Tables[w] != glyph.Unicode[w] {
			t.Errorf("weight %d differs from glyph.Unicode", w)
		}
	}
	if UNICODE.Rounded != glyph.UnicodeRounded {
		t.Error("rounded corners differ")
	}
	if UNICODE.ASCII || !UNICODE.Braille {
		t.Error("unicode should draw literals in Unicode and allow braille")
	}
	if ASCII.Tables[glyph.Light] != glyph.ASCII[glyph.Light] || !ASCII.ASCII || ASCII.Braille {
		t.Error("ascii charset is not the seven-bit set")
	}
}

func TestFailedHeavySwapsOnlyTheHeavyTable(t *testing.T) {
	cs := CharSetFor(glyph.Resolve(glyph.DefaultSet(), []glyph.Family{glyph.BoxHeavy}))
	if cs.Tables[glyph.Light] != glyph.Unicode[glyph.Light] {
		t.Error("light table changed")
	}
	if cs.Tables[glyph.Heavy] != glyph.Unicode[glyph.Light] {
		t.Errorf("heavy resolves %q, want the light table", string(cs.Tables[glyph.Heavy][glyph.Horizontal]))
	}
	if cs.Tables[glyph.Double] != glyph.Unicode[glyph.Double] || cs.Rounded != glyph.UnicodeRounded {
		t.Error("double or rounded changed")
	}
	if cs.ArrowRight != '►' || cs.Fills.Dark != '▓' {
		t.Error("arrows or fills changed")
	}
}

func TestFailedBlocksDegradesFillsToASCII(t *testing.T) {
	cs := CharSetFor(glyph.Resolve(glyph.DefaultSet(), []glyph.Family{glyph.Blocks}))
	if cs.Fills.Dark != '#' || cs.Fills.Medium != '=' || cs.Fills.Light != '-' {
		t.Errorf("fills = %q %q %q", cs.Fills.Dark, cs.Fills.Medium, cs.Fills.Light)
	}
	if !cs.Braille {
		t.Error("braille was not marked failed and stays")
	}
	both := CharSetFor(glyph.Resolve(glyph.DefaultSet(), []glyph.Family{glyph.Blocks, glyph.Braille}))
	if both.Braille {
		t.Error("braille failed with its fallback resolves past it to ascii")
	}
	if cs.Tables[glyph.Light] != glyph.Unicode[glyph.Light] || cs.ArrowRight != '►' {
		t.Error("boxes or arrows changed")
	}
}

func TestBuiltInSetsBuildDistinctTables(t *testing.T) {
	heavy := CharSetFor(glyph.Sets["heavy"])
	if heavy.Tables[glyph.Light] != glyph.Unicode[glyph.Heavy] {
		t.Error("heavy set should draw the light stroke heavy")
	}
	if heavy.Rounded[0] != '┗' {
		t.Errorf("heavy set rounded corner is %q", heavy.Rounded[0])
	}
	double := CharSetFor(glyph.Sets["double"])
	if double.Tables[glyph.Light] != glyph.Unicode[glyph.Double] {
		t.Error("double set should draw the light stroke double")
	}
	rounded := CharSetFor(glyph.Sets["rounded"])
	if rounded.Tables[glyph.Light][glyph.TopLeft] != '╭' || rounded.Tables[glyph.Light][glyph.Horizontal] != '─' {
		t.Error("rounded set should round the light table's corners only")
	}
	legacy := CharSetFor(glyph.Sets["legacy"])
	if legacy.Fills.Dark != '\U0001FB97' || legacy.Tables[glyph.Light] != glyph.Unicode[glyph.Light] {
		t.Error("legacy set should swap fills and nothing else")
	}
}

func TestGlyphSamplesCoverEveryFamily(t *testing.T) {
	samples := GlyphSamples(glyph.DefaultSet())
	if len(samples) != len(glyph.Families) {
		t.Fatalf("%d samples for %d families", len(samples), len(glyph.Families))
	}
	for i, s := range samples {
		if s.Family != glyph.Families[i] {
			t.Errorf("sample %d is %s, want %s", i, s.Family, glyph.Families[i])
		}
		if s.Text == "" || s.Reference == "" || s.Width == 0 {
			t.Errorf("%s: empty sample %+v", s.Family, s)
		}
	}
	legacy := GlyphSamples(glyph.Sets["legacy"])
	if legacy[6].Family != glyph.Blocks || legacy[6].Text != figures[glyph.LegacyFills] {
		t.Errorf("legacy set's blocks line shows %q", legacy[6].Text)
	}
	failed := GlyphSamples(glyph.Resolve(glyph.DefaultSet(), []glyph.Family{glyph.BoxHeavy}))
	if failed[2].Text != figures[glyph.BoxLight] {
		t.Errorf("failed heavy shows %q, want the light figure", failed[2].Text)
	}
}
