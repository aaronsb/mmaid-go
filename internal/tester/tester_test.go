package tester

import (
	"bufio"
	"io"
	"strconv"
	"strings"
	"testing"

	"github.com/aaronsb/mmaid-go/internal/config"
	"github.com/aaronsb/mmaid-go/internal/glyph"
	"github.com/aaronsb/mmaid-go/internal/renderer"
)

func reader(s string) *bufio.Reader { return bufio.NewReader(strings.NewReader(s)) }

func TestSamplesAreTheDefaultSetWhateverTheConfigurationNames(t *testing.T) {
	got := Samples()
	want := renderer.GlyphSamples(glyph.DefaultSet())
	legacy := renderer.GlyphSamples(glyph.Sets["legacy"])
	if len(got) != len(want) {
		t.Fatalf("%d samples, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("line %d: %+v, want %+v", i+1, got[i], want[i])
		}
	}
	if got[6].Family != glyph.Blocks || got[6].Text == legacy[6].Text {
		t.Errorf("the blocks line must show block runes, not the legacy set's %q", got[6].Text)
	}
}

func TestAskMarksTheNumberedFamilies(t *testing.T) {
	samples := Samples()
	var out strings.Builder
	got, err := Ask(reader("2 5\n"), &out, samples, nil)
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
	samples := Samples()
	probed := []Failure{{Family: glyph.Octants, Cause: Advance}}
	var out strings.Builder
	got, err := Ask(reader("10 x 99 12\n"), &out, samples, probed)
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
	samples := Samples()
	for _, in := range []string{"\n", ""} {
		got, err := Ask(reader(in), io.Discard, samples, nil)
		if err != nil || len(got) != 0 {
			t.Errorf("input %q: failures %+v, err %v", in, got, err)
		}
	}
}

func TestOneReaderServesTheAskAndTheReplacePrompt(t *testing.T) {
	// printf '\ny\n' | mmaid config init: an empty answer, then yes to the
	// replace prompt, both through the same reader.
	in := reader("\ny\n")
	got, err := Ask(in, io.Discard, Samples(), nil)
	if err != nil || len(got) != 0 {
		t.Fatalf("failures %+v, err %v", got, err)
	}
	if !Confirm(in, io.Discard, "Replace?") {
		t.Error("the y on the second line was lost")
	}
	if Confirm(reader("no\n"), io.Discard, "Replace?") || Confirm(reader(""), io.Discard, "Replace?") {
		t.Error("only y or yes confirms")
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

// script is a terminal's input as the raw-mode reader sees it: each entry is
// bytes delivered by one read, and "" is a read that timed out with nothing.
// After the script is spent every read reports EOF.
type script struct {
	chunks []string
	writes strings.Builder
}

func (s *script) Read(p []byte) (int, error) {
	if len(s.chunks) == 0 {
		return 0, io.EOF
	}
	chunk := s.chunks[0]
	if chunk == "" {
		s.chunks = s.chunks[1:]
		return 0, nil
	}
	n := copy(p, chunk)
	if n == len(chunk) {
		s.chunks = s.chunks[1:]
	} else {
		s.chunks[0] = chunk[n:]
	}
	return n, nil
}

func (s *script) querier() *Querier { return &Querier{In: s, Out: &s.writes} }

func cpr(col int) string { return "\x1b[1;" + strconv.Itoa(col) + "R" }

func TestAdvanceOfThroughAFakeCursorReport(t *testing.T) {
	sample := renderer.Sample{Family: glyph.BoxHeavy, Text: "┏━┓", Width: 3, Narrow: 3, Wide: 6}

	ok := &script{chunks: []string{cpr(1), cpr(4)}}
	failed, err := AdvanceFails(ok.querier(), sample)
	if err != nil || failed {
		t.Errorf("3 columns for width 3: failed=%v err=%v", failed, err)
	}
	if !strings.Contains(ok.writes.String(), "┏━┓") || !strings.Contains(ok.writes.String(), "\x1b[6n") {
		t.Errorf("the probe wrote %q", ok.writes.String())
	}

	wide := &script{chunks: []string{cpr(1), cpr(7)}}
	failed, err = AdvanceFails(wide.querier(), sample)
	if err != nil || !failed {
		t.Errorf("6 columns for width 3: failed=%v err=%v", failed, err)
	}

	mute := &script{chunks: []string{""}}
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

// probeScript answers the DA1 query, the liveness cursor report, then two
// reports per non-ASCII sample: the baseline column 1 and 1+advance.
func probeScript(t *testing.T, samples []renderer.Sample, advance func(renderer.Sample) int) *script {
	t.Helper()
	t.Setenv("TERM_PROGRAM", "")
	t.Setenv("TERM", "xterm-256color")
	chunks := []string{"\x1b[?62;c", cpr(1)}
	for _, s := range samples {
		if s.Family == glyph.ASCIIFamily {
			continue
		}
		chunks = append(chunks, cpr(1), cpr(1+advance(s)))
	}
	return &script{chunks: chunks}
}

func TestRunMarksAWrongAdvanceAndNothingElse(t *testing.T) {
	samples := Samples()
	sc := probeScript(t, samples, func(s renderer.Sample) int {
		if s.Family == glyph.Octants {
			return s.Narrow * 2
		}
		return s.Narrow
	})
	res := run(sc.querier(), samples)
	if !res.Probed || res.Terminal != "kitty" || res.AmbiguousWide {
		t.Errorf("result = %+v", res)
	}
	if len(res.Failed) != 1 || res.Failed[0] != (Failure{glyph.Octants, Advance}) {
		t.Errorf("failed = %+v", res.Failed)
	}
}

func TestRunReadsAmbiguousWidthFromTheBoxLightSample(t *testing.T) {
	samples := Samples()
	if samples[0].Family != glyph.BoxLight || samples[0].Wide != 2*samples[0].Narrow {
		t.Fatalf("box-light must lead and be all ambiguous runes: %+v", samples[0])
	}
	// A CJK terminal: ambiguous runes advance two columns, the rest one,
	// and the braille line is genuinely broken.
	sc := probeScript(t, samples, func(s renderer.Sample) int {
		if s.Family == glyph.Braille {
			return s.Wide + 1
		}
		return s.Wide
	})
	res := run(sc.querier(), samples)
	if !res.Probed || !res.AmbiguousWide {
		t.Fatalf("result = %+v", res)
	}
	if len(res.Failed) != 1 || res.Failed[0].Family != glyph.Braille {
		t.Errorf("failed = %+v, want braille alone", res.Failed)
	}
}

func TestRunDrainsALateReplyBeforeTheAsk(t *testing.T) {
	samples := Samples()
	t.Setenv("TERM_PROGRAM", "")
	t.Setenv("TERM", "xterm-256color")
	// The terminal is slow: DA1 and the liveness report time out, the
	// report then lands late, and the user's answer follows.
	sc := &script{chunks: []string{"", "", cpr(12), "", "2 5\n"}}
	res := run(sc.querier(), samples)
	if res.Probed || len(res.Failed) != 0 {
		t.Fatalf("an unanswered probe must mark nothing: %+v", res)
	}
	got, err := Ask(bufio.NewReader(sc), io.Discard, samples, res.Failed)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Family != glyph.BoxRounded || got[1].Family != glyph.Diagonals {
		t.Errorf("the late report reached the answer: %+v", got)
	}
}

func TestReportNamesTheResolvedFallback(t *testing.T) {
	var out strings.Builder
	Report(&out, []Failure{{glyph.BoxHeavy, Advance}, {glyph.Braille, Shape}, {glyph.Blocks, Shape}})
	s := out.String()
	for _, want := range []string{
		"box-heavy     advance  falls back to box-light; DejaVu Sans Mono covers it",
		"braille       shape    falls back to ascii; Symbola or Unifont covers it",
		"blocks        shape    falls back to ascii; DejaVu Sans Mono covers it",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("report lacks %q:\n%s", want, s)
		}
	}
	if strings.Count(s, "\n") != 3 {
		t.Errorf("want one line per failure:\n%s", s)
	}
	out.Reset()
	Report(&out, []Failure{{glyph.LegacyFills, Shape}})
	if !strings.Contains(out.String(), "falls back to sextants") {
		t.Errorf("legacy-fills alone falls back to sextants:\n%s", out.String())
	}
}

func TestMergeTouchesOnlyItsKeys(t *testing.T) {
	theme := "blueprint"
	links := true
	f := config.File{Profiles: map[string]config.Settings{
		"WezTerm": {Theme: &theme, Hyperlinks: &links, Failed: []string{"octants"}},
	}}
	res := Result{Identity: "WezTerm", Terminal: "WezTerm", Truecolor: false, AmbiguousWide: true, Probed: true}
	if !Merge(&f, res, []Failure{{glyph.BoxHeavy, Shape}}) {
		t.Error("the profile existed")
	}
	p := f.Profiles["WezTerm"]
	if p.Theme == nil || *p.Theme != "blueprint" || p.Hyperlinks == nil || !*p.Hyperlinks {
		t.Error("other keys changed")
	}
	if p.Truecolor == nil || *p.Truecolor || len(p.Failed) != 1 || p.Failed[0] != "box-heavy" {
		t.Errorf("profile = %+v", p)
	}
	if p.Terminal == nil || *p.Terminal != "WezTerm" || p.AmbiguousWide == nil || !*p.AmbiguousWide {
		t.Errorf("terminal %v ambiguous_wide %v", p.Terminal, p.AmbiguousWide)
	}

	// An asked-only run knows neither the terminal nor the width.
	if Merge(&f, Result{Identity: "xterm-kitty", Truecolor: true}, nil) {
		t.Error("xterm-kitty did not exist")
	}
	k := f.Profiles["xterm-kitty"]
	if k.Failed != nil || k.Truecolor == nil || !*k.Truecolor || k.Terminal != nil || k.AmbiguousWide != nil {
		t.Errorf("new profile = %+v", k)
	}
}
