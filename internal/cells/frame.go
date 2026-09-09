// Package cells turns mmaid's ANSI output into a grid of coloured cells,
// reads and writes the .cells interchange format, compares a frame with a
// recorded reference, and lints the glyph grid for structural defects.
package cells

// Cell is one terminal cell: a codepoint and the colours it resolved to.
// A wide character occupies its first cell; each continuation cell carries
// codepoint 0.
type Cell struct {
	Cp rune
	Fg [3]uint8
	Bg [3]uint8
}

// Frame is a W by H grid of cells in row-major order.
type Frame struct {
	W, H  int
	Cells []Cell
}

// DefaultFg and DefaultBg are the colours of an unstyled cell.
var (
	DefaultFg = [3]uint8{229, 229, 229}
	DefaultBg = [3]uint8{0, 0, 0}
)

// Blank is a space in the default colours.
func Blank() Cell { return Cell{Cp: ' ', Fg: DefaultFg, Bg: DefaultBg} }

// NewFrame returns a w by h frame of blank cells.
func NewFrame(w, h int) *Frame {
	f := &Frame{W: w, H: h, Cells: make([]Cell, w*h)}
	for i := range f.Cells {
		f.Cells[i] = Blank()
	}
	return f
}

// At returns the cell at (row, col).
func (f *Frame) At(row, col int) Cell { return f.Cells[row*f.W+col] }

// Set writes the cell at (row, col).
func (f *Frame) Set(row, col int, c Cell) { f.Cells[row*f.W+col] = c }
