package cells

import (
	"strings"
	"testing"
)

func findings(f *Frame, rule int) []Finding {
	var out []Finding
	for _, x := range Lint(f) {
		if x.Rule == rule {
			out = append(out, x)
		}
	}
	return out
}

func TestRule4HalfGlyphIsAnOpenEnd(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		want  int
	}{
		{"stub against a border tee", []string{
			"   │   ",
			"───┴───",
			"   ╷   ",
			"   ▼   ",
		}, 0},
		{"stub against a border tee, horizontal", []string{
			"──╴├──►",
		}, 0},
		{"stub into text", []string{"──╴Text"}, 0},
		{"stub into a marker", []string{"──╴●"}, 0},
		{"stub into space", []string{"──╴ "}, 1},
		{"stub against a plain border", []string{
			"   │   ",
			"───────",
			"   ╷   ",
			"   ▼   ",
		}, 1},
		{"stub against a tee pointing at it", []string{
			"   │   ",
			"───┬───",
			"   ╷   ",
			"   ▼   ",
		}, 1},
	}
	for _, c := range cases {
		got := findings(frameOf(c.lines...), 4)
		if len(got) != c.want {
			t.Errorf("%s: %d rule 4 findings, want %d: %v\n%s", c.name, len(got), c.want, got, strings.Join(c.lines, "\n"))
		}
	}
}
