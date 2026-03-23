// Command termaid renders Mermaid diagram syntax as terminal art.
//
// Usage:
//
//	termaid [flags] [file]
//
// If no file is given and stdin is a pipe, input is read from stdin.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	termaid "github.com/termaid/termaid-go"
	"github.com/termaid/termaid-go/internal/renderer"
)

const version = "0.2.0"

// ANSI helpers for CLI output
const (
	ansiReset  = "\033[0m"
	ansiBold   = "\033[1m"
	ansiDim    = "\033[2m"
	ansiCyan   = "\033[36m"
	ansiYellow = "\033[33m"
	ansiGreen  = "\033[32m"
	ansiWhite  = "\033[37m"
)

func main() {
	// GNU-style: both short (-a) and long (--ascii) forms
	var (
		ascii      bool
		paddingX   int
		paddingY   int
		sharpEdges bool
		theme      string
		showVer    bool
		listThemes bool
		demo       string
	)

	flag.BoolVar(&ascii, "ascii", false, "")
	flag.BoolVar(&ascii, "a", false, "")
	flag.IntVar(&paddingX, "padding-x", 4, "")
	flag.IntVar(&paddingY, "padding-y", 2, "")
	flag.BoolVar(&sharpEdges, "sharp-edges", false, "")
	flag.StringVar(&theme, "theme", "", "")
	flag.StringVar(&theme, "t", "", "")
	flag.BoolVar(&showVer, "version", false, "")
	flag.BoolVar(&showVer, "v", false, "")
	flag.BoolVar(&listThemes, "themes", false, "")
	flag.StringVar(&demo, "demo", "", "")

	flag.Usage = func() { printUsage() }
	flag.Parse()

	if showVer {
		fmt.Printf("%stermaid%s %s%s%s\n", ansiBold+ansiCyan, ansiReset, ansiYellow, version, ansiReset)
		os.Exit(0)
	}

	if listThemes {
		printThemes()
		os.Exit(0)
	}

	if demo != "" {
		runDemo(demo)
		os.Exit(0)
	}

	input, err := readInput(flag.Args())
	if err != nil {
		fmt.Fprintf(os.Stderr, "%stermaid:%s %v\n", ansiBold+ansiCyan, ansiReset, err)
		os.Exit(1)
	}

	var opts []termaid.Option
	if ascii {
		opts = append(opts, termaid.WithASCII())
	}
	if paddingX != 4 || paddingY != 2 {
		opts = append(opts, termaid.WithPadding(paddingX, paddingY))
	}
	if sharpEdges {
		opts = append(opts, termaid.WithSharpEdges())
	}
	if theme != "" {
		opts = append(opts, termaid.WithTheme(theme))
	}

	result := termaid.Render(input, opts...)
	fmt.Println(result)
}

func printUsage() {
	w := os.Stderr
	fmt.Fprintf(w, "\n  %stermaid%s — render Mermaid diagrams as terminal art\n\n", ansiBold+ansiCyan, ansiReset)
	fmt.Fprintf(w, "  %sUSAGE%s\n", ansiBold+ansiWhite, ansiReset)
	fmt.Fprintf(w, "    termaid [flags] [file]\n")
	fmt.Fprintf(w, "    cat diagram.mmd | termaid -t blueprint\n\n")
	fmt.Fprintf(w, "  %sFLAGS%s\n", ansiBold+ansiWhite, ansiReset)
	fmt.Fprintf(w, "    %s-a%s, %s--ascii%s          Use ASCII characters instead of Unicode\n", ansiYellow, ansiReset, ansiYellow, ansiReset)
	fmt.Fprintf(w, "    %s-t%s, %s--theme%s %sNAME%s    Color theme (use %s--themes%s to list)\n", ansiYellow, ansiReset, ansiYellow, ansiReset, ansiDim, ansiReset, ansiYellow, ansiReset)
	fmt.Fprintf(w, "    %s-v%s, %s--version%s        Print version and exit\n", ansiYellow, ansiReset, ansiYellow, ansiReset)
	fmt.Fprintf(w, "        %s--themes%s         List available color themes\n", ansiYellow, ansiReset)
	fmt.Fprintf(w, "        %s--demo%s %sTHEME%s    Show sample diagrams with a theme\n", ansiYellow, ansiReset, ansiDim, ansiReset)
	fmt.Fprintf(w, "        %s--padding-x%s %sN%s   Horizontal node padding (default: 4)\n", ansiYellow, ansiReset, ansiDim, ansiReset)
	fmt.Fprintf(w, "        %s--padding-y%s %sN%s   Vertical node padding (default: 2)\n", ansiYellow, ansiReset, ansiDim, ansiReset)
	fmt.Fprintf(w, "        %s--sharp-edges%s    Sharp corners on edge routing\n\n", ansiYellow, ansiReset)
	fmt.Fprintf(w, "  %sDIAGRAM TYPES%s\n", ansiBold+ansiWhite, ansiReset)
	types := []struct{ keyword, desc string }{
		{"flowchart", "Flowcharts and directed graphs"},
		{"sequenceDiagram", "Interaction sequences"},
		{"classDiagram", "UML class relationships"},
		{"erDiagram", "Entity-relationship schemas"},
		{"stateDiagram-v2", "State machines"},
		{"pie", "Pie charts (circular)"},
		{"gitGraph", "Git branch/merge flows"},
		{"block-beta", "Block layouts"},
		{"gantt", "Project schedules"},
		{"timeline", "Chronological events"},
		{"kanban", "Task boards"},
		{"mindmap", "Hierarchical maps"},
		{"quadrantChart", "2×2 matrix plots"},
		{"xychart-beta", "Bar and line charts"},
		{"treemap-beta", "Proportional treemaps"},
	}
	maxKW := 0
	for _, t := range types {
		if len(t.keyword) > maxKW {
			maxKW = len(t.keyword)
		}
	}
	for _, t := range types {
		pad := strings.Repeat(" ", maxKW-len(t.keyword))
		fmt.Fprintf(w, "    %s%s%s%s  %s%s%s\n", ansiGreen, t.keyword, ansiReset, pad, ansiDim, t.desc, ansiReset)
	}
	fmt.Fprintln(w)
}

var demoSamples = map[string]string{
	"flowchart": `graph LR
    A[Request] --> B{Auth?}
    B -->|Yes| C[Process]
    B -->|No| D[Reject]
    C --> E[Response]`,
	"sequence": `sequenceDiagram
    participant Client
    participant API
    participant DB
    Client->>API: GET /users
    API->>DB: SELECT *
    DB-->>API: rows
    API-->>Client: 200 OK`,
	"pie": `pie title Resource Allocation
    "Compute" : 45
    "Storage" : 25
    "Network" : 15
    "Security" : 10
    "Other" : 5`,
	"gantt": `gantt
    title Sprint Plan
    dateFormat YYYY-MM-DD
    section Backend
        API endpoints    :a1, 2026-03-17, 10d
        Database work    :a2, 2026-03-20, 7d
    section Frontend
        UI components    :b1, 2026-03-19, 12d
    section QA
        Testing          :c1, after a2, 8d`,
	"kanban": `kanban
  col1[Backlog]
    t1[Design API]
    t2[Write tests]
  col2[In Progress]
    t3[Build parser]
  col3[Done]
    t4[Setup CI]`,
	"mindmap": `mindmap
  root((System))
    Frontend
      React
      Tailwind
    Backend
      Go
      PostgreSQL`,
	"treemap": `treemap-beta
    "Services"
        "API": 40
        "Web": 30
        "Worker": 20
    "Infra"
        "DB": 25
        "Cache": 15`,
}

func runDemo(themeName string) {
	if _, ok := renderer.Themes[themeName]; !ok {
		fmt.Fprintf(os.Stderr, "%stermaid:%s unknown theme %q (use --themes to list)\n", ansiBold+ansiCyan, ansiReset, themeName)
		os.Exit(1)
	}

	samples := []struct{ name, key string }{
		{"Flowchart", "flowchart"},
		{"Sequence Diagram", "sequence"},
		{"Pie Chart", "pie"},
		{"Gantt Chart", "gantt"},
		{"Kanban Board", "kanban"},
		{"Mindmap", "mindmap"},
		{"Treemap", "treemap"},
	}

	fmt.Printf("\n  %sTheme: %s%s\n", ansiBold+ansiCyan, themeName, ansiReset)

	for _, s := range samples {
		fmt.Printf("\n  %s%s%s\n\n", ansiBold+ansiWhite, s.name, ansiReset)
		result := termaid.Render(demoSamples[s.key], termaid.WithTheme(themeName))
		fmt.Println(result)
	}
}

func printThemes() {
	fmt.Printf("\n  %sAvailable themes:%s\n\n", ansiBold+ansiWhite, ansiReset)
	for name, t := range renderer.Themes {
		marker := "  "
		extra := ""
		if t.HasDepthColors() {
			marker = "● "
			extra = fmt.Sprintf("  %s(solid backgrounds, region colors)%s", ansiDim, ansiReset)
		}
		fmt.Printf("    %s%s%s%s%s%s\n", ansiCyan, marker, ansiBold, name, ansiReset, extra)
	}
	fmt.Printf("\n  %s●%s = supports wallpaper fills and depth-based coloring\n\n", ansiCyan, ansiReset)
}

// readInput returns the mermaid source from a file argument or stdin.
func readInput(args []string) (string, error) {
	if len(args) > 0 {
		data, err := os.ReadFile(args[0])
		if err != nil {
			return "", fmt.Errorf("reading %s: %w", args[0], err)
		}
		return string(data), nil
	}

	// Check if stdin has data (piped input).
	info, err := os.Stdin.Stat()
	if err != nil {
		return "", fmt.Errorf("stat stdin: %w", err)
	}
	if info.Mode()&os.ModeCharDevice != 0 {
		// stdin is a terminal, not a pipe — no input available.
		printUsage()
		os.Exit(1)
	}

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", fmt.Errorf("reading stdin: %w", err)
	}
	return string(data), nil
}
