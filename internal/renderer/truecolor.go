package renderer

import (
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
)

// noTruecolor is set when the terminal does not understand 38;2 and 48;2, and
// the theme's RGB is approximated in the 256-colour palette instead. The sense
// is inverted so that the zero value is 24-bit colour, which is what the
// package's variable initializers see.
var noTruecolor atomic.Bool

// SetTruecolor selects between 24-bit colour and the 256-colour palette for
// every colour a theme emits.
func SetTruecolor(on bool) { noTruecolor.Store(!on) }

// Truecolor reports the current setting.
func Truecolor() bool { return !noTruecolor.Load() }

// cubeLevels are the six channel values of the xterm 216-colour cube.
var cubeLevels = [6]int{0, 95, 135, 175, 215, 255}

// nearestCubeLevel returns the cube index whose level is closest to v.
func nearestCubeLevel(v int) int {
	best, bestDist := 0, 1<<30
	for i, level := range cubeLevels {
		d := level - v
		if d < 0 {
			d = -d
		}
		if d < bestDist {
			best, bestDist = i, d
		}
	}
	return best
}

// rgbTo256 returns the xterm palette index closest to an RGB triple, choosing
// between the 6x6x6 cube (16-231) and the 24-step grey ramp (232-255).
func rgbTo256(r, g, b int) int {
	ri, gi, bi := nearestCubeLevel(r), nearestCubeLevel(g), nearestCubeLevel(b)
	cube := 16 + 36*ri + 6*gi + bi
	cubeDist := squares(r-cubeLevels[ri], g-cubeLevels[gi], b-cubeLevels[bi])

	// The grey ramp runs 8, 18, ... 238; step to the nearest rung.
	step := (r + g + b + 1) / 3
	rung := (step - 8 + 5) / 10
	rung = max(0, min(23, rung))
	grey := 8 + 10*rung
	greyDist := squares(r-grey, g-grey, b-grey)

	if greyDist < cubeDist {
		return 232 + rung
	}
	return cube
}

func squares(a, b, c int) int { return a*a + b*b + c*c }

// downgraded returns the theme with every 38;2 and 48;2 sequence rewritten as
// its nearest 256-colour index.
func (t Theme) downgraded() Theme {
	for _, field := range []*string{
		&t.Node, &t.Edge, &t.Arrow, &t.Subgraph, &t.Label, &t.EdgeLabel,
		&t.SubgraphLabel, &t.Default, &t.BoldLabel, &t.ItalicLabel,
		&t.Note, &t.SubgraphFill,
	} {
		*field = Downgrade(*field)
	}
	return t
}

// Downgrade rewrites the 24-bit colour sequences in an ANSI string as
// 256-colour ones, leaving everything else alone.
func Downgrade(ansi string) string {
	if ansi == "" || !strings.Contains(ansi, ";2;") {
		return ansi
	}
	var b strings.Builder
	rest := ansi
	for {
		i := strings.Index(rest, "\033[")
		if i < 0 {
			b.WriteString(rest)
			return b.String()
		}
		// A CSI runs to its final byte; only 'm' carries colour, and the
		// others (ESC[2J and the like) pass through untouched.
		end := i + 2
		for end < len(rest) && (rest[end] < 0x40 || rest[end] > 0x7e) {
			end++
		}
		if end >= len(rest) {
			b.WriteString(rest)
			return b.String()
		}
		b.WriteString(rest[:i])
		if rest[end] == 'm' {
			b.WriteString(downgradeSGR(rest[i+2 : end]))
		} else {
			b.WriteString(rest[i : end+1])
		}
		rest = rest[end+1:]
	}
}

// downgradeSGR rewrites one SGR body (the text between ESC [ and m).
func downgradeSGR(body string) string {
	fields := strings.Split(body, ";")
	out := make([]string, 0, len(fields))
	for i := 0; i < len(fields); i++ {
		if (fields[i] == "38" || fields[i] == "48") && i+4 < len(fields) && fields[i+1] == "2" {
			r, e1 := strconv.Atoi(fields[i+2])
			g, e2 := strconv.Atoi(fields[i+3])
			bl, e3 := strconv.Atoi(fields[i+4])
			if e1 == nil && e2 == nil && e3 == nil {
				out = append(out, fields[i], "5", strconv.Itoa(rgbTo256(r, g, bl)))
				i += 4
				continue
			}
		}
		out = append(out, fields[i])
	}
	return fmt.Sprintf("\033[%sm", strings.Join(out, ";"))
}
