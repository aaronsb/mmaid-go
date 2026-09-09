package mmaid

import (
	"strings"
	"testing"

	"github.com/aaronsb/mmaid-go/internal/cells"
)

const clickSource = `flowchart LR
    A[Start] --> B[Finish]
    click A "https://example.com/start"
    click B href "https://example.com/finish" _blank
`

func TestHyperlinksWrapTheLabel(t *testing.T) {
	out := Render(clickSource, WithTheme("default"), WithHyperlinks())

	for _, want := range []string{
		"\033]8;;https://example.com/start\033\\",
		"\033]8;;https://example.com/finish\033\\",
		"\033]8;;\033\\",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q", want)
		}
	}

	// The pair brackets the label rather than the whole line.
	start := strings.Index(out, "\033]8;;https://example.com/start\033\\")
	label := start + len("\033]8;;https://example.com/start\033\\")
	rest := out[label:]
	if i := strings.Index(rest, "\033]8;;\033\\"); i < 0 || !strings.Contains(stripANSI(rest[:i]), "Start") {
		t.Errorf("the link does not close around the label: %q", rest[:min(60, len(rest))])
	}
}

func TestHyperlinksOffByDefault(t *testing.T) {
	if out := Render(clickSource, WithTheme("default")); strings.Contains(out, "\033]8;;") {
		t.Error("a click line should not emit OSC 8 unless hyperlinks are on")
	}
}

// TestHyperlinksLeaveTheFrameAlone is ADR-101's guarantee: the interpreter skips
// OSC, so a .cells frame is the same either way.
func TestHyperlinksLeaveTheFrameAlone(t *testing.T) {
	plain, err := cells.Interpret(Render(clickSource, WithTheme("default")))
	if err != nil {
		t.Fatalf("interpret plain: %v", err)
	}
	linked, err := cells.Interpret(Render(clickSource, WithTheme("default"), WithHyperlinks()))
	if err != nil {
		t.Fatalf("interpret linked: %v", err)
	}
	report := cells.Compare(plain, linked)
	if report.Identical != 100 {
		t.Errorf("frames differ: %.2f%% identical", report.Identical)
	}
}

// stripANSI removes CSI sequences so an assertion can read the text.
func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '\033' && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && s[j] != 'm' {
				j++
			}
			i = j + 1
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}
