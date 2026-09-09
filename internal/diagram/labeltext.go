package diagram

import (
	"regexp"
	"strings"

	"github.com/aaronsb/mmaid-go/internal/graph"
)

// Helpers shared by the parsers that target the flowchart engine
// (requirement, C4, use case).

// labelSep is the line break the layout and the renderer both split node
// labels on: the literal two-character sequence, not a newline.
const labelSep = `\n`

var (
	reLineBreakTag = regexp.MustCompile(`(?i)<br\s*/?>`)
	reClassSuffix  = regexp.MustCompile(`:::([A-Za-z0-9_-]+)\s*$`)
)

// joinLabel builds a multi-line node label from its lines, dropping empties.
func joinLabel(lines []string) string {
	kept := make([]string, 0, len(lines))
	for _, l := range lines {
		if l = strings.TrimSpace(l); l != "" {
			kept = append(kept, l)
		}
	}
	return strings.Join(kept, labelSep)
}

// unquote strips one layer of matching quotes and converts `<br/>` to a label
// line break.
func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			s = s[1 : len(s)-1]
		}
	}
	return reLineBreakTag.ReplaceAllString(s, labelSep)
}

// stripLineComment removes a `%%` comment, leaving text inside quotes alone.
func stripLineComment(line string) string {
	inQuote := false
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case '"':
			inQuote = !inQuote
		case '%':
			if !inQuote && i+1 < len(line) && line[i+1] == '%' {
				return line[:i]
			}
		}
	}
	return line
}

// splitClassSuffix separates a trailing `:::className` from a name.
func splitClassSuffix(s string) (name, class string) {
	if m := reClassSuffix.FindStringSubmatch(s); m != nil {
		return strings.TrimSpace(s[:len(s)-len(m[0])]), m[1]
	}
	return strings.TrimSpace(s), ""
}

// normalizeDirection maps a Mermaid direction token to a graph direction,
// folding TD into TB.
func normalizeDirection(token string) graph.Direction {
	dir := graph.Direction(strings.ToUpper(strings.TrimSpace(token)))
	if dir == graph.DirTD {
		return graph.DirTB
	}
	return dir
}

// splitArgs splits a macro argument list on commas that are outside quotes.
func splitArgs(s string) []string {
	var args []string
	var cur strings.Builder
	inQuote := false
	for _, r := range s {
		switch {
		case r == '"':
			inQuote = !inQuote
			cur.WriteRune(r)
		case r == ',' && !inQuote:
			args = append(args, strings.TrimSpace(cur.String()))
			cur.Reset()
		default:
			cur.WriteRune(r)
		}
	}
	if last := strings.TrimSpace(cur.String()); last != "" || len(args) > 0 {
		args = append(args, last)
	}
	return args
}
