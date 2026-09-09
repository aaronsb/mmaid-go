package routing

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/aaronsb/mmaid-go/internal/graph"
	"github.com/aaronsb/mmaid-go/internal/layout"
	"github.com/aaronsb/mmaid-go/internal/textwidth"
)

// labelSpace is the draw-space bookkeeping label placement needs: the cells
// no label may cover, the cells edges have drawn lines through so far, the
// node rectangles a label keeps away from, and the labels placed so far
// with the grid cells they hold.
type labelSpace struct {
	l       *layout.GridLayout
	blocked map[Point]bool
	lines   map[Point]bool
	nodes   []rect
	// aprons are the grid cells a node side's ports step into; a label
	// never reserves one, so labels do not steer which side an edge uses.
	aprons map[layout.GridCoord]bool
	// held counts the labels reserving each grid cell.
	held   map[layout.GridCoord]int
	placed map[int]*placement // by RoutedEdge.Index
	warned map[int]bool
	warn   io.Writer
}

// placement is where one label sits and what it reserved.
type placement struct {
	row, col, w int
	cells       []layout.GridCoord
}

// rect is an inclusive draw-space rectangle.
type rect struct {
	x0, y0, x1, y1 int
}

func newLabelSpace(g *graph.Graph, l *layout.GridLayout, aprons map[layout.GridCoord]bool, warn io.Writer) *labelSpace {
	s := &labelSpace{
		l:       l,
		blocked: make(map[Point]bool),
		lines:   make(map[Point]bool),
		aprons:  aprons,
		held:    make(map[layout.GridCoord]int),
		placed:  make(map[int]*placement),
		warned:  make(map[int]bool),
		warn:    warn,
	}
	for _, nid := range g.NodeOrder {
		p, ok := l.Placements[nid]
		if !ok {
			continue
		}
		r := rect{p.DrawX, p.DrawY, p.DrawX + p.DrawWidth - 1, p.DrawY + p.DrawHeight - 1}
		s.nodes = append(s.nodes, r)
		s.block(r)
	}
	for _, sb := range l.SubgraphBounds {
		if sb.Width <= 0 || sb.Height <= 0 {
			continue
		}
		x1, y1 := sb.X+sb.Width-1, sb.Y+sb.Height-1
		s.block(rect{sb.X, sb.Y, x1, sb.Y})
		s.block(rect{sb.X, y1, x1, y1})
		s.block(rect{sb.X, sb.Y, sb.X, y1})
		s.block(rect{x1, sb.Y, x1, y1})
		title := sb.Subgraph.Label
		if title == "" {
			title = sb.Subgraph.ID
		}
		if w := textwidth.String(title); w > 0 {
			s.block(rect{sb.X + 2, sb.Y + 1, sb.X + 1 + w, sb.Y + 1})
		}
	}
	return s
}

func (s *labelSpace) block(r rect) {
	for y := r.y0; y <= r.y1; y++ {
		for x := r.x0; x <= r.x1; x++ {
			s.blocked[Point{x, y}] = true
		}
	}
}

// addLines records every cell of a draw path and returns the indices of
// the labels the path runs under, which must move.
func (s *labelSpace) addLines(path []Point) []int {
	var hit []int
	for i := 1; i < len(path); i++ {
		a, b := path[i-1], path[i]
		dx, dy := sign(b.Col-a.Col), sign(b.Row-a.Row)
		for p := a; ; p = (Point{p.Col + dx, p.Row + dy}) {
			s.lines[p] = true
			if idx, ok := s.labelAt(p); ok && !contains(hit, idx) {
				hit = append(hit, idx)
			}
			if p == b {
				break
			}
		}
	}
	return hit
}

// labelAt returns the index of the label covering a cell.
func (s *labelSpace) labelAt(p Point) (int, bool) {
	for idx, pl := range s.placed {
		if p.Row == pl.row && p.Col >= pl.col && p.Col < pl.col+pl.w {
			return idx, true
		}
	}
	return 0, false
}

func contains(xs []int, x int) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

// place finds the edge's label a home beside its path and reserves the
// cells: the last segment first, then each earlier one; the whole label
// first, then its first word with the ellipsis; never over a line, a node,
// a subgraph border or title, or another label. A label that fits nowhere
// is dropped with a warning. Placing an edge's label again first releases
// what it held.
func (s *labelSpace) place(re *RoutedEdge, ellipsis string) {
	s.release(re)
	if re.Label == "" || len(re.DrawPath) < 2 {
		return
	}
	texts := []string{re.Label}
	if words := strings.Fields(re.Label); len(words) > 1 {
		texts = append(texts, words[0]+ellipsis)
	}
	arrows := arrowCells(re)
	for _, text := range texts {
		if row, col, ok := s.find(re.DrawPath, arrows, text); ok {
			s.commit(re, text, row, col)
			return
		}
	}
	if s.warn != nil && !s.warned[re.Index] {
		s.warned[re.Index] = true
		fmt.Fprintf(s.warn, "mmaid: edge label %q dropped\n", re.Label)
	}
}

// commit records a placement on the edge, blocks its cells, and reserves
// the grid cells under it that are not aprons.
func (s *labelSpace) commit(re *RoutedEdge, text string, row, col int) {
	w := textwidth.String(text)
	re.LabelText, re.LabelRow, re.LabelCol = text, row, col
	s.block(rect{col, row, col + w - 1, row})
	pl := &placement{row: row, col: col, w: w}
	c0, r0 := s.l.DrawToGrid(col, row)
	c1, _ := s.l.DrawToGrid(col+w-1, row)
	for c := c0; c <= c1; c++ {
		gc := layout.GridCoord{Col: c, Row: r0}
		if s.aprons[gc] {
			continue
		}
		s.held[gc]++
		s.l.Reserve(c, r0)
		pl.cells = append(pl.cells, gc)
	}
	s.placed[re.Index] = pl
}

// release gives back the cells an edge's label held.
func (s *labelSpace) release(re *RoutedEdge) {
	pl, ok := s.placed[re.Index]
	if !ok {
		return
	}
	delete(s.placed, re.Index)
	for x := pl.col; x < pl.col+pl.w; x++ {
		delete(s.blocked, Point{x, pl.row})
	}
	for _, gc := range pl.cells {
		if s.held[gc]--; s.held[gc] <= 0 {
			delete(s.held, gc)
			delete(s.l.Reserved, gc)
		}
	}
	re.LabelText, re.LabelRow, re.LabelCol = "", 0, 0
}

// arrowCells returns the cells the edge's arrowheads occupy.
func arrowCells(re *RoutedEdge) map[Point]bool {
	out := make(map[Point]bool, 2)
	for _, p := range arrowCellsOf(re.DrawPath, [2]bool{re.Edge.HasArrowStart, re.Edge.HasArrowEnd}) {
		out[p] = true
	}
	return out
}

// stepToward returns the cell one step from a toward b.
func stepToward(a, b Point) Point {
	return Point{a.Col + sign(b.Col-a.Col), a.Row + sign(b.Row-a.Row)}
}

// find tries each segment from the last back to the first and returns the
// first cell the label fits at. A label sits beside a plain run cell: not a
// segment's end, which is a corner or a border, and not an arrowhead. A
// segment with no such cell is skipped.
func (s *labelSpace) find(path []Point, arrows map[Point]bool, text string) (row, col int, ok bool) {
	w := textwidth.String(text)
	if w == 0 {
		return 0, 0, false
	}
	for i := len(path) - 2; i >= 0; i-- {
		a, b := path[i], path[i+1]
		var spots []Point
		switch {
		case a.Col == b.Col:
			rows := plainRun(min(a.Row, b.Row), max(a.Row, b.Row), func(v int) bool { return arrows[Point{a.Col, v}] })
			if rows == nil {
				continue
			}
			spots = s.besideVertical(a.Col, rows, w)
		case a.Row == b.Row:
			cols := plainRun(min(a.Col, b.Col), max(a.Col, b.Col), func(v int) bool { return arrows[Point{v, a.Row}] })
			if cols == nil {
				continue
			}
			spots = s.besideHorizontal(a.Row, cols, w)
		}
		for _, p := range spots {
			if s.fits(p.Row, p.Col, w) {
				return p.Row, p.Col, true
			}
		}
	}
	return 0, 0, false
}

// plainRun returns the coordinates strictly inside lo..hi that are not
// arrowheads, or nil when there are none.
func plainRun(lo, hi int, arrow func(int) bool) []int {
	var out []int
	for v := lo + 1; v < hi; v++ {
		if !arrow(v) {
			out = append(out, v)
		}
	}
	return out
}

// fits reports whether a label of width w at (row, col) covers no blocked
// cell and no line cell.
func (s *labelSpace) fits(row, col, w int) bool {
	if row < 0 || col < 0 {
		return false
	}
	for x := col; x < col+w; x++ {
		p := Point{x, row}
		if s.blocked[p] || s.lines[p] {
			return false
		}
	}
	return true
}

// besideVertical lists the top-left cells for a label beside a vertical
// segment at x whose plain cells are rows: the side away from the nearest
// node first, and on each side the rows from the middle outward.
func (s *labelSpace) besideVertical(x int, rows []int, w int) []Point {
	y0, y1 := rows[0], rows[len(rows)-1]
	right, left := x+1, x-w
	cols := []int{right, left}
	if s.nearest(x, y0, y1, true) < s.nearest(x, y0, y1, false) {
		cols = []int{left, right}
	}
	var out []Point
	for _, c := range cols {
		for _, y := range fromMiddle(rows) {
			out = append(out, Point{c, y})
		}
	}
	return out
}

// besideHorizontal lists the top-left cells for a label beside a horizontal
// segment at y whose plain cells are cols: the side away from the nearest
// node first, and on each side the columns that put the label over a plain
// cell, nearest the run's centre first.
func (s *labelSpace) besideHorizontal(y int, cols []int, w int) []Point {
	x0, x1 := cols[0], cols[len(cols)-1]
	above, below := y-1, y+1
	rows := []int{above, below}
	if s.nearestRow(y, x0, x1, false) < s.nearestRow(y, x0, x1, true) {
		rows = []int{below, above}
	}
	plain := make(map[int]bool, len(cols))
	for _, c := range cols {
		plain[c] = true
	}
	var starts []int
	for c := x0 - w + 1; c <= x1; c++ {
		for x := c; x < c+w; x++ {
			if plain[x] {
				starts = append(starts, c)
				break
			}
		}
	}
	// Twice the distance between the label's centre and the run's.
	dist := func(c int) int { return abs(2*c + w - 1 - x0 - x1) }
	sort.SliceStable(starts, func(a, b int) bool { return dist(starts[a]) < dist(starts[b]) })
	var out []Point
	for _, r := range rows {
		for _, c := range starts {
			out = append(out, Point{c, r})
		}
	}
	return out
}

// nearest returns the distance from a vertical segment to the closest node
// beside it on one side, or a large number when none is.
func (s *labelSpace) nearest(x, y0, y1 int, right bool) int {
	best := 1 << 30
	for _, n := range s.nodes {
		if n.y1 < y0 || n.y0 > y1 {
			continue
		}
		switch {
		case right && n.x0 > x:
			best = min(best, n.x0-x)
		case !right && n.x1 < x:
			best = min(best, x-n.x1)
		}
	}
	return best
}

// nearestRow is nearest for a horizontal segment.
func (s *labelSpace) nearestRow(y, x0, x1 int, below bool) int {
	best := 1 << 30
	for _, n := range s.nodes {
		if n.x1 < x0 || n.x0 > x1 {
			continue
		}
		switch {
		case below && n.y0 > y:
			best = min(best, n.y0-y)
		case !below && n.y1 < y:
			best = min(best, y-n.y1)
		}
	}
	return best
}

// fromMiddle reorders a sorted list to start at its middle and alternate
// outward.
func fromMiddle(xs []int) []int {
	mid := (len(xs) - 1) / 2
	out := []int{xs[mid]}
	for d := 1; mid-d >= 0 || mid+d < len(xs); d++ {
		if mid+d < len(xs) {
			out = append(out, xs[mid+d])
		}
		if mid-d >= 0 {
			out = append(out, xs[mid-d])
		}
	}
	return out
}

func sign(x int) int {
	switch {
	case x > 0:
		return 1
	case x < 0:
		return -1
	}
	return 0
}
