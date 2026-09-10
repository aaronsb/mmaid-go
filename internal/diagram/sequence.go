// Package diagram provides specialized diagram renderers that bypass the
// generic graph layout pipeline.
package diagram

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/aaronsb/mmaid-go/internal/glyph"
	"github.com/aaronsb/mmaid-go/internal/renderer"
	"github.com/aaronsb/mmaid-go/internal/textwidth"
)

// ── layout constants ────────────────────────────────────────────────

const (
	boxPad        = 4  // horizontal padding inside participant boxes
	boxHeight     = 3  // participant box height
	actorHeight   = 5  // actor stick-figure height
	minGap        = 16 // minimum gap between participant centers
	eventRowH     = 2  // rows per message event
	noteRowH      = 4  // rows per note event (3-row box + 1 gap)
	blockStartH   = 3  // rows for block start (border + label + gap)
	blockSectionH = 2  // rows for section break (dashed line + gap)
	blockEndH     = 2  // rows for block end (bottom border + gap)
	topMargin     = 0
	bottomMargin  = 1
)

// kindHeight maps participant kind to header height.
var kindHeight = map[string]int{
	"participant": 3,
	"actor":       5,
	"database":    5,
	"queue":       5,
	"boundary":    5,
	"control":     5,
	"entity":      5,
	"collections": 5,
}

// ── Model types (unexported) ────────────────────────────────────────

type participant struct {
	id    string
	label string
	kind  string // participant|actor|database|queue|boundary|control|entity|collections
}

type message struct {
	source    string
	target    string
	label     string
	lineType  string // solid|dotted
	arrowType string // arrow|cross|open|async|bidirectional
}

type note struct {
	text         string
	position     string   // rightof|leftof|over
	participants []string // participant ids
}

type activateEvent struct {
	participant string
	active      bool
}

type blockSection struct {
	label  string
	events []interface{}
}

type block struct {
	kind     string // loop|alt|opt|par|critical|break|rect
	label    string
	events   []interface{}
	sections []*blockSection
}

type destroyEvent struct {
	participant string
}

type sequenceDiagram struct {
	// title is drawn above the participants. Mermaid's own sequence parser
	// has no title statement; ZenUML's does, and both render through this
	// model.
	title        string
	participants []*participant
	events       []interface{}
	autonumber   bool
	warnings     []string
}

// ── Sentinel types for flattened events ─────────────────────────────

type blockStart struct {
	blk   *block
	depth int
}

type blockSectionBreak struct {
	section *blockSection
	depth   int
}

type blockEnd struct {
	blk   *block
	depth int
}

// ── Parser ──────────────────────────────────────────────────────────

// Arrow patterns ordered by specificity (longest match first).
var arrowPatterns = []struct {
	pattern   string
	lineType  string
	arrowType string
}{
	{"<<-->>", "dotted", "bidirectional"},
	{"<<->>", "solid", "bidirectional"},
	{"-->>", "dotted", "arrow"},
	{"->>", "solid", "arrow"},
	{"--x", "dotted", "cross"},
	{"-x", "solid", "cross"},
	{"--)", "dotted", "async"},
	{"-)", "solid", "async"},
	{"-->", "dotted", "open"},
	{"->", "solid", "open"},
}

// Build the message regex from arrow patterns.
var messageRE *regexp.Regexp

func init() {
	parts := make([]string, len(arrowPatterns))
	for i, ap := range arrowPatterns {
		parts[i] = regexp.QuoteMeta(ap.pattern)
	}
	arrowAlt := strings.Join(parts, "|")
	messageRE = regexp.MustCompile(`^\s*(\S+?)\s*(` + arrowAlt + `)\s*(\S+?)\s*(?::\s*(.*?))?\s*$`)
}

var participantKindRE = regexp.MustCompile(
	`(?i)^\s*(?:create\s+)?(participant|actor|database|queue|boundary|control|entity|collections)\s+(\S+)(?:\s+as\s+(.+?))?\s*$`)

var noteRE = regexp.MustCompile(
	`(?i)^\s*Note\s+(right\s+of|left\s+of|over)\s+(\S+?)(?:\s*,\s*(\S+?))?\s*:\s*(.*?)\s*$`)

var blockStartRE = regexp.MustCompile(
	`(?i)^\s*(loop|alt|opt|par|critical|break|rect)\b\s*(.*?)\s*$`)

var blockSectionRE = regexp.MustCompile(
	`(?i)^\s*(else|and|option)\b\s*(.*?)\s*$`)

var blockEndRE = regexp.MustCompile(`(?i)^\s*end\s*$`)

var activateRE = regexp.MustCompile(`(?i)^\s*(activate|deactivate)\s+(\S+)\s*$`)

var destroyRE = regexp.MustCompile(`(?i)^\s*destroy\s+(\S+)\s*$`)

var brRE = regexp.MustCompile(`(?i)<br\s*/?>`)

func lookupArrow(arrowStr string) (string, string) {
	for _, ap := range arrowPatterns {
		if arrowStr == ap.pattern {
			return ap.lineType, ap.arrowType
		}
	}
	return "solid", "open"
}

func ensureParticipant(diagram *sequenceDiagram, pid string) {
	for _, p := range diagram.participants {
		if p.id == pid {
			return
		}
	}
	diagram.participants = append(diagram.participants, &participant{id: pid, label: pid, kind: "participant"})
}

func parseSequenceDiagram(text string) *sequenceDiagram {
	diagram := &sequenceDiagram{}

	// Stack-based parsing for nested blocks.
	// eventStack[0] is the top-level events list.
	eventStack := []*[]interface{}{&diagram.events}
	var blockStack []*block

	lines := strings.Split(text, "\n")
	for _, line := range lines {
		stripped := strings.TrimSpace(line)

		// Skip empty lines, header, and comments
		if stripped == "" || strings.HasPrefix(strings.ToLower(stripped), "sequencediagram") || strings.HasPrefix(stripped, "%%") {
			continue
		}

		// Autonumber keyword
		if strings.ToLower(stripped) == "autonumber" {
			diagram.autonumber = true
			continue
		}

		// Block end
		if blockEndRE.MatchString(stripped) {
			if len(blockStack) > 0 {
				blockStack = blockStack[:len(blockStack)-1]
				eventStack = eventStack[:len(eventStack)-1]
			}
			continue
		}

		// Block section (else / and)
		if m := blockSectionRE.FindStringSubmatch(stripped); m != nil && len(blockStack) > 0 {
			sectionLabel := strings.TrimSpace(m[2])
			sec := &blockSection{label: sectionLabel}
			blockStack[len(blockStack)-1].sections = append(blockStack[len(blockStack)-1].sections, sec)
			// Switch current event target to this section's events
			eventStack[len(eventStack)-1] = &sec.events
			continue
		}

		// Block start (loop, alt, opt, par, critical, break)
		if m := blockStartRE.FindStringSubmatch(stripped); m != nil {
			kind := strings.ToLower(m[1])
			label := strings.TrimSpace(m[2])
			blk := &block{kind: kind, label: label}
			*eventStack[len(eventStack)-1] = append(*eventStack[len(eventStack)-1], blk)
			blockStack = append(blockStack, blk)
			eventStack = append(eventStack, &blk.events)
			continue
		}

		// Activate/deactivate keywords
		if m := activateRE.FindStringSubmatch(stripped); m != nil {
			keyword := strings.ToLower(m[1])
			pid := m[2]
			ensureParticipant(diagram, pid)
			*eventStack[len(eventStack)-1] = append(*eventStack[len(eventStack)-1], &activateEvent{
				participant: pid,
				active:      keyword == "activate",
			})
			continue
		}

		// Note declarations
		if m := noteRE.FindStringSubmatch(stripped); m != nil {
			posRaw, p1, p2, noteText := m[1], m[2], m[3], m[4]
			position := strings.ToLower(strings.ReplaceAll(posRaw, " ", ""))
			participants := []string{p1}
			if p2 != "" {
				participants = append(participants, p2)
			}
			ensureParticipant(diagram, p1)
			if p2 != "" {
				ensureParticipant(diagram, p2)
			}
			noteText = brRE.ReplaceAllString(strings.TrimSpace(noteText), "\n")
			*eventStack[len(eventStack)-1] = append(*eventStack[len(eventStack)-1], &note{
				text:         noteText,
				position:     position,
				participants: participants,
			})
			continue
		}

		// Destroy keyword
		if m := destroyRE.FindStringSubmatch(stripped); m != nil {
			pid := m[1]
			ensureParticipant(diagram, pid)
			*eventStack[len(eventStack)-1] = append(*eventStack[len(eventStack)-1], &destroyEvent{
				participant: pid,
			})
			continue
		}

		// Participant kind declarations
		if m := participantKindRE.FindStringSubmatch(stripped); m != nil {
			kind := strings.ToLower(m[1])
			pid := m[2]
			label := strings.TrimSpace(m[3])
			if label == "" {
				label = pid
			}
			found := false
			for _, p := range diagram.participants {
				if p.id == pid {
					p.label = label
					p.kind = kind
					found = true
					break
				}
			}
			if !found {
				diagram.participants = append(diagram.participants, &participant{id: pid, label: label, kind: kind})
			}
			continue
		}

		// Message lines
		if m := messageRE.FindStringSubmatch(stripped); m != nil {
			rawSource, arrow, rawTarget := m[1], m[2], m[3]
			msgLabel := ""
			if len(m) > 4 {
				msgLabel = strings.TrimSpace(m[4])
			}

			// Detect inline activation markers (+/-) on source/target
			var sourceActivate *bool
			var targetActivate *bool

			source := rawSource
			if len(source) > 0 && (source[0] == '+' || source[0] == '-') {
				val := source[0] == '+'
				sourceActivate = &val
				source = source[1:]
			}
			target := rawTarget
			if len(target) > 0 && (target[0] == '+' || target[0] == '-') {
				val := target[0] == '+'
				targetActivate = &val
				target = target[1:]
			}
			// Also check trailing +/- on target
			if len(target) > 0 && (target[len(target)-1] == '+' || target[len(target)-1] == '-') {
				val := target[len(target)-1] == '+'
				targetActivate = &val
				target = target[:len(target)-1]
			}

			ensureParticipant(diagram, source)
			ensureParticipant(diagram, target)
			lineType, arrowType := lookupArrow(arrow)
			*eventStack[len(eventStack)-1] = append(*eventStack[len(eventStack)-1], &message{
				source:    source,
				target:    target,
				label:     msgLabel,
				lineType:  lineType,
				arrowType: arrowType,
			})

			// Emit ActivateEvents for inline markers
			if sourceActivate != nil {
				*eventStack[len(eventStack)-1] = append(*eventStack[len(eventStack)-1], &activateEvent{
					participant: source,
					active:      *sourceActivate,
				})
			}
			if targetActivate != nil {
				*eventStack[len(eventStack)-1] = append(*eventStack[len(eventStack)-1], &activateEvent{
					participant: target,
					active:      *targetActivate,
				})
			}
			continue
		}

		// Unrecognized line
		diagram.warnings = append(diagram.warnings, fmt.Sprintf("Unrecognized line: %q", stripped))
	}

	return diagram
}

// ── Flatten events ──────────────────────────────────────────────────

func flattenEvents(events []interface{}, depth int) []interface{} {
	var result []interface{}
	for _, ev := range events {
		switch e := ev.(type) {
		case *block:
			result = append(result, &blockStart{blk: e, depth: depth})
			result = append(result, flattenEvents(e.events, depth+1)...)
			for _, sec := range e.sections {
				result = append(result, &blockSectionBreak{section: sec, depth: depth})
				result = append(result, flattenEvents(sec.events, depth+1)...)
			}
			result = append(result, &blockEnd{blk: e, depth: depth})
		default:
			result = append(result, ev)
		}
	}
	return result
}

// ── Helper functions ────────────────────────────────────────────────

func noteLines(n *note) []string {
	if strings.Contains(n.text, "\n") {
		return strings.Split(n.text, "\n")
	}
	return []string{n.text}
}

func participantIndex(diagram *sequenceDiagram, pid string) int {
	for i, p := range diagram.participants {
		if p.id == pid {
			return i
		}
	}
	return -1
}

func effectiveLabel(msg *message, msgNumber *int) string {
	if msgNumber != nil {
		prefix := fmt.Sprintf("%d: ", *msgNumber)
		if msg.label != "" {
			return prefix + msg.label
		}
		return strings.TrimRight(prefix, " ")
	}
	return msg.label
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func absInt(a int) int {
	if a < 0 {
		return -a
	}
	return a
}

// ── Layout computation ──────────────────────────────────────────────

type layoutResult struct {
	colCenters   []int
	boxWidths    []int
	canvasWidth  int
	canvasHeight int
	titleRows    int
	headerHeight int
	rowOffsets   []int
}

func computeLayout(diagram *sequenceDiagram, autonumber bool, flatEvents []interface{}) *layoutResult {
	n := len(diagram.participants)
	if n == 0 {
		return &layoutResult{}
	}

	// Box widths based on label length
	boxWidths := make([]int, n)
	for i, p := range diagram.participants {
		boxWidths[i] = maxInt(textwidth.String(p.label)+boxPad, 12)
	}

	// Header height: tallest participant kind
	headerHeight := 3
	for _, p := range diagram.participants {
		h, ok := kindHeight[p.kind]
		if !ok {
			h = 3
		}
		headerHeight = maxInt(headerHeight, h)
	}

	// Compute per-event heights and effective labels for gap computation
	eventHeights := make([]int, len(flatEvents))
	effectiveLabels := make([]string, len(flatEvents))
	msgCounter := 0

	for i, ev := range flatEvents {
		switch e := ev.(type) {
		case *activateEvent:
			eventHeights[i] = 0
			effectiveLabels[i] = ""
		case *destroyEvent:
			_ = e
			eventHeights[i] = eventRowH
			effectiveLabels[i] = ""
		case *note:
			lines := noteLines(e)
			noteH := len(lines) + 2 + 1
			eventHeights[i] = noteH
			effectiveLabels[i] = ""
		case *blockStart:
			eventHeights[i] = blockStartH
			effectiveLabels[i] = ""
		case *blockSectionBreak:
			eventHeights[i] = blockSectionH
			effectiveLabels[i] = ""
		case *blockEnd:
			eventHeights[i] = blockEndH
			effectiveLabels[i] = ""
		case *message:
			msgCounter++
			var num *int
			if autonumber {
				n := msgCounter
				num = &n
			}
			eff := effectiveLabel(e, num)
			effectiveLabels[i] = eff
			eventHeights[i] = eventRowH
		default:
			eventHeights[i] = 0
			effectiveLabels[i] = ""
		}
	}

	// Compute per-gap minimum widths
	gapMins := make([]int, n-1)
	for i := range gapMins {
		gapMins[i] = minGap
	}

	for evIdx, ev := range flatEvents {
		switch e := ev.(type) {
		case *note:
			lines := noteLines(e)
			noteWidth := 4
			for _, line := range lines {
				noteWidth = maxInt(noteWidth, textwidth.String(line)+4)
			}
			for _, pid := range e.participants {
				pi := participantIndex(diagram, pid)
				if pi < 0 {
					continue
				}
				if e.position == "rightof" && pi < n-1 {
					gapMins[pi] = maxInt(gapMins[pi], noteWidth+4)
				} else if e.position == "leftof" && pi > 0 {
					gapMins[pi-1] = maxInt(gapMins[pi-1], noteWidth+4)
				}
			}
			if e.position == "over" && len(e.participants) == 2 {
				p1i := participantIndex(diagram, e.participants[0])
				p2i := participantIndex(diagram, e.participants[1])
				if p1i >= 0 && p2i >= 0 {
					lo := minInt(p1i, p2i)
					hi := maxInt(p1i, p2i)
					spans := hi - lo
					perGap := (noteWidth + spans - 1) / spans
					for g := lo; g < hi; g++ {
						gapMins[g] = maxInt(gapMins[g], perGap)
					}
				}
			}

		case *message:
			eff := effectiveLabels[evIdx]
			si := participantIndex(diagram, e.source)
			ti := participantIndex(diagram, e.target)
			if si < 0 || ti < 0 || si == ti {
				continue
			}
			lo := minInt(si, ti)
			hi := maxInt(si, ti)
			labelNeed := textwidth.String(eff) + 6
			spans := hi - lo
			perGap := (labelNeed + spans - 1) / spans
			for g := lo; g < hi; g++ {
				gapMins[g] = maxInt(gapMins[g], perGap)
			}
		}
	}

	// Scale gaps to terminal width: expand if slack, compress if overflow
	if n > 1 {
		naturalW := boxWidths[0]/2 + 1
		for i := 0; i < n-1; i++ {
			naturalW += gapMins[i]
		}
		naturalW += boxWidths[n-1]/2 + 2
		scaledGap := scaleGap(minGap, n-1, naturalW, 10, 40)
		for i := range gapMins {
			if gapMins[i] < scaledGap {
				gapMins[i] = scaledGap
			}
		}
	}

	// Build center positions cumulatively
	colCenters := make([]int, n)
	colCenters[0] = boxWidths[0]/2 + 1
	for i := 1; i < n; i++ {
		colCenters[i] = colCenters[i-1] + gapMins[i-1]
	}

	// Account for self-messages and right-side notes extending right
	maxRight := colCenters[n-1] + boxWidths[n-1]/2 + 2
	for _, ev := range flatEvents {
		switch e := ev.(type) {
		case *message:
			si := participantIndex(diagram, e.source)
			ti := participantIndex(diagram, e.target)
			if si >= 0 && si == ti {
				loopWidth := maxInt(textwidth.String(e.label)+4, 8)
				needed := colCenters[si] + loopWidth + 1
				maxRight = maxInt(maxRight, needed)
			}
		case *note:
			for _, pid := range e.participants {
				pi := participantIndex(diagram, pid)
				if pi >= 0 && e.position == "rightof" {
					lines := noteLines(e)
					noteWidth := 4
					for _, line := range lines {
						noteWidth = maxInt(noteWidth, textwidth.String(line)+4)
					}
					needed := colCenters[pi] + 2 + noteWidth + 1
					maxRight = maxInt(maxRight, needed)
				}
			}
		}
	}

	// Compute row offsets (cumulative event heights)
	titleRows := 0
	if diagram.title != "" {
		titleRows = 2
	}
	lifelineStart := topMargin + titleRows + headerHeight
	rowOffsets := make([]int, len(flatEvents))
	cumulative := lifelineStart + 1
	for i, h := range eventHeights {
		rowOffsets[i] = cumulative
		cumulative += h
	}

	canvasWidth := maxInt(maxRight, textwidth.String(diagram.title)+1)
	canvasHeight := cumulative + bottomMargin

	return &layoutResult{
		colCenters:   colCenters,
		boxWidths:    boxWidths,
		canvasWidth:  canvasWidth,
		canvasHeight: canvasHeight,
		titleRows:    titleRows,
		headerHeight: headerHeight,
		rowOffsets:   rowOffsets,
	}
}

// ── Participant drawing functions ───────────────────────────────────

func drawActor(canvas *renderer.Canvas, cx, y int, label string, useASCII bool) {
	style := "node"
	canvas.Put(y, cx, 'O', style)
	canvas.Put(y+1, cx-1, '/', style)
	canvas.Put(y+1, cx, '|', style)
	canvas.Put(y+1, cx+1, '\\', style)
	canvas.Put(y+2, cx-1, '/', style)
	canvas.Put(y+2, cx+1, '\\', style)
	labelCol := cx - textwidth.String(label)/2
	canvas.PutText(y+4, labelCol, label, "label")
}

func drawDatabase(canvas *renderer.Canvas, cx, y, width int, label string, cs renderer.CharSet) {
	bx := cx - width/2
	renderer.DrawCylinder(canvas, bx, y, width, 5, label, cs, "node")
}

func drawQueue(canvas *renderer.Canvas, cx, y, width int, label string, cs renderer.CharSet, useASCII bool) {
	bx := cx - width/2
	style := "node"
	h := 5

	// Top border
	canvas.Arm(y, bx, glyph.TopLeft, glyph.Light, false, style)
	for c := bx + 1; c < bx+width-1; c++ {
		canvas.Arm(y, c, glyph.Horizontal, glyph.Light, false, style)
	}
	if !useASCII {
		canvas.Arm(y, bx+width-1, glyph.TopRight, glyph.Light, true, style)
	} else {
		canvas.Arm(y, bx+width-1, glyph.TopRight, glyph.Light, false, style)
	}

	// Bottom border
	canvas.Arm(y+h-1, bx, glyph.BottomLeft, glyph.Light, false, style)
	for c := bx + 1; c < bx+width-1; c++ {
		canvas.Arm(y+h-1, c, glyph.Horizontal, glyph.Light, false, style)
	}
	if !useASCII {
		canvas.Arm(y+h-1, bx+width-1, glyph.BottomRight, glyph.Light, true, style)
	} else {
		canvas.Arm(y+h-1, bx+width-1, glyph.BottomRight, glyph.Light, false, style)
	}

	// Side borders
	for r := y + 1; r < y+h-1; r++ {
		canvas.Arm(r, bx, glyph.Vertical, glyph.Light, false, style)
		if !useASCII {
			canvas.Put(r, bx+width-1, '\u2551', style) // ║
		} else {
			canvas.Arm(r, bx+width-1, glyph.Vertical, glyph.Light, false, style)
		}
	}

	// Label centered
	labelCol := bx + (width-textwidth.String(label))/2
	labelRow := y + h/2
	canvas.PutText(labelRow, labelCol, label, "label")
}

func drawBoundary(canvas *renderer.Canvas, cx, y int, label string, cs renderer.CharSet, useASCII bool) {
	style := "node"
	boxLeft := cx - 1
	boxRight := cx + 1

	// Top of box
	canvas.Arm(y, boxLeft, glyph.TopLeft, glyph.Light, false, style)
	canvas.Arm(y, cx, glyph.Horizontal, glyph.Light, false, style)
	canvas.Arm(y, boxRight, glyph.TopRight, glyph.Light, false, style)

	// Middle row: bar extending left + box sides
	barStart := cx - 3
	canvas.Put(y+1, barStart, cs.Rune(glyph.Horizontal, glyph.Light), style)
	canvas.Put(y+1, barStart+1, cs.Rune(glyph.Horizontal, glyph.Light), style)
	if !useASCII {
		canvas.Arm(y+1, boxLeft, glyph.TeeLeft, glyph.Light, false, style)
	} else {
		canvas.Arm(y+1, boxLeft, glyph.Vertical, glyph.Light, false, style)
	}
	canvas.Arm(y+1, boxRight, glyph.Vertical, glyph.Light, false, style)

	// Bottom of box
	canvas.Arm(y+2, boxLeft, glyph.BottomLeft, glyph.Light, false, style)
	canvas.Arm(y+2, cx, glyph.Horizontal, glyph.Light, false, style)
	canvas.Arm(y+2, boxRight, glyph.BottomRight, glyph.Light, false, style)

	// Label below
	labelCol := cx - textwidth.String(label)/2
	canvas.PutText(y+4, labelCol, label, "label")
}

func drawControl(canvas *renderer.Canvas, cx, y int, label string, cs renderer.CharSet, useASCII bool) {
	style := "node"
	// Arrowhead
	if useASCII {
		canvas.Put(y, cx, '<', style)
	} else {
		canvas.Put(y, cx, '\u25C1', style) // ◁
	}

	// Small rounded box
	if !useASCII {
		canvas.Arm(y+1, cx-1, glyph.TopLeft, glyph.Light, true, style)
		canvas.Arm(y+1, cx, glyph.Horizontal, glyph.Light, false, style)
		canvas.Arm(y+1, cx+1, glyph.TopRight, glyph.Light, true, style)
		canvas.Arm(y+2, cx-1, glyph.BottomLeft, glyph.Light, true, style)
		canvas.Arm(y+2, cx, glyph.Horizontal, glyph.Light, false, style)
		canvas.Arm(y+2, cx+1, glyph.BottomRight, glyph.Light, true, style)
	} else {
		canvas.Arm(y+1, cx-1, glyph.TopLeft, glyph.Light, false, style)
		canvas.Arm(y+1, cx, glyph.Horizontal, glyph.Light, false, style)
		canvas.Arm(y+1, cx+1, glyph.TopRight, glyph.Light, false, style)
		canvas.Arm(y+2, cx-1, glyph.BottomLeft, glyph.Light, false, style)
		canvas.Arm(y+2, cx, glyph.Horizontal, glyph.Light, false, style)
		canvas.Arm(y+2, cx+1, glyph.BottomRight, glyph.Light, false, style)
	}

	// Label below
	labelCol := cx - textwidth.String(label)/2
	canvas.PutText(y+4, labelCol, label, "label")
}

func drawEntity(canvas *renderer.Canvas, cx, y int, label string, cs renderer.CharSet, useASCII bool) {
	style := "node"
	// Small rounded box
	if !useASCII {
		canvas.Arm(y, cx-1, glyph.TopLeft, glyph.Light, true, style)
		canvas.Arm(y, cx, glyph.Horizontal, glyph.Light, false, style)
		canvas.Arm(y, cx+1, glyph.TopRight, glyph.Light, true, style)
		canvas.Arm(y+1, cx-1, glyph.BottomLeft, glyph.Light, true, style)
		canvas.Arm(y+1, cx, glyph.Horizontal, glyph.Light, false, style)
		canvas.Arm(y+1, cx+1, glyph.BottomRight, glyph.Light, true, style)
	} else {
		canvas.Arm(y, cx-1, glyph.TopLeft, glyph.Light, false, style)
		canvas.Arm(y, cx, glyph.Horizontal, glyph.Light, false, style)
		canvas.Arm(y, cx+1, glyph.TopRight, glyph.Light, false, style)
		canvas.Arm(y+1, cx-1, glyph.BottomLeft, glyph.Light, false, style)
		canvas.Arm(y+1, cx, glyph.Horizontal, glyph.Light, false, style)
		canvas.Arm(y+1, cx+1, glyph.BottomRight, glyph.Light, false, style)
	}

	// Underline
	canvas.Put(y+2, cx-1, cs.Rune(glyph.Horizontal, glyph.Light), style)
	canvas.Put(y+2, cx, cs.Rune(glyph.Horizontal, glyph.Light), style)
	canvas.Put(y+2, cx+1, cs.Rune(glyph.Horizontal, glyph.Light), style)

	// Label below
	labelCol := cx - textwidth.String(label)/2
	canvas.PutText(y+4, labelCol, label, "label")
}

func drawCollections(canvas *renderer.Canvas, cx, y, width int, label string, cs renderer.CharSet, useASCII bool) {
	style := "node"
	bx := cx - width/2
	h := 5

	// Back rectangle (offset +1 right) — just top and right edges visible
	for c := bx + 2; c < bx+width+1; c++ {
		canvas.Arm(y, c, glyph.Horizontal, glyph.Light, false, style)
	}
	canvas.Arm(y, bx+1, glyph.TopLeft, glyph.Light, false, style)
	canvas.Arm(y, bx+width, glyph.TopRight, glyph.Light, false, style)
	// Right edge of back rectangle
	canvas.Arm(y+1, bx+width, glyph.Vertical, glyph.Light, false, style)

	// Front rectangle
	canvas.Arm(y+1, bx, glyph.TopLeft, glyph.Light, false, style)
	for c := bx + 1; c < bx+width-1; c++ {
		canvas.Arm(y+1, c, glyph.Horizontal, glyph.Light, false, style)
	}
	canvas.Arm(y+1, bx+width-1, glyph.TopRight, glyph.Light, false, style)

	// Bottom of back rect merges
	canvas.Arm(y+2, bx+width, glyph.BottomRight, glyph.Light, false, style)

	// Side borders of front
	for r := y + 2; r < y+h-1; r++ {
		canvas.Arm(r, bx, glyph.Vertical, glyph.Light, false, style)
		canvas.Arm(r, bx+width-1, glyph.Vertical, glyph.Light, false, style)
	}

	// Bottom border of back rect stub
	if !useASCII {
		canvas.Arm(y+2, bx+width-1, glyph.TeeLeft, glyph.Light, false, style)
	} else {
		canvas.Arm(y+2, bx+width-1, glyph.Vertical, glyph.Light, false, style)
	}
	canvas.Arm(y+2, bx+width, glyph.BottomRight, glyph.Light, false, style)

	// Bottom border of front
	canvas.Arm(y+h-1, bx, glyph.BottomLeft, glyph.Light, false, style)
	for c := bx + 1; c < bx+width-1; c++ {
		canvas.Arm(y+h-1, c, glyph.Horizontal, glyph.Light, false, style)
	}
	canvas.Arm(y+h-1, bx+width-1, glyph.BottomRight, glyph.Light, false, style)

	// Label centered in front rectangle
	labelCol := bx + (width-textwidth.String(label))/2
	labelRow := y + 1 + (h-1)/2
	canvas.PutText(labelRow, labelCol, label, "label")
}

func drawParticipantHeader(
	canvas *renderer.Canvas, cx, bw, top, headerHeight int,
	p *participant, cs renderer.CharSet, useASCII bool,
) {
	kind := p.kind
	label := p.label

	switch kind {
	case "actor":
		actorY := top + (headerHeight - actorHeight)
		drawActor(canvas, cx, actorY, label, useASCII)
	case "database":
		dbY := top + (headerHeight - 5)
		drawDatabase(canvas, cx, dbY, bw, label, cs)
	case "queue":
		qY := top + (headerHeight - 5)
		drawQueue(canvas, cx, qY, bw, label, cs, useASCII)
	case "boundary":
		bY := top + (headerHeight - 5)
		drawBoundary(canvas, cx, bY, label, cs, useASCII)
	case "control":
		cY := top + (headerHeight - 5)
		drawControl(canvas, cx, cY, label, cs, useASCII)
	case "entity":
		eY := top + (headerHeight - 5)
		drawEntity(canvas, cx, eY, label, cs, useASCII)
	case "collections":
		colY := top + (headerHeight - 5)
		drawCollections(canvas, cx, colY, bw, label, cs, useASCII)
	default:
		// Default: participant box
		boxY := top + (headerHeight - boxHeight)
		bx := cx - bw/2
		renderer.DrawRectangle(canvas, bx, boxY, bw, boxHeight, label, cs, "node")
	}
}

// ── Activation ranges ───────────────────────────────────────────────

func computeActivationRanges(flatEvents []interface{}, rowOffsets []int) map[string][][2]int {
	openActivations := map[string][]int{} // pid -> [start_rows...]
	ranges := map[string][][2]int{}

	for idx, ev := range flatEvents {
		ae, ok := ev.(*activateEvent)
		if !ok {
			continue
		}
		row := rowOffsets[idx]
		pid := ae.participant
		if ae.active {
			openActivations[pid] = append(openActivations[pid], row)
		} else {
			// Close the most recent activation
			if starts, ok := openActivations[pid]; ok && len(starts) > 0 {
				start := starts[len(starts)-1]
				openActivations[pid] = starts[:len(starts)-1]
				ranges[pid] = append(ranges[pid], [2]int{start, row})
			}
		}
	}

	// Close any still-open activations at the last row
	maxRow := 0
	if len(rowOffsets) > 0 {
		for _, r := range rowOffsets {
			maxRow = maxInt(maxRow, r)
		}
		maxRow++
	}
	for pid, starts := range openActivations {
		for _, start := range starts {
			ranges[pid] = append(ranges[pid], [2]int{start, maxRow})
		}
	}

	return ranges
}

func isActivated(ranges map[string][][2]int, pid string, row int) bool {
	for _, r := range ranges[pid] {
		if r[0] <= row && row <= r[1] {
			return true
		}
	}
	return false
}

// ── Block frame drawing ─────────────────────────────────────────────

func blockFrameBounds(colCenters []int, depth int) (int, int) {
	indent := depth * 2
	left := indent
	if len(colCenters) > 0 {
		left = maxInt(0, colCenters[0]-6+indent)
	}
	right := 20 - indent
	if len(colCenters) > 0 {
		right = colCenters[len(colCenters)-1] + 6 - indent
	}
	return left, right
}

func drawBlockStart(
	canvas *renderer.Canvas, ev *blockStart, row int,
	colCenters []int, cs renderer.CharSet, useASCII bool,
) {
	left, right := blockFrameBounds(colCenters, ev.depth)
	hChar := cs.Rune(glyph.Horizontal, glyph.Light)
	style := "node"

	// Top border
	canvas.PutBox(row, left, cs.Rune(glyph.TopLeft, glyph.Light), style)
	for c := left + 1; c < minInt(right, canvas.Width); c++ {
		canvas.PutBox(row, c, hChar, style)
	}
	if right < canvas.Width {
		canvas.PutBox(row, right, cs.Rune(glyph.TopRight, glyph.Light), style)
	}

	// Label row: [kind] label. Only the label's own cells are cleared, so
	// every lifeline it does not cover runs on through the row (ADR-402).
	label := fmt.Sprintf("[%s]", ev.blk.kind)
	if ev.blk.label != "" {
		label = fmt.Sprintf("[%s] %s", ev.blk.kind, ev.blk.label)
	}
	labelCol := left + 1
	if row+1 < canvas.Height {
		canvas.PutBox(row+1, left, cs.Rune(glyph.Vertical, glyph.Light), style)
		if right < canvas.Width {
			canvas.PutBox(row+1, right, cs.Rune(glyph.Vertical, glyph.Light), style)
		}
		clearSpan(canvas, row+1, labelCol, runeLen(label))
		canvas.PutText(row+1, labelCol, label, "edge_label")
	}
}

func drawBlockSection(
	canvas *renderer.Canvas, ev *blockSectionBreak, row int,
	colCenters []int, cs renderer.CharSet, useASCII bool,
) {
	left, right := blockFrameBounds(colCenters, ev.depth)
	style := "node"

	// The dashed rule runs from side to side, so each side becomes a tee.
	canvas.PutBox(row, left, cs.Rune(glyph.Vertical, glyph.Light), style)
	canvas.Segment(row, left, row, minInt(right, canvas.Width-1), glyph.Dashed, false, style)
	if right < canvas.Width {
		canvas.PutBox(row, right, cs.Rune(glyph.Vertical, glyph.Light), style)
	}

	// Section label after left border
	if ev.section.label != "" {
		canvas.PutText(row, left+2, fmt.Sprintf("[%s]", ev.section.label), "edge_label")
	}
}

func drawBlockEnd(
	canvas *renderer.Canvas, ev *blockEnd, row int,
	colCenters []int, cs renderer.CharSet, useASCII bool,
) {
	left, right := blockFrameBounds(colCenters, ev.depth)
	hChar := cs.Rune(glyph.Horizontal, glyph.Light)
	style := "node"

	canvas.PutBox(row, left, cs.Rune(glyph.BottomLeft, glyph.Light), style)
	for c := left + 1; c < minInt(right, canvas.Width); c++ {
		canvas.PutBox(row, c, hChar, style)
	}
	if right < canvas.Width {
		canvas.PutBox(row, right, cs.Rune(glyph.BottomRight, glyph.Light), style)
	}
}

// ── Note drawing ────────────────────────────────────────────────────

func drawNote(
	canvas *renderer.Canvas, n *note, row int,
	colCenters []int, diagram *sequenceDiagram,
	cs renderer.CharSet, useASCII bool,
) {
	lines := noteLines(n)
	noteWidth := 4
	for _, line := range lines {
		noteWidth = maxInt(noteWidth, textwidth.String(line)+4)
	}
	noteHeight := len(lines) + 2

	var noteX int
	if n.position == "rightof" {
		pi := participantIndex(diagram, n.participants[0])
		if pi < 0 {
			return
		}
		noteX = colCenters[pi] + 2
	} else if n.position == "leftof" {
		pi := participantIndex(diagram, n.participants[0])
		if pi < 0 {
			return
		}
		noteX = colCenters[pi] - 2 - noteWidth
	} else if n.position == "over" {
		if len(n.participants) == 2 {
			p1i := participantIndex(diagram, n.participants[0])
			p2i := participantIndex(diagram, n.participants[1])
			if p1i < 0 || p2i < 0 {
				return
			}
			center := (colCenters[p1i] + colCenters[p2i]) / 2
			spanWidth := absInt(colCenters[p1i]-colCenters[p2i]) + 4
			noteWidth = maxInt(noteWidth, spanWidth)
			noteX = center - noteWidth/2
		} else {
			pi := participantIndex(diagram, n.participants[0])
			if pi < 0 {
				return
			}
			center := colCenters[pi]
			noteX = center - noteWidth/2
		}
	} else {
		return
	}

	// Clamp noteX to 0
	noteX = maxInt(0, noteX)

	// Clear the interior so lifeline chars don't bleed through
	for r := row; r < row+noteHeight; r++ {
		for c := noteX; c < noteX+noteWidth; c++ {
			canvas.ClearCell(r, c)
		}
	}

	renderer.DrawRectangle(canvas, noteX, row, noteWidth, noteHeight, n.text, cs, "node")

	// A lifeline the note covers tees into its top and bottom borders
	// instead of being cut (ADR-402).
	bottom := row + noteHeight - 1
	for _, cx := range colCenters {
		if cx <= noteX || cx >= noteX+noteWidth-1 {
			continue
		}
		if canvas.Arms(row-1, cx)&glyph.S != 0 {
			canvas.Arm(row, cx, glyph.N, glyph.Dashed, false, "node")
		}
		if canvas.Arms(bottom+1, cx)&glyph.N != 0 {
			canvas.Arm(bottom, cx, glyph.S, glyph.Dashed, false, "node")
		}
	}
}

// ── Message drawing ─────────────────────────────────────────────────

func drawMessage(
	canvas *renderer.Canvas, srcCol, tgtCol, row int,
	msg *message, displayLabel string,
	cs renderer.CharSet,
) {
	w := glyph.Light
	if msg.lineType == "dotted" {
		w = glyph.Dashed
	}

	// The arrowhead sits in the cell before the target lifeline, which runs
	// on unbroken. The source cell keeps the lifeline's arms, so it resolves
	// to a tee, and an intermediate lifeline the line crosses to a cross
	// (ADR-402).
	step := 1
	if tgtCol < srcCol {
		step = -1
	}
	head := tgtCol - step
	tail := srcCol
	if msg.arrowType == "bidirectional" {
		tail = srcCol + step
	}
	canvas.Segment(row, tail, row, head, w, false, "edge")

	ahead, aback := cs.ArrowRight, cs.ArrowLeft
	async := ')'
	if step < 0 {
		ahead, aback = cs.ArrowLeft, cs.ArrowRight
		async = '('
	}

	switch msg.arrowType {
	case "bidirectional":
		canvas.Put(row, head, ahead, "arrow")
		canvas.Put(row, tail, aback, "arrow")
	case "arrow":
		canvas.Put(row, head, ahead, "arrow")
	case "cross":
		canvas.Put(row, head, 'x', "arrow")
	case "async":
		canvas.Put(row, head, async, "arrow")
	default:
		// "open" — no arrowhead, so the line runs into the lifeline.
		canvas.Segment(row, tail, row, tgtCol, w, false, "edge")
	}

	// Label above the line
	if displayLabel != "" {
		canvas.PutText(row-1, minInt(srcCol, tgtCol)+2, displayLabel, "edge_label")
	}
}

func drawSelfMessage(
	canvas *renderer.Canvas, col, row int,
	msg *message, displayLabel string,
	cs renderer.CharSet,
) {
	loopWidth := maxInt(textwidth.String(displayLabel)+4, 8)
	rightCol := col + loopWidth - 1

	w := glyph.Light
	if msg.lineType == "dotted" {
		w = glyph.Dashed
	}

	// Out of the lifeline, down, and back to the cell before it, which holds
	// the arrowhead; the lifeline runs on unbroken (ADR-402).
	canvas.Segment(row, col, row, rightCol, w, false, "edge")
	canvas.Segment(row, rightCol, row+1, rightCol, w, false, "edge")
	canvas.Segment(row+1, col+1, row+1, rightCol, w, false, "edge")

	switch msg.arrowType {
	case "cross":
		canvas.Put(row+1, col+1, 'x', "arrow")
	case "async":
		canvas.Put(row+1, col+1, '(', "arrow")
	case "arrow", "bidirectional":
		canvas.Put(row+1, col+1, cs.ArrowLeft, "arrow")
	default:
		canvas.Segment(row+1, col, row+1, col+1, w, false, "edge")
	}

	// Label above the top line
	if displayLabel != "" {
		canvas.PutText(row-1, col+2, displayLabel, "edge_label")
	}
}

// ── Main render function ────────────────────────────────────────────

// RenderSequence parses a Mermaid sequence diagram source and renders it to a Canvas.
func RenderSequence(source string, cs renderer.CharSet) *renderer.Canvas {
	return renderSequenceModel(parseSequenceDiagram(source), cs)
}

// renderSequenceModel lays out and draws a parsed sequence model. Every
// parser that targets this model — Mermaid's own syntax and ZenUML's — ends
// here.
func renderSequenceModel(diagram *sequenceDiagram, cs renderer.CharSet) *renderer.Canvas {
	useASCII := cs.ASCII

	// Flatten events for linear layout
	flatEvents := flattenEvents(diagram.events, 0)

	layout := computeLayout(diagram, diagram.autonumber, flatEvents)
	if layout.canvasWidth == 0 {
		return renderer.NewCanvas(1, 1)
	}

	canvas := renderer.NewCanvas(layout.canvasWidth, layout.canvasHeight)
	canvas.SetCharSet(cs)

	if diagram.title != "" {
		canvas.PutText(0, 0, diagram.title, "bold_label")
	}

	// Compute activation ranges
	activationRanges := computeActivationRanges(flatEvents, layout.rowOffsets)

	// 1. Draw participant headers at top
	headerTop := topMargin + layout.titleRows
	for i, p := range diagram.participants {
		cx := layout.colCenters[i]
		bw := layout.boxWidths[i]
		drawParticipantHeader(canvas, cx, bw, headerTop, layout.headerHeight, p, cs, useASCII)
	}

	// Compute destroyed participants and their destruction rows
	destroyed := map[string]int{}
	for idx, ev := range flatEvents {
		if de, ok := ev.(*destroyEvent); ok {
			if _, exists := destroyed[de.participant]; !exists {
				destroyed[de.participant] = layout.rowOffsets[idx]
			}
		}
	}

	// 2. Draw lifelines
	lifelineStart := headerTop + layout.headerHeight
	lifelineEnd := layout.canvasHeight - bottomMargin - 1
	var lifelineChar, activeChar rune
	if useASCII {
		lifelineChar = ':'
		activeChar = '['
	} else {
		lifelineChar = '\u2506' // ┆
		activeChar = '\u2551'   // ║
	}
	for i, p := range diagram.participants {
		cx := layout.colCenters[i]
		endRow := lifelineEnd + 1
		if dr, ok := destroyed[p.id]; ok {
			endRow = dr
		}
		// A lifeline tees into the header's bottom border where the header
		// is a box; where it is not, the header's label closes it (ADR-402).
		if canvas.Arms(lifelineStart-1, cx) != 0 {
			canvas.Arm(lifelineStart-1, cx, glyph.S, glyph.Dashed, false, "edge")
		}
		for r := lifelineStart; r < minInt(endRow, lifelineEnd+1); r++ {
			if isActivated(activationRanges, p.id, r) {
				canvas.PutBox(r, cx, activeChar, "edge")
			} else {
				canvas.PutBox(r, cx, lifelineChar, "edge")
			}
		}
		// A lifeline that reaches the bottom ends at a marker; a destroyed
		// one ends at its cross.
		if endRow > lifelineEnd {
			canvas.Put(lifelineEnd+1, cx, cs.Middot, "edge")
		}
	}

	// 2.5 Draw continuous block side borders
	type borderEntry struct {
		left, right, startRow int
	}
	var blockBorderStack []borderEntry
	for idx, ev := range flatEvents {
		row := layout.rowOffsets[idx]
		switch e := ev.(type) {
		case *blockStart:
			left, right := blockFrameBounds(layout.colCenters, e.depth)
			blockBorderStack = append(blockBorderStack, borderEntry{left, right, row})
		case *blockEnd:
			_ = e
			if len(blockBorderStack) > 0 {
				entry := blockBorderStack[len(blockBorderStack)-1]
				blockBorderStack = blockBorderStack[:len(blockBorderStack)-1]
				// Fill block interior: set "node" fill on all cells so
				// background-color themes fill the region. Content styles
				// (edge, arrow, etc.) keep their foreground; the fill
				// provides the background layer underneath.
				for r := entry.startRow; r <= row; r++ {
					endCol := entry.right
					if endCol >= canvas.Width {
						endCol = canvas.Width - 1
					}
					for col := entry.left; col <= endCol; col++ {
						canvas.SetFill(r, col, "node")
					}
				}
				// Draw side borders on top of fill
				for r := entry.startRow + 1; r < row; r++ {
					canvas.PutBox(r, entry.left, cs.Rune(glyph.Vertical, glyph.Light), "node")
					if entry.right < canvas.Width {
						canvas.PutBox(r, entry.right, cs.Rune(glyph.Vertical, glyph.Light), "node")
					}
				}
			}
		}
	}

	// 3. Draw events (messages, notes, blocks)
	msgCounter := 0
	var labelRows []int
	for idx, ev := range flatEvents {
		row := layout.rowOffsets[idx]

		switch e := ev.(type) {
		case *activateEvent:
			_ = e
			continue

		case *destroyEvent:
			pi := participantIndex(diagram, e.participant)
			if pi >= 0 {
				cx := layout.colCenters[pi]
				var xChar rune
				if useASCII {
					xChar = 'X'
				} else {
					xChar = '\u2573' // ╳
				}
				canvas.Put(row, cx, xChar, "arrow")
			}

		case *note:
			drawNote(canvas, e, row, layout.colCenters, diagram, cs, useASCII)

		case *blockStart:
			drawBlockStart(canvas, e, row, layout.colCenters, cs, useASCII)
			labelRows = append(labelRows, row+1)

		case *blockSectionBreak:
			drawBlockSection(canvas, e, row, layout.colCenters, cs, useASCII)

		case *blockEnd:
			drawBlockEnd(canvas, e, row, layout.colCenters, cs, useASCII)

		case *message:
			msgCounter++
			var num *int
			if diagram.autonumber {
				n := msgCounter
				num = &n
			}
			displayLabel := effectiveLabel(e, num)

			si := participantIndex(diagram, e.source)
			ti := participantIndex(diagram, e.target)
			if si < 0 || ti < 0 {
				continue
			}

			if si == ti {
				drawSelfMessage(canvas, layout.colCenters[si], row, e, displayLabel, cs)
			} else {
				drawMessage(canvas, layout.colCenters[si], layout.colCenters[ti], row, e, displayLabel, cs)
			}
		}
	}

	// 4. A fragment's label is written over the lifelines, and its own
	// spaces would leave a hole in one, so each lifeline is drawn again
	// through the cells the label left blank (ADR-402).
	for _, r := range labelRows {
		if r < lifelineStart || r > lifelineEnd {
			continue
		}
		for i, p := range diagram.participants {
			cx := layout.colCenters[i]
			if dr, ok := destroyed[p.id]; ok && r >= dr {
				continue
			}
			if canvas.Get(r, cx) != ' ' || canvas.Arms(r, cx) != 0 {
				continue
			}
			if isActivated(activationRanges, p.id, r) {
				canvas.PutBox(r, cx, activeChar, "edge")
			} else {
				canvas.PutBox(r, cx, lifelineChar, "edge")
			}
		}
	}

	return canvas
}
