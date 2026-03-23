package renderer

import (
	"fmt"
	"strconv"
	"strings"
)

// Theme maps semantic style keys to ANSI color/style sequences.
type Theme struct {
	Name          string
	Node          string
	Edge          string
	Arrow         string
	Subgraph      string
	Label         string
	EdgeLabel     string
	SubgraphLabel string
	Default       string
	BoldLabel     string
	ItalicLabel   string
	Note          string
}

// ansi helpers

func bold(s string) string   { return "\033[1m" + s }
func dim(s string) string    { return "\033[2m" + s }
func italic(s string) string { return "\033[3m" + s }
func reset() string          { return "\033[0m" }

func fg256(r, g, b int) string {
	return fmt.Sprintf("\033[38;2;%d;%d;%dm", r, g, b)
}

func bg256(r, g, b int) string {
	return fmt.Sprintf("\033[48;2;%d;%d;%dm", r, g, b)
}

func hexBgColor(hex string) string {
	r, g, b := parseHex(hex)
	return bg256(r, g, b)
}

func parseHex(hex string) (int, int, int) {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) == 3 {
		hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
	}
	if len(hex) != 6 {
		return 255, 255, 255
	}
	r, _ := strconv.ParseInt(hex[0:2], 16, 32)
	g, _ := strconv.ParseInt(hex[2:4], 16, 32)
	b, _ := strconv.ParseInt(hex[4:6], 16, 32)
	return int(r), int(g), int(b)
}

func hexColor(hex string) string {
	r, g, b := parseHex(hex)
	return fg256(r, g, b)
}

// Named ANSI colors
var namedColors = map[string]string{
	"black":   "\033[30m",
	"red":     "\033[31m",
	"green":   "\033[32m",
	"yellow":  "\033[33m",
	"blue":    "\033[34m",
	"magenta": "\033[35m",
	"cyan":    "\033[36m",
	"white":   "\033[37m",
}

// buildANSI converts a rich-like style string to ANSI escape sequence.
// Supports: "bold", "dim", "italic", named colors, "#hex" colors, and combos.
func buildANSI(style string) string {
	if style == "" {
		return ""
	}
	parts := strings.Fields(style)
	var codes []string
	for i := 0; i < len(parts); i++ {
		p := parts[i]
		switch p {
		case "bold":
			codes = append(codes, "\033[1m")
		case "dim":
			codes = append(codes, "\033[2m")
		case "italic":
			codes = append(codes, "\033[3m")
		case "on":
			// "on #hex" or "on colorname" = background color
			if i+1 < len(parts) {
				i++
				bg := parts[i]
				if strings.HasPrefix(bg, "#") {
					codes = append(codes, hexBgColor(bg))
				} else if ansi, ok := namedColors[bg]; ok {
					// Convert fg to bg: \033[3Xm -> \033[4Xm
					codes = append(codes, strings.Replace(ansi, "\033[3", "\033[4", 1))
				}
			}
		default:
			if strings.HasPrefix(p, "#") {
				codes = append(codes, hexColor(p))
			} else if ansi, ok := namedColors[p]; ok {
				codes = append(codes, ansi)
			}
		}
	}
	return strings.Join(codes, "")
}

func buildTheme(name string, node, edge, arrow, subgraph, label, edgeLabel, sgLabel, def string) Theme {
	return Theme{
		Name:          name,
		Node:          buildANSI(node),
		Edge:          buildANSI(edge),
		Arrow:         buildANSI(arrow),
		Subgraph:      buildANSI(subgraph),
		Label:         buildANSI(label),
		EdgeLabel:     buildANSI(edgeLabel),
		SubgraphLabel: buildANSI(sgLabel),
		Default:       buildANSI(def),
		BoldLabel:     buildANSI("bold " + label),
		ItalicLabel:   buildANSI("italic " + label),
		Note:          buildANSI(node),
	}
}

// Themes is the set of available color themes.
var Themes = map[string]Theme{
	"default": buildTheme("default",
		"cyan",             // node
		"dim white",        // edge
		"bold yellow",      // arrow
		"dim cyan",         // subgraph
		"bold white",       // label
		"italic dim",       // edge_label
		"bold cyan",        // subgraph_label
		"",                 // default
	),
	"terra": buildTheme("terra",
		"bold #D4845A",  // node
		"#8B7E6A",       // edge
		"bold #E8A87C",  // arrow
		"#A07858",       // subgraph
		"#F5E6D3",       // label
		"italic #B89A7A", // edge_label
		"bold #E8A87C",  // subgraph_label
		"",
	),
	"neon": buildTheme("neon",
		"bold magenta",  // node
		"dim cyan",      // edge
		"bold green",    // arrow
		"dim magenta",   // subgraph
		"bold white",    // label
		"italic cyan",   // edge_label
		"bold cyan",     // subgraph_label
		"",
	),
	"mono": buildTheme("mono",
		"bold white",    // node
		"dim",           // edge
		"bold white",    // arrow
		"dim",           // subgraph
		"white",         // label
		"italic dim",    // edge_label
		"bold white",    // subgraph_label
		"",
	),
	"amber": buildTheme("amber",
		"bold #FFB000",  // node
		"#806000",       // edge
		"bold #FFD080",  // arrow
		"#906800",       // subgraph
		"#FFD580",       // label
		"italic #B08030", // edge_label
		"bold #FFC040",  // subgraph_label
		"",
	),
	"blueprint": buildTheme("blueprint",
		"bold #FFFFFF on #1A3A5C",   // node: white on dark blue
		"#6699CC on #1A3A5C",        // edge: light blue on dark blue
		"bold #FFD700 on #1A3A5C",   // arrow: gold on dark blue
		"#4488AA on #1A3A5C",        // subgraph
		"bold #FFFFFF on #1A3A5C",   // label
		"italic #88BBDD on #1A3A5C", // edge_label
		"bold #88CCFF on #1A3A5C",   // subgraph_label
		"on #1A3A5C",               // default: just background
	),
	"slate": buildTheme("slate",
		"bold #E0E0E0 on #2D2D2D",   // node: light gray on dark gray
		"#808080 on #2D2D2D",        // edge
		"bold #FF6B35 on #2D2D2D",   // arrow: orange
		"#666666 on #2D2D2D",        // subgraph
		"bold #FFFFFF on #2D2D2D",   // label
		"italic #999999 on #2D2D2D", // edge_label
		"bold #BBBBBB on #2D2D2D",   // subgraph_label
		"on #2D2D2D",               // default
	),
	"phosphor": buildTheme("phosphor",
		"bold #33FF33",  // node
		"#1A8C1A",       // edge
		"bold #66FF66",  // arrow
		"#228B22",       // subgraph
		"#AAFFAA",       // label
		"italic #339933", // edge_label
		"bold #55DD55",  // subgraph_label
		"",
	),
}

// GetTheme returns a theme by name, falling back to "default".
func GetTheme(name string) Theme {
	if t, ok := Themes[name]; ok {
		return t
	}
	return Themes["default"]
}

// ToColorString renders the canvas using ANSI colors based on the theme.
// Each cell's style key is mapped to the theme's ANSI sequence.
func (c *Canvas) ToColorString(theme Theme) string {
	styleMap := map[string]string{
		"node":            theme.Node,
		"edge":            theme.Edge,
		"arrow":           theme.Arrow,
		"subgraph":        theme.Subgraph,
		"label":           theme.Label,
		"edge_label":      theme.EdgeLabel,
		"subgraph_label":  theme.SubgraphLabel,
		"default":         theme.Default,
		"bold_label":      theme.BoldLabel,
		"italic_label":    theme.ItalicLabel,
		"note":            theme.Note,
	}

	rst := reset()
	var b strings.Builder
	b.Grow(c.Height * (c.Width*4 + 1)) // rough estimate with escape codes

	lastNonEmpty := c.Height - 1
	for lastNonEmpty >= 0 {
		empty := true
		for x := range c.Width {
			if c.grid[lastNonEmpty][x] != ' ' {
				empty = false
				break
			}
		}
		if !empty {
			break
		}
		lastNonEmpty--
	}

	for y := 0; y <= lastNonEmpty; y++ {
		// Find last non-space column for trimming
		lastCol := c.Width - 1
		for lastCol >= 0 && c.grid[y][lastCol] == ' ' {
			lastCol--
		}

		prevStyle := ""
		for x := 0; x <= lastCol; x++ {
			ch := c.grid[y][x]
			styleKey := c.styleGrid[y][x]
			ansi := styleMap[styleKey]

			if ansi == "" {
				if prevStyle != "" {
					b.WriteString(rst)
					prevStyle = ""
				}
				b.WriteRune(ch)
			} else {
				if ansi != prevStyle {
					if prevStyle != "" {
						b.WriteString(rst)
					}
					b.WriteString(ansi)
					prevStyle = ansi
				}
				b.WriteRune(ch)
			}
		}
		if prevStyle != "" {
			b.WriteString(rst)
		}
		if y < lastNonEmpty {
			b.WriteByte('\n')
		}
	}

	return b.String()
}
