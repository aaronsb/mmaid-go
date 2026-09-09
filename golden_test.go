package mmaid

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/aaronsb/mmaid-go/internal/cells"
	"github.com/aaronsb/mmaid-go/internal/diagram"
)

const (
	fixtureDir = "testdata/fixtures"
	goldenDir  = "testdata/golden"
	knownBad   = fixtureDir + "/known-bad.txt"

	// The harness always sets width explicitly so a frame does not depend on
	// terminal detection.
	defaultWidth = 120
	defaultTheme = "default"
)

// fixture is one .mmd file and the render settings its directive names.
type fixture struct {
	name        string
	source      string
	opts        []Option
	width       int
	orientation string
	ascii       bool
}

// loadFixtures reads every .mmd file in the fixture directory, in name order.
func loadFixtures(t *testing.T) []fixture {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(fixtureDir, "*.mmd"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatalf("no fixtures in %s", fixtureDir)
	}
	out := make([]fixture, 0, len(paths))
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		f, err := newFixture(strings.TrimSuffix(filepath.Base(p), ".mmd"), string(data))
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		out = append(out, f)
	}
	return out
}

// newFixture parses a leading "%% mmaid: <flags>" directive into render
// settings. The directive is a test-harness convention, not parser syntax.
func newFixture(name, source string) (fixture, error) {
	f := fixture{name: name, source: source, width: defaultWidth}
	theme := defaultTheme

	first, _, _ := strings.Cut(source, "\n")
	rest, ok := strings.CutPrefix(strings.TrimSpace(first), "%% mmaid:")
	if !ok {
		f.opts = append(f.opts, WithTheme(theme))
		return f, nil
	}

	fields := strings.Fields(rest)
	next := func(i int, flag string) (string, error) {
		if i+1 >= len(fields) {
			return "", fmt.Errorf("%s needs a value", flag)
		}
		return fields[i+1], nil
	}
	for i := 0; i < len(fields); i++ {
		var err error
		switch flag := fields[i]; flag {
		case "-t", "--theme":
			if theme, err = next(i, flag); err != nil {
				return f, err
			}
			i++
		case "-w", "--width":
			v, err := next(i, flag)
			if err != nil {
				return f, err
			}
			if f.width, err = strconv.Atoi(v); err != nil {
				return f, fmt.Errorf("%s %q: %w", flag, v, err)
			}
			i++
		case "--orientation":
			if f.orientation, err = next(i, flag); err != nil {
				return f, err
			}
			i++
		case "-a", "--ascii":
			f.ascii = true
			f.opts = append(f.opts, WithASCII())
		case "--sharp-edges":
			f.opts = append(f.opts, WithSharpEdges())
		default:
			return f, fmt.Errorf("unknown flag %q in the mmaid directive", flag)
		}
	}
	f.opts = append(f.opts, WithTheme(theme))
	return f, nil
}

// frame renders the fixture and interprets the resulting stream.
func (f fixture) frame() (*cells.Frame, error) {
	diagram.SetWidthOverride(f.width)
	defer diagram.SetWidthOverride(0)
	if f.orientation != "" {
		diagram.SetOrientationOverride(f.orientation)
		defer diagram.SetOrientationOverride("")
	}
	return cells.Interpret(Render(f.source, f.opts...))
}

func goldenPath(name string) string {
	return filepath.Join(goldenDir, name+".cells")
}

// TestGolden renders every fixture and compares it with its reference frame.
// GOLDEN_RECORD=1 rewrites the references; GOLDEN_DUMP=<dir> also writes each
// rendered frame there.
func TestGolden(t *testing.T) {
	record := os.Getenv("GOLDEN_RECORD") != ""
	dump := os.Getenv("GOLDEN_DUMP")
	if dump != "" {
		if err := os.MkdirAll(dump, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if record {
		if err := os.MkdirAll(goldenDir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	tol := cells.FromEnv()
	fixtures := loadFixtures(t)
	var failures strings.Builder
	allIdentical := true

	for _, fx := range fixtures {
		frame, err := fx.frame()
		if err != nil {
			t.Errorf("%s: %v", fx.name, err)
			continue
		}
		if dump != "" {
			if err := writeFrame(filepath.Join(dump, fx.name+".cells"), frame); err != nil {
				t.Fatal(err)
			}
		}
		if record {
			if err := writeFrame(goldenPath(fx.name), frame); err != nil {
				t.Fatal(err)
			}
			fmt.Fprintf(os.Stderr, "golden: recorded %-16s %dx%d\n", fx.name, frame.W, frame.H)
			continue
		}

		reference, err := readFrame(goldenPath(fx.name))
		if err != nil {
			fmt.Fprintf(os.Stderr, "golden: %-16s %v\n", fx.name, err)
			fmt.Fprintf(&failures, "  %s: %v\n", fx.name, err)
			allIdentical = false
			continue
		}
		report, err := cells.Compare(reference, frame)
		if err != nil {
			fmt.Fprintf(os.Stderr, "golden: %-16s %v  FAIL\n", fx.name, err)
			fmt.Fprintf(&failures, "  %s: %v\n", fx.name, err)
			allIdentical = false
			continue
		}
		ok := tol.Passes(report)
		mark := ""
		if !ok {
			mark = "  FAIL"
		}
		fmt.Fprintf(os.Stderr, "golden: %-16s %s%s\n", fx.name, report.Summary(), mark)
		if len(report.Diffs) > 0 {
			allIdentical = false
		}
		if !ok {
			fmt.Fprintf(&failures, "  %s: %s\n%s", fx.name, report.Summary(), report.Listing())
		}
	}

	if record {
		fmt.Fprintf(os.Stderr, "golden: recorded %d frames in %s\n", len(fixtures), goldenDir)
		return
	}
	if failures.Len() > 0 {
		t.Fatalf("golden frames differ from %s (%s):\n%sIf the change is intended, run 'make golden-record' and say which frames changed and why in the commit.",
			goldenDir, tol.Describe(), failures.String())
	}
	if allIdentical {
		fmt.Fprintln(os.Stderr, "golden: all frames identical")
	} else {
		fmt.Fprintf(os.Stderr, "golden: all frames within tolerance (%s)\n", tol.Describe())
	}
}

func writeFrame(path string, f *cells.Frame) error {
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()
	return cells.Write(out, f)
}

func readFrame(path string) (*cells.Frame, error) {
	in, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("no reference frame at %s", path)
	}
	defer in.Close()
	f, err := cells.Read(in)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return f, nil
}

// TestEveryReferenceHasAFixture catches a reference left behind by a renamed
// or deleted fixture.
func TestEveryReferenceHasAFixture(t *testing.T) {
	names := map[string]bool{}
	for _, fx := range loadFixtures(t) {
		names[fx.name] = true
	}
	refs, err := filepath.Glob(filepath.Join(goldenDir, "*.cells"))
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range refs {
		stem := strings.TrimSuffix(filepath.Base(r), ".cells")
		if !names[stem] {
			t.Errorf("%s has no fixture in %s", r, fixtureDir)
		}
	}
}

// TestFixturesLint runs the structural lint over every fixture. Fixtures
// listed in known-bad.txt are expected to fail; the test fails if one passes.
func TestFixturesLint(t *testing.T) {
	bad, err := loadKnownBad()
	if err != nil {
		t.Fatal(err)
	}
	for _, fx := range loadFixtures(t) {
		if fx.ascii {
			t.Logf("%s: skipped, the lint reads Unicode box-drawing glyphs only", fx.name)
			if bad[fx.name] {
				t.Errorf("%s lists %s, whose ASCII frame the lint skips", knownBad, fx.name)
			}
			delete(bad, fx.name)
			continue
		}
		frame, err := fx.frame()
		if err != nil {
			t.Errorf("%s: %v", fx.name, err)
			continue
		}
		findings := cells.Lint(frame)
		switch {
		case bad[fx.name] && len(findings) == 0:
			t.Errorf("%s passes the lint; remove it from %s", fx.name, knownBad)
		case !bad[fx.name] && len(findings) > 0:
			var b strings.Builder
			for _, f := range findings {
				fmt.Fprintf(&b, "  %s\n", f)
			}
			t.Errorf("%s has %d structural findings:\n%s", fx.name, len(findings), b.String())
		}
		delete(bad, fx.name)
	}
	for name := range bad {
		t.Errorf("%s lists %s, which is not a fixture", knownBad, name)
	}
}

// loadKnownBad reads the fixture stems expected to fail the lint, one per
// line, '#' starting a comment.
func loadKnownBad() (map[string]bool, error) {
	data, err := os.ReadFile(knownBad)
	if err != nil {
		return nil, err
	}
	bad := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		if line = strings.TrimSpace(line); line != "" {
			bad[line] = true
		}
	}
	return bad, nil
}
