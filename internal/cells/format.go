package cells

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Write emits f in the .cells text format: a header line "W H", then one line
// per cell in row-major order, "cp fr fg fb br bg bb".
func Write(w io.Writer, f *Frame) error {
	b := bufio.NewWriter(w)
	if _, err := fmt.Fprintf(b, "%d %d\n", f.W, f.H); err != nil {
		return err
	}
	for _, c := range f.Cells {
		if _, err := fmt.Fprintf(b, "%d %d %d %d %d %d %d\n",
			c.Cp, c.Fg[0], c.Fg[1], c.Fg[2], c.Bg[0], c.Bg[1], c.Bg[2]); err != nil {
			return err
		}
	}
	return b.Flush()
}

// Read parses the .cells text format.
func Read(r io.Reader) (*Frame, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)

	if !sc.Scan() {
		return nil, fmt.Errorf("no header line")
	}
	head := strings.Fields(sc.Text())
	if len(head) != 2 {
		return nil, fmt.Errorf("header %q is not \"W H\"", sc.Text())
	}
	w, err := strconv.Atoi(head[0])
	if err != nil {
		return nil, fmt.Errorf("header width %q: %w", head[0], err)
	}
	h, err := strconv.Atoi(head[1])
	if err != nil {
		return nil, fmt.Errorf("header height %q: %w", head[1], err)
	}
	if w < 0 || h < 0 || (w != 0 && h > (1<<26)/w) {
		return nil, fmt.Errorf("implausible size %dx%d", w, h)
	}

	f := &Frame{W: w, H: h, Cells: make([]Cell, w*h)}
	for i := range f.Cells {
		if !sc.Scan() {
			if err := sc.Err(); err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("cell %d of %d: file ends early", i, len(f.Cells))
		}
		c, err := parseCell(sc.Text())
		if err != nil {
			return nil, fmt.Errorf("cell %d: %w", i, err)
		}
		f.Cells[i] = c
	}
	if sc.Scan() && strings.TrimSpace(sc.Text()) != "" {
		return nil, fmt.Errorf("trailing data after %d cells", len(f.Cells))
	}
	return f, sc.Err()
}

// parseCell reads one "cp fr fg fb br bg bb" line.
func parseCell(line string) (Cell, error) {
	fields := strings.Fields(line)
	if len(fields) != 7 {
		return Cell{}, fmt.Errorf("%q has %d fields, want 7", line, len(fields))
	}
	var n [7]int
	for i, f := range fields {
		v, err := strconv.Atoi(f)
		if err != nil {
			return Cell{}, fmt.Errorf("field %d of %q: %w", i, line, err)
		}
		n[i] = v
	}
	if n[0] < 0 || n[0] > 0x10FFFF {
		return Cell{}, fmt.Errorf("U+%X is not a codepoint", n[0])
	}
	for i := 1; i < 7; i++ {
		if n[i] < 0 || n[i] > 255 {
			return Cell{}, fmt.Errorf("colour value %d is out of range", n[i])
		}
	}
	return Cell{
		Cp: rune(n[0]),
		Fg: [3]uint8{uint8(n[1]), uint8(n[2]), uint8(n[3])},
		Bg: [3]uint8{uint8(n[4]), uint8(n[5]), uint8(n[6])},
	}, nil
}
