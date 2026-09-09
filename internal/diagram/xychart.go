package diagram

import (
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/aaronsb/mmaid-go/internal/glyph"
	"github.com/aaronsb/mmaid-go/internal/renderer"
	"github.com/aaronsb/mmaid-go/internal/textwidth"
)

type xyChartData struct {
	title    string
	xLabels  []string
	yLabel   string
	yMin     float64
	yMax     float64
	autoY    bool
	barData  []float64
	lineData []float64
}

var (
	reXYHeader = regexp.MustCompile(`(?i)^\s*xychart-beta\s*$`)
	reXYTitle  = regexp.MustCompile(`(?i)^\s*title\s+"?([^"]*)"?\s*$`)
	reXYXAxis  = regexp.MustCompile(`(?i)^\s*x-axis\s+\[(.+)\]\s*$`)
	reXYYAxis  = regexp.MustCompile(`(?i)^\s*y-axis\s+"?([^"]*?)"?\s*([0-9.]+)\s*-->\s*([0-9.]+)\s*$`)
	reXYYLabel = regexp.MustCompile(`(?i)^\s*y-axis\s+"([^"]+)"\s*$`)
	reXYBar    = regexp.MustCompile(`(?i)^\s*bar\s+\[(.+)\]\s*$`)
	reXYLine   = regexp.MustCompile(`(?i)^\s*line\s+\[(.+)\]\s*$`)
)

func parseXYChart(source string) *xyChartData {
	xd := &xyChartData{autoY: true}
	lines := strings.Split(source, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "%%") {
			continue
		}
		if reXYHeader.MatchString(trimmed) {
			continue
		}
		if m := reXYTitle.FindStringSubmatch(trimmed); m != nil {
			xd.title = strings.TrimSpace(m[1])
			continue
		}
		if m := reXYXAxis.FindStringSubmatch(trimmed); m != nil {
			for _, label := range strings.Split(m[1], ",") {
				l := strings.TrimSpace(label)
				l = strings.Trim(l, "\"")
				if l != "" {
					xd.xLabels = append(xd.xLabels, l)
				}
			}
			continue
		}
		if m := reXYYAxis.FindStringSubmatch(trimmed); m != nil {
			xd.yLabel = strings.TrimSpace(m[1])
			xd.yMin, _ = strconv.ParseFloat(m[2], 64)
			xd.yMax, _ = strconv.ParseFloat(m[3], 64)
			xd.autoY = false
			continue
		}
		if m := reXYYLabel.FindStringSubmatch(trimmed); m != nil {
			xd.yLabel = strings.TrimSpace(m[1])
			continue
		}
		if m := reXYBar.FindStringSubmatch(trimmed); m != nil {
			xd.barData = parseFloatList(m[1])
			continue
		}
		if m := reXYLine.FindStringSubmatch(trimmed); m != nil {
			xd.lineData = parseFloatList(m[1])
			continue
		}
	}

	// Auto-determine y range
	if xd.autoY {
		xd.yMin = math.Inf(1)
		xd.yMax = math.Inf(-1)
		for _, v := range xd.barData {
			if v < xd.yMin {
				xd.yMin = v
			}
			if v > xd.yMax {
				xd.yMax = v
			}
		}
		for _, v := range xd.lineData {
			if v < xd.yMin {
				xd.yMin = v
			}
			if v > xd.yMax {
				xd.yMax = v
			}
		}
		if math.IsInf(xd.yMin, 1) {
			xd.yMin = 0
		}
		if math.IsInf(xd.yMax, -1) {
			xd.yMax = 100
		}
		// Round to nice bounds
		if xd.yMin > 0 {
			xd.yMin = 0
		}
		xd.yMax = math.Ceil(xd.yMax*1.1/100) * 100
	}

	return xd
}

func parseFloatList(s string) []float64 {
	var vals []float64
	for _, part := range strings.Split(s, ",") {
		v, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
		if err == nil {
			vals = append(vals, v)
		}
	}
	return vals
}

// RenderXYChart parses and renders a Mermaid xychart-beta diagram.
func RenderXYChart(source string, cs renderer.CharSet, theme *renderer.Theme) *renderer.Canvas {
	useASCII := cs.ASCII
	xd := parseXYChart(source)

	plotH := 15

	yLabelW := textwidth.String(formatNum(xd.yMax)) + 1
	if yLabelW < textwidth.String(formatNum(xd.yMin))+1 {
		yLabelW = textwidth.String(formatNum(xd.yMin)) + 1
	}
	if xd.yLabel != "" && textwidth.String(xd.yLabel)+2 > yLabelW {
		yLabelW = textwidth.String(xd.yLabel) + 2
	}

	// Scale plot width to terminal, with minimum from label count
	minPlotW := 30
	if len(xd.xLabels) > 0 {
		if lw := len(xd.xLabels) * 6; lw > minPlotW {
			minPlotW = lw
		}
	}
	plotW := scaleWidth(yLabelW+4, minPlotW, maxScaledWidth)

	titleRows := 0
	if xd.title != "" {
		titleRows = 2
	}

	canvasWidth := yLabelW + plotW + 2
	canvasHeight := titleRows + plotH + 4

	c := renderer.NewCanvas(canvasWidth, canvasHeight)
	c.SetCharSet(cs)

	// Wallpaper: base background behind entire diagram
	if theme != nil && theme.HasDepthColors() {
		for r := 0; r < canvasHeight; r++ {
			for col := 0; col < canvasWidth; col++ {
				c.SetFill(r, col, "subgraph_fill")
			}
		}
	}

	// Title
	if xd.title != "" {
		titleCol := (canvasWidth - textwidth.String(xd.title)) / 2
		c.PutText(0, titleCol, xd.title, "bold_label")
	}

	plotX := yLabelW
	plotY := titleRows
	plotBottom := plotY + plotH

	vLine := '│'
	hLine := '─'
	barCh := cs.Fills.Dark
	lineDot := cs.Dot
	if useASCII {
		vLine = '|'
		hLine = '-'
	}

	// Y axis
	for r := plotY; r <= plotBottom; r++ {
		c.Put(r, plotX-1, vLine, "edge")
	}

	// X axis
	for col := plotX; col < plotX+plotW; col++ {
		c.Put(plotBottom, col, hLine, "edge")
	}

	// Fill plot area background
	useRegion := theme != nil && theme.HasDepthColors()
	if useRegion {
		plotFill := "_ansi:" + theme.RegionStyle(0, 0)
		for r := plotY; r < plotBottom; r++ {
			for col := plotX; col < plotX+plotW; col++ {
				c.SetFill(r, col, plotFill)
			}
		}
	}

	// Y axis labels (top, middle, bottom)
	topLabel := formatNum(xd.yMax)
	midLabel := formatNum((xd.yMin + xd.yMax) / 2)
	botLabel := formatNum(xd.yMin)
	c.PutText(plotY, plotX-1-textwidth.String(topLabel), topLabel, "default")
	c.PutText(plotY+plotH/2, plotX-1-textwidth.String(midLabel), midLabel, "default")
	c.PutText(plotBottom, plotX-1-textwidth.String(botLabel), botLabel, "default")

	// Y axis label (vertical, abbreviated)
	if xd.yLabel != "" {
		c.PutText(plotY, 0, xd.yLabel, "label")
	}

	// Determine number of data points
	nPoints := len(xd.barData)
	if len(xd.lineData) > nPoints {
		nPoints = len(xd.lineData)
	}
	if len(xd.xLabels) > nPoints {
		nPoints = len(xd.xLabels)
	}
	if nPoints == 0 {
		return c
	}

	colW := plotW / nPoints
	if colW < 3 {
		colW = 3
	}
	barW := colW - 2
	if barW < 1 {
		barW = 1
	}

	yRange := xd.yMax - xd.yMin
	if yRange <= 0 {
		yRange = 1
	}

	// Draw bars
	for i, v := range xd.barData {
		barH := int((v - xd.yMin) / yRange * float64(plotH-1))
		if barH < 1 && v > xd.yMin {
			barH = 1
		}
		barX := plotX + i*colW + (colW-barW)/2
		barStyle := "node"
		if useRegion {
			barStyle = "_ansi:" + theme.RegionBarStyle(i+1, 1)
		}
		for row := plotBottom - barH; row < plotBottom; row++ {
			for dx := 0; dx < barW; dx++ {
				c.Put(row, barX+dx, barCh, barStyle)
			}
		}
	}

	// Draw line (after bars so dots render on top)
	if len(xd.lineData) > 0 {
		// Use bright white for line elements so they stand out on colored bars
		lineStyle := "arrow"
		connStyle := "bold_label"
		if useRegion {
			lineStyle = "_ansi:\033[1m\033[38;2;255;215;0m"   // bold gold
			connStyle = "_ansi:\033[1m\033[38;2;255;255;255m" // bold white
		}

		prevX, prevY := -1, -1
		for i, v := range xd.lineData {
			lx := plotX + i*colW + colW/2
			ly := plotBottom - 1 - int((v-xd.yMin)/yRange*float64(plotH-2))
			if ly < plotY {
				ly = plotY
			}

			c.Put(ly, lx, lineDot, lineStyle)

			// Connect to previous point. The runs reach the dot cells, whose
			// glyphs hide the arms, and pass over any bar in the way.
			if prevX >= 0 {
				if prevY == ly {
					xyConnect(c, ly, prevX, ly, lx, barCh, connStyle)
				} else {
					midCol := (prevX + lx) / 2
					xyConnect(c, prevY, prevX, prevY, midCol, barCh, connStyle)
					xyConnect(c, prevY, midCol, ly, midCol, barCh, connStyle)
					xyConnect(c, ly, midCol, ly, lx, barCh, connStyle)
				}
			}

			prevX, prevY = lx, ly
		}
	}

	// X axis labels — use section-colored text when themed
	for i, label := range xd.xLabels {
		if i >= nPoints {
			break
		}
		lx := plotX + i*colW + colW/2 - textwidth.String(label)/2
		if lx < 0 {
			lx = 0
		}
		xLabelStyle := "label"
		if useRegion {
			xLabelStyle = "_ansi:" + theme.RegionTextStyle(i+1, 0)
		}
		c.PutText(plotBottom+1, lx, label, xLabelStyle)
	}

	return c
}

func formatNum(v float64) string {
	if v == math.Trunc(v) {
		return strconv.Itoa(int(v))
	}
	return strconv.FormatFloat(v, 'f', 1, 64)
}

// xyConnect draws one straight run of a line series. A bar cell on the way
// is cleared first so the line shows over it, as the dots do.
func xyConnect(c *renderer.Canvas, r1, c1, r2, c2 int, barCh rune, style string) {
	for r := min(r1, r2); r <= max(r1, r2); r++ {
		for col := min(c1, c2); col <= max(c1, c2); col++ {
			if c.Get(r, col) == barCh {
				c.ClearCell(r, col)
			}
		}
	}
	if r1 == r2 {
		c.DrawHorizontal(r1, c1, c2, glyph.Light, style)
	} else {
		c.DrawVertical(c1, r1, r2, glyph.Light, style)
	}
}
