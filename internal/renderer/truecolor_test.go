package renderer

import (
	"strings"
	"testing"

	"github.com/aaronsb/mmaid-go/internal/cells"
)

func TestRGBTo256(t *testing.T) {
	cases := []struct {
		hex  string
		want int
	}{
		{"#000000", 16},  // cube origin
		{"#ffffff", 231}, // cube corner
		{"#ff0000", 196},
		{"#00ff00", 46},
		{"#0000ff", 21},
		{"#808080", 244}, // grey ramp, 8+10*23 is 238; 128 is rung 12 at 128
		{"#5F87AF", 67},  // an exact cube colour: 95,135,175
		{"#080808", 232}, // the ramp's first rung beats the cube's black
		{"#FEFEFE", 231}, // the cube's white beats the ramp, which stops at 238
		{"#737373", 243}, // grey 118 beats the cube, which is 20 off on each channel
	}
	for _, tc := range cases {
		r, g, b := parseHex(tc.hex)
		if got := rgbTo256(r, g, b); got != tc.want {
			t.Errorf("rgbTo256(%s) = %d, want %d", tc.hex, got, tc.want)
		}
	}
}

func TestNearestCubeLevelTie(t *testing.T) {
	// 115 sits between 95 and 135; the comparison is strict, so the lower level
	// wins.
	if got := nearestCubeLevel(115); got != 1 {
		t.Errorf("nearestCubeLevel(115) = %d, want 1 (level 95)", got)
	}
}

func TestDowngradeLeavesNonSGRAlone(t *testing.T) {
	in := "\033[2J\033[H\033[38;2;255;215;0m\033[48;2;0;0;0mx"
	want := "\033[2J\033[H\033[38;5;220m\033[48;5;16mx"
	if got := Downgrade(in); got != want {
		t.Errorf("Downgrade = %q, want %q", got, want)
	}
	both := "\033[38;2;255;215;0;48;2;95;135;175m"
	if got, want := Downgrade(both), "\033[38;5;220;48;5;67m"; got != want {
		t.Errorf("Downgrade = %q, want %q", got, want)
	}
}

func TestSetTruecolorSwitchesTheSequence(t *testing.T) {
	t.Cleanup(func() { SetTruecolor(true) })

	if got := hexColor("#5F87AF"); got != "\033[38;2;95;135;175m" {
		t.Errorf("truecolor foreground = %q", got)
	}
	SetTruecolor(false)
	if got := hexColor("#5F87AF"); got != "\033[38;5;67m" {
		t.Errorf("256-colour foreground = %q", got)
	}
	if got := hexBgColor("#5F87AF"); got != "\033[48;5;67m" {
		t.Errorf("256-colour background = %q", got)
	}
	SetTruecolor(true)
	if got := hexBgColor("#5F87AF"); got != "\033[48;2;95;135;175m" {
		t.Errorf("truecolor background = %q", got)
	}
}

func TestDowngradeRewritesOnlyTruecolor(t *testing.T) {
	in := "\033[1m\033[38;2;255;215;0m\033[48;2;0;0;0m\033[3m"
	want := "\033[1m\033[38;5;220m\033[48;5;16m\033[3m"
	if got := Downgrade(in); got != want {
		t.Errorf("Downgrade = %q, want %q", got, want)
	}
	for _, s := range []string{"", "\033[1m", "\033[38;5;67m", "plain"} {
		if got := Downgrade(s); got != s {
			t.Errorf("Downgrade(%q) = %q, want it unchanged", s, got)
		}
	}
}

// TestInterpreterReadsBackTheApproximation renders one styled cell without
// truecolor and checks that the ADR-101 interpreter recovers a colour close to
// the one the theme asked for.
func TestInterpreterReadsBackTheApproximation(t *testing.T) {
	t.Cleanup(func() { SetTruecolor(true) })
	SetTruecolor(false)

	const hex = "#F92672" // monokai pink: 249, 38, 114
	want := [3]int{249, 38, 114}

	c := NewCanvas(1, 1)
	c.Put(0, 0, 'x', "_ansi:"+hexColor(hex))
	frame, err := cells.Interpret(c.ToColorString(GetTheme("default")))
	if err != nil {
		t.Fatalf("interpret: %v", err)
	}
	got := frame.Cells[0].Fg

	var dist float64
	for i := range 3 {
		d := float64(int(got[i]) - want[i])
		dist += d * d
	}
	if dist > 3*40*40 {
		t.Errorf("interpreted %v for %s, too far from %v", got, hex, want)
	}
}

func TestThemeDowngradedWhenTruecolorOff(t *testing.T) {
	t.Cleanup(func() { SetTruecolor(true) })
	SetTruecolor(false)
	theme := GetTheme("monokai")
	for _, s := range []string{theme.Node, theme.Label, theme.SubgraphFill} {
		if strings.Contains(s, "38;2;") || strings.Contains(s, "48;2;") {
			t.Errorf("theme sequence %q still carries 24-bit colour", s)
		}
	}
	SetTruecolor(true)
	if !strings.Contains(GetTheme("monokai").Node, "38;2;") {
		t.Error("the truecolor theme should be untouched")
	}
}
