package diagram

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/termaid/termaid-go/internal/renderer"
)

// pieSlice represents a single slice in a pie chart.
type pieSlice struct {
	label string
	value float64
}

// pieChart represents a parsed pie chart definition.
type pieChart struct {
	title    string
	showData bool
	slices   []pieSlice
}

// unicodeFills are fill characters for unicode mode.
var unicodeFills = []rune{'█', '▓', '░', '▒', '▞', '▚', '▖', '▗'}

// asciiFills are fill characters for ascii mode.
var asciiFills = []rune{'#', '*', '+', '~', ':', '.', 'o', '='}

const (
	pieBarWidth = 40
	pieMargin   = 2
)

// Regex patterns for pie chart parsing.
var (
	rePieHeader = regexp.MustCompile(`(?i)^\s*pie\s*(showData)?\s*$`)
	rePieTitle  = regexp.MustCompile(`(?i)^\s*title\s+(.+)$`)
	rePieSlice  = regexp.MustCompile(`^\s*"([^"]+)"\s*:\s*([0-9]*\.?[0-9]+)\s*$`)
)

// parsePieChart parses pie chart source text into a pieChart model.
func parsePieChart(source string) *pieChart {
	pc := &pieChart{}
	lines := strings.Split(source, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip empty lines and comments
		if trimmed == "" || strings.HasPrefix(trimmed, "%%") {
			continue
		}

		// Header
		if m := rePieHeader.FindStringSubmatch(trimmed); m != nil {
			if strings.EqualFold(m[1], "showData") {
				pc.showData = true
			}
			continue
		}

		// Title
		if m := rePieTitle.FindStringSubmatch(trimmed); m != nil {
			pc.title = strings.TrimSpace(m[1])
			continue
		}

		// Slice
		if m := rePieSlice.FindStringSubmatch(trimmed); m != nil {
			val, err := strconv.ParseFloat(m[2], 64)
			if err != nil {
				continue
			}
			pc.slices = append(pc.slices, pieSlice{
				label: m[1],
				value: val,
			})
			continue
		}
	}

	return pc
}

// RenderPieChart parses and renders a Mermaid pie chart as a horizontal bar chart.
func RenderPieChart(source string, useASCII bool) *renderer.Canvas {
	pc := parsePieChart(source)
	if len(pc.slices) == 0 {
		c := renderer.NewCanvas(30, 1)
		c.PutText(0, 0, "[pie] no data", "default")
		return c
	}

	fills := unicodeFills
	if useASCII {
		fills = asciiFills
	}

	// Compute total
	total := 0.0
	for _, s := range pc.slices {
		total += s.value
	}
	if total == 0 {
		total = 1 // avoid division by zero
	}

	// Compute max label width for right-alignment
	maxLabelWidth := 0
	for _, s := range pc.slices {
		if len(s.label) > maxLabelWidth {
			maxLabelWidth = len(s.label)
		}
	}

	// Compute suffix widths for sizing
	maxSuffixWidth := 0
	for _, s := range pc.slices {
		pct := s.value / total * 100
		suffix := fmt.Sprintf(" %.1f%%", pct)
		if pc.showData {
			suffix = fmt.Sprintf(" %.1f%% (%.0f)", pct, s.value)
		}
		if len(suffix) > maxSuffixWidth {
			maxSuffixWidth = len(suffix)
		}
	}

	// Canvas dimensions
	// Each row: MARGIN + right-aligned label + " ┃ " + bar + suffix
	separatorWidth := 3 // " ┃ " or " | "
	canvasWidth := pieMargin + maxLabelWidth + separatorWidth + pieBarWidth + maxSuffixWidth + pieMargin
	titleRow := 0
	barStartRow := 0
	if pc.title != "" {
		barStartRow = 2
	}
	canvasHeight := barStartRow + len(pc.slices) + 1

	c := renderer.NewCanvas(canvasWidth, canvasHeight)

	// Draw title
	if pc.title != "" {
		titleCol := (canvasWidth - len(pc.title)) / 2
		if titleCol < 0 {
			titleCol = 0
		}
		c.PutText(titleRow, titleCol, pc.title, "default")
	}

	// Separator character
	sepChar := "┃"
	if useASCII {
		sepChar = "|"
	}

	// Draw each bar
	for i, s := range pc.slices {
		row := barStartRow + i
		fillChar := fills[i%len(fills)]

		// Right-aligned label
		padding := maxLabelWidth - len(s.label)
		labelCol := pieMargin + padding
		c.PutText(row, labelCol, s.label, "default")

		// Separator
		sepCol := pieMargin + maxLabelWidth + 1
		c.PutText(row, sepCol, sepChar, "default")

		// Bar
		barCol := pieMargin + maxLabelWidth + separatorWidth
		pct := s.value / total
		barLen := int(pct * float64(pieBarWidth))
		if barLen < 1 && s.value > 0 {
			barLen = 1
		}
		for j := 0; j < barLen; j++ {
			c.Put(row, barCol+j, fillChar, false, "default")
		}

		// Suffix with percentage (and optionally value)
		suffixCol := barCol + pieBarWidth
		pctVal := s.value / total * 100
		suffix := fmt.Sprintf(" %.1f%%", pctVal)
		if pc.showData {
			suffix = fmt.Sprintf(" %.1f%% (%.0f)", pctVal, s.value)
		}
		c.PutText(row, suffixCol, suffix, "default")
	}

	return c
}
