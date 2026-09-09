package diagram

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/aaronsb/mmaid-go/internal/renderer"
)

// Read, against https://mermaid.js.org/syntax/sankey.html:
//   - the `sankey` header and its original `sankey-beta` spelling;
//   - rows of three CSV fields — source, target, value — per RFC 4180, so a
//     field may be quoted, a quoted field may hold commas, and a pair of
//     quotes inside a quoted field is one quote;
//   - empty lines between rows, which plain CSV forbids and this diagram
//     allows;
//   - `%%` comment lines.
//
// Skipped:
//   - `linkColor`, `nodeAlignment`, `labelStyle`, `nodeWidth`, `nodePadding`,
//     `nodeColors`, `showValues`, `width` and `height`: every one is
//     frontmatter or init config, which `Render` strips before a parser sees
//     the source. A band takes its source node's colour, which is upstream's
//     `linkColor: source`, and a node's value is always shown.
//   - a newline inside a quoted field. RFC 4180 allows it; here a row is one
//     line.
//   - a row without exactly three fields, or whose third field is not a
//     number. Upstream fails the parse; this drops the row.

// sankeyNode is one node: what flows into and out of it, which column it
// sits in, and where its bar lands.
type sankeyNode struct {
	name    string
	in, out float64
	depth   int

	top, height   int
	inRow, outRow int // the next free row for an incoming and outgoing band
}

// value is the flow the node's bar stands for.
func (n *sankeyNode) value() float64 {
	return max(n.in, n.out)
}

// sankeyLink is one row of the CSV, and the band it draws.
type sankeyLink struct {
	source, target string
	value          float64
	sy, ty, h      int // the band's first row at each end, and its thickness
}

// sankeyData is the parsed diagram: nodes in order of first appearance and
// links in file order.
type sankeyData struct {
	nodes []*sankeyNode
	index map[string]int
	links []sankeyLink
}

// node returns the named node, adding it at the end if it is new.
func (sd *sankeyData) node(name string) *sankeyNode {
	if i, ok := sd.index[name]; ok {
		return sd.nodes[i]
	}
	n := &sankeyNode{name: name}
	sd.index[name] = len(sd.nodes)
	sd.nodes = append(sd.nodes, n)
	return n
}

var reSankeyHeader = regexp.MustCompile(`(?i)^sankey(-beta)?$`)

// parseSankey reads the CSV rows under a `sankey` header.
//
//	sankey-beta
//	Electricity grid,Industry,342.165
//	Electricity grid,"Heating and cooling, homes",113.726
func parseSankey(source string) *sankeyData {
	sd := &sankeyData{index: map[string]int{}}
	for _, raw := range strings.Split(source, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "%%") || reSankeyHeader.MatchString(line) {
			continue
		}
		fields := splitCSVRow(line)
		if len(fields) != 3 {
			continue
		}
		value, err := strconv.ParseFloat(fields[2], 64)
		if err != nil || fields[0] == "" || fields[1] == "" {
			continue
		}
		sd.node(fields[0]).out += value
		sd.node(fields[1]).in += value
		sd.links = append(sd.links, sankeyLink{source: fields[0], target: fields[1], value: value})
	}
	return sd
}

// splitCSVRow splits one CSV row. An unquoted field is trimmed; a quoted one
// is taken as written, with `""` standing for a quote.
func splitCSVRow(line string) []string {
	var fields []string
	var cur strings.Builder
	quoted, sawQuote := false, false

	flush := func() {
		s := cur.String()
		if !sawQuote {
			s = strings.TrimSpace(s)
		}
		fields = append(fields, s)
		cur.Reset()
		sawQuote = false
	}
	for i := 0; i < len(line); i++ {
		switch ch := line[i]; {
		case ch == '"' && quoted && i+1 < len(line) && line[i+1] == '"':
			cur.WriteByte('"')
			i++
		case ch == '"':
			quoted = !quoted
			sawQuote = true
		case ch == ',' && !quoted:
			flush()
		default:
			cur.WriteByte(ch)
		}
	}
	flush()
	return fields
}

// sankeyDepths lays the nodes out in columns by longest path from a source.
// A cycle stops the walk after one pass per node, leaving the nodes it
// reached in the order it reached them.
func sankeyDepths(sd *sankeyData) {
	for range sd.nodes {
		changed := false
		for _, l := range sd.links {
			s, t := sd.nodes[sd.index[l.source]], sd.nodes[sd.index[l.target]]
			if s == t {
				continue
			}
			if t.depth < s.depth+1 {
				t.depth, changed = s.depth+1, true
			}
		}
		if !changed {
			return
		}
	}
}

const (
	sankeyBarWidth = 2  // columns a node's bar is wide
	sankeyNodeGap  = 1  // blank rows between two bars in a column
	sankeyMaxRows  = 18 // rows the heaviest column's flow is drawn in
	sankeyFlowGap  = 10 // columns a band runs through, before scaling
)

// sankeyPlan is the whole diagram laid out.
type sankeyPlan struct {
	sd            *sankeyData
	cols          [][]*sankeyNode
	barX          []int // the left column of each column's bars
	width, height int
}

func (p sankeyPlan) node(name string) *sankeyNode {
	return p.sd.nodes[p.sd.index[name]]
}

// sankeyValue formats a flow the way the source wrote it, without a trailing
// zero the parse introduced.
func sankeyValue(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func sankeyLabel(n *sankeyNode) string {
	return n.name + " " + sankeyValue(n.value())
}

// sankeyLayout places the columns across the canvas and the bars down each
// one, then allocates every band its rows at both ends.
func sankeyLayout(sd *sankeyData) sankeyPlan {
	sankeyDepths(sd)

	depth := 0
	for _, n := range sd.nodes {
		depth = max(depth, n.depth)
	}
	p := sankeyPlan{sd: sd, cols: make([][]*sankeyNode, depth+1)}
	for _, n := range sd.nodes {
		p.cols[n.depth] = append(p.cols[n.depth], n)
	}

	// One scale for the whole diagram, so a row is the same flow everywhere:
	// the heaviest column fills sankeyMaxRows.
	heaviest := 0.0
	for _, col := range p.cols {
		total := 0.0
		for _, n := range col {
			total += n.value()
		}
		heaviest = max(heaviest, total)
	}
	scale := float64(sankeyMaxRows)
	if heaviest > 0 {
		scale /= heaviest
	}
	rows := func(v float64) int { return max(1, int(math.Round(v*scale))) }

	for _, col := range p.cols {
		for _, n := range col {
			n.height = rows(n.value())
		}
	}

	// Every column starts at the top, so a flow that passes straight through
	// a node draws as a straight band.
	for _, col := range p.cols {
		row := 0
		for _, n := range col {
			n.top, n.inRow, n.outRow = row, row, row
			row += n.height + sankeyNodeGap
		}
		p.height = max(p.height, row-sankeyNodeGap)
	}

	// A label sits left of its bar, so its column reserves the width.
	labelW := make([]int, len(p.cols))
	for c, col := range p.cols {
		for _, n := range col {
			labelW[c] = max(labelW[c], runeLen(sankeyLabel(n)))
		}
	}
	natural := 0
	for c := range p.cols {
		natural += labelW[c] + 1 + sankeyBarWidth
		if c < len(p.cols)-1 {
			natural += sankeyFlowGap
		}
	}
	gap := sankeyFlowGap
	if len(p.cols) > 1 {
		gap = scaleGap(sankeyFlowGap, len(p.cols)-1, natural, sankeyFlowGap, 40)
	}

	p.barX = make([]int, len(p.cols))
	x := 0
	for c := range p.cols {
		x += labelW[c] + 1
		p.barX[c] = x
		x += sankeyBarWidth
		if c < len(p.cols)-1 {
			x += gap
		}
	}
	p.width = x + 1

	for i := range sd.links {
		l := &sd.links[i]
		s, t := p.node(l.source), p.node(l.target)
		h := rows(l.value)
		h = min(h, s.top+s.height-s.outRow)
		h = min(h, t.top+t.height-t.inRow)
		l.h = max(1, h)
		l.sy, l.ty = s.outRow, t.inRow
		s.outRow += l.h
		t.inRow += l.h
	}
	return p
}

// sankeyColors returns one colour per node, cycling the pie palette a theme
// with a base hue shades for itself.
func sankeyColors(theme *renderer.Theme, n int) [][3]int {
	if theme == nil {
		return nil
	}
	if theme.HasPieBase() {
		return theme.PieColors(n)
	}
	return pieColors
}

// ansiFG is the foreground escape a colour draws with, as the pie legend
// builds it.
func ansiFG(c [3]int) string {
	return fmt.Sprintf("\033[38;2;%d;%d;%dm", c[0], c[1], c[2])
}

// sankeyStyle is the style key a node and everything flowing out of it draw
// with: an ANSI foreground when a theme set the colours, else the fallback.
func sankeyStyle(colors [][3]int, i int, fallback string) string {
	if len(colors) == 0 {
		return fallback
	}
	return "_ansi:" + ansiFG(colors[i%len(colors)])
}

// sankeyBand draws one link as a ribbon: each of its rows leaves the source
// bar and slides evenly across the gap to the row it arrives on, and a column
// the slide skips a row over is filled so the ribbon stays whole.
func sankeyBand(c *renderer.Canvas, p sankeyPlan, l sankeyLink, ch rune, style string) {
	s, t := p.node(l.source), p.node(l.target)
	x0 := p.barX[s.depth] + sankeyBarWidth
	x1 := p.barX[t.depth] - 1
	if x1 < x0 {
		return
	}
	drop := float64(l.ty - l.sy)
	span := float64(x1 - x0)

	for k := range l.h {
		row, prev := l.sy+k, l.sy+k
		for x := x0; x <= x1; x++ {
			if span > 0 {
				row = l.sy + k + int(math.Round(drop*float64(x-x0)/span))
			} else {
				row = l.ty + k
			}
			for r := min(prev, row); r <= max(prev, row); r++ {
				c.Put(r, x, ch, style)
			}
			prev = row
		}
	}
}

// RenderSankey parses and renders a Mermaid sankey diagram: nodes as vertical
// bars in columns by depth, each bar as tall as the flow through it, and one
// shaded band per link. The canvas is as wide as the model needs, so a wide
// diagram scrolls rather than wrapping.
func RenderSankey(source string, cs renderer.CharSet, theme *renderer.Theme) *renderer.Canvas {
	sd := parseSankey(source)
	if len(sd.links) == 0 {
		c := renderer.NewCanvas(30, 1)
		c.PutText(0, 0, "[sankey] no links", "default")
		return c
	}

	p := sankeyLayout(sd)
	c := renderer.NewCanvas(p.width, p.height)
	c.SetCharSet(cs)

	colors := sankeyColors(theme, len(sd.nodes))
	shades := [3]rune{cs.Fills.Dark, cs.Fills.Medium, cs.Fills.Light}

	// Bands first: a bar is what a band runs from, and stays whole. A band
	// takes its source's colour and its own shade, so neither the bands
	// meeting one bar nor the bands leaving it merge into each other.
	for i, l := range sd.links {
		style := sankeyStyle(colors, sd.index[l.source], "edge")
		sankeyBand(c, p, l, shades[i%len(shades)], style)
	}
	for i, n := range sd.nodes {
		style := sankeyStyle(colors, i, "node")
		for row := n.top; row < n.top+n.height; row++ {
			for col := p.barX[n.depth]; col < p.barX[n.depth]+sankeyBarWidth; col++ {
				c.Put(row, col, cs.Fills.Full, style)
			}
		}
	}
	// Labels last, on a plate cut out of the bands reaching the bar.
	for _, n := range sd.nodes {
		label := sankeyLabel(n)
		row := n.top + n.height/2
		col := max(0, p.barX[n.depth]-1-runeLen(label))
		for x := col - 1; x < col+runeLen(label)+1; x++ {
			c.ClearCell(row, x)
		}
		c.PutText(row, col, label, "label")
	}
	return c
}
