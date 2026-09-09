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
	key      string
	name     string
	practice string
}

// cynefinDomains is the fixed five, in the order they are drawn.
var cynefinDomains = []cynefinDomain{
	{"complex", "Complex", "Probe · Sense · Respond — emergent practice"},
	{"complicated", "Complicated", "Sense · Analyse · Respond — good practice"},
	{"chaotic", "Chaotic", "Act · Sense · Respond — novel practice"},
	{"clear", "Clear", "Sense · Categorise · Respond — best practice"},
	{"confusion", "Confusion", "which domain is unknown"},
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

// RenderCynefin parses and renders a Mermaid Cynefin diagram.
func RenderCynefin(source string, cs renderer.CharSet, theme *renderer.Theme) *renderer.Canvas {
	cf := parseCynefin(source)
	bullet, arrowRight := cs.Dot, cs.ArrowRight

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

	top, bottom := titleRows, titleRows+plotH-1
	left, right := 0, plotW-1
	midCol := plotW / 2
	midRow := top + plotH/2

	confHalf := 11
	confLeft, confRight := midCol-confHalf, midCol+confHalf
	confTop, confBottom := midRow-2, midRow+3

	// Regions first, so the frame and the text sit over the fill.
	regions := [][4]int{
		{top + 1, midRow - 1, left + 1, midCol - 1},     // complex
		{top + 1, midRow - 1, midCol + 1, right - 1},    // complicated
		{midRow + 1, bottom - 1, left + 1, midCol - 1},  // chaotic
		{midRow + 1, bottom - 1, midCol + 1, right - 1}, // clear
	}
	useRegion := theme != nil && theme.HasDepthColors()
	if useRegion {
		for i, reg := range regions {
			fill := "_ansi:" + theme.RegionStyle(i, 0)
			for r := reg[0]; r <= reg[1]; r++ {
				for col := reg[2]; col <= reg[3]; col++ {
					c.SetFill(r, col, fill)
				}
			}
		}
	}

	c.Segment(top, left, top, right, glyph.Light, false, "edge")
	c.Segment(bottom, left, bottom, right, glyph.Light, false, "edge")
	c.Segment(top, left, bottom, left, glyph.Light, false, "edge")
	c.Segment(top, right, bottom, right, glyph.Light, false, "edge")

	// The ordered and unordered halves part along the middle, the divider
	// opening around the Confusion region rather than running through it.
	c.Segment(top, midCol, confTop, midCol, glyph.Light, false, "edge")
	c.Segment(confBottom, midCol, bottom, midCol, glyph.Light, false, "edge")
	c.Segment(midRow, left, midRow, confLeft, glyph.Light, false, "edge")
	c.Segment(midRow, confRight, midRow, right, glyph.Light, false, "edge")

	c.Segment(confTop, confLeft, confTop, confRight, glyph.Light, false, "node")
	c.Segment(confBottom, confLeft, confBottom, confRight, glyph.Light, false, "node")
	c.Segment(confTop, confLeft, confBottom, confLeft, glyph.Light, false, "node")
	c.Segment(confTop, confRight, confBottom, confRight, glyph.Light, false, "node")
	for r := confTop; r <= confBottom; r++ {
		for col := confLeft; col <= confRight; col++ {
			c.SetFill(r, col, "subgraph_fill")
		}
	}

	// The Confusion region reaches into all four quadrants, so the two below
	// it start under it and the two above it end over it.
	for i, d := range cynefinDomains[:4] {
		reg := regions[i]
		style, subStyle := "subgraph_label", "label"
		if useRegion {
			style = "_ansi:" + theme.RegionLabelStyle(i, 0)
			subStyle = style
		}
		col := reg[2] + 1
		width := reg[3] - col + 1
		nameRow, lastRow := reg[0]+1, confTop-1
		if i >= 2 {
			nameRow, lastRow = confBottom+1, reg[1]
		}
		c.PutText(nameRow, col, d.name, style)
		c.PutText(nameRow+1, col, textwidth.Truncate(d.practice, width), subStyle)
		row := nameRow + 3
		for _, item := range cf.items[d.key] {
			if row > lastRow {
				break
			}
			c.Put(row, col, bullet, subStyle)
			c.PutText(row, col+2, textwidth.Truncate(item, width-2), subStyle)
			row++
		}
	}

	// Confusion: a centred label and up to three items, the rest counted.
	c.PutText(confTop+1, midCol-textwidth.String(cynefinDomains[4].name)/2, cynefinDomains[4].name, "subgraph_label")
	conf := cf.items["confusion"]
	shown := min(len(conf), 2)
	for i, item := range conf[:shown] {
		text := textwidth.Truncate(item, confRight-confLeft-1)
		c.PutText(confTop+2+i, midCol-textwidth.String(text)/2, text, "label")
	}
	if len(conf) > shown {
		more := fmt.Sprintf("+%d more", len(conf)-shown)
		c.PutText(confTop+2+shown, midCol-textwidth.String(more)/2, more, "label")
	}

	drawCynefinMoves(c, cf.moves, bottom+2, plotW, arrowRight)
	return c
}

// drawCynefinMoves lists the transitions under the frame, each an arrow from
// one domain name to the other with its label beside it.
func drawCynefinMoves(c *renderer.Canvas, moves []cynefinMove, row, width int, arrowRight rune) {
	name := map[string]string{}
	for _, d := range cynefinDomains {
		name[d.key] = d.name
	}
	fromW, toW := 0, 0
	for _, m := range moves {
		fromW = max(fromW, textwidth.String(name[m.from]))
		toW = max(toW, textwidth.String(name[m.to]))
	}
	for i, m := range moves {
		r := row + i
		c.PutText(r, 2, name[m.from], "label")
		shaft := 2 + fromW + 1
		c.Segment(r, shaft, r, shaft+3, glyph.Light, false, "arrow")
		c.Put(r, shaft+3, arrowRight, "arrow")
		c.PutText(r, shaft+5, name[m.to], "label")
		if m.label != "" {
			col := shaft + 5 + toW + 2
			c.PutText(r, col, textwidth.Truncate(m.label, max(width-col-1, 0)), "label")
		}
	}
}
