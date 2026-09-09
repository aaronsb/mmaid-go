package diagram

import (
	"fmt"
	"math"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/aaronsb/mmaid-go/internal/renderer"
)

// Read, against https://mermaid.js.org/syntax/sankey.html:
//   - the `sankey` header and its original `sankey-beta` spelling;
//   - rows of three CSV fields — source, target, value — per RFC 4180, so a
//     field may be quoted, a quoted field may hold commas, and a pair of
//     quotes inside a quoted field is one quote. Every field is trimmed, as
//     `sankey.jison` trims both the escaped and the plain form;
//   - a value in decimal or exponent notation, which is what upstream's
//     `parseFloat` reads;
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
//   - a hexadecimal float, a digit separator and the words `NaN`, `Inf` and
//     `Infinity`, all of which Go's `ParseFloat` reads and upstream's
//     `parseFloat` does not.
//   - a row whose value is negative or does not resolve to a finite number.
//     A sankey has no negative flow, and one unbounded row sets the scale for
//     every other. The parse warns once, naming the row.
//   - a link that closes a cycle, and a link from a node to itself. Upstream
//     refuses the whole diagram as a circular link; this drops the link,
//     leaves it out of both nodes' totals, and warns once.

// sankeyNode is one node: what flows into and out of it, which column it
// sits in, and where its bar lands.
type sankeyNode struct {
	name    string
	in, out float64
	depth   int

	top, height int
}

// value is the flow the node's bar stands for.
func (n *sankeyNode) value() float64 {
	return max(n.in, n.out)
}

// sankeyLink is one row of the CSV, and the band it draws. A band is as thick
// as its share of each bar it meets, which is not the same number at both
// ends when the flow through a node is not balanced.
type sankeyLink struct {
	source, target string
	value          float64
	back           bool // closes a cycle: kept for the record, never drawn
	sy, hs         int  // the band's first row and thickness at the source
	ty, ht         int  // and at the target
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

var (
	reSankeyHeader = regexp.MustCompile(`(?i)^sankey(-beta)?$`)
	// reSankeyValue is what upstream's parseFloat reads: decimal or exponent
	// notation, and nothing else.
	reSankeyValue = regexp.MustCompile(`^[+-]?(?:\d+(?:\.\d*)?|\.\d+)(?:[eE][+-]?\d+)?$`)
)

// parseSankey reads the CSV rows under a `sankey` header, drops the links
// that close a cycle, and totals what flows through each node.
//
//	sankey-beta
//	Electricity grid,Industry,342.165
//	Electricity grid,"Heating and cooling, homes",113.726
func parseSankey(source string) *sankeyData {
	sd := &sankeyData{index: map[string]int{}}
	warned := false
	for _, raw := range strings.Split(source, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "%%") || reSankeyHeader.MatchString(line) {
			continue
		}
		fields := splitCSVRow(line)
		if len(fields) != 3 || fields[0] == "" || fields[1] == "" {
			continue
		}
		if !reSankeyValue.MatchString(fields[2]) {
			continue
		}
		value, err := strconv.ParseFloat(fields[2], 64)
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
			if !warned {
				fmt.Fprintf(os.Stderr, "mmaid: sankey: dropped %q: a flow is a finite, non-negative number\n", line)
				warned = true
			}
			continue
		}
		sd.node(fields[0])
		sd.node(fields[1])
		sd.links = append(sd.links, sankeyLink{source: fields[0], target: fields[1], value: value})
	}
	sankeyMarkCycles(sd)
	for _, l := range sd.links {
		if l.back {
			continue
		}
		sd.nodes[sd.index[l.source]].out += l.value
		sd.nodes[sd.index[l.target]].in += l.value
	}
	return sd
}

// splitCSVRow splits one CSV row into trimmed fields. A quoted field carries
// commas, and `""` inside one is a single quote.
func splitCSVRow(line string) []string {
	var fields []string
	var cur strings.Builder

	quoted := false
	flush := func() {
		fields = append(fields, strings.TrimSpace(cur.String()))
		cur.Reset()
	}
	for i := 0; i < len(line); i++ {
		switch ch := line[i]; {
		case ch == '"' && quoted && i+1 < len(line) && line[i+1] == '"':
			cur.WriteByte('"')
			i++
		case ch == '"':
			quoted = !quoted
		case ch == ',' && !quoted:
			flush()
		default:
			cur.WriteByte(ch)
		}
	}
	flush()
	return fields
}

// sankeyMarkCycles marks every link that closes a cycle, so what is left is a
// directed acyclic graph. A link to the node itself closes the shortest cycle
// there is. It warns once, naming the first link it drops.
func sankeyMarkCycles(sd *sankeyData) {
	out := make([][]int, len(sd.nodes))
	for i, l := range sd.links {
		out[sd.index[l.source]] = append(out[sd.index[l.source]], i)
	}
	const (
		white = iota
		grey
		black
	)
	state := make([]int, len(sd.nodes))
	dropped := 0
	first := ""

	var visit func(int)
	visit = func(n int) {
		state[n] = grey
		for _, li := range out[n] {
			t := sd.index[sd.links[li].target]
			switch {
			case t == n || state[t] == grey:
				sd.links[li].back = true
				dropped++
				if first == "" {
					first = sd.links[li].source + " -> " + sd.links[li].target
				}
			case state[t] == white:
				visit(t)
			}
		}
		state[n] = black
	}
	for i := range sd.nodes {
		if state[i] == white {
			visit(i)
		}
	}
	switch {
	case dropped == 1:
		fmt.Fprintf(os.Stderr, "mmaid: sankey: %s closes a cycle and is not drawn\n", first)
	case dropped > 1:
		fmt.Fprintf(os.Stderr, "mmaid: sankey: %s closes a cycle and is not drawn, with %d more\n", first, dropped-1)
	}
}

// sankeyDepths lays the nodes out in columns by longest path from a source.
// The links that close a cycle are already out of the walk, so it settles
// after one pass per node at the most.
func sankeyDepths(sd *sankeyData) {
	for range sd.nodes {
		changed := false
		for _, l := range sd.links {
			if l.back {
				continue
			}
			s, t := sd.nodes[sd.index[l.source]], sd.nodes[sd.index[l.target]]
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

// sankeyValue formats a flow in at most six significant digits, and a whole
// number as a whole number.
func sankeyValue(v float64) string {
	if v == math.Trunc(v) && math.Abs(v) < 1e15 {
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	return strconv.FormatFloat(v, 'g', 6, 64)
}

func sankeyLabel(n *sankeyNode) string {
	return n.name + " " + sankeyValue(n.value())
}

// sankeyApportion divides total rows among weights by largest remainder, so
// the shares add up to total exactly. A share may be zero: a flow too small
// for a row of its own does not borrow one from a bigger flow.
func sankeyApportion(weights []float64, total int) []int {
	shares := make([]int, len(weights))
	sum := 0.0
	for _, w := range weights {
		sum += w
	}
	if len(weights) == 0 || total <= 0 || !(sum > 0) || math.IsInf(sum, 0) {
		return shares
	}
	type remainder struct {
		i    int
		frac float64
	}
	rems := make([]remainder, len(weights))
	assigned := 0
	for i, w := range weights {
		exact := w / sum * float64(total)
		shares[i] = int(exact)
		assigned += shares[i]
		rems[i] = remainder{i, exact - float64(shares[i])}
	}
	sort.SliceStable(rems, func(a, b int) bool { return rems[a].frac > rems[b].frac })
	for k := 0; assigned < total && k < len(rems); k++ {
		shares[rems[k].i]++
		assigned++
	}
	return shares
}

// sankeyAllocateBands gives every band its rows at each end. A bar's rows are
// apportioned among the links that meet that side of it, so the bands leaving
// a bar cover it exactly and none of them starts below it.
func sankeyAllocateBands(sd *sankeyData, rows func(float64) int) {
	out := make([][]int, len(sd.nodes))
	in := make([][]int, len(sd.nodes))
	for i, l := range sd.links {
		if l.back {
			continue
		}
		s, t := sd.index[l.source], sd.index[l.target]
		out[s] = append(out[s], i)
		in[t] = append(in[t], i)
	}
	weights := func(idx []int) []float64 {
		w := make([]float64, len(idx))
		for k, li := range idx {
			w[k] = sd.links[li].value
		}
		return w
	}
	for ni, n := range sd.nodes {
		row := n.top
		for k, share := range sankeyApportion(weights(out[ni]), rows(n.out)) {
			l := &sd.links[out[ni][k]]
			l.sy, l.hs = row, share
			row += share
		}
		row = n.top
		for k, share := range sankeyApportion(weights(in[ni]), rows(n.in)) {
			l := &sd.links[in[ni][k]]
			l.ty, l.ht = row, share
			row += share
		}
	}
}

// sankeyColumns groups the nodes into columns by depth. A depth no node
// reached holds no column: a dropped cycle leaves gaps in the numbering, and
// an empty column would still reserve a label, a bar and a gap.
func sankeyColumns(sd *sankeyData) [][]*sankeyNode {
	used := map[int]bool{}
	for _, n := range sd.nodes {
		used[n.depth] = true
	}
	depths := make([]int, 0, len(used))
	for d := range used {
		depths = append(depths, d)
	}
	sort.Ints(depths)
	at := make(map[int]int, len(depths))
	for i, d := range depths {
		at[d] = i
	}
	cols := make([][]*sankeyNode, len(depths))
	for _, n := range sd.nodes {
		n.depth = at[n.depth]
		cols[n.depth] = append(cols[n.depth], n)
	}
	return cols
}

// sankeyScale returns the rows one unit of flow draws in, and the function
// that converts a flow to whole rows. The heaviest column fills sankeyMaxRows;
// a total that is not a finite positive number leaves every bar at its
// minimum rather than setting an unbounded scale.
func sankeyScale(cols [][]*sankeyNode) func(float64) int {
	heaviest := 0.0
	for _, col := range cols {
		total := 0.0
		for _, n := range col {
			total += n.value()
		}
		heaviest = max(heaviest, total)
	}
	scale := 0.0
	if heaviest > 0 && !math.IsInf(heaviest, 0) {
		scale = float64(sankeyMaxRows) / heaviest
	}
	return func(v float64) int {
		if !(v > 0) || math.IsInf(v, 0) {
			return 0
		}
		return min(int(math.Round(v*scale)), sankeyMaxRows)
	}
}

// sankeyLayout places the columns across the canvas and the bars down each
// one, then allocates every band its rows at both ends.
func sankeyLayout(sd *sankeyData) sankeyPlan {
	sankeyDepths(sd)
	p := sankeyPlan{sd: sd, cols: sankeyColumns(sd)}
	rows := sankeyScale(p.cols)

	// A bar stands for a flow, so it is never shorter than one row even when
	// the flow rounds to none.
	for _, col := range p.cols {
		for _, n := range col {
			n.height = max(1, rows(n.value()))
		}
	}

	// Every column starts at the top, so a flow that passes straight through
	// a node draws as a straight band.
	for _, col := range p.cols {
		row := 0
		for _, n := range col {
			n.top = row
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

	sankeyAllocateBands(sd, rows)
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

// sankeyBand draws one link as a ribbon: it leaves the source bar as thick as
// its share of that bar, slides evenly across the gap, and arrives as thick as
// its share of the target's. A column the slide steps a row over is filled so
// the ribbon stays whole.
func sankeyBand(c *renderer.Canvas, p sankeyPlan, l sankeyLink, ch rune, style string) {
	if l.back || (l.hs <= 0 && l.ht <= 0) {
		return
	}
	s, t := p.node(l.source), p.node(l.target)
	x0 := p.barX[s.depth] + sankeyBarWidth
	x1 := p.barX[t.depth] - 1
	if x1 < x0 {
		return
	}
	span := float64(x1 - x0)
	prevLo, prevHi := 0, -1

	for x := x0; x <= x1; x++ {
		at := 1.0
		if span > 0 {
			at = float64(x-x0) / span
		}
		lo := int(math.Round(float64(l.sy) + float64(l.ty-l.sy)*at))
		h := int(math.Round(float64(l.hs) + float64(l.ht-l.hs)*at))
		if h <= 0 {
			prevHi = -1
			continue
		}
		hi := lo + h - 1
		// Close the step between this column's span and the last one's, so a
		// steep ribbon has no holes in it.
		if prevHi >= prevLo {
			lo = min(lo, prevHi+1)
			hi = max(hi, prevLo-1)
		}
		for r := lo; r <= hi; r++ {
			c.Put(r, x, ch, style)
		}
		prevLo, prevHi = lo, hi
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
