package diagram

import (
	"regexp"
	"strings"

	"github.com/aaronsb/mmaid-go/internal/renderer"
)

// timelineEvent holds one or more items at a time point.
type timelineEvent struct {
	period string
	items  []string
}

// timelineData holds the parsed timeline.
type timelineData struct {
	title  string
	events []timelineEvent
}

var (
	reTimelineHeader = regexp.MustCompile(`(?i)^\s*timeline\s*$`)
	reTimelineTitle  = regexp.MustCompile(`(?i)^\s*title\s+(.+)$`)
)

func parseTimeline(source string) *timelineData {
	td := &timelineData{}
	lines := strings.Split(source, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "%%") {
			continue
		}
		if reTimelineHeader.MatchString(trimmed) {
			continue
		}
		if m := reTimelineTitle.FindStringSubmatch(trimmed); m != nil {
			td.title = strings.TrimSpace(m[1])
			continue
		}

		// "section X" lines become period labels for subsequent events
		if strings.HasPrefix(strings.ToLower(trimmed), "section ") {
			td.events = append(td.events, timelineEvent{
				period: strings.TrimSpace(trimmed[8:]),
			})
			continue
		}

		// Event line: "period : item1 : item2" or continuation ": item"
		parts := strings.Split(trimmed, ":")
		if len(parts) >= 2 {
			period := strings.TrimSpace(parts[0])
			var items []string
			for _, p := range parts[1:] {
				item := strings.TrimSpace(p)
				if item != "" {
					items = append(items, item)
				}
			}
			if period != "" {
				td.events = append(td.events, timelineEvent{period: period, items: items})
			} else if len(td.events) > 0 {
				// Continuation line — append items to last event
				td.events[len(td.events)-1].items = append(td.events[len(td.events)-1].items, items...)
			}
		}
	}

	return td
}

// RenderTimeline parses and renders a Mermaid timeline diagram.
func RenderTimeline(source string, useASCII bool, theme *renderer.Theme) *renderer.Canvas {
	td := parseTimeline(source)
	if len(td.events) == 0 {
		c := renderer.NewCanvas(30, 1)
		c.PutText(0, 0, "[timeline] no events", "default")
		return c
	}

	// Layout: horizontal axis with events spaced evenly.
	// Each event is a column with the period on the axis and items above/below.
	colWidth := 0
	for _, e := range td.events {
		if len(e.period) > colWidth {
			colWidth = len(e.period)
		}
		for _, item := range e.items {
			if len(item)+4 > colWidth { // +4 for box padding
				colWidth = len(item) + 4
			}
		}
	}
	colWidth += 2 // spacing between columns

	// Determine max items per event (for height)
	maxItems := 0
	for _, e := range td.events {
		if len(e.items) > maxItems {
			maxItems = len(e.items)
		}
	}

	// Canvas layout:
	// row 0: title (optional)
	// rows: item boxes (each takes 3 rows: border, text, border)
	// axis row: ───●───────●───────●───
	// period row: period labels

	titleRows := 0
	if td.title != "" {
		titleRows = 2
	}
	itemHeight := maxItems * 3 // each item: top border, text, bottom border
	axisRow := titleRows + itemHeight
	periodRow := axisRow + 1

	canvasWidth := len(td.events)*colWidth + 2
	canvasHeight := periodRow + 2

	c := renderer.NewCanvas(canvasWidth, canvasHeight)

	// Wallpaper: base background behind entire diagram
	if theme != nil && theme.HasDepthColors() {
		for r := 0; r < canvasHeight; r++ {
			for col := 0; col < canvasWidth; col++ {
				c.SetFill(r, col, "subgraph_fill")
			}
		}
	}

	// Title
	if td.title != "" {
		titleCol := (canvasWidth - len(td.title)) / 2
		if titleCol < 0 {
			titleCol = 0
		}
		c.PutText(0, titleCol, td.title, "bold_label")
	}

	hLine := '─'
	dot := '●'
	tl := '╭'
	tr := '╮'
	bl := '╰'
	br := '╯'
	vLine := '│'
	if useASCII {
		hLine = '-'
		dot = 'o'
		tl = '+'
		tr = '+'
		bl = '+'
		br = '+'
		vLine = '|'
	}

	// Draw axis line
	for x := 0; x < canvasWidth; x++ {
		c.Put(axisRow, x, hLine, false, "edge")
	}

	// Draw each event
	useRegion := theme != nil && theme.HasDepthColors()
	for i, e := range td.events {
		centerX := i*colWidth + colWidth/2

		// Dot on axis
		c.Put(axisRow, centerX, dot, false, "arrow")

		// Period label below axis — colored text, no bg
		periodStyle := "label"
		if useRegion {
			periodStyle = "_ansi:" + theme.RegionTextStyle(i, 0)
		}
		labelX := centerX - len(e.period)/2
		if labelX < 0 {
			labelX = 0
		}
		c.PutText(periodRow, labelX, e.period, periodStyle)

		// Item boxes above axis
		for j, item := range e.items {
			boxW := len(item) + 2
			boxX := centerX - boxW/2
			if boxX < 0 {
				boxX = 0
			}
			// Stack items upward from axis
			boxBottom := axisRow - 1 - j*3
			boxTop := boxBottom - 2

			borderStyle := "node"
			labelStyle := "label"
			if useRegion {
				borderStyle = "_ansi:" + theme.RegionBorderStyle(i, 1)
				labelStyle = "_ansi:" + theme.RegionLabelStyle(i, 1)
			}

			// Draw box
			c.Put(boxTop, boxX, tl, false, borderStyle)
			c.DrawHorizontal(boxTop, boxX+1, boxX+boxW-1, hLine, borderStyle)
			c.Put(boxTop, boxX+boxW, tr, false, borderStyle)

			c.Put(boxTop+1, boxX, vLine, false, borderStyle)
			c.PutText(boxTop+1, boxX+1, " "+item+" ", labelStyle)
			c.Put(boxTop+1, boxX+boxW, vLine, false, borderStyle)

			c.Put(boxTop+2, boxX, bl, false, borderStyle)
			c.DrawHorizontal(boxTop+2, boxX+1, boxX+boxW-1, hLine, borderStyle)
			c.Put(boxTop+2, boxX+boxW, br, false, borderStyle)

			// Fill box interior for solid background
			if useRegion {
				fillStyle := "_ansi:" + theme.RegionStyle(i, 1)
				for row := boxTop; row <= boxTop+2; row++ {
					for col := boxX; col <= boxX+boxW; col++ {
						c.SetFill(row, col, fillStyle)
					}
				}
			}
			for col := boxX + 1; col < boxX+boxW; col++ {
				if c.Get(boxTop+1, col) == ' ' {
					c.SetStyle(boxTop+1, col, borderStyle)
				}
			}

			// Connector from box to axis
			if boxBottom < axisRow-1 {
				c.Put(axisRow-1, centerX, vLine, false, "edge")
			}
		}
	}

	return c
}
