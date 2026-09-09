// Package tester is `mmaid config init`: it probes what the terminal can
// answer, asks about what only eyes can, and merges the result into the
// terminal's profile (ADR-500).
package tester

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aaronsb/mmaid-go/internal/config"
	"github.com/aaronsb/mmaid-go/internal/glyph"
	"github.com/aaronsb/mmaid-go/internal/renderer"
	"github.com/aaronsb/mmaid-go/internal/term"
)

// Cause says how a family was found to fail.
type Cause string

const (
	// Missing: the terminal drew nothing where the glyph should be.
	Missing Cause = "missing"
	// Advance: the probe saw the sample move the cursor by the wrong count.
	Advance Cause = "advance"
	// Shape: the user said the line looks wrong.
	Shape Cause = "shape"
)

// Failure is one family the terminal cannot draw, and why.
type Failure struct {
	Family glyph.Family
	Cause  Cause
}

// Fallback is the family the renderer draws instead.
func (f Failure) Fallback() glyph.Family {
	if fb, ok := glyph.Fallback[f.Family]; ok {
		return fb
	}
	return glyph.ASCIIFamily
}

// Font suggests a font that covers the family.
func Font(f glyph.Family) string {
	switch f {
	case glyph.Braille:
		return "Symbola or Unifont"
	case glyph.Sextants, glyph.Octants, glyph.LegacyFills:
		return "Cascadia Code or Iosevka"
	}
	return "DejaVu Sans Mono"
}

// Timeout is how long a probe waits for the terminal's answer.
const Timeout = 200 * time.Millisecond

// Result is what the probes found.
type Result struct {
	Identity  string
	Truecolor bool
	Failed    []Failure
	// Probed says the terminal answered the queries; when false, every
	// family is asked.
	Probed bool
}

// TruecolorFromEnv reads COLORTERM.
func TruecolorFromEnv() bool {
	switch os.Getenv("COLORTERM") {
	case "truecolor", "24bit":
		return true
	}
	return false
}

// FromEnv answers what the environment can without touching the terminal.
func FromEnv() Result {
	return Result{Identity: config.ProbedIdentity(nil), Truecolor: TruecolorFromEnv()}
}

// Probe runs the terminal probes over in and out, which must both be
// terminals. Raw mode is restored on every return, a panic included. Where
// raw mode is unsupported or the terminal does not answer, the result is
// FromEnv with Probed false.
func Probe(in, out *os.File, samples []renderer.Sample) Result {
	if !term.IsTerminal(int(in.Fd())) || !term.IsTerminal(int(out.Fd())) {
		return FromEnv()
	}
	state, err := term.MakeRaw(int(in.Fd()), Timeout)
	if err != nil {
		return FromEnv()
	}
	defer term.Restore(int(in.Fd()), state)

	q := &Querier{In: in, Out: out}
	res := Result{Truecolor: TruecolorFromEnv()}
	res.Identity = config.ProbedIdentity(func(request string) (string, error) {
		return q.Query(request, 'c')
	})

	if _, err := CursorColumn(q.Query); err != nil {
		res.Probed = false
		return res
	}
	res.Probed = true
	for _, s := range samples {
		if s.Family == glyph.ASCIIFamily {
			continue
		}
		failed, err := AdvanceFails(q, s)
		if err != nil {
			res.Probed = false
			res.Failed = nil
			return res
		}
		if failed {
			res.Failed = append(res.Failed, Failure{Family: s.Family, Cause: Advance})
		}
	}
	return res
}

// Querier writes a request to the terminal and reads its answer.
type Querier struct {
	In  io.Reader
	Out io.Writer
}

// Query writes request and reads until the final byte arrives or a read
// returns nothing, which raw mode does after the timeout.
func (q *Querier) Query(request string, final byte) (string, error) {
	if _, err := io.WriteString(q.Out, request); err != nil {
		return "", err
	}
	var b strings.Builder
	buf := make([]byte, 1)
	for {
		n, err := q.In.Read(buf)
		if n == 0 {
			if err != nil && err != io.EOF {
				return b.String(), err
			}
			return b.String(), fmt.Errorf("no answer to %q", request)
		}
		b.WriteByte(buf[0])
		if buf[0] == final {
			return b.String(), nil
		}
	}
}

// Write puts text on the terminal without reading anything back.
func (q *Querier) Write(text string) error {
	_, err := io.WriteString(q.Out, text)
	return err
}

// CursorColumn asks for a cursor position report and returns the column.
func CursorColumn(query func(request string, final byte) (string, error)) (int, error) {
	reply, err := query("\x1b[6n", 'R')
	if err != nil {
		return 0, err
	}
	return parseCPR(reply)
}

// parseCPR reads the column out of `ESC [ row ; col R`.
func parseCPR(reply string) (int, error) {
	i := strings.LastIndex(reply, "\x1b[")
	if i < 0 || !strings.HasSuffix(reply, "R") {
		return 0, fmt.Errorf("not a cursor report: %q", reply)
	}
	body := strings.TrimSuffix(reply[i+2:], "R")
	_, colText, ok := strings.Cut(body, ";")
	if !ok {
		return 0, fmt.Errorf("not a cursor report: %q", reply)
	}
	col, err := strconv.Atoi(colText)
	if err != nil {
		return 0, fmt.Errorf("not a cursor report: %q", reply)
	}
	return col, nil
}

// AdvanceFails writes the sample at the start of a line, measures how far the
// cursor moved, and reports whether that differs from the sample's width.
// The line is erased afterwards.
func AdvanceFails(q *Querier, s renderer.Sample) (bool, error) {
	if err := q.Write("\r"); err != nil {
		return false, err
	}
	before, err := CursorColumn(q.Query)
	if err != nil {
		return false, err
	}
	if err := q.Write(s.Text); err != nil {
		return false, err
	}
	after, err := CursorColumn(q.Query)
	if err != nil {
		return false, err
	}
	if err := q.Write("\r\x1b[2K"); err != nil {
		return false, err
	}
	return AdvanceMismatch(before, after, s.Width), nil
}

// AdvanceMismatch compares the columns before and after writing a sample
// with its expected width.
func AdvanceMismatch(before, after, width int) bool {
	return after-before != width
}

// Ask prints the numbered sheet, one line per family, and reads the numbers
// of the lines that look wrong. A family the probe already failed keeps the
// probe's cause; any other the user names fails with Shape. The probe's
// failures are returned whether or not the user repeats them.
func Ask(in io.Reader, out io.Writer, samples []renderer.Sample, probed []Failure) ([]Failure, error) {
	byFamily := make(map[glyph.Family]Failure, len(probed))
	for _, f := range probed {
		byFamily[f.Family] = f
	}

	nameW, textW := 0, 0
	for _, s := range samples {
		nameW = max(nameW, len(s.Family))
		textW = max(textW, s.Width)
	}
	fmt.Fprintln(out, "Each line shows a glyph family, then the same figure in the fallback set.")
	for i, s := range samples {
		note := ""
		if f, ok := byFamily[s.Family]; ok {
			note = "  (" + string(f.Cause) + ": fails)"
		}
		fmt.Fprintf(out, "%2d  %-*s  %s%s  %s%s\n",
			i+1, nameW, s.Family, s.Text, strings.Repeat(" ", textW-s.Width), s.Reference, note)
	}
	fmt.Fprint(out, "Which lines look wrong? (numbers separated by spaces, Enter for none): ")

	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && err != io.EOF {
		return nil, err
	}
	fmt.Fprintln(out)

	chosen := map[glyph.Family]bool{}
	for _, field := range strings.Fields(line) {
		n, err := strconv.Atoi(field)
		if err != nil || n < 1 || n > len(samples) {
			fmt.Fprintf(out, "ignoring %q: not a line number\n", field)
			continue
		}
		chosen[samples[n-1].Family] = true
	}

	var out2 []Failure
	for _, s := range samples {
		switch f, probedFail := byFamily[s.Family]; {
		case probedFail:
			out2 = append(out2, f)
		case chosen[s.Family] && s.Family != glyph.ASCIIFamily:
			out2 = append(out2, Failure{Family: s.Family, Cause: Shape})
		}
	}
	return out2, nil
}

// Report writes one line per failure: the family, the cause, the fallback
// the renderer will use, and a font that covers the family.
func Report(out io.Writer, failures []Failure) {
	if len(failures) == 0 {
		fmt.Fprintln(out, "No family failed.")
		return
	}
	for _, f := range failures {
		fmt.Fprintf(out, "%-13s %-8s falls back to %s; %s covers it\n",
			f.Family, f.Cause, f.Fallback(), Font(f.Family))
	}
}

// Merge sets truecolor and failed on the identity's profile and leaves its
// other keys alone. It reports whether the profile existed.
func Merge(file *config.File, identity string, truecolor bool, failures []Failure) (existed bool) {
	if file.Profiles == nil {
		file.Profiles = map[string]config.Settings{}
	}
	prof, existed := file.Profiles[identity]
	prof.Truecolor = &truecolor
	prof.Failed = nil
	for _, f := range failures {
		prof.Failed = append(prof.Failed, string(f.Family))
	}
	file.Profiles[identity] = prof
	return existed
}
