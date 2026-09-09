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

// RenderWardley parses and renders a Mermaid Wardley map.
func RenderWardley(source string, cs renderer.CharSet, theme *renderer.Theme) *renderer.Canvas {
	wm := parseWardley(source)
	if len(wm.nodes) == 0 {
		c := renderer.NewCanvas(26, 1)
		c.PutText(0, 0, "[wardley] no components", "default")
		return c
	}

	marks := wardleyMarks(cs)
	point, anchorPoint, inertiaBar := cs.Dot, cs.DoubleRing, cs.CrossEndpoint
	arrowRight, arrowLeft := cs.ArrowRight, cs.ArrowLeft

	const leftPad = 2 // the value-chain caption and a gap
	plotH := 22
	plotW := scaleWidth(leftPad+2, 40, maxScaledWidth)

	titleRows := 0
	if wm.title != "" {
		titleRows = 2
	}
	canvasWidth := leftPad + plotW + 1
	canvasHeight := titleRows + plotH + 3

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

	top, bottom := titleRows, titleRows+plotH-1
	left, right := leftPad, leftPad+plotW-1
	c.Segment(top, left, top, right, glyph.Light, false, "edge")
	c.Segment(bottom, left, bottom, right, glyph.Light, false, "edge")
	c.Segment(top, left, bottom, left, glyph.Light, false, "edge")
	c.Segment(top, right, bottom, right, glyph.Light, false, "edge")

	inLeft, inRight := left+1, right-1
	inTop, inBottom := top+1, bottom-1
	colOf := func(evo float64) int {
		return inLeft + int(clamp01(evo)*float64(inRight-inLeft))
	}
	rowOf := func(vis float64) int {
		return inBottom - int(clamp01(vis)*float64(inBottom-inTop))
	}

	// Evolution bands, each closed on the frame at both ends so its dashed
	// run resolves into a tee rather than ending in the air.
	start := 0.0
	for i, st := range wm.stages {
		if i < len(wm.stages)-1 {
			c.Segment(top, colOf(st.end), bottom, colOf(st.end), glyph.Dashed, false, "edge")
		}
		mid := colOf((start+st.end)/2) - textwidth.String(st.name)/2
		c.PutText(bottom+1, min(max(mid, 0), canvasWidth-textwidth.String(st.name)), st.name, "label")
		start = st.end
	}

	// The value chain runs up the left margin, a letter to a row.
	caption := "Value chain"
	for i, r := range caption {
		row := top + (plotH-len(caption))/2 + i
		if row > bottom {
			break
		}
		c.Put(row, 0, r, "label")
	}

	// Dependencies before the points, so a point and its label sit over the
	// line rather than under it.
	at := map[string][2]int{}
	for _, n := range wm.nodes {
		at[n.name] = [2]int{rowOf(n.vis), colOf(n.evo)}
	}
	for _, l := range wm.links {
		a, b := at[l.from], at[l.to]
		c.Segment(a[0], a[1], b[0], a[1], glyph.Light, false, "edge")
		c.Segment(b[0], a[1], b[0], b[1], glyph.Light, false, "edge")
		if l.label != "" {
			w := textwidth.String(l.label)
			lo, hi := min(a[1], b[1]), max(a[1], b[1])
			if hi-lo >= w+2 {
				c.PutText(b[0], (lo+hi-w)/2, l.label, "label")
			}
		}
	}

	// Evolution arrows run a row under their component, clear of the
	// dependency legs that lie on a source's column and a target's row, and
	// they stand down where something already reaches the arrowhead's point.
	for _, e := range wm.evolves {
		p := at[e.name]
		row, target := min(p[0]+1, inBottom), colOf(e.target)
		if target == p[1] {
			continue
		}
		head, beyond := arrowRight, glyph.W
		if target < p[1] {
			head, beyond = arrowLeft, glyph.E
		}
		if c.Arms(row, target+sign(target-p[1]))&beyond != 0 {
			continue
		}
		c.Segment(row, p[1], row, target, glyph.Light, false, "arrow")
		c.Put(row, target, head, "arrow")
	}

	for _, n := range wm.notes {
		col := min(max(colOf(n.evo), inLeft), inRight)
		c.PutText(rowOf(n.vis), col, textwidth.Truncate(n.text, inRight-col+1), "label")
	}

	for _, n := range wm.nodes {
		p := at[n.name]
		mark := point
		if m, ok := marks[n.sourced]; ok {
			mark = m
		} else if n.anchor {
			mark = anchorPoint
		}
		c.Put(p[0], p[1], mark, "arrow")
		if n.inertia {
			c.Put(p[0], min(p[1]+1, inRight), inertiaBar, "arrow")
		}
	}
	for _, n := range wm.nodes {
		p := at[n.name]
		style := "label"
		if n.anchor {
			style = "bold_label"
		}
		w := textwidth.String(n.label)
		gap := 2
		if n.inertia {
			gap = 3
		}
		if p[1]+gap+w <= inRight {
			c.PutText(p[0], p[1]+gap, n.label, style)
			continue
		}
		c.PutText(p[0], max(p[1]-w-1, inLeft), n.label, style)
	}

	return c
}

func clamp01(v float64) float64 {
	return min(max(v, 0), 1)
}
