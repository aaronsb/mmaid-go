// Package mmaid renders Mermaid diagram syntax as Unicode (or ASCII) terminal art.
package mmaid

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/aaronsb/mmaid-go/internal/diagram"
	"github.com/aaronsb/mmaid-go/internal/glyph"
	"github.com/aaronsb/mmaid-go/internal/graph"
	"github.com/aaronsb/mmaid-go/internal/parser"
	"github.com/aaronsb/mmaid-go/internal/renderer"
)

// config holds rendering options.
type config struct {
	glyphs       string   // a glyph set name; "" is unicode
	failed       []string // families the profile marked failed
	paddingX     int
	paddingY     int
	roundedEdges bool
	theme        string // "" = no color, "default", "terra", etc.
	hyperlinks   bool
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
// It is WithGlyphs("ascii", nil).
func WithASCII() Option {
	return WithGlyphs("ascii", nil)
}

// WithGlyphs selects a built-in glyph set by name (unicode, rounded, heavy,
// double, legacy, ascii) and marks families failed, each of which is drawn
// from its fallback (ADR-500). An unknown name selects unicode and an unknown
// family is ignored; the command line warns about both before calling.
func WithGlyphs(name string, failed []string) Option {
	return func(c *config) {
		c.glyphs = name
		c.failed = failed
	}
}

// charset resolves the configured set into the tables the canvas draws from.
func (c config) charset() renderer.CharSet {
	set, ok := glyph.LookupSet(c.glyphs)
	if !ok {
		set = glyph.DefaultSet()
	}
	families, _ := glyph.ParseFamilies(c.failed)
	return renderer.CharSetFor(glyph.Resolve(set, families))
}

// GlyphSheet returns the sample sheet `mmaid --glyphs-sample` prints for a
// set: one numbered line per family with its sample and reference figure.
func GlyphSheet(name string, failed []string) string {
	cfg := config{glyphs: name, failed: failed}
	set, ok := glyph.LookupSet(cfg.glyphs)
	if !ok {
		set = glyph.DefaultSet()
	}
	families, _ := glyph.ParseFamilies(cfg.failed)
	return renderer.FormatSamples(renderer.GlyphSamples(glyph.Resolve(set, families)))
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

// WithHyperlinks wraps the label of every node carrying a `click ID "url"` line
// in an OSC 8 hyperlink.
func WithHyperlinks() Option {
	return func(c *config) { c.hyperlinks = true }
}

// frontmatterRe matches YAML frontmatter at the start of a document.
var frontmatterRe = regexp.MustCompile(`(?s)\A---\s*\n.*?\n---\s*\n`)

// stripFrontmatter removes YAML frontmatter from the beginning of source.
// stripFrontmatter removes a YAML front-matter block and a leading UTF-8
// byte order mark, so the first non-empty line is the diagram's header.
func stripFrontmatter(source string) string {
	source = strings.TrimPrefix(source, "\uFEFF")
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
		case strings.HasPrefix(lower, "treeview"):
			return "treeview"
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
		case strings.HasPrefix(lower, "journey"):
			return "journey"
		case strings.HasPrefix(lower, "packet"):
			return "packet"
		case strings.HasPrefix(lower, "sankey"):
			return "sankey"
		case strings.HasPrefix(lower, "zenuml"):
			return "zenuml"
		case strings.HasPrefix(lower, "eventmodeling"):
			return "eventmodeling"
		case strings.HasPrefix(lower, "ishikawa"):
			return "ishikawa"
		case strings.HasPrefix(lower, "requirementdiagram"):
			return "requirement"
		case strings.HasPrefix(lower, "c4context"), strings.HasPrefix(lower, "c4container"),
			strings.HasPrefix(lower, "c4component"), strings.HasPrefix(lower, "c4dynamic"),
			strings.HasPrefix(lower, "c4deployment"):
			return "c4"
		case strings.HasPrefix(lower, "usecase"):
			return "usecase"
		case strings.HasPrefix(lower, "radar"):
			return "radar"
		case strings.HasPrefix(lower, "venn"):
			return "venn"
		case strings.HasPrefix(lower, "wardley"):
			return "wardley"
		case strings.HasPrefix(lower, "cynefin"):
			return "cynefin"
		case strings.HasPrefix(lower, "swimlane"):
			return "swimlane"
		case strings.HasPrefix(lower, "architecture"):
			return "architecture"
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
	cs := cfg.charset()

	// Get a canvas for any diagram type
	var canvas *renderer.Canvas
	switch dtype {
	case "sequence":
		canvas = diagram.RenderSequence(source, cs)
	case "class":
		canvas = diagram.RenderClassDiagram(source, cs)
	case "er":
		canvas = diagram.RenderERDiagram(source, cs)
	case "pie":
		canvas = diagram.RenderPieChart(source, cs, cfg.theme != "", getThemePtr(cfg.theme))
	case "state":
		g := diagram.ParseStateDiagram(source)
		canvas = renderer.RenderGraphCanvas(g, cs, cfg.paddingX, cfg.paddingY, cfg.roundedEdges, diagram.UsableWidth())
	case "block":
		canvas = diagram.RenderBlockDiagram(source, cs)
	case "gitgraph":
		canvas = diagram.RenderGitGraph(source, cs)
	case "treemap":
		canvas = diagram.RenderTreemap(source, cs, getThemePtr(cfg.theme))
	case "treeview":
		canvas = diagram.RenderTreeView(source, cs)
	case "gantt":
		canvas = diagram.RenderGantt(source, cs, getThemePtr(cfg.theme))
	case "timeline":
		canvas = diagram.RenderTimeline(source, cs, getThemePtr(cfg.theme))
	case "mindmap":
		canvas = diagram.RenderMindmap(source, cs)
	case "quadrant":
		canvas = diagram.RenderQuadrantChart(source, cs, getThemePtr(cfg.theme))
	case "xychart":
		canvas = diagram.RenderXYChart(source, cs, getThemePtr(cfg.theme))
	case "kanban":
		canvas = diagram.RenderKanban(source, cs, getThemePtr(cfg.theme))
	case "journey":
		canvas = diagram.RenderJourney(source, cs, getThemePtr(cfg.theme))
	case "packet":
		canvas = diagram.RenderPacket(source, cs, getThemePtr(cfg.theme))
	case "sankey":
		canvas = diagram.RenderSankey(source, cs, getThemePtr(cfg.theme))
	case "zenuml":
		canvas = diagram.RenderZenUML(source, cs)
	case "eventmodeling":
		canvas = diagram.RenderEventModeling(source, cs, getThemePtr(cfg.theme))
	case "ishikawa":
		canvas = diagram.RenderIshikawa(source, cs, getThemePtr(cfg.theme))
	case "requirement":
		g := diagram.ParseRequirementDiagram(source)
		canvas = renderer.RenderGraphCanvas(g, cs, cfg.paddingX, cfg.paddingY, cfg.roundedEdges, diagram.UsableWidth())
	case "c4":
		g := diagram.ParseC4Diagram(source)
		canvas = renderer.RenderGraphCanvas(g, cs, cfg.paddingX, cfg.paddingY, cfg.roundedEdges, diagram.UsableWidth())
	case "usecase":
		g := diagram.ParseUseCaseDiagram(source)
		canvas = renderer.RenderGraphCanvas(g, cs, cfg.paddingX, cfg.paddingY, cfg.roundedEdges, diagram.UsableWidth())
	case "radar":
		canvas = diagram.RenderRadar(source, cs, cfg.theme != "", getThemePtr(cfg.theme))
	case "venn":
		canvas = diagram.RenderVenn(source, cs, cfg.theme != "", getThemePtr(cfg.theme))
	case "wardley":
		canvas = diagram.RenderWardley(source, cs, getThemePtr(cfg.theme))
	case "cynefin":
		canvas = diagram.RenderCynefin(source, cs, getThemePtr(cfg.theme))
	case "swimlane":
		g := parser.ParseSwimlane(source)
		canvas = renderer.RenderGraphCanvas(g, cs, cfg.paddingX, cfg.paddingY, cfg.roundedEdges, diagram.UsableWidth())
	case "architecture":
		g := parser.ParseArchitecture(source)
		canvas = renderer.RenderGraphCanvas(g, cs, cfg.paddingX, cfg.paddingY, cfg.roundedEdges, diagram.UsableWidth())
	default:
		g := parser.ParseFlowchart(source)
		canvas = renderer.RenderGraphCanvas(g, cs, cfg.paddingX, cfg.paddingY, cfg.roundedEdges, diagram.UsableWidth())
	}

	if canvas == nil {
		return ""
	}

	// The link layer is recorded whatever the setting says; this decides
	// whether it is emitted.
	canvas.SetHyperlinks(cfg.hyperlinks)

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
	case "swimlane":
		return parser.ParseSwimlane(source)
	default:
		return graph.NewGraph()
	}
}
