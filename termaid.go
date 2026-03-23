// Package termaid renders Mermaid diagram syntax as Unicode (or ASCII) terminal art.
package termaid

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/termaid/termaid-go/internal/graph"
	"github.com/termaid/termaid-go/internal/parser"
	"github.com/termaid/termaid-go/internal/renderer"
)

// config holds rendering options.
type config struct {
	useASCII     bool
	paddingX     int
	paddingY     int
	roundedEdges bool
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
			result = fmt.Sprintf("[termaid] internal error: %v", r)
		}
	}()

	cfg := defaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	source = stripFrontmatter(source)
	dtype := detectDiagramType(source)

	switch dtype {
	case "sequence":
		return "[termaid] Sequence diagrams not yet supported"
	case "class":
		return "[termaid] Class diagrams not yet supported"
	case "er":
		return "[termaid] ER diagrams not yet supported"
	case "block":
		return "[termaid] Block diagrams not yet supported"
	case "gitgraph":
		return "[termaid] Git graphs not yet supported"
	case "pie":
		return "[termaid] Pie charts not yet supported"
	case "treemap":
		return "[termaid] Treemap diagrams not yet supported"
	case "state":
		return "[termaid] State diagrams not yet supported"
	default:
		// flowchart
		g := parser.ParseFlowchart(source)
		return renderer.RenderGraph(g, cfg.useASCII, cfg.paddingX, cfg.paddingY, cfg.roundedEdges)
	}
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
