package tester

import (
	"io"
	"strings"
	"testing"

	"github.com/aaronsb/mmaid-go/internal/config"
	"github.com/aaronsb/mmaid-go/internal/glyph"
	"github.com/aaronsb/mmaid-go/internal/renderer"
)

func TestAskMarksTheNumberedFamilies(t *testing.T) {
	samples := renderer.GlyphSamples(glyph.DefaultSet())
	var out strings.Builder
	got, err := Ask(strings.NewReader("2 5\n"), &out, samples, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Family != glyph.BoxRounded || got[1].Family != glyph.Diagonals {
		t.Fatalf("failures = %+v", got)
	}
	for _, f := range got {
		if f.Cause != Shape {
			t.Errorf("%s: cause %s, want shape", f.Family, f.Cause)
		}
	}
	sheet := out.String()
	if !strings.Contains(sheet, " 1  box-light") || !strings.Contains(sheet, "12  ascii") {
		t.Errorf("sheet lacks numbered lines:\n%s", sheet)
	}
	if !strings.Contains(sheet, "Which lines look wrong? (numbers separated by spaces, Enter for none): ") {
		t.Errorf("prompt missing:\n%s", sheet)
	}
}

func TestAskKeepsTheProbeCauseAndIgnoresJunk(t *testing.T) {
	samples := renderer.GlyphSamples(glyph.DefaultSet())
	probed := []Failure{{Family: glyph.Octants, Cause: Advance}}
	var out strings.Builder
	got, err := Ask(strings.NewReader("10 x 99 12\n"), &out, samples, probed)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Family != glyph.Octants || got[0].Cause != Advance {
		t.Fatalf("failures = %+v", got)
	}
	if !strings.Contains(out.String(), "(advance: fails)") {
		t.Error("the probed line should say the probe failed it")
	}
	if !strings.Contains(out.String(), `ignoring "x"`) || !strings.Contains(out.String(), `ignoring "99"`) {
		t.Errorf("junk not reported:\n%s", out.String())
	}
}

func TestAskEmptyAnswerAndEOF(t *testing.T) {
	samples := renderer.GlyphSamples(glyph.DefaultSet())
	for _, in := range []string{"\n", ""} {
		got, err := Ask(strings.NewReader(in), io.Discard, samples, nil)
		if err != nil || len(got) != 0 {
			t.Errorf("input %q: failures %+v, err %v", in, got, err)
		}
	}
}

func TestAdvanceMismatch(t *testing.T) {
	if AdvanceMismatch(1, 16, 15) {
		t.Error("15 columns for width 15 is a match")
	}
	if !AdvanceMismatch(1, 31, 15) {
		t.Error("30 columns for width 15 is a mismatch")
	}
	if !AdvanceMismatch(1, 1, 4) {
		t.Error("no movement for width 4 is a mismatch")
	}
}

// fakeTerminal answers cursor position reports from a list of columns.
type fakeTerminal struct {
	cols   []int
	writes strings.Builder
	reader *strings.Reader
}

func (f *fakeTerminal) querier() *Querier {
	var replies strings.Builder
	for _, c := range f.cols {
		replies.WriteString("\x1b[1;" + itoa(c) + "R")
	}
	f.reader = strings.NewReader(replies.String())
	return &Querier{In: f.reader, Out: &f.writes}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for ; n > 0; n /= 10 {
		b = append([]byte{byte('0' + n%10)}, b...)
	}
	return string(b)
}

func TestAdvanceFailsThroughAFakeCursorReport(t *testing.T) {
	sample := renderer.Sample{Family: glyph.BoxHeavy, Text: "┏━┓", Width: 3}

	ok := &fakeTerminal{cols: []int{1, 4}}
	failed, err := AdvanceFails(ok.querier(), sample)
	if err != nil || failed {
		t.Errorf("3 columns for width 3: failed=%v err=%v", failed, err)
	}
	if !strings.Contains(ok.writes.String(), "┏━┓") || !strings.Contains(ok.writes.String(), "\x1b[6n") {
		t.Errorf("the probe wrote %q", ok.writes.String())
	}

	wide := &fakeTerminal{cols: []int{1, 7}}
	failed, err = AdvanceFails(wide.querier(), sample)
	if err != nil || !failed {
		t.Errorf("6 columns for width 3: failed=%v err=%v", failed, err)
	}

	mute := &fakeTerminal{cols: nil}
	if _, err := AdvanceFails(mute.querier(), sample); err == nil {
		t.Error("a terminal that never answers should be an error")
	}
}

func TestCursorColumnParsesTheReport(t *testing.T) {
	col, err := CursorColumn(func(string, byte) (string, error) { return "\x1b[12;34R", nil })
	if err != nil || col != 34 {
		t.Errorf("col %d err %v", col, err)
	}
	if _, err := CursorColumn(func(string, byte) (string, error) { return "garbage", nil }); err == nil {
		t.Error("garbage should not parse")
	}
}

func TestReportNamesFallbackAndFont(t *testing.T) {
	var out strings.Builder
	Report(&out, []Failure{{glyph.BoxHeavy, Advance}, {glyph.Braille, Shape}})
	s := out.String()
	for _, want := range []string{"box-heavy", "advance", "box-light", "DejaVu Sans Mono", "braille", "shape", "blocks", "Symbola or Unifont"} {
		if !strings.Contains(s, want) {
			t.Errorf("report lacks %q:\n%s", want, s)
		}
	}
	if strings.Count(s, "\n") != 2 {
		t.Errorf("want one line per failure:\n%s", s)
	}
}

func TestMergeTouchesOnlyTruecolorAndFailed(t *testing.T) {
	theme := "blueprint"
	links := true
	f := config.File{Profiles: map[string]config.Settings{
		"WezTerm": {Theme: &theme, Hyperlinks: &links, Failed: []string{"octants"}},
	}}
	existed := Merge(&f, "WezTerm", false, []Failure{{glyph.BoxHeavy, Shape}})
	if !existed {
		t.Error("the profile existed")
	}
	p := f.Profiles["WezTerm"]
	if p.Theme == nil || *p.Theme != "blueprint" || p.Hyperlinks == nil || !*p.Hyperlinks {
		t.Error("other keys changed")
	}
	if p.Truecolor == nil || *p.Truecolor || len(p.Failed) != 1 || p.Failed[0] != "box-heavy" {
		t.Errorf("profile = %+v", p)
	}
	if Merge(&f, "kitty", true, nil) {
		t.Error("kitty did not exist")
	}
	if k := f.Profiles["kitty"]; k.Failed != nil || k.Truecolor == nil || !*k.Truecolor {
		t.Errorf("new profile = %+v", k)
	}
}
