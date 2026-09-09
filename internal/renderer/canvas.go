package renderer

import (
	"strings"

	"github.com/aaronsb/mmaid-go/internal/glyph"
	"github.com/aaronsb/mmaid-go/internal/textwidth"
)

// Continuation occupies the second cell of a wide rune. It is a literal the
// resolver skips and never carries arms; it is never emitted.
const Continuation rune = -1

// lineCell is the line layer of one cell: which sides a line leaves through,
// its weight, and whether a two-arm corner draws rounded.
type lineCell struct {
	arms    glyph.Arms
	weight  glyph.Weight
	rounded bool
}

// Canvas is a 2D character grid with a line layer beside it. Lines are drawn
// as arms and resolve to glyphs in a final pass; anything written with Put is
// a literal and wins over the arms in its cell.
// Indexing is row-major: grid[row][col].
type Canvas struct {
	Width     int
	Height    int
	grid      [][]rune
	styleGrid [][]string
	fillGrid  [][]string // background fill layer (composed with styleGrid in ToColorString)
	linkGrid  [][]string // hyperlink layer: the URL a cell belongs to, if any
	lines     [][]lineCell
	literal   [][]bool
	cs        CharSet

	// currentLink is stamped on every cell of text written while it is set,
	// so a node's label carries its link without the shape renderers knowing
	// anything about links.
	currentLink string

	// hyperlinks says whether serialization emits the link layer as OSC 8.
	hyperlinks bool
}

// NewCanvas creates a Canvas of the given width and height, filled with spaces.
// Lines resolve through UNICODE until SetCharSet says otherwise.
func NewCanvas(width, height int) *Canvas {
	c := &Canvas{cs: UNICODE}
	c.grid = make([][]rune, 0, height)
	c.styleGrid = make([][]string, 0, height)
	c.fillGrid = make([][]string, 0, height)
	c.linkGrid = make([][]string, 0, height)
	c.lines = make([][]lineCell, 0, height)
	c.literal = make([][]bool, 0, height)
	for range height {
		c.appendRow(width)
	}
	c.Width = width
	c.Height = height
	return c
}

func (c *Canvas) appendRow(width int) {
	row := make([]rune, width)
	srow := make([]string, width)
	for i := range width {
		row[i] = ' '
		srow[i] = "default"
	}
	c.grid = append(c.grid, row)
	c.styleGrid = append(c.styleGrid, srow)
	c.fillGrid = append(c.fillGrid, make([]string, width))
	c.linkGrid = append(c.linkGrid, make([]string, width))
	c.lines = append(c.lines, make([]lineCell, width))
	c.literal = append(c.literal, make([]bool, width))
}

// SetCharSet selects the tables the line layer resolves through.
func (c *Canvas) SetCharSet(cs CharSet) {
	c.cs = cs
}

func (c *Canvas) inBounds(row, col int) bool {
	return row >= 0 && row < c.Height && col >= 0 && col < c.Width
}

// Get returns the glyph at (row, col) as it will render: a literal, or the
// resolved line glyph. Out-of-bounds returns a space.
func (c *Canvas) Get(row, col int) rune {
	if !c.inBounds(row, col) {
		return ' '
	}
	if lc := c.lines[row][col]; lc.arms != 0 && !c.literal[row][col] {
		return c.cs.Glyph(lc.arms, lc.weight, lc.rounded)
	}
	return c.grid[row][col]
}

// Arms returns the arm bits at (row, col).
func (c *Canvas) Arms(row, col int) glyph.Arms {
	if !c.inBounds(row, col) {
		return 0
	}
	return c.lines[row][col].arms
}

// clearRightHalf blanks a continuation cell orphaned by a write at (row, col).
func (c *Canvas) clearRightHalf(row, col int) {
	if col+1 < c.Width && c.grid[row][col+1] == Continuation {
		c.grid[row][col+1] = ' '
		c.styleGrid[row][col+1] = "default"
		c.literal[row][col+1] = false
	}
}

// clearLeftHalf blanks the wide rune whose continuation cell (row, col) is
// being overwritten.
func (c *Canvas) clearLeftHalf(row, col int) {
	if c.grid[row][col] == Continuation && col > 0 {
		c.grid[row][col-1] = ' '
		c.styleGrid[row][col-1] = "default"
		c.literal[row][col-1] = false
	}
}

// Put writes a literal glyph. A literal wins over any arms in its cell.
// Spaces and out-of-bounds writes are silently ignored.
func (c *Canvas) Put(row, col int, ch rune, style string) {
	if !c.inBounds(row, col) || ch == ' ' {
		return
	}
	c.clearRightHalf(row, col)
	if ch != Continuation {
		c.clearLeftHalf(row, col)
	}
	c.grid[row][col] = ch
	c.literal[row][col] = true
	if style != "" {
		c.styleGrid[row][col] = style
	}
}

// PutBox writes a box-drawing rune as the arms it draws, so it merges with
// other lines through the cell; any other rune is a literal.
func (c *Canvas) PutBox(row, col int, ch rune, style string) {
	if a, w, rounded, ok := glyph.Of(ch); ok {
		c.Arm(row, col, a, w, rounded, style)
		return
	}
	c.Put(row, col, ch, style)
}

// Arm ORs arm bits into a cell. The first arm into an empty cell sets its
// style and weight; later arms keep the style and merge to the heavier
// weight, dashed yielding to any solid stroke. A literal already in the cell
// keeps its style, as it keeps its glyph. Rounded is set when any
// contributor asks.
func (c *Canvas) Arm(row, col int, a glyph.Arms, w glyph.Weight, rounded bool, style string) {
	if !c.inBounds(row, col) || a == 0 || c.grid[row][col] == Continuation {
		return
	}
	lc := &c.lines[row][col]
	if lc.arms == 0 {
		lc.weight = w
		if style != "" && !c.literal[row][col] {
			c.styleGrid[row][col] = style
		}
	} else if glyph.Heavier(w, lc.weight) {
		lc.weight = w
	}
	lc.arms |= a
	lc.rounded = lc.rounded || rounded
}

// Segment draws a straight line between two cells. Endpoint cells get only
// the arm pointing inward; interior cells get both along-axis arms. A single
// cell gets both horizontal arms; a diagonal draws nothing.
func (c *Canvas) Segment(r1, c1, r2, c2 int, w glyph.Weight, rounded bool, style string) {
	switch {
	case r1 == r2 && c1 == c2:
		c.Arm(r1, c1, glyph.Horizontal, w, rounded, style)
	case r1 == r2 && c1 != c2:
		if c1 > c2 {
			c1, c2 = c2, c1
		}
		c.Arm(r1, c1, glyph.E, w, rounded, style)
		for col := c1 + 1; col < c2; col++ {
			c.Arm(r1, col, glyph.Horizontal, w, rounded, style)
		}
		c.Arm(r1, c2, glyph.W, w, rounded, style)
	case c1 == c2 && r1 != r2:
		if r1 > r2 {
			r1, r2 = r2, r1
		}
		c.Arm(r1, c1, glyph.S, w, rounded, style)
		for row := r1 + 1; row < r2; row++ {
			c.Arm(row, c1, glyph.Vertical, w, rounded, style)
		}
		c.Arm(r2, c1, glyph.N, w, rounded, style)
	}
}

// PutText places a string starting at (row, col).
func (c *Canvas) PutText(row, col int, text string, style string) {
	offset := 0
	for _, ch := range text {
		offset += c.putWide(row, col+offset, ch, style)
	}
}

// putWide writes one rune and, when it is two columns wide, its continuation
// cell. A rune of no width leaves no cell. It returns the columns consumed.
func (c *Canvas) putWide(row, col int, ch rune, style string) int {
	w := textwidth.Rune(ch)
	if w == 0 {
		return 0
	}
	// A wide rune in the last column needs one more for its continuation.
	if w == 2 && row >= 0 && row < c.Height && col == c.Width-1 {
		c.Resize(c.Width+1, c.Height)
	}
	c.Put(row, col, ch, style)
	if w == 2 {
		c.Put(row, col+1, Continuation, style)
		if c.inBounds(row, col+1) {
			c.lines[row][col+1] = lineCell{}
		}
	}
	if c.currentLink != "" {
		c.SetLink(row, col, c.currentLink)
		if w == 2 {
			c.SetLink(row, col+1, c.currentLink)
		}
	}
	return w
}

// PutStyledText places text with per-segment style keys.
// Each segment is a (text, style) pair.
func (c *Canvas) PutStyledText(row, col int, segments []StyledSegment) {
	offset := 0
	for _, seg := range segments {
		for _, ch := range seg.Text {
			offset += c.putWide(row, col+offset, ch, seg.Style)
		}
	}
}

// StyledSegment is a run of text with an associated style key.
type StyledSegment struct {
	Text  string
	Style string
}

// ClearCell sets a cell back to a space with default style and no arms.
// Clearing either half of a wide rune clears both.
func (c *Canvas) ClearCell(row, col int) {
	if !c.inBounds(row, col) {
		return
	}
	c.clearRightHalf(row, col)
	c.clearLeftHalf(row, col)
	c.grid[row][col] = ' '
	c.styleGrid[row][col] = "default"
	c.linkGrid[row][col] = ""
	c.lines[row][col] = lineCell{}
	c.literal[row][col] = false
}

// SetFill sets a background fill style at (row, col).
// This is a separate layer that composes with the cell's content style in ToColorString.
// Content drawn on top keeps its foreground; the fill provides the background.
func (c *Canvas) SetFill(row, col int, fill string) {
	if !c.inBounds(row, col) {
		return
	}
	c.fillGrid[row][col] = fill
}

// SetLink records that the cell at (row, col) belongs to a hyperlink. An empty
// URL clears it.
func (c *Canvas) SetLink(row, col int, url string) {
	if !c.inBounds(row, col) {
		return
	}
	c.linkGrid[row][col] = url
}

// GetLink returns the URL the cell at (row, col) belongs to, or "".
func (c *Canvas) GetLink(row, col int) string {
	if !c.inBounds(row, col) {
		return ""
	}
	return c.linkGrid[row][col]
}

// SetCurrentLink makes the following text belong to a hyperlink. Callers set it
// around the drawing of one node and clear it afterwards.
func (c *Canvas) SetCurrentLink(url string) {
	c.currentLink = url
}

// SetHyperlinks says whether serialization emits the link layer as OSC 8. The
// links are recorded either way, so the decision belongs to the caller that
// knows the setting.
func (c *Canvas) SetHyperlinks(on bool) {
	c.hyperlinks = on
}

// GetFill returns the fill style at (row, col).
func (c *Canvas) GetFill(row, col int) string {
	if !c.inBounds(row, col) {
		return ""
	}
	return c.fillGrid[row][col]
}

// SetStyle sets the style key at (row, col) without changing the character.
func (c *Canvas) SetStyle(row, col int, style string) {
	if !c.inBounds(row, col) {
		return
	}
	if style != "" {
		c.styleGrid[row][col] = style
	}
}

// GetStyle returns the style key at (row, col).
func (c *Canvas) GetStyle(row, col int) string {
	if !c.inBounds(row, col) {
		return "default"
	}
	return c.styleGrid[row][col]
}

// DrawHorizontal draws a horizontal segment from colStart to colEnd
// (inclusive).
func (c *Canvas) DrawHorizontal(row, colStart, colEnd int, w glyph.Weight, style string) {
	c.Segment(row, colStart, row, colEnd, w, false, style)
}

// DrawVertical draws a vertical segment from rowStart to rowEnd (inclusive).
// A single cell gets both vertical arms, the axis being known here.
func (c *Canvas) DrawVertical(col, rowStart, rowEnd int, w glyph.Weight, style string) {
	if rowStart == rowEnd {
		c.Arm(rowStart, col, glyph.Vertical, w, false, style)
		return
	}
	c.Segment(rowStart, col, rowEnd, col, w, false, style)
}

// Resize expands the canvas to at least the given dimensions.
func (c *Canvas) Resize(newWidth, newHeight int) {
	if newWidth <= c.Width && newHeight <= c.Height {
		return
	}
	w := max(c.Width, newWidth)
	h := max(c.Height, newHeight)
	for r := range c.Height {
		for range w - c.Width {
			c.grid[r] = append(c.grid[r], ' ')
			c.styleGrid[r] = append(c.styleGrid[r], "default")
			c.fillGrid[r] = append(c.fillGrid[r], "")
			c.linkGrid[r] = append(c.linkGrid[r], "")
			c.lines[r] = append(c.lines[r], lineCell{})
			c.literal[r] = append(c.literal[r], false)
		}
	}
	for range h - c.Height {
		c.appendRow(w)
	}
	c.Width = w
	c.Height = h
}

// Resolve writes one glyph from the canvas's charset into every armed cell
// that holds no literal. It is idempotent and runs before every
// serialization.
func (c *Canvas) Resolve() {
	for r := range c.Height {
		for col := range c.Width {
			lc := c.lines[r][col]
			if lc.arms == 0 || c.literal[r][col] {
				continue
			}
			c.grid[r][col] = c.cs.Glyph(lc.arms, lc.weight, lc.rounded)
		}
	}
}

// ToString renders the canvas to a string, trimming trailing whitespace.
func (c *Canvas) ToString() string {
	c.Resolve()
	lines := make([]string, c.Height)
	for y := range c.Height {
		var b strings.Builder
		open := ""
		for x := range c.Width {
			if c.grid[y][x] == Continuation {
				continue
			}
			open = c.writeLink(&b, y, x, open)
			b.WriteRune(c.grid[y][x])
		}
		if open != "" {
			b.WriteString(oscClose)
		}
		lines[y] = strings.TrimRight(b.String(), " ")
	}
	// Trim trailing empty lines
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}

// flipVerticalMap maps literal runes to their vertically flipped counterparts.
// Lines flip through their arm bits.
var flipVerticalMap = map[rune]rune{
	'▼': '▲', '▲': '▼',
	'▽': '△', '△': '▽',
	'v': '^', '^': 'v',
	'╱': '╲', '╲': '╱',
	'/': '\\', '\\': '/',
}

// flipHorizontalMap maps literal runes to their horizontally flipped counterparts.
var flipHorizontalMap = map[rune]rune{
	'►': '◄', '◄': '►',
	'▷': '◁', '◁': '▷',
	'>': '<', '<': '>',
	'╱': '╲', '╲': '╱',
	'/': '\\', '\\': '/',
	'(': ')', ')': '(',
}

// FlipVertical flips the canvas vertically: rows reversed, north and south
// arms swapped, literals remapped.
func (c *Canvas) FlipVertical() {
	for i, j := 0, c.Height-1; i < j; i, j = i+1, j-1 {
		c.grid[i], c.grid[j] = c.grid[j], c.grid[i]
		c.styleGrid[i], c.styleGrid[j] = c.styleGrid[j], c.styleGrid[i]
		c.fillGrid[i], c.fillGrid[j] = c.fillGrid[j], c.fillGrid[i]
		c.lines[i], c.lines[j] = c.lines[j], c.lines[i]
		c.literal[i], c.literal[j] = c.literal[j], c.literal[i]
	}
	for r := range c.Height {
		for col := range c.Width {
			c.lines[r][col].arms = swapArms(c.lines[r][col].arms, glyph.N, glyph.S)
			if mapped, ok := flipVerticalMap[c.grid[r][col]]; ok && c.literal[r][col] {
				c.grid[r][col] = mapped
			}
		}
	}
}

// FlipHorizontal flips the canvas horizontally: columns reversed, east and
// west arms swapped, literals remapped.
func (c *Canvas) FlipHorizontal() {
	for r := range c.Height {
		for i, j := 0, c.Width-1; i < j; i, j = i+1, j-1 {
			c.grid[r][i], c.grid[r][j] = c.grid[r][j], c.grid[r][i]
			c.styleGrid[r][i], c.styleGrid[r][j] = c.styleGrid[r][j], c.styleGrid[r][i]
			c.fillGrid[r][i], c.fillGrid[r][j] = c.fillGrid[r][j], c.fillGrid[r][i]
			c.lines[r][i], c.lines[r][j] = c.lines[r][j], c.lines[r][i]
			c.literal[r][i], c.literal[r][j] = c.literal[r][j], c.literal[r][i]
		}
		// Reversal puts each continuation cell before its wide rune
		for col := 0; col < c.Width-1; col++ {
			if c.grid[r][col] == Continuation && textwidth.Rune(c.grid[r][col+1]) == 2 {
				c.grid[r][col], c.grid[r][col+1] = c.grid[r][col+1], c.grid[r][col]
				c.styleGrid[r][col], c.styleGrid[r][col+1] = c.styleGrid[r][col+1], c.styleGrid[r][col]
			}
		}
		for col := range c.Width {
			c.lines[r][col].arms = swapArms(c.lines[r][col].arms, glyph.E, glyph.W)
			if mapped, ok := flipHorizontalMap[c.grid[r][col]]; ok && c.literal[r][col] {
				c.grid[r][col] = mapped
			}
		}
	}
}

// swapArms exchanges the two given arm bits in a.
func swapArms(a, x, y glyph.Arms) glyph.Arms {
	out := a &^ (x | y)
	if a&x != 0 {
		out |= y
	}
	if a&y != 0 {
		out |= x
	}
	return out
}

// StyledPair holds a rune and its associated style string.
type StyledPair struct {
	Char  rune
	Style string
}

// ToStyledPairs returns the canvas content as a 2D slice of StyledPairs.
func (c *Canvas) ToStyledPairs() [][]StyledPair {
	c.Resolve()
	result := make([][]StyledPair, c.Height)
	for y := range c.Height {
		row := make([]StyledPair, 0, c.Width)
		for x := range c.Width {
			if c.grid[y][x] == Continuation {
				continue
			}
			row = append(row, StyledPair{Char: c.grid[y][x], Style: c.styleGrid[y][x]})
		}
		result[y] = row
	}
	return result
}
