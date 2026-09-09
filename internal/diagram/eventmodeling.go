package diagram

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/aaronsb/mmaid-go/internal/glyph"
	"github.com/aaronsb/mmaid-go/internal/renderer"
)

// Skipped, against the grammar and db at mermaid fe0e2375:
//   - `entity`, `data`, `note` and `gwt` statements, and a frame's inline or
//     `[[ref]]` payload. None of them carries a position in the model.
//   - `accTitle` and `accDescr`.
//   - A frame whose entity type is outside the grammar's closed set. Upstream
//     fails the parse; this warns once on stderr and drops the line.
//   - A `->>` reference to a later frame. Upstream drops it too: the source
//     box is not positioned yet when the relation is decided.
//
// Divergences:
//   - A reset frame draws a heavy border. `renderer.ts` has no reset-specific
//     styling; a terminal has no fill to tell the two kinds of frame apart.
//   - A namespace keeps one lane wherever it next appears. That is the lookup
//     `db.ts` attempts, but `SwimlaneProps.namespace` is never set at this
//     commit, so `findSwimlaneByNamespace` never matches and upstream in fact
//     opens a fresh lane per namespaced frame. This port follows the declared
//     intent rather than the behaviour.

// emFrame is one time frame or reset frame: an entity of some kind at a point
// in the model's time order, optionally fed by earlier frames.
type emFrame struct {
	id      string
	kind    string // ui, pcr, cmd, rmo or evt
	reset   bool
	ns      string // the qualified name's namespace, if any
	name    string
	sources []string // the ids the `->>` references name
}

// emData is the parsed `eventmodeling` diagram.
type emData struct {
	title  string
	frames []emFrame
}

var (
	reEMHeader = regexp.MustCompile(`(?i)^eventmodeling$`)
	// EM_TITLE is case-sensitive and takes the rest of the line after one
	// space or tab, or nothing at all.
	reEMTitle  = regexp.MustCompile(`^title(?:[ \t](.*))?$`)
	reEMFrame  = regexp.MustCompile(`(?i)^(tf|timeframe|rf|resetframe)\s+(\d{1,3})\s+([A-Za-z]+)\s+([A-Za-z_][\w.]*)(.*)$`)
	reEMSource = regexp.MustCompile(`->>\s*(\d{1,3})`)
)

// emKind normalises the grammar's entity-type aliases. The set is closed, so
// anything else is a parse error rather than an event.
func emKind(s string) (string, bool) {
	switch strings.ToLower(s) {
	case "ui":
		return "ui", true
	case "pcr", "processor":
		return "pcr", true
	case "cmd", "command":
		return "cmd", true
	case "rmo", "readmodel":
		return "rmo", true
	case "evt", "event":
		return "evt", true
	}
	return "", false
}

// emBand returns the swimlane band an entity type belongs to: automation,
// command and read model, or the event stream.
func emBand(kind string) int {
	switch kind {
	case "ui", "pcr":
		return 0
	case "cmd", "rmo":
		return 1
	default:
		return 2
	}
}

var emBandLabel = [3]string{"UI/Automation", "Command/Read Model", "Events"}
var emBandPrefix = [3]string{"UI/A: ", "C/RM: ", "Stream: "}

// parseEventModeling parses a Mermaid eventmodeling definition.
//
//	eventmodeling
//	    tf 01 ui CartUI
//	    tf 02 cmd Inventory.AddItem ->> 01
//	    tf 03 evt Inventory.ItemAdded ->> 02
func parseEventModeling(source string) *emData {
	ed := &emData{}
	warned := false
	for _, line := range strings.Split(source, "\n") {
		// EM_SINGLE_LINE_COMMENT is a hidden terminal, so a `%%` comment may
		// follow a statement on the same line.
		if i := strings.Index(line, "%%"); i >= 0 {
			line = line[:i]
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || reEMHeader.MatchString(trimmed) {
			continue
		}
		if m := reEMTitle.FindStringSubmatch(trimmed); m != nil {
			ed.title = strings.TrimSpace(m[1])
			continue
		}
		m := reEMFrame.FindStringSubmatch(trimmed)
		if m == nil {
			continue
		}
		kind, ok := emKind(m[3])
		if !ok {
			if !warned {
				fmt.Fprintf(os.Stderr, "mmaid: eventmodeling: unknown entity type %q\n", m[3])
				warned = true
			}
			continue
		}
		f := emFrame{
			id:    m[2],
			kind:  kind,
			reset: strings.EqualFold(m[1][:1], "r"),
			name:  m[4],
		}
		// A namespace is one dot: `extractNamespace` splits on "." and takes
		// the first part only when there are exactly two, so
		// `Shop.Inventory.AddItem` is a plain name in its band's own lane.
		if parts := strings.Split(f.name, "."); len(parts) == 2 {
			f.ns, f.name = parts[0], parts[1]
		}
		for _, s := range reEMSource.FindAllStringSubmatch(m[5], -1) {
			f.sources = append(f.sources, s[1])
		}
		ed.frames = append(ed.frames, f)
	}
	return ed
}

// emLane is one swimlane: a band, or a namespace inside one.
type emLane struct {
	index int
	label string
	top   int // first row of the lane's boxes
}

// emBox is a frame placed on the canvas.
type emBox struct {
	frame emFrame
	lane  int   // index into the sorted lane slice
	from  []int // the boxes an arrow reaches this one from
	x, w  int
	row   int // the box's middle row
}

// emAssignLanes maps each frame to a lane. A namespace owns one lane wherever
// it first appears, so `Inventory.AddItem` and `Inventory.ItemAdded` share it;
// an unqualified name goes to its band's own lane.
func emAssignLanes(frames []emFrame) ([]emLane, map[string]int) {
	byIndex := map[int]*emLane{}
	nsLane := map[string]int{}
	var order []int

	add := func(index int, label string) {
		if _, ok := byIndex[index]; ok {
			return
		}
		byIndex[index] = &emLane{index: index, label: label}
		order = append(order, index)
	}
	nextInBand := func(band int) int {
		lo, hi := band*100, band*100+100
		next := lo
		for i := range byIndex {
			if i > lo && i < hi && i > next {
				next = i
			}
		}
		return next + 1
	}

	laneOf := make(map[string]int, len(frames))
	for _, f := range frames {
		band := emBand(f.kind)
		switch {
		case f.ns == "":
			add(band*100, emBandLabel[band])
			laneOf[f.id] = band * 100
		default:
			if i, ok := nsLane[f.ns]; ok {
				laneOf[f.id] = i
				continue
			}
			i := nextInBand(band)
			add(i, emBandPrefix[band]+f.ns)
			nsLane[f.ns] = i
			laneOf[f.id] = i
		}
	}

	// Sort the lane indices ascending: bands stay in order and a namespace
	// lane sits under the band it was opened in.
	for i := 1; i < len(order); i++ {
		for j := i; j > 0 && order[j] < order[j-1]; j-- {
			order[j], order[j-1] = order[j-1], order[j]
		}
	}
	lanes := make([]emLane, 0, len(order))
	slot := map[int]int{}
	for i, idx := range order {
		lanes = append(lanes, *byIndex[idx])
		slot[idx] = i
	}
	for id, idx := range laneOf {
		laneOf[id] = slot[idx]
	}
	return lanes, laneOf
}

const (
	emLaneRows = 4 // three box rows plus the separator under the lane
	emSlotGap  = 3 // free columns between one frame's box and the next
)

// emPlan is the whole diagram laid out: where each lane and box sits, and
// which boxes an arrow runs between.
type emPlan struct {
	title         string
	lanes         []emLane
	boxes         []emBox
	contentX      int
	width, height int
}

// emResolveSources fills in each box's incoming arrows. A reset frame never
// receives one; a frame with no `->>` takes the nearest earlier frame in a
// different lane, which is what `decidePositionRelation` falls back to.
func emResolveSources(boxes []emBox) {
	byID := make(map[string]int, len(boxes))
	for i, b := range boxes {
		byID[b.frame.id] = i
	}
	for i := range boxes {
		if boxes[i].frame.reset {
			continue
		}
		if ids := boxes[i].frame.sources; len(ids) > 0 {
			for _, id := range ids {
				if j, ok := byID[id]; ok {
					boxes[i].from = append(boxes[i].from, j)
				}
			}
			continue
		}
		for j := i - 1; j >= 0; j-- {
			if boxes[j].lane != boxes[i].lane {
				boxes[i].from = append(boxes[i].from, j)
				break
			}
		}
	}
}

// emLayout places the lanes down the canvas and the frames along it.
func emLayout(ed *emData) emPlan {
	lanes, laneOf := emAssignLanes(ed.frames)

	gutter := 0
	for _, l := range lanes {
		gutter = max(gutter, runeLen(l.label))
	}
	plan := emPlan{title: ed.title, lanes: lanes, contentX: gutter + 2}

	topRow := 0
	if ed.title != "" {
		topRow = 2
	}
	for i := range plan.lanes {
		plan.lanes[i].top = topRow + i*emLaneRows
	}

	plan.boxes = make([]emBox, 0, len(ed.frames))
	x := plan.contentX
	for _, f := range ed.frames {
		lane := laneOf[f.id]
		w := max(runeLen(f.name)+4, runeLen(f.id)+runeLen(f.kind)+6)
		plan.boxes = append(plan.boxes, emBox{
			frame: f, lane: lane, x: x, w: w, row: plan.lanes[lane].top + 1,
		})
		x += w + emSlotGap
	}
	emResolveSources(plan.boxes)

	plan.width = x + 1
	plan.height = topRow + len(plan.lanes)*emLaneRows
	return plan
}

// emDrawLanes writes each lane's label and the rule that closes it.
func emDrawLanes(c *renderer.Canvas, p emPlan, theme *renderer.Theme, useRegion bool) {
	for i, l := range p.lanes {
		style := "subgraph_label"
		ruleStyle := "subgraph"
		if useRegion {
			style = "_ansi:" + theme.RegionLabelStyle(i, 0)
			ruleStyle = "_ansi:" + theme.RegionBorderStyle(i, 0)
			for r := l.top; r < l.top+3; r++ {
				for col := range p.width {
					c.SetFill(r, col, "_ansi:"+theme.RegionStyle(i, 0))
				}
			}
		}
		c.PutText(l.top+1, 0, l.label, style)
		if i < len(p.lanes)-1 {
			c.Segment(l.top+3, p.contentX, l.top+3, p.width-2, glyph.Light, false, ruleStyle)
		}
	}
}

// emDrawBoxes draws one box per frame, its id and entity type riding on the
// top border where they do not steal the content row.
func emDrawBoxes(c *renderer.Canvas, p emPlan, theme *renderer.Theme, useRegion bool) {
	for _, b := range p.boxes {
		weight := glyph.Light
		if b.frame.reset {
			weight = glyph.Heavy
		}
		borderStyle := "node"
		labelStyle := "label"
		if useRegion {
			borderStyle = "_ansi:" + theme.RegionBorderStyle(b.lane, 1)
			labelStyle = "_ansi:" + theme.RegionLabelStyle(b.lane, 1)
		}
		top, bot := b.row-1, b.row+1
		left, right := b.x, b.x+b.w-1
		c.Segment(top, left, top, right, weight, false, borderStyle)
		c.Segment(bot, left, bot, right, weight, false, borderStyle)
		c.Segment(top, left, bot, left, weight, false, borderStyle)
		c.Segment(top, right, bot, right, weight, false, borderStyle)

		if useRegion {
			fill := "_ansi:" + theme.RegionStyle(b.lane, 1)
			for r := top; r <= bot; r++ {
				for col := left; col <= right; col++ {
					c.SetFill(r, col, fill)
				}
			}
		}
		c.PutText(b.row, left+(b.w-runeLen(b.frame.name))/2, b.frame.name, labelStyle)
		c.PutText(top, left+1, b.frame.id, "edge_label")
		c.PutText(top, right-runeLen(b.frame.kind), b.frame.kind, "edge_label")
	}
}

// emRouteArrows draws one arrow per resolved relation, through the free gap
// beside a box so no arm reaches a border.
func emRouteArrows(c *renderer.Canvas, p emPlan, cs renderer.CharSet) {
	occupied := func(row, c1, c2 int) bool {
		for _, b := range p.boxes {
			if b.row == row && b.x <= c2 && c1 <= b.x+b.w-1 {
				return true
			}
		}
		return false
	}
	for _, target := range p.boxes {
		for _, si := range target.from {
			src := p.boxes[si]
			srcRight := src.x + src.w - 1
			head := target.x - 1
			if head-2 < srcRight+1 {
				continue // the two boxes are too close to route between
			}
			// Prefer the channel beside the target and the run along the
			// source's row; fall back to the mirror when a box is in the way.
			channel := head - 1
			runRow := src.row
			if occupied(runRow, srcRight+1, channel) {
				channel = srcRight + 2
				runRow = target.row
				if occupied(runRow, channel, head-1) {
					continue
				}
			}
			c.Segment(src.row, srcRight, src.row, channel, glyph.Light, false, "edge")
			c.Segment(src.row, channel, target.row, channel, glyph.Light, false, "edge")
			// The run ends on the arrowhead's own cell, so its tail cell
			// keeps both arms and nothing reaches the target's border.
			c.Segment(target.row, channel, target.row, head, glyph.Light, false, "edge")
			c.Put(target.row, head, cs.ArrowRight, "arrow")
		}
	}
}

// RenderEventModeling parses and renders a Mermaid eventmodeling diagram:
// swimlanes as rows, frames as boxes left to right in time order, and an
// arrow into every frame that has a source. The canvas is as wide as the
// model needs, so a long model scrolls rather than wrapping.
func RenderEventModeling(source string, cs renderer.CharSet, theme *renderer.Theme) *renderer.Canvas {
	ed := parseEventModeling(source)
	if len(ed.frames) == 0 {
		c := renderer.NewCanvas(30, 1)
		c.PutText(0, 0, "[eventmodeling] no frames", "default")
		return c
	}

	p := emLayout(ed)
	c := renderer.NewCanvas(p.width, p.height)
	c.SetCharSet(cs)
	useRegion := theme != nil && theme.HasDepthColors()

	if p.title != "" {
		c.PutText(0, 0, p.title, "bold_label")
	}
	emDrawLanes(c, p, theme, useRegion)
	emDrawBoxes(c, p, theme, useRegion)
	emRouteArrows(c, p, cs)
	return c
}
