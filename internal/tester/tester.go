// Package tester is `mmaid config init`: it probes what the terminal can
// answer, asks about what only eyes can, and merges the result into the
// terminal's profile (ADR-500).
package tester

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
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

// Samples is the sheet the tester shows: every family in its own runes,
// whatever set the configuration names, so each line tests one family.
func Samples() []renderer.Sample {
	return renderer.GlyphSamples(glyph.DefaultSet())
}

// Result is what the probes found.
type Result struct {
	// Identity is the profile key: TERM_PROGRAM, else TERM.
	Identity string
	// Terminal is the name the DA1 probe detected, or "".
	Terminal  string
	Truecolor bool
	// AmbiguousWide says the box-light sample advanced two columns per
	// rune. It is meaningful only when Probed.
	AmbiguousWide bool
	Failed        []Failure
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
	return Result{Identity: config.TerminalIdentity(), Truecolor: TruecolorFromEnv()}
}

// Probe runs the terminal probes over in and out, which must both be
// terminals. Raw mode is restored on every return, a panic or a signal
// included, and the error returned is Restore's. Where raw mode is
// unsupported the result is FromEnv with Probed false.
func Probe(in, out *os.File, samples []renderer.Sample) (Result, error) {
	fd := int(in.Fd())
	if !term.IsTerminal(fd) || !term.IsTerminal(int(out.Fd())) {
		return FromEnv(), nil
	}
	state, err := term.MakeRaw(fd, Timeout)
	if err != nil {
		return FromEnv(), nil
	}

	// A signal from outside would exit without running the deferred
	// Restore; ISIG is off, so the keyboard cannot send one.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		select {
		case sig := <-sigs:
			if err := term.Restore(fd, state); err != nil {
				fmt.Fprintf(os.Stderr, "mmaid: restoring the terminal: %v\n", err)
			}
			fmt.Fprintf(os.Stderr, "mmaid: %v\n", sig)
			os.Exit(130)
		case <-done:
		}
	}()
	defer func() {
		close(done)
		signal.Stop(sigs)
	}()

	res := run(&Querier{In: in, Out: out}, samples)
	return res, term.Restore(fd, state)
}

// run is Probe without the terminal handling: identity, truecolor, DA1,
// then one cursor report per family. It drains late replies before it
// returns so they do not reach the asked phase.
func run(q *Querier, samples []renderer.Sample) Result {
	res := FromEnv()
	defer Drain(q.In)

	res.Terminal = config.DetectTerminal(func(request string) (string, error) {
		return q.Query(request, 'c')
	})

	if _, err := CursorColumn(q.Query); err != nil {
		return res
	}
	res.Probed = true
	for i, s := range samples {
		if s.Family == glyph.ASCIIFamily {
			continue
		}
		advance, err := AdvanceOf(q, s)
		if err != nil {
			res.Probed = false
			res.Failed = nil
			res.AmbiguousWide = false
			return res
		}
		// The first sample, box-light, is all ambiguous-width runes: an
		// advance of twice its width is the terminal's ambiguous setting,
		// not a failure, and every later sample is judged by it.
		if i == 0 && s.Wide != s.Narrow && advance == s.Wide {
			res.AmbiguousWide = true
		}
		expected := s.Narrow
		if res.AmbiguousWide {
			expected = s.Wide
		}
		if advance != expected {
			res.Failed = append(res.Failed, Failure{Family: s.Family, Cause: Advance})
		}
	}
	return res
}

// Drain reads and discards input until a read returns nothing, which raw
// mode does once a timeout window passes with no bytes.
func Drain(r io.Reader) {
	buf := make([]byte, 64)
	for {
		n, err := r.Read(buf)
		if n == 0 || err != nil {
			return
		}
	}
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

// AdvanceOf writes the sample at the start of a line and returns how many
// columns the cursor moved. The line is erased afterwards.
func AdvanceOf(q *Querier, s renderer.Sample) (int, error) {
	if err := q.Write("\r"); err != nil {
		return 0, err
	}
	before, err := CursorColumn(q.Query)
	if err != nil {
		return 0, err
	}
	if err := q.Write(s.Text); err != nil {
		return 0, err
	}
	after, err := CursorColumn(q.Query)
	if err != nil {
		return 0, err
	}
	if err := q.Write("\r\x1b[2K"); err != nil {
		return 0, err
	}
	return after - before, nil
}

// AdvanceFails reports whether the sample moved the cursor by other than
// its width.
func AdvanceFails(q *Querier, s renderer.Sample) (bool, error) {
	advance, err := AdvanceOf(q, s)
	if err != nil {
		return false, err
	}
	return AdvanceMismatch(0, advance, s.Width), nil
}

// AdvanceMismatch compares the columns before and after writing a sample
// with its expected width.
func AdvanceMismatch(before, after, width int) bool {
	return after-before != width
}

// Ask prints the numbered sheet, one line per family, and reads the numbers
// of the lines that look wrong from in, the one reader the tester holds on
// stdin. A family the probe already failed keeps the probe's cause; any
// other the user names fails with Shape. The probe's failures are returned
// whether or not the user repeats them.
func Ask(in *bufio.Reader, out io.Writer, samples []renderer.Sample, probed []Failure) ([]Failure, error) {
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

	line, err := in.ReadString('\n')
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

	var failures []Failure
	for _, s := range samples {
		switch f, probedFail := byFamily[s.Family]; {
		case probedFail:
			failures = append(failures, f)
		case chosen[s.Family] && s.Family != glyph.ASCIIFamily:
			failures = append(failures, Failure{Family: s.Family, Cause: Shape})
		}
	}
	return failures, nil
}

// Confirm prints a yes/no prompt and reads one line from in; only y or yes
// answers true.
func Confirm(in *bufio.Reader, out io.Writer, prompt string) bool {
	fmt.Fprint(out, prompt, " [y/N]: ")
	line, _ := in.ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	}
	return false
}

// Remedies maps each failed family to what the renderer draws instead once
// every failure is applied, so a family whose fallback also failed names
// the family past it.
func Remedies(failures []Failure) map[glyph.Family]glyph.Family {
	families := make([]glyph.Family, 0, len(failures))
	for _, f := range failures {
		families = append(families, f.Family)
	}
	resolved := glyph.Resolve(glyph.DefaultSet(), families)
	out := make(map[glyph.Family]glyph.Family, len(failures))
	for _, f := range failures {
		out[f.Family] = resolved.Binding(f.Family)
	}
	return out
}

// Report writes one line per failure: the family, the cause, the fallback
// the renderer will use, and a font that covers the family.
func Report(out io.Writer, failures []Failure) {
	if len(failures) == 0 {
		fmt.Fprintln(out, "No family failed.")
		return
	}
	remedies := Remedies(failures)
	for _, f := range failures {
		fmt.Fprintf(out, "%-13s %-8s falls back to %s; %s covers it\n",
			f.Family, f.Cause, remedies[f.Family], Font(f.Family))
	}
}

// Merge sets truecolor and failed on the identity's profile, records the
// detected terminal when there is one and ambiguous_wide when it was probed,
// and leaves the other keys alone. It reports whether the profile existed.
func Merge(file *config.File, res Result, failures []Failure) (existed bool) {
	if file.Profiles == nil {
		file.Profiles = map[string]config.Settings{}
	}
	prof, existed := file.Profiles[res.Identity]
	if res.Terminal != "" {
		terminal := res.Terminal
		prof.Terminal = &terminal
	}
	if res.Probed {
		wide := res.AmbiguousWide
		prof.AmbiguousWide = &wide
	}
	truecolor := res.Truecolor
	prof.Truecolor = &truecolor
	prof.Failed = nil
	for _, f := range failures {
		prof.Failed = append(prof.Failed, string(f.Family))
	}
	file.Profiles[res.Identity] = prof
	return existed
}
