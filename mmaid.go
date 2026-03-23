// Package mmaid renders Mermaid diagram syntax as Unicode (or ASCII) terminal art.
package mmaid

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/aaronsb/mmaid-go/internal/diagram"
	"github.com/aaronsb/mmaid-go/internal/graph"
	"github.com/aaronsb/mmaid-go/internal/parser"
	"github.com/aaronsb/mmaid-go/internal/renderer"
)

// config holds rendering options.
type config struct {
	useASCII     bool
	paddingX     int
	paddingY     int
	roundedEdges bool
	theme        string // "" = no color, "default", "terra", etc.
}

func defaultConfig() config {
	return config{
		paddingX:     4,
		paddingY:     2,
		roundedEdges: true,
	}
}

// Option configures rendering.
type Option func(*config)

// WithASCII forces ASCII-only output instead of Unicode box-drawing characters.
func WithASCII() Option {
	return func(c *config) { c.useASCII = true }
}

// WithPadding sets horizontal and vertical padding inside node boxes.
func WithPadding(x, y int) Option {
	return func(c *config) {
		c.paddingX = x
		c.paddingY = y
	}
}

// WithSharpEdges disables rounded corners on edge turns.
func WithSharpEdges() Option {
	return func(c *config) { c.roundedEdges = false }
}

// WithTheme enables colored output with the given theme name.
// Available themes: default, terra, neon, mono, amber, phosphor.
func WithTheme(name string) Option {
	return func(c *config) { c.theme = name }
}

// frontmatterRe matches YAML frontmatter at the start of a document.
var frontmatterRe = regexp.MustCompile(`(?s)\A---\s*\n.*?\n---\s*\n`)

// stripFrontmatter removes YAML frontmatter from the beginning of source.
func stripFrontmatter(source string) string {
	return frontmatterRe.ReplaceAllString(source, "")
}

// detectDiagramType returns the diagram type keyword from the first non-empty line.
func detectDiagramType(source string) string {
	for _, line := range strings.Split(source, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "%%") {
			continue
		}
		lower := strings.ToLower(trimmed)
		switch {
		case strings.HasPrefix(lower, "sequencediagram"):
			return "sequence"
		case strings.HasPrefix(lower, "classdiagram"):
			return "class"
		case strings.HasPrefix(lower, "erdiagram"):
			return "er"
		case strings.HasPrefix(lower, "block"):
			return "block"
		case strings.HasPrefix(lower, "gitgraph"):
			return "gitgraph"
		case strings.HasPrefix(lower, "%%{init") && strings.Contains(lower, "gitgraph"):
			return "gitgraph"
		case strings.HasPrefix(lower, "pie"):
			return "pie"
		case strings.HasPrefix(lower, "treemap"):
			return "treemap"
		case strings.HasPrefix(lower, "statediagram"):
			return "state"
		case strings.HasPrefix(lower, "gantt"):
			return "gantt"
		case strings.HasPrefix(lower, "timeline"):
			return "timeline"
		case strings.HasPrefix(lower, "mindmap"):
			return "mindmap"
		case strings.HasPrefix(lower, "quadrantchart"):
			return "quadrant"
		case strings.HasPrefix(lower, "xychart"):
			return "xychart"
		case strings.HasPrefix(lower, "kanban"):
			return "kanban"
		default:
			return "flowchart"
		}
	}
	return "flowchart"
}

// Render renders mermaid syntax as Unicode (or ASCII) art.
//
// It detects the diagram type from the source and dispatches to the appropriate
// parser and renderer. Currently only flowcharts are supported; other diagram
// types return a placeholder message.
func Render(source string, opts ...Option) (result string) {
	// Recover from panics in parser/renderer and return an error message.
	defer func() {
		if r := recover(); r != nil {
			result = fmt.Sprintf("[mmaid] internal error: %v", r)
		}
	}()

	cfg := defaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	source = stripFrontmatter(source)
	dtype := detectDiagramType(source)

	// Get a canvas for any diagram type
	var canvas *renderer.Canvas
	switch dtype {
	case "sequence":
		canvas = diagram.RenderSequence(source, cfg.useASCII)
	case "class":
		canvas = diagram.RenderClassDiagram(source, cfg.useASCII)
	case "er":
		canvas = diagram.RenderERDiagram(source, cfg.useASCII)
	case "pie":
		canvas = diagram.RenderPieChart(source, cfg.useASCII, cfg.theme != "", getThemePtr(cfg.theme))
	case "state":
		g := diagram.ParseStateDiagram(source)
		canvas = renderer.RenderGraphCanvas(g, cfg.useASCII, cfg.paddingX, cfg.paddingY, cfg.roundedEdges)
	case "block":
		canvas = diagram.RenderBlockDiagram(source, cfg.useASCII)
	case "gitgraph":
		canvas = diagram.RenderGitGraph(source, cfg.useASCII)
	case "treemap":
		canvas = diagram.RenderTreemap(source, cfg.useASCII, getThemePtr(cfg.theme))
	case "gantt":
		canvas = diagram.RenderGantt(source, cfg.useASCII, getThemePtr(cfg.theme))
	case "timeline":
		canvas = diagram.RenderTimeline(source, cfg.useASCII, getThemePtr(cfg.theme))
	case "mindmap":
		canvas = diagram.RenderMindmap(source, cfg.useASCII)
	case "quadrant":
		canvas = diagram.RenderQuadrantChart(source, cfg.useASCII, getThemePtr(cfg.theme))
	case "xychart":
		canvas = diagram.RenderXYChart(source, cfg.useASCII, getThemePtr(cfg.theme))
	case "kanban":
		canvas = diagram.RenderKanban(source, cfg.useASCII, getThemePtr(cfg.theme))
	default:
		g := parser.ParseFlowchart(source)
		canvas = renderer.RenderGraphCanvas(g, cfg.useASCII, cfg.paddingX, cfg.paddingY, cfg.roundedEdges)
	}

	if canvas == nil {
		return ""
	}

	// Apply theme if set, otherwise plain text
	if cfg.theme != "" {
		theme := renderer.GetTheme(cfg.theme)
		return canvas.ToColorString(theme)
	}
	return canvas.ToString()
}

func getThemePtr(name string) *renderer.Theme {
	if name == "" {
		return nil
	}
	t := renderer.GetTheme(name)
	return &t
}

// Parse parses mermaid syntax and returns a Graph model.
//
// It detects the diagram type and dispatches to the appropriate parser.
// Currently only flowcharts are supported; other types return an empty graph.
func Parse(source string) *graph.Graph {
	source = stripFrontmatter(source)
	dtype := detectDiagramType(source)

	switch dtype {
	case "flowchart":
		return parser.ParseFlowchart(source)
	default:
		return graph.NewGraph()
	}
}
