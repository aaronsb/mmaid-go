package cells

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/aaronsb/mmaid-go/internal/textwidth"
)

// advance returns the number of cells a rune occupies (ADR-401). A rune of
// width 0 leaves no cell at all.
func advance(r rune) int { return textwidth.Rune(r) }

// palette16 is the xterm rendering of the eight named colours (SGR 30-37)
// followed by their bright variants (SGR 90-97).
var palette16 = [16][3]uint8{
	{0, 0, 0}, {205, 0, 0}, {0, 205, 0}, {205, 205, 0},
	{0, 0, 238}, {205, 0, 205}, {0, 205, 205}, {229, 229, 229},
	{127, 127, 127}, {255, 0, 0}, {0, 255, 0}, {255, 255, 0},
	{92, 92, 255}, {255, 0, 255}, {0, 255, 255}, {255, 255, 255},
}

// cubeLevels are the six channel values of the 216-colour cube.
var cubeLevels = [6]uint8{0, 95, 135, 175, 215, 255}

// xterm256 maps a 256-colour index to RGB.
func xterm256(n int) [3]uint8 {
	switch {
	case n < 16:
		return palette16[n]
	case n < 232:
		n -= 16
		return [3]uint8{cubeLevels[n/36], cubeLevels[(n/6)%6], cubeLevels[n%6]}
	default:
		v := uint8(8 + (n-232)*10)
		return [3]uint8{v, v, v}
	}
}

// state is the terminal's SGR state while the stream is read.
type state struct {
	fg      [3]uint8
	bg      [3]uint8
	fgNamed int // index into the eight named colours, or -1
	bold    bool
	dim     bool
}

func newState() state {
	return state{fg: DefaultFg, bg: DefaultBg, fgNamed: -1}
}

// cellFg folds bold and dim into the foreground the way a classic terminal
// does: bold on a named colour selects the bright variant, dim then scales to
// 60 percent.
func (s *state) cellFg() [3]uint8 {
	fg := s.fg
	if s.bold && s.fgNamed >= 0 {
		fg = palette16[s.fgNamed+8]
	}
	if s.dim {
		for i := range fg {
			fg[i] = uint8(math.Round(float64(fg[i]) * 0.6))
		}
	}
	return fg
}

// Interpret reads an ANSI stream as a frame. It understands the SGR codes
// mmaid emits and skips OSC sequences; any other escape is an error naming
// its byte offset. Lines split on '\n', the frame is as wide as the widest
// line, and short lines are padded with default cells.
func Interpret(ansi string) (*Frame, error) {
	st := newState()
	var lines [][]Cell
	var cur []Cell

	for i := 0; i < len(ansi); {
		switch b := ansi[i]; {
		case b == '\n':
			lines = append(lines, cur)
			cur = nil
			i++
		case b == 0x1b:
			n, err := st.escape(ansi, i)
			if err != nil {
				return nil, err
			}
			i += n
		default:
			r, size := utf8.DecodeRuneInString(ansi[i:])
			i += size
			w := advance(r)
			if w == 0 {
				continue
			}
			c := Cell{Cp: r, Fg: st.cellFg(), Bg: st.bg}
			cur = append(cur, c)
			for k := w; k > 1; k-- {
				cur = append(cur, Cell{Cp: 0, Fg: c.Fg, Bg: c.Bg})
			}
		}
	}
	lines = append(lines, cur)

	w := 0
	for _, l := range lines {
		if len(l) > w {
			w = len(l)
		}
	}
	f := NewFrame(w, len(lines))
	for row, l := range lines {
		copy(f.Cells[row*w:], l)
	}
	return f, nil
}

// escape consumes the escape sequence starting at ansi[at] and returns its
// length in bytes.
func (s *state) escape(ansi string, at int) (int, error) {
	rest := ansi[at:]
	if len(rest) < 2 {
		return 0, fmt.Errorf("truncated escape at byte %d: %q", at, rest)
	}
	switch rest[1] {
	case '[':
		j := 2
		for j < len(rest) && rest[j] >= 0x30 && rest[j] <= 0x3f {
			j++
		}
		for j < len(rest) && rest[j] >= 0x20 && rest[j] <= 0x2f {
			j++
		}
		if j >= len(rest) {
			return 0, fmt.Errorf("truncated escape at byte %d: %q", at, rest)
		}
		seq := rest[:j+1]
		if rest[j] != 'm' {
			return 0, fmt.Errorf("unsupported escape at byte %d: %q", at, seq)
		}
		if err := s.sgr(rest[2:j]); err != nil {
			return 0, fmt.Errorf("unsupported escape at byte %d: %q: %w", at, seq, err)
		}
		return j + 1, nil
	case ']':
		for j := 2; j < len(rest); j++ {
			if rest[j] == 0x07 {
				return j + 1, nil
			}
			if rest[j] == 0x1b && j+1 < len(rest) && rest[j+1] == '\\' {
				return j + 2, nil
			}
		}
		return 0, fmt.Errorf("unterminated OSC at byte %d: %q", at, rest)
	default:
		return 0, fmt.Errorf("unsupported escape at byte %d: %q", at, rest[:2])
	}
}

// sgr applies the parameters of one SGR sequence.
func (s *state) sgr(params string) error {
	fields := strings.Split(params, ";")
	for i := 0; i < len(fields); i++ {
		n, err := sgrNum(fields[i])
		if err != nil {
			return err
		}
		switch {
		case n == 0:
			*s = newState()
		case n == 1:
			s.bold = true
		case n == 2:
			s.dim = true
		case n == 3, n == 23:
			// Italic has no representation in a frame.
		case n == 22:
			s.bold, s.dim = false, false
		case n >= 30 && n <= 37:
			s.fg, s.fgNamed = palette16[n-30], n-30
		case n == 39:
			s.fg, s.fgNamed = DefaultFg, -1
		case n >= 40 && n <= 47:
			s.bg = palette16[n-40]
		case n == 49:
			s.bg = DefaultBg
		case n >= 90 && n <= 97:
			s.fg, s.fgNamed = palette16[n-90+8], -1
		case n >= 100 && n <= 107:
			s.bg = palette16[n-100+8]
		case n == 38 || n == 48:
			rgb, used, err := extended(fields[i:])
			if err != nil {
				return err
			}
			if n == 38 {
				s.fg, s.fgNamed = rgb, -1
			} else {
				s.bg = rgb
			}
			i += used - 1
		default:
			return fmt.Errorf("SGR %d is not supported", n)
		}
	}
	return nil
}

// extended reads a 38 or 48 colour selector; fields[0] is the 38 or 48 itself.
// It returns the colour and how many fields it consumed.
func extended(fields []string) ([3]uint8, int, error) {
	var zero [3]uint8
	if len(fields) < 2 {
		return zero, 0, fmt.Errorf("SGR %s names no colour space", fields[0])
	}
	space, err := sgrNum(fields[1])
	if err != nil {
		return zero, 0, err
	}
	switch space {
	case 2:
		if len(fields) < 5 {
			return zero, 0, fmt.Errorf("SGR %s;2 needs three channels", fields[0])
		}
		var rgb [3]uint8
		for k := 0; k < 3; k++ {
			v, err := channel(fields[2+k])
			if err != nil {
				return zero, 0, err
			}
			rgb[k] = v
		}
		return rgb, 5, nil
	case 5:
		if len(fields) < 3 {
			return zero, 0, fmt.Errorf("SGR %s;5 needs an index", fields[0])
		}
		v, err := channel(fields[2])
		if err != nil {
			return zero, 0, err
		}
		return xterm256(int(v)), 3, nil
	default:
		return zero, 0, fmt.Errorf("SGR %s;%d is not a colour space", fields[0], space)
	}
}

// sgrNum parses one SGR parameter; an empty parameter is 0.
func sgrNum(s string) (int, error) {
	if s == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("SGR parameter %q is not a number", s)
	}
	return n, nil
}

// channel parses a colour channel or palette index in 0-255.
func channel(s string) (uint8, error) {
	n, err := sgrNum(s)
	if err != nil {
		return 0, err
	}
	if n < 0 || n > 255 {
		return 0, fmt.Errorf("colour value %d is out of range", n)
	}
	return uint8(n), nil
}
