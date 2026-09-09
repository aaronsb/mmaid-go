package diagram

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/aaronsb/mmaid-go/internal/glyph"
	"github.com/aaronsb/mmaid-go/internal/renderer"
	"github.com/aaronsb/mmaid-go/internal/textwidth"
)

// Cynefin (`cynefin-beta`), from https://mermaid.js.org/syntax/cynefin.html.
//
// Read: `title`; the five domain keywords `complex`, `complicated`, `clear`,
// `chaotic` and `confusion`, each opening a block of quoted item labels; and
// top-level `a --> b : "label"` transitions, self-loops ignored as the
// grammar says. Domain positions are fixed whatever order they are declared
// in: Complex top left, Complicated top right, Chaotic bottom left, Clear
// bottom right, Confusion in the centre.
//
// Skipped: `accTitle` and `accDescr`, which the canvas has nowhere to put;
// the `cynefin` config keys, whose pixel width, height, padding, boundary
// amplitude and seed describe an SVG — the wavy boundary and the Clear/Chaotic
// cliff among them.

type cynefinDomain struct {
	key   string
	name  string
	model []string // the decision model, one step per entry
	kind  string   // the practice the model yields
}

// practice reads "Probe · Sense · Respond — emergent practice", the
// punctuation coming from the glyph set so an ASCII render carries no
// Unicode. The string is the port's own, not the grammar's.
func (d cynefinDomain) practice(m chartMarks) string {
	if len(d.model) == 0 {
		return d.kind
	}
	return strings.Join(d.model, " "+m.separate+" ") + " " + m.dash + " " + d.kind
}

// cynefinDomains is the fixed five, in the order they are drawn.
var cynefinDomains = []cynefinDomain{
	{"complex", "Complex", []string{"Probe", "Sense", "Respond"}, "emergent practice"},
	{"complicated", "Complicated", []string{"Sense", "Analyse", "Respond"}, "good practice"},
	{"chaotic", "Chaotic", []string{"Act", "Sense", "Respond"}, "novel practice"},
	{"clear", "Clear", []string{"Sense", "Categorise", "Respond"}, "best practice"},
	{"confusion", "Confusion", nil, "which domain is unknown"},
}

type cynefinMove struct {
	from, to string
	label    string
}

type cynefinFramework struct {
	title string
	items map[string][]string
	moves []cynefinMove
}

var (
	reCynefinHeader = regexp.MustCompile(`(?i)^cynefin-beta\b`)
	reCynefinTitle  = regexp.MustCompile(`(?i)^title\s+(.+)$`)
	reCynefinDomain = regexp.MustCompile(`(?i)^(complex|complicated|clear|chaotic|confusion)\s*$`)
	reCynefinItem   = regexp.MustCompile(`^"([^"]*)"\s*$`)
	reCynefinMove   = regexp.MustCompile(`(?i)^(complex|complicated|clear|chaotic|confusion)\s*-->\s*(complex|complicated|clear|chaotic|confusion)\s*(?::\s*"?([^"]*?)"?\s*)?$`)
	reCynefinAcc    = regexp.MustCompile(`(?i)^acc(Title|Descr)\b`)
)

// parseCynefin parses Cynefin source into a cynefinFramework.
func parseCynefin(source string) *cynefinFramework {
	cf := &cynefinFramework{items: map[string][]string{}}
	open := ""

	for _, raw := range strings.Split(source, "\n") {
		line := strings.TrimSpace(stripLineComment(raw))
		if line == "" || reCynefinHeader.MatchString(line) || reCynefinAcc.MatchString(line) {
			continue
		}
		if m := reCynefinTitle.FindStringSubmatch(line); m != nil {
			cf.title = strings.Trim(strings.TrimSpace(m[1]), `"`)
			open = ""
			continue
		}
		// A transition is read before a bare domain keyword, the two sharing
		// their first word.
		if m := reCynefinMove.FindStringSubmatch(line); m != nil {
			from, to := strings.ToLower(m[1]), strings.ToLower(m[2])
			if from != to {
				cf.moves = append(cf.moves, cynefinMove{from: from, to: to, label: m[3]})
			}
			open = ""
			continue
		}
		if m := reCynefinDomain.FindStringSubmatch(line); m != nil {
			open = strings.ToLower(m[1])
			if _, ok := cf.items[open]; !ok {
				cf.items[open] = nil
			}
			continue
		}
		if m := reCynefinItem.FindStringSubmatch(line); m != nil && open != "" {
			cf.items[open] = append(cf.items[open], m[1])
			continue
		}
	}
	return cf
}

// cynefinLayout is where the frame, the four quadrants and the Confusion
// region sit once the canvas width is known.
type cynefinLayout struct {
	c                        *renderer.Canvas
	cs                       renderer.CharSet
	marks                    chartMarks
	theme                    *renderer.Theme
	region                   bool
	top, bottom, left, right int
	midRow, midCol           int
	confTop, confBottom      int
	confLeft, confRight      int
	quadrants                [4][4]int // top, bottom, left, right per domain
}

// RenderCynefin parses and renders a Mermaid Cynefin diagram.
func RenderCynefin(source string, cs renderer.CharSet, theme *renderer.Theme) *renderer.Canvas {
	cf := parseCynefin(source)

	plotH := 24
	plotW := scaleWidth(4, 60, maxScaledWidth)

	titleRows := 0
	if cf.title != "" {
		titleRows = 2
	}
	moveRows := 0
	if len(cf.moves) > 0 {
		moveRows = len(cf.moves) + 1
	}
	canvasWidth := plotW
	canvasHeight := titleRows + plotH + moveRows + 1

	c := renderer.NewCanvas(canvasWidth, canvasHeight)
	c.SetCharSet(cs)
	if theme != nil && theme.HasDepthColors() {
		for r := range canvasHeight {
			for col := range canvasWidth {
				c.SetFill(r, col, "subgraph_fill")
			}
		}
	}
	if cf.title != "" {
		c.PutText(0, max((canvasWidth-textwidth.String(cf.title))/2, 0), cf.title, "bold_label")
	}

	midCol := plotW / 2
	midRow := titleRows + plotH/2
	const confHalf = 11
	l := cynefinLayout{
		c: c, cs: cs, marks: marksFor(cs), theme: theme,
		region:     theme != nil && theme.HasDepthColors(),
		top:        titleRows,
		bottom:     titleRows + plotH - 1,
		left:       0,
		right:      plotW - 1,
		midRow:     midRow,
		midCol:     midCol,
		confTop:    midRow - 2,
		confBottom: midRow + 3,
		confLeft:   midCol - confHalf,
		confRight:  midCol + confHalf,
	}
	l.quadrants = [4][4]int{
		{l.top + 1, midRow - 1, l.left + 1, midCol - 1},     // complex
		{l.top + 1, midRow - 1, midCol + 1, l.right - 1},    // complicated
		{midRow + 1, l.bottom - 1, l.left + 1, midCol - 1},  // chaotic
		{midRow + 1, l.bottom - 1, midCol + 1, l.right - 1}, // clear
	}

	l.fillRegions()
	l.drawFrame()
	l.drawQuadrants(cf)
	l.drawConfusion(cf)
	l.drawMoves(cf)
	return c
}

// fillRegions paints the four quadrants before anything is drawn over them.
func (l cynefinLayout) fillRegions() {
	if !l.region {
		return
	}
	for i, q := range l.quadrants {
		fill := "_ansi:" + l.theme.RegionStyle(i, 0)
		for r := q[0]; r <= q[1]; r++ {
			for col := q[2]; col <= q[3]; col++ {
				l.c.SetFill(r, col, fill)
			}
		}
	}
}

// drawFrame draws the border, the two dividers and the Confusion box. The
// dividers stop on the box rather than running through it, so its border
// resolves into tees and the centre reads as a region, not a crossing.
func (l cynefinLayout) drawFrame() {
	c := l.c
	c.Segment(l.top, l.left, l.top, l.right, glyph.Light, false, "edge")
	c.Segment(l.bottom, l.left, l.bottom, l.right, glyph.Light, false, "edge")
	c.Segment(l.top, l.left, l.bottom, l.left, glyph.Light, false, "edge")
	c.Segment(l.top, l.right, l.bottom, l.right, glyph.Light, false, "edge")

	c.Segment(l.top, l.midCol, l.confTop, l.midCol, glyph.Light, false, "edge")
	c.Segment(l.confBottom, l.midCol, l.bottom, l.midCol, glyph.Light, false, "edge")
	c.Segment(l.midRow, l.left, l.midRow, l.confLeft, glyph.Light, false, "edge")
	c.Segment(l.midRow, l.confRight, l.midRow, l.right, glyph.Light, false, "edge")

	c.Segment(l.confTop, l.confLeft, l.confTop, l.confRight, glyph.Light, false, "node")
	c.Segment(l.confBottom, l.confLeft, l.confBottom, l.confRight, glyph.Light, false, "node")
	c.Segment(l.confTop, l.confLeft, l.confBottom, l.confLeft, glyph.Light, false, "node")
	c.Segment(l.confTop, l.confRight, l.confBottom, l.confRight, glyph.Light, false, "node")
	for r := l.confTop; r <= l.confBottom; r++ {
		for col := l.confLeft; col <= l.confRight; col++ {
			c.SetFill(r, col, "subgraph_fill")
		}
	}
}

// drawQuadrants writes each domain's name, its decision model and practice
// type, and its items. The Confusion region reaches into all four quadrants,
// so the two below it start under it and the two above it end over it, and a
// quadrant with more items than rows counts the rest the way Confusion does.
func (l cynefinLayout) drawQuadrants(cf *cynefinFramework) {
	for i, d := range cynefinDomains[:4] {
		q := l.quadrants[i]
		style, subStyle := "subgraph_label", "label"
		if l.region {
			style = "_ansi:" + l.theme.RegionLabelStyle(i, 0)
			subStyle = style
		}
		col := q[2] + 1
		width := q[3] - col + 1
		nameRow, lastRow := q[0]+1, l.confTop-1
		if i >= 2 {
			nameRow, lastRow = l.confBottom+1, q[1]
		}
		l.c.PutText(nameRow, col, d.name, style)
		l.c.PutText(nameRow+1, col, truncateMark(d.practice(l.marks), width, l.marks), subStyle)

		items := cf.items[d.key]
		rows := lastRow - (nameRow + 3) + 1
		shown := len(items)
		if shown > rows {
			shown = max(rows-1, 0) // the last row counts what did not fit
		}
		row := nameRow + 3
		for _, item := range items[:shown] {
			l.c.Put(row, col, l.cs.Dot, subStyle)
			l.c.PutText(row, col+2, truncateMark(item, width-2, l.marks), subStyle)
			row++
		}
		if shown < len(items) && row <= lastRow {
			l.c.PutText(row, col, fmt.Sprintf("+%d more", len(items)-shown), subStyle)
		}
	}
}

// drawConfusion writes the centre region's name and as many items as its box
// holds, counting the rest.
func (l cynefinLayout) drawConfusion(cf *cynefinFramework) {
	name := cynefinDomains[4].name
	l.c.PutText(l.confTop+1, l.midCol-textwidth.String(name)/2, name, "subgraph_label")

	items := cf.items["confusion"]
	rows := l.confBottom - (l.confTop + 2)
	shown := len(items)
	if shown > rows {
		shown = max(rows-1, 0)
	}
	row := l.confTop + 2
	for _, item := range items[:shown] {
		text := truncateMark(item, l.confRight-l.confLeft-1, l.marks)
		l.c.PutText(row, l.midCol-textwidth.String(text)/2, text, "label")
		row++
	}
	if shown < len(items) {
		more := fmt.Sprintf("+%d more", len(items)-shown)
		l.c.PutText(row, l.midCol-textwidth.String(more)/2, more, "label")
	}
}

// drawMoves lists the transitions under the frame, each an arrow from one
// domain name to the other with its label beside it. Drawn across the
// quadrants they would run through the item text they are about.
func (l cynefinLayout) drawMoves(cf *cynefinFramework) {
	name := map[string]string{}
	for _, d := range cynefinDomains {
		name[d.key] = d.name
	}
	fromW, toW := 0, 0
	for _, m := range cf.moves {
		fromW = max(fromW, textwidth.String(name[m.from]))
		toW = max(toW, textwidth.String(name[m.to]))
	}
	for i, m := range cf.moves {
		r := l.bottom + 2 + i
		l.c.PutText(r, 2, name[m.from], "label")
		shaft := 2 + fromW + 1
		l.c.Segment(r, shaft, r, shaft+3, glyph.Light, false, "arrow")
		l.c.Put(r, shaft+3, l.cs.ArrowRight, "arrow")
		l.c.PutText(r, shaft+5, name[m.to], "label")
		if m.label == "" {
			continue
		}
		col := shaft + 5 + toW + 2
		l.c.PutText(r, col, truncateMark(m.label, max(l.c.Width-col-1, 0), l.marks), "label")
	}
}
