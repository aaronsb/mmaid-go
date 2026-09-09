package diagram

import (
	"fmt"
	"math"
	"math/bits"
	"regexp"
	"strconv"
	"strings"

	"github.com/aaronsb/mmaid-go/internal/glyph"
	"github.com/aaronsb/mmaid-go/internal/renderer"
	"github.com/aaronsb/mmaid-go/internal/textwidth"
)

// Venn (`venn-beta`), from https://mermaid.js.org/syntax/venn.html.
//
// Read: `title`, `set` with a bare or quoted identifier, an optional
// `["Label"]` and an optional `:N` size, and `union` over two or more
// identifiers a `set` line declared, with the same optional label and size.
//
// Skipped: `text` nodes, which need room inside a region the cell grid does
// not have beside the region's own label; `style`, whose fill, stroke and
// opacity are the theme's here.

type vennSet struct {
	id    string
	label string
	size  float64
}

type vennUnion struct {
	mask  int // the sets it covers, one bit per set index
	label string
	size  float64
}

type vennDiagram struct {
	title  string
	sets   []vennSet
	unions []vennUnion
}

var (
	reVennHeader = regexp.MustCompile(`(?i)^venn-beta\b`)
	reVennTitle  = regexp.MustCompile(`(?i)^title\s+(.+)$`)
	reVennSet    = regexp.MustCompile(`(?i)^set\s+(.+)$`)
	reVennUnion  = regexp.MustCompile(`(?i)^union\s+(.+)$`)
	reVennDecl   = regexp.MustCompile(`^(.+?)\s*(?:\[\s*"?([^"\]]*)"?\s*\])?\s*(?::\s*([0-9]*\.?[0-9]+))?$`)
)

// parseVenn parses venn source into a vennDiagram.
func parseVenn(source string) *vennDiagram {
	vd := &vennDiagram{}
	index := map[string]int{}

	for _, raw := range strings.Split(source, "\n") {
		line := strings.TrimSpace(stripLineComment(raw))
		if line == "" || reVennHeader.MatchString(line) {
			continue
		}
		if m := reVennTitle.FindStringSubmatch(line); m != nil {
			vd.title = strings.Trim(strings.TrimSpace(m[1]), `"`)
			continue
		}
		if m := reVennSet.FindStringSubmatch(line); m != nil {
			d := reVennDecl.FindStringSubmatch(strings.TrimSpace(m[1]))
			if d == nil {
				continue
			}
			id := strings.Trim(strings.TrimSpace(d[1]), `"`)
			if id == "" || len(vd.sets) >= 3 {
				continue
			}
			label := d[2]
			if label == "" {
				label = id
			}
			index[id] = len(vd.sets)
			vd.sets = append(vd.sets, vennSet{id: id, label: label, size: parseVennSize(d[3])})
			continue
		}
		if m := reVennUnion.FindStringSubmatch(line); m != nil {
			d := reVennDecl.FindStringSubmatch(strings.TrimSpace(m[1]))
			if d == nil {
				continue
			}
			mask, ok := vennMask(d[1], index)
			if !ok {
				continue
			}
			vd.unions = append(vd.unions, vennUnion{mask: mask, label: d[2], size: parseVennSize(d[3])})
			continue
		}
	}
	return vd
}

// vennMask turns a union's comma-separated member list into a set bitmask.
// A member no `set` line declared makes the whole union unreadable, which is
// what the grammar says it is.
func vennMask(list string, index map[string]int) (int, bool) {
	mask := 0
	for _, name := range strings.Split(list, ",") {
		i, ok := index[strings.Trim(strings.TrimSpace(name), `"`)]
		if !ok {
			return 0, false
		}
		mask |= 1 << i
	}
	return mask, bits.OnesCount(uint(mask)) >= 2
}

func parseVennSize(s string) float64 {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v <= 0 {
		return 0
	}
	return v
}

// vennCircle is one set's disc in half-pixel coordinates, where a column is
// one unit across and a row two units down, so a disc renders round.
type vennCircle struct {
	cx, cy, r float64
}

// circles lays the sets out: one centred, two side by side, three on a
// triangle, each disc's radius scaled by the square root of its size so the
// areas are in proportion.
func (vd *vennDiagram) circles(cx, cy, base float64) []vennCircle {
	n := len(vd.sets)
	maxSize := 0.0
	for _, s := range vd.sets {
		maxSize = math.Max(maxSize, s.size)
	}
	radius := func(i int) float64 {
		if maxSize == 0 || vd.sets[i].size == 0 {
			return base
		}
		return base * math.Max(math.Sqrt(vd.sets[i].size/maxSize), 0.45)
	}
	out := make([]vennCircle, n)
	for i := range n {
		switch n {
		case 1:
			out[i] = vennCircle{cx, cy, radius(i)}
		case 2:
			out[i] = vennCircle{cx + base*0.55*float64(2*i-1), cy, radius(i)}
		default:
			a := -math.Pi/2 + 2*math.Pi*float64(i)/3
			out[i] = vennCircle{cx + base*0.62*math.Cos(a), cy + base*0.62*math.Sin(a), radius(i)}
		}
	}
	return out
}

// maskAt returns the sets covering a half-pixel.
func maskAt(circles []vennCircle, x, y float64) int {
	mask := 0
	for i, c := range circles {
		dx, dy := x-c.cx, y-c.cy
		if dx*dx+dy*dy <= c.r*c.r {
			mask |= 1 << i
		}
	}
	return mask
}

// RenderVenn parses and renders a Mermaid venn diagram.
func RenderVenn(source string, cs renderer.CharSet, useColor bool, theme *renderer.Theme) *renderer.Canvas {
	vd := parseVenn(source)
	if len(vd.sets) == 0 {
		c := renderer.NewCanvas(20, 1)
		c.PutText(0, 0, "[venn] no sets", "default")
		return c
	}

	colors := pieColors
	if theme != nil && theme.HasPieBase() {
		colors = theme.PieColors(len(vd.sets))
	}

	legendW, legendGap := vd.legendWidth(), 3
	base := float64(usableWidth()-legendGap-legendW-4) / 4.6
	base = math.Min(math.Max(base, 7), 15)

	spread := base * 0.62
	halfW := int(math.Ceil(base + spread))
	cx, cy := float64(halfW), float64(halfW)
	plotCols := 2*halfW + 1
	plotRows := (2*halfW + 2) / 2

	titleRows := 0
	if vd.title != "" {
		titleRows = 2
	}
	canvasWidth := plotCols + legendGap + legendW + 1
	canvasHeight := titleRows + max(plotRows, vd.legendHeight()) + 1

	c := renderer.NewCanvas(canvasWidth, canvasHeight)
	c.SetCharSet(cs)
	if vd.title != "" {
		c.PutText(0, max((canvasWidth-textwidth.String(vd.title))/2, 0), vd.title, "bold_label")
	}

	circles := vd.circles(cx, cy, base)
	regionOrder := vd.regionOrder(circles, plotCols, plotRows)

	for row := range plotRows {
		for col := range plotCols {
			top := maskAt(circles, float64(col), float64(row*2))
			bot := maskAt(circles, float64(col), float64(row*2+1))
			if top == 0 && bot == 0 {
				continue
			}
			ch, style := vennCell(top, bot, regionOrder, colors, cs.Fills, useColor)
			c.Put(titleRows+row, col, ch, style)
		}
	}

	vd.drawRegionLabels(c, circles, titleRows, plotCols, plotRows, colors, useColor)
	vd.drawLegend(c, titleRows, plotCols+legendGap, legendW, colors, cs.Fills, regionOrder, useColor)
	return c
}

// vennCell picks the rune and style for one cell from the sets covering its
// two half-pixels. In colour a cell is a half block over two region colours,
// as the pie's circle is; without colour it is the region's own fill.
func vennCell(top, bot int, order map[int]int, colors [][3]int, fills renderer.Fills, useColor bool) (rune, string) {
	if !useColor {
		mask := top
		if mask == 0 {
			mask = bot
		}
		return fills.Bars[order[mask]%len(fills.Bars)], "label"
	}
	switch {
	case top != 0 && bot != 0 && top == bot:
		r, g, b := vennColor(top, colors)
		return fills.Full, fmt.Sprintf("_ansi:\033[38;2;%d;%d;%dm", r, g, b)
	case top != 0 && bot != 0:
		tr, tg, tb := vennColor(top, colors)
		br, bg, bb := vennColor(bot, colors)
		return fills.Upper, fmt.Sprintf("_ansi:\033[38;2;%d;%d;%dm\033[48;2;%d;%d;%dm", tr, tg, tb, br, bg, bb)
	case top != 0:
		r, g, b := vennColor(top, colors)
		return fills.Upper, fmt.Sprintf("_ansi:\033[38;2;%d;%d;%dm", r, g, b)
	default:
		r, g, b := vennColor(bot, colors)
		return fills.Lower, fmt.Sprintf("_ansi:\033[38;2;%d;%d;%dm", r, g, b)
	}
}

// vennColor averages the colours of the sets a region belongs to and lifts
// the result, so an overlap reads as brighter than either set alone.
func vennColor(mask int, colors [][3]int) (int, int, int) {
	var sum [3]int
	n := 0
	for i := range 3 {
		if mask&(1<<i) == 0 {
			continue
		}
		clr := colors[i%len(colors)]
		sum[0] += clr[0]
		sum[1] += clr[1]
		sum[2] += clr[2]
		n++
	}
	if n == 0 {
		return 0, 0, 0
	}
	lift := 1.0 + 0.28*float64(n-1)
	clamp := func(v int) int { return min(int(float64(v)/float64(n)*lift), 255) }
	return clamp(sum[0]), clamp(sum[1]), clamp(sum[2])
}

// regionOrder numbers the regions the layout actually produces, the plain
// sets first and then the overlaps, so each takes the next fill.
func (vd *vennDiagram) regionOrder(circles []vennCircle, cols, rows int) map[int]int {
	seen := map[int]bool{}
	for row := range rows * 2 {
		for col := range cols {
			if m := maskAt(circles, float64(col), float64(row)); m != 0 {
				seen[m] = true
			}
		}
	}
	order := map[int]int{}
	next := 0
	for pop := 1; pop <= 3; pop++ {
		for m := 1; m < 8; m++ {
			if seen[m] && bits.OnesCount(uint(m)) == pop {
				order[m] = next
				next++
			}
		}
	}
	return order
}

// drawRegionLabels writes each set's label in the part of its disc no other
// disc covers, and each union's in the overlap it names.
func (vd *vennDiagram) drawRegionLabels(c *renderer.Canvas, circles []vennCircle, titleRows, cols, rows int, colors [][3]int, useColor bool) {
	type target struct {
		mask  int
		label string
	}
	targets := make([]target, 0, len(vd.sets)+len(vd.unions))
	for i, s := range vd.sets {
		targets = append(targets, target{1 << i, s.label})
	}
	for _, u := range vd.unions {
		if u.label != "" {
			targets = append(targets, target{u.mask, u.label})
		}
	}

	style := "bold_label"
	if useColor {
		style = "_ansi:\033[1m\033[38;2;255;255;255m"
	}
	for _, t := range targets {
		w := textwidth.String(t.label)
		bestRow, bestCol, bestRun := -1, 0, 0
		for row := range rows {
			run, start := 0, 0
			for col := 0; col <= cols; col++ {
				if col < cols && maskAt(circles, float64(col), float64(row*2)) == t.mask {
					if run == 0 {
						start = col
					}
					run++
					continue
				}
				if run > bestRun || (run == bestRun && absInt(row-rows/2) < absInt(bestRow-rows/2)) {
					bestRun, bestRow, bestCol = run, row, start+(run-w)/2
				}
				run = 0
			}
		}
		if bestRow < 0 || bestRun < w {
			continue
		}
		col := max(bestCol, 0)
		if useColor {
			r, g, b := vennColor(t.mask, colors)
			for i := range w {
				c.SetFill(titleRows+bestRow, col+i, fmt.Sprintf("_ansi:\033[48;2;%d;%d;%dm", r, g, b))
			}
		}
		c.PutText(titleRows+bestRow, col, t.label, style)
	}
}

// legendRows lists what the legend names: every set, then every union that
// carries a label or a size.
func (vd *vennDiagram) legendRows() []string {
	out := make([]string, 0, len(vd.sets)+len(vd.unions))
	for _, s := range vd.sets {
		out = append(out, vennLegendText(s.label, s.size))
	}
	for _, u := range vd.unions {
		label := u.label
		if label == "" {
			label = vd.unionName(u.mask)
		}
		out = append(out, vennLegendText(label, u.size))
	}
	return out
}

func vennLegendText(label string, size float64) string {
	if size == 0 {
		return label
	}
	return fmt.Sprintf("%s  %s", label, strconv.FormatFloat(size, 'f', -1, 64))
}

// unionName joins the ids of the sets a union covers.
func (vd *vennDiagram) unionName(mask int) string {
	var parts []string
	for i, s := range vd.sets {
		if mask&(1<<i) != 0 {
			parts = append(parts, s.id)
		}
	}
	return strings.Join(parts, " ∩ ")
}

func (vd *vennDiagram) legendWidth() int {
	w := 0
	for _, row := range vd.legendRows() {
		w = max(w, textwidth.String(row))
	}
	return w + 8
}

func (vd *vennDiagram) legendHeight() int {
	return len(vd.legendRows()) + 2
}

// drawLegend draws the pie's legend box, a swatch per region beside its name.
func (vd *vennDiagram) drawLegend(c *renderer.Canvas, titleRows, left, width int, colors [][3]int, fills renderer.Fills, order map[int]int, useColor bool) {
	rows := vd.legendRows()
	height := len(rows) + 2
	top := titleRows

	c.Put(top, left, '┌', "node")
	c.DrawHorizontal(top, left, left+width-1, glyph.Light, "node")
	c.Put(top, left+width-1, '┐', "node")
	for row := top + 1; row < top+height-1; row++ {
		c.Put(row, left, '│', "node")
		c.Put(row, left+width-1, '│', "node")
	}
	c.Put(top+height-1, left, '└', "node")
	c.DrawHorizontal(top+height-1, left, left+width-1, glyph.Light, "node")
	c.Put(top+height-1, left+width-1, '┘', "node")
	for row := top; row < top+height; row++ {
		for col := left; col < left+width; col++ {
			c.SetFill(row, col, "subgraph_fill")
		}
	}

	masks := make([]int, 0, len(rows))
	for i := range vd.sets {
		masks = append(masks, 1<<i)
	}
	for _, u := range vd.unions {
		masks = append(masks, u.mask)
	}

	for i, text := range rows {
		row := top + 1 + i
		swatch := left + 2
		style := "label"
		ch := fills.Bars[order[masks[i]]%len(fills.Bars)]
		if useColor {
			r, g, b := vennColor(masks[i], colors)
			style = fmt.Sprintf("_ansi:\033[38;2;%d;%d;%dm", r, g, b)
			ch = fills.Full
		}
		c.Put(row, swatch, ch, style)
		c.Put(row, swatch+1, ch, style)
		c.PutText(row, swatch+3, text, "label")
	}
}
