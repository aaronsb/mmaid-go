package diagram

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/aaronsb/mmaid-go/internal/glyph"
	"github.com/aaronsb/mmaid-go/internal/renderer"
	"github.com/aaronsb/mmaid-go/internal/textwidth"
)

// Radar (`radar-beta`), from https://mermaid.js.org/syntax/radar.html.
//
// Read: `title`, `axis` with an id and an optional `["Label"]`, several to a
// line; `curve` with an id, an optional label and either a positional
// `{1, 2, 3}` or a keyed `{axis1: 20, axis2: 10}` value list, several to a
// line; `showLegend`, `max`, `min`, `graticule circle|polygon`, `ticks`.
//
// Skipped: `axisScaleFactor`, `axisLabelFactor` and `curveTension`, which
// shape an SVG the cell grid does not have; the `cScale${i}` and `radar` theme
// variables, colour coming from the theme this port already resolves.

type radarAxis struct {
	id    string
	label string
}

type radarCurve struct {
	id     string
	label  string
	body   string // the brace list, read once every axis is known
	values []float64
}

type radarChart struct {
	title      string
	axes       []radarAxis
	curves     []radarCurve
	showLegend bool
	min, max   float64
	hasMin     bool
	hasMax     bool
	polygon    bool
	ticks      int
}

var (
	reRadarHeader    = regexp.MustCompile(`(?i)^radar-beta\b`)
	reRadarTitle     = regexp.MustCompile(`(?i)^title\s+(.+)$`)
	reRadarAxis      = regexp.MustCompile(`(?i)^axis\s+(.+)$`)
	reRadarCurve     = regexp.MustCompile(`(?i)^curve\s+(.+)$`)
	reRadarOption    = regexp.MustCompile(`(?i)^(showLegend|max|min|ticks|graticule)\b\s*(.*)$`)
	reRadarDecl      = regexp.MustCompile(`^([A-Za-z_][\w-]*)\s*(?:\[\s*"?([^"\]]*)"?\s*\])?\s*(?:\{(.*)\})?$`)
	reRadarKeyedItem = regexp.MustCompile(`^([A-Za-z_][\w-]*)\s*:\s*(-?[0-9]*\.?[0-9]+)$`)
)

// splitTopLevel splits on a separator that is outside brackets, braces and
// quotes, so a curve's own comma-separated value list stays one field.
func splitTopLevel(s string, sep rune) []string {
	var out []string
	depth, quoted, start := 0, false, 0
	for i, r := range s {
		switch {
		case r == '"':
			quoted = !quoted
		case quoted:
		case r == '[' || r == '{' || r == '(':
			depth++
		case r == ']' || r == '}' || r == ')':
			if depth > 0 {
				depth--
			}
		case r == sep && depth == 0:
			out = append(out, strings.TrimSpace(s[start:i]))
			start = i + len(string(r))
		}
	}
	out = append(out, strings.TrimSpace(s[start:]))
	return out
}

// parseRadar parses radar source into a radarChart.
func parseRadar(source string) *radarChart {
	rc := &radarChart{showLegend: true, ticks: 5}

	for _, raw := range strings.Split(source, "\n") {
		line := strings.TrimSpace(stripLineComment(raw))
		if line == "" || reRadarHeader.MatchString(line) {
			continue
		}
		if m := reRadarTitle.FindStringSubmatch(line); m != nil {
			rc.title = strings.Trim(strings.TrimSpace(m[1]), `"`)
			continue
		}
		if m := reRadarAxis.FindStringSubmatch(line); m != nil {
			for _, decl := range splitTopLevel(m[1], ',') {
				d := reRadarDecl.FindStringSubmatch(decl)
				if d == nil {
					continue
				}
				label := d[2]
				if label == "" {
					label = d[1]
				}
				rc.axes = append(rc.axes, radarAxis{id: d[1], label: label})
			}
			continue
		}
		if m := reRadarCurve.FindStringSubmatch(line); m != nil {
			for _, decl := range splitTopLevel(m[1], ',') {
				d := reRadarDecl.FindStringSubmatch(decl)
				if d == nil {
					continue
				}
				label := d[2]
				if label == "" {
					label = d[1]
				}
				rc.curves = append(rc.curves, radarCurve{id: d[1], label: label, body: d[3]})
			}
			continue
		}
		if m := reRadarOption.FindStringSubmatch(line); m != nil {
			rc.setOption(strings.ToLower(m[1]), strings.TrimSpace(m[2]))
			continue
		}
	}
	// A keyed list names axes, so it is read once every `axis` line has been:
	// a `curve` written above its axes resolves the same as one written below.
	for i := range rc.curves {
		rc.curves[i].values = parseRadarValues(rc.curves[i].body, rc.axes)
	}
	return rc
}

// setOption applies one of the five option keywords.
func (rc *radarChart) setOption(key, value string) {
	switch key {
	case "showlegend":
		rc.showLegend = value == "" || !strings.EqualFold(value, "false")
	case "graticule":
		rc.polygon = strings.EqualFold(value, "polygon")
	case "ticks":
		if n, err := strconv.Atoi(value); err == nil && n > 0 {
			rc.ticks = n
		}
	case "max":
		if v, err := strconv.ParseFloat(value, 64); err == nil {
			rc.max, rc.hasMax = v, true
		}
	case "min":
		if v, err := strconv.ParseFloat(value, 64); err == nil {
			rc.min, rc.hasMin = v, true
		}
	}
}

// parseRadarValues reads a curve's brace list in either form: a keyed list
// against the axis ids, a positional one in axis order. An axis the keyed list
// does not name gets NaN, which the render reads as the low bound — the same
// place a positional list too short for the axes puts it.
func parseRadarValues(body string, axes []radarAxis) []float64 {
	items := splitTopLevel(body, ',')
	keyed := false
	for _, it := range items {
		if reRadarKeyedItem.MatchString(it) {
			keyed = true
			break
		}
	}
	if !keyed {
		var out []float64
		for _, it := range items {
			v, err := strconv.ParseFloat(strings.TrimSpace(it), 64)
			if err != nil {
				continue
			}
			out = append(out, v)
		}
		return out
	}
	index := make(map[string]int, len(axes))
	for i, a := range axes {
		index[a.id] = i
	}
	out := make([]float64, len(axes))
	for i := range out {
		out[i] = math.NaN()
	}
	for _, it := range items {
		m := reRadarKeyedItem.FindStringSubmatch(it)
		if m == nil {
			continue
		}
		i, ok := index[m[1]]
		if !ok {
			continue
		}
		out[i], _ = strconv.ParseFloat(m[2], 64)
	}
	return out
}

// radarMarkers is one marker per curve, so the curves stay apart without
// colour.
func radarMarkers(cs renderer.CharSet) []rune {
	return []rune{cs.Dot, cs.CircleEndpoint, cs.Diamond, cs.Bullseye, cs.Hexagon, cs.CrossEndpoint}
}

// bounds returns the value range the radius is scaled over, and it always
// returns a finite range wider than nothing.
//
// A declared range that is not one — `max 0` under a `min` of 0, `min 5` with
// `max 5`, or either written as NaN or Inf, all of which `ParseFloat` accepts
// — makes `(v-lo)/span` NaN, and `int(math.Round(NaN))` is `math.MinInt64`.
// A coordinate that size reaches the rasteriser as a span of 9e18 cells, which
// fills memory before `Render`'s recover can see anything. The declared range
// is therefore used only when it is usable, and the range over the finite
// values stands in when it is not.
func (rc *radarChart) bounds() (lo, hi float64) {
	lo = 0
	if rc.hasMin && finite(rc.min) {
		lo = rc.min
	}
	if rc.hasMax && finite(rc.max) && rc.max > lo {
		return lo, rc.max
	}
	declared := rc.hasMax || (rc.hasMin && !finite(rc.min))

	hi = math.Inf(-1)
	dropped := false
	for _, cv := range rc.curves {
		for _, v := range cv.values {
			switch {
			case math.IsNaN(v):
				// A keyed list leaves NaN where it named no axis; that is the
				// low bound, not a value out of range.
			case !finite(v):
				dropped = true
			default:
				hi = math.Max(hi, v)
			}
		}
	}
	if math.IsInf(hi, -1) || hi <= lo {
		hi = lo + 1
	}
	if declared || dropped {
		warnf("radar: %s; scaling over %g to %g instead", radarBoundsCause(rc, dropped), lo, hi)
	}
	return lo, hi
}

// radarBoundsCause names what made the declared range unusable.
func radarBoundsCause(rc *radarChart, dropped bool) string {
	switch {
	case rc.hasMax && !finite(rc.max):
		return "max is not a finite number"
	case rc.hasMin && !finite(rc.min):
		return "min is not a finite number"
	case rc.hasMax:
		return "max is not above min"
	case dropped:
		return "a curve value is not a finite number"
	}
	return "the declared range is unusable"
}

func finite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

// RenderRadar parses and renders a Mermaid radar diagram.
func RenderRadar(source string, cs renderer.CharSet, useColor bool, theme *renderer.Theme) *renderer.Canvas {
	rc := parseRadar(source)
	if len(rc.axes) < 3 {
		c := renderer.NewCanvas(34, 1)
		c.PutText(0, 0, "[radar] needs three or more axes", "default")
		return c
	}

	colors := pieColors
	if theme != nil && theme.HasPieBase() {
		colors = theme.PieColors(len(rc.curves))
	}
	markers := radarMarkers(cs)
	pen := pathPen{slash: cs.Slash, backslash: cs.Backslash, dot: cs.Dot, weight: glyph.Light}

	// Room for the axis labels beside the rim, and for the legend beside both.
	labelPad := 0
	for _, a := range rc.axes {
		labelPad = max(labelPad, textwidth.String(a.label))
	}
	labelPad += 2

	legendW, legendGap := 0, 0
	if rc.showLegend && len(rc.curves) > 0 {
		maxCurveW := 0
		for _, cv := range rc.curves {
			maxCurveW = max(maxCurveW, textwidth.String(cv.label))
		}
		legendW = maxCurveW + 8
		legendGap = 3
	}

	rx := (usableWidth() - 2*labelPad - legendGap - legendW - 2) / 2
	rx = min(max(rx, 10), 20)
	ry := max(rx/2, 5)

	titleRows := 0
	if rc.title != "" {
		titleRows = 2
	}

	plotW := 2*rx + 1
	canvasWidth := labelPad + plotW + labelPad + legendGap + legendW + 1
	plotRows := 2*ry + 3 // a row above and below the rim for the polar labels
	canvasHeight := titleRows + max(plotRows, len(rc.curves)+2) + 1

	c := renderer.NewCanvas(canvasWidth, canvasHeight)
	c.SetCharSet(cs)
	plot := canvasRect(c)

	if rc.title != "" {
		titleCol := max((canvasWidth-textwidth.String(rc.title))/2, 0)
		c.PutText(0, titleCol, rc.title, "bold_label")
	}

	cc := labelPad + rx
	cr := titleRows + 1 + ry

	// Graticule: one closed ring per tick, the outermost the rim.
	for t := 1; t <= rc.ticks; t++ {
		frac := float64(t) / float64(rc.ticks)
		ring := pen
		ring.style, ring.rounded, ring.weight = "edge", true, glyph.Dashed
		drawCellPath(c, cellPath(rc.ringVertices(cr, cc, rx, ry, frac), true, false, plot), true, ring)
	}

	// Spokes, drawn from the centre out so the rim cell's only arm points
	// back down the spoke.
	for i := range rc.axes {
		rr, rcol := radarPoint(cr, cc, rx, ry, i, len(rc.axes), 1)
		spoke := pen
		spoke.style, spoke.weight = "edge", glyph.Dashed
		drawCellPath(c, cellPath([][2]int{{cr, cc}, {rr, rcol}}, false, false, plot), false, spoke)
	}

	lo, hi := rc.bounds()
	span := hi - lo

	for ci, cv := range rc.curves {
		style := "label"
		if useColor {
			clr := colors[ci%len(colors)]
			style = fmt.Sprintf("_ansi:\033[38;2;%d;%d;%dm", clr[0], clr[1], clr[2])
		}
		var vertices [][2]int
		for i := range rc.axes {
			v := lo
			if i < len(cv.values) && finite(cv.values[i]) {
				v = cv.values[i]
			}
			frac := math.Min(math.Max((v-lo)/span, 0), 1)
			r, col := radarPoint(cr, cc, rx, ry, i, len(rc.axes), frac)
			vertices = append(vertices, [2]int{r, col})
		}
		curve := pen
		curve.style = style
		drawCellPath(c, cellPath(vertices, true, true, plot), true, curve)
		for _, v := range vertices {
			c.Put(v[0], v[1], markers[ci%len(markers)], style)
		}
	}

	rc.drawAxisLabels(c, cr, cc, rx, ry)
	if rc.showLegend && len(rc.curves) > 0 {
		rc.drawLegend(c, titleRows, labelPad+plotW+labelPad+legendGap, legendW, plotRows, colors, markers, useColor)
	}
	return c
}

// radarPoint returns the cell at fraction frac along axis i of n, the first
// axis pointing up and the rest running clockwise.
func radarPoint(cr, cc, rx, ry, i, n int, frac float64) (int, int) {
	a := -math.Pi/2 + 2*math.Pi*float64(i)/float64(n)
	col := cc + int(math.Round(float64(rx)*frac*math.Cos(a)))
	row := cr + int(math.Round(float64(ry)*frac*math.Sin(a)))
	return row, col
}

// ringVertices returns the vertices of one graticule ring: the axis points
// for a polygon graticule, a finely sampled ellipse for a circular one.
func (rc *radarChart) ringVertices(cr, cc, rx, ry int, frac float64) [][2]int {
	n := len(rc.axes)
	if !rc.polygon {
		n = max(24, 2*(rx+ry))
	}
	out := make([][2]int, 0, n)
	for i := range n {
		r, col := radarPoint(cr, cc, rx, ry, i, n, frac)
		out = append(out, [2]int{r, col})
	}
	return out
}

// drawAxisLabels writes each axis label beyond its spoke's end, outboard on
// the side the spoke points.
func (rc *radarChart) drawAxisLabels(c *renderer.Canvas, cr, cc, rx, ry int) {
	n := len(rc.axes)
	for i, a := range rc.axes {
		r, col := radarPoint(cr, cc, rx, ry, i, n, 1)
		w := textwidth.String(a.label)
		switch angle := -math.Pi/2 + 2*math.Pi*float64(i)/float64(n); {
		case math.Cos(angle) > 0.25:
			c.PutText(r, col+2, a.label, "label")
		case math.Cos(angle) < -0.25:
			c.PutText(r, max(col-w-2, 0), a.label, "label")
		case math.Sin(angle) < 0:
			c.PutText(r-1, max(col-w/2, 0), a.label, "label")
		default:
			c.PutText(r+1, max(col-w/2, 0), a.label, "label")
		}
	}
}

// drawLegend draws the pie's legend box: a swatch per curve beside its label.
func (rc *radarChart) drawLegend(c *renderer.Canvas, titleRows, left, width, plotRows int, colors [][3]int, markers []rune, useColor bool) {
	height := len(rc.curves) + 2
	top := max(titleRows+(plotRows-len(rc.curves))/2-1, titleRows)

	drawLegendBox(c, top, left, height, width, "node")

	for i, cv := range rc.curves {
		row := top + 1 + i
		style := "label"
		if useColor {
			clr := colors[i%len(colors)]
			style = fmt.Sprintf("_ansi:\033[38;2;%d;%d;%dm", clr[0], clr[1], clr[2])
		}
		swatch := left + 2
		c.Put(row, swatch, markers[i%len(markers)], style)
		c.PutText(row, swatch+2, cv.label, style)
	}
}
