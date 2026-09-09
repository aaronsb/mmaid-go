package diagram

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/aaronsb/mmaid-go/internal/glyph"
	"github.com/aaronsb/mmaid-go/internal/renderer"
	"github.com/aaronsb/mmaid-go/internal/textwidth"
)

// Wardley maps (`wardley-beta`), from https://mermaid.js.org/syntax/wardley.html.
//
// Read: `title`; `evolution` stage names, dual labels and `@` boundaries;
// `component` and `anchor` at `[visibility, evolution]` with the `(inertia)`,
// `(build)`, `(buy)`, `(outsource)` and `(market)` decorators; the link forms
// `->`, `-->`, `-.->`, `+>`, `+<`, `+<>` and `+'text'>`, with a `; label`;
// `evolve`; `note`.
//
// Skipped: `size`, the canvas being the terminal's; `pipeline`, whose nested
// components share one visibility the cell grid cannot stack; `annotations`
// and `annotation`, `accelerator` and `deaccelerator`, and the `-.- (x, y)`
// trend, each a second glyph vocabulary over the same points; the
// `label [dx, dy]` offset, labels being placed by the space beside a point.

type wardleyNode struct {
	name    string
	label   string
	vis     float64
	evo     float64
	anchor  bool
	inertia bool
	sourced string // build, buy, outsource or market
}

type wardleyLink struct {
	from, to string
	label    string
}

type wardleyEvolve struct {
	name   string
	target float64
}

type wardleyNote struct {
	text     string
	vis, evo float64
}

type wardleyStage struct {
	name string
	end  float64
}

type wardleyMap struct {
	title   string
	stages  []wardleyStage
	nodes   []wardleyNode
	links   []wardleyLink
	evolves []wardleyEvolve
	notes   []wardleyNote
}

var (
	reWardleyHeader = regexp.MustCompile(`(?i)^wardley-beta\b`)
	reWardleyTitle  = regexp.MustCompile(`(?i)^title\s+(.+)$`)
	reWardleyEvoAx  = regexp.MustCompile(`(?i)^evolution\s+(.+)$`)
	reWardleyNode   = regexp.MustCompile(`(?i)^(component|anchor)\s+(?:"([^"]*)"|([^"\[\]]+?))\s*\[\s*(-?[0-9]*\.?[0-9]+)\s*,\s*(-?[0-9]*\.?[0-9]+)\s*\](.*)$`)
	reWardleyDecor  = regexp.MustCompile(`\((inertia|build|buy|outsource|market)\)`)
	reWardleyLink   = regexp.MustCompile(`^(?:"([^"]*)"|([^"]+?))\s*(-\.->|-->|->|\+<>|\+<|\+>|\+'[^']*'>)\s*(?:"([^"]*)"|([^";]+?))\s*(?:;\s*(.*?))?\s*$`)
	reWardleyEvolve = regexp.MustCompile(`(?i)^evolve\s+(?:"([^"]*)"|(.+?))\s+(-?[0-9]*\.?[0-9]+)\s*$`)
	reWardleyNote   = regexp.MustCompile(`(?i)^note\s+"([^"]*)"\s*\[\s*(-?[0-9]*\.?[0-9]+)\s*,\s*(-?[0-9]*\.?[0-9]+)\s*\]\s*$`)
	reWardleyStage  = regexp.MustCompile(`^(.*?)(?:@\s*([0-9]*\.?[0-9]+))?$`)
)

// defaultWardleyStages is the evolution axis an `evolution` line replaces.
var defaultWardleyStages = []wardleyStage{
	{"Genesis", 0.25}, {"Custom", 0.5}, {"Product", 0.75}, {"Commodity", 1},
}

// parseWardley parses Wardley map source into a wardleyMap.
func parseWardley(source string) *wardleyMap {
	wm := &wardleyMap{stages: defaultWardleyStages}
	declared := map[string]bool{}

	for _, raw := range strings.Split(source, "\n") {
		line := strings.TrimSpace(stripLineComment(raw))
		if line == "" || reWardleyHeader.MatchString(line) {
			continue
		}
		if m := reWardleyTitle.FindStringSubmatch(line); m != nil {
			wm.title = strings.Trim(strings.TrimSpace(m[1]), `"`)
			continue
		}
		if m := reWardleyEvoAx.FindStringSubmatch(line); m != nil {
			if stages := parseWardleyStages(m[1]); len(stages) > 0 {
				wm.stages = stages
			}
			continue
		}
		if m := reWardleyNode.FindStringSubmatch(line); m != nil {
			n := wardleyNode{
				name:   pick(m[2], m[3]),
				vis:    parseFloatOr(m[4], 0),
				evo:    parseFloatOr(m[5], 0),
				anchor: strings.EqualFold(m[1], "anchor"),
			}
			n.label = n.name
			for _, d := range reWardleyDecor.FindAllStringSubmatch(m[6], -1) {
				if strings.EqualFold(d[1], "inertia") {
					n.inertia = true
				} else {
					n.sourced = strings.ToLower(d[1])
				}
			}
			declared[n.name] = true
			wm.nodes = append(wm.nodes, n)
			continue
		}
		if m := reWardleyEvolve.FindStringSubmatch(line); m != nil {
			wm.evolves = append(wm.evolves, wardleyEvolve{name: pick(m[1], m[2]), target: parseFloatOr(m[3], 0)})
			continue
		}
		if m := reWardleyNote.FindStringSubmatch(line); m != nil {
			wm.notes = append(wm.notes, wardleyNote{text: m[1], vis: parseFloatOr(m[2], 0), evo: parseFloatOr(m[3], 0)})
			continue
		}
		if m := reWardleyLink.FindStringSubmatch(line); m != nil {
			from, to := pick(m[1], m[2]), pick(m[4], m[5])
			if declared[from] && declared[to] {
				wm.links = append(wm.links, wardleyLink{from: from, to: to, label: pick(wardleyFlowLabel(m[3]), m[6])})
			}
			continue
		}
	}
	// An evolve names a component; one that names nothing declared is dropped.
	kept := wm.evolves[:0]
	for _, e := range wm.evolves {
		if declared[e.name] {
			kept = append(kept, e)
		}
	}
	wm.evolves = kept
	return wm
}

// wardleyFlowLabel returns the text a `+'text'>` flow carries.
func wardleyFlowLabel(op string) string {
	if strings.HasPrefix(op, "+'") {
		if i := strings.LastIndex(op, "'"); i > 1 {
			return op[2:i]
		}
	}
	return ""
}

// parseWardleyStages reads an evolution axis: names joined by `->`, each
// optionally `Name / Alias` and optionally `@boundary`. Boundaries left
// unstated are spread evenly.
func parseWardleyStages(spec string) []wardleyStage {
	parts := strings.Split(spec, "->")
	out := make([]wardleyStage, 0, len(parts))
	for i, p := range parts {
		m := reWardleyStage.FindStringSubmatch(strings.TrimSpace(p))
		name := strings.TrimSpace(m[1])
		if name == "" {
			continue
		}
		end := float64(i+1) / float64(len(parts))
		if m[2] != "" {
			end = parseFloatOr(m[2], end)
		}
		out = append(out, wardleyStage{name: name, end: end})
	}
	return out
}

func pick(a, b string) string {
	if a != "" {
		return a
	}
	return strings.TrimSpace(b)
}

func parseFloatOr(s string, fallback float64) float64 {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return fallback
	}
	return v
}

// wardleyMarks is the point glyph a sourcing decorator draws.
func wardleyMarks(cs renderer.CharSet) map[string]rune {
	return map[string]rune{
		"build":     cs.Hexagon,
		"buy":       cs.Diamond,
		"outsource": cs.Ring,
		"market":    cs.CircleEndpoint,
	}
}

// wardleyLayout is where the frame, the bands and every point sit once the
// canvas width is known. The draw helpers take it rather than recomputing it.
type wardleyLayout struct {
	c                        *renderer.Canvas
	cs                       renderer.CharSet
	marks                    chartMarks
	top, bottom, left, right int
	inTop, inBottom          int
	inLeft, inRight          int
	at                       map[string][2]int
}

func (l wardleyLayout) colOf(evo float64) int {
	return l.inLeft + int(clamp01(evo)*float64(l.inRight-l.inLeft))
}

func (l wardleyLayout) rowOf(vis float64) int {
	return l.inBottom - int(clamp01(vis)*float64(l.inBottom-l.inTop))
}

// RenderWardley parses and renders a Mermaid Wardley map.
func RenderWardley(source string, cs renderer.CharSet, theme *renderer.Theme) *renderer.Canvas {
	wm := parseWardley(source)
	if len(wm.nodes) == 0 {
		c := renderer.NewCanvas(26, 1)
		c.PutText(0, 0, "[wardley] no components", "default")
		return c
	}

	const leftPad = 2 // the value-chain caption and a gap
	plotH := 22
	plotW := scaleWidth(leftPad+2, 40, maxScaledWidth)

	titleRows := 0
	if wm.title != "" {
		titleRows = 2
	}
	canvasWidth := leftPad + plotW + 1
	canvasHeight := titleRows + plotH + 4

	c := renderer.NewCanvas(canvasWidth, canvasHeight)
	c.SetCharSet(cs)
	if theme != nil && theme.HasDepthColors() {
		for r := range canvasHeight {
			for col := range canvasWidth {
				c.SetFill(r, col, "subgraph_fill")
			}
		}
	}
	if wm.title != "" {
		c.PutText(0, max((canvasWidth-textwidth.String(wm.title))/2, 0), wm.title, "bold_label")
	}

	l := wardleyLayout{
		c: c, cs: cs, marks: marksFor(cs),
		top: titleRows, bottom: titleRows + plotH - 1,
		left: leftPad, right: leftPad + plotW - 1,
		inTop: titleRows + 1, inBottom: titleRows + plotH - 2,
		inLeft: leftPad + 1, inRight: leftPad + plotW - 2,
		at: map[string][2]int{},
	}
	for _, n := range wm.nodes {
		l.at[n.name] = [2]int{l.rowOf(n.vis), l.colOf(n.evo)}
	}

	l.drawFrame(wm, plotH)
	l.drawLinks(wm)
	l.drawEvolves(wm)
	l.drawMarks(wm)
	l.drawLabels(wm)
	l.drawNotes(wm)
	return c
}

// drawFrame draws the plot border, the evolution bands and their names, and
// the value-chain caption up the left margin.
func (l wardleyLayout) drawFrame(wm *wardleyMap, plotH int) {
	c := l.c
	c.Segment(l.top, l.left, l.top, l.right, glyph.Light, false, "edge")
	c.Segment(l.bottom, l.left, l.bottom, l.right, glyph.Light, false, "edge")
	c.Segment(l.top, l.left, l.bottom, l.left, glyph.Light, false, "edge")
	c.Segment(l.top, l.right, l.bottom, l.right, glyph.Light, false, "edge")

	// Each band closes on the frame at both ends, so its dashed run resolves
	// into a tee rather than ending in the air. A name wider than its band is
	// cut; one that would land on the name beside it drops a row.
	start := 0.0
	for i, st := range wm.stages {
		if i < len(wm.stages)-1 {
			c.Segment(l.top, l.colOf(st.end), l.bottom, l.colOf(st.end), glyph.Dashed, false, "edge")
		}
		name := truncateMark(st.name, l.colOf(st.end)-l.colOf(start)-1, l.marks)
		mid := l.colOf((start+st.end)/2) - textwidth.String(name)/2
		col := min(max(mid, 0), c.Width-textwidth.String(name))
		placeText(c, l.bottom+1, col, 2, name, "label", l.marks, spanFree)
		start = st.end
	}

	caption := "Value chain"
	for i, r := range caption {
		row := l.top + (plotH-len(caption))/2 + i
		if row > l.bottom {
			break
		}
		c.Put(row, 0, r, "label")
	}
}

// drawLinks draws each dependency as two axis-aligned legs, so a crossing
// merges into a junction instead of overwriting one.
func (l wardleyLayout) drawLinks(wm *wardleyMap) {
	for _, link := range wm.links {
		a, b := l.at[link.from], l.at[link.to]
		l.c.Segment(a[0], a[1], b[0], a[1], glyph.Light, false, "edge")
		l.c.Segment(b[0], a[1], b[0], b[1], glyph.Light, false, "edge")
		if link.label == "" {
			continue
		}
		w := textwidth.String(link.label)
		lo, hi := min(a[1], b[1]), max(a[1], b[1])
		if hi-lo >= w+2 {
			col := (lo + hi - w) / 2
			clearSpan(l.c, b[0], col, w)
			l.c.PutText(b[0], col, link.label, "label")
		}
	}
}

// drawEvolves draws each evolution arrow a row off its component, clear of the
// dependency legs that lie on a source's column and a target's row.
//
// An arrow is never dropped for want of a clear row: the statement is the
// user's and the map has to carry it. Where every row within reach is taken
// the arrow is drawn anyway and its head, a literal, wins the cell it lands
// on, which still satisfies ADR-101's rule 1 for the arm that was there.
func (l wardleyLayout) drawEvolves(wm *wardleyMap) {
	for _, e := range wm.evolves {
		p := l.at[e.name]
		target := l.colOf(e.target)
		if target == p[1] {
			continue
		}
		head, beyond := l.cs.ArrowRight, glyph.W
		if target < p[1] {
			head, beyond = l.cs.ArrowLeft, glyph.E
		}
		row := p[0]
		for _, candidate := range []int{p[0] + 1, p[0] + 2, p[0] - 1, p[0] - 2} {
			if candidate < l.inTop || candidate > l.inBottom {
				continue
			}
			row = candidate
			if l.c.Arms(candidate, target+sign(target-p[1]))&beyond == 0 {
				break
			}
		}
		l.c.Segment(row, p[1], row, target, glyph.Light, false, "arrow")
		l.c.Put(row, target, head, "arrow")
	}
}

// drawMarks draws a point per component, its glyph chosen by the sourcing
// decorator, with an inertia mark beside it.
func (l wardleyLayout) drawMarks(wm *wardleyMap) {
	byDecorator := wardleyMarks(l.cs)
	for _, n := range wm.nodes {
		p := l.at[n.name]
		mark := l.cs.Dot
		if m, ok := byDecorator[n.sourced]; ok {
			mark = m
		} else if n.anchor {
			mark = l.cs.DoubleRing
		}
		l.c.Put(p[0], p[1], mark, "arrow")
		if n.inertia {
			l.c.Put(p[0], min(p[1]+1, l.inRight), l.cs.CrossEndpoint, "arrow")
		}
	}
}

// drawLabels writes each component's name beside its point, clearing the cells
// first: `Put` skips a space, so a label written straight over a dependency
// leg would keep the leg between its words and read as one.
func (l wardleyLayout) drawLabels(wm *wardleyMap) {
	for _, n := range wm.nodes {
		p := l.at[n.name]
		style := "label"
		if n.anchor {
			style = "bold_label"
		}
		w := textwidth.String(n.label)
		gap := 2
		if n.inertia {
			gap = 3
		}
		col := p[1] + gap
		if col+w > l.inRight {
			col = max(p[1]-w-1, l.inLeft)
		}
		clearSpan(l.c, p[0], col, w)
		l.c.PutText(p[0], col, n.label, style)
	}
}

// drawNotes writes the notes last, over nothing: a note that would land on a
// mark or a label drops to the next free row, and where none is free within
// reach it is cut to the space it has.
func (l wardleyLayout) drawNotes(wm *wardleyMap) {
	for _, n := range wm.notes {
		col := min(max(l.colOf(n.evo), l.inLeft), l.inRight)
		text := truncateMark(n.text, l.inRight-col+1, l.marks)
		row := min(max(l.rowOf(n.vis), l.inTop), l.inBottom)
		placeText(l.c, row, col, 3, text, "label", l.marks, spanClearOfText)
	}
}

func clamp01(v float64) float64 {
	return min(max(v, 0), 1)
}
