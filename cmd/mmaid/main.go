// Command mmaid renders Mermaid diagram syntax as terminal art.
//
// Usage:
//
//	mmaid [flags] [file]
//
// If no file is given and stdin is a pipe, input is read from stdin.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	mmaid "github.com/aaronsb/mmaid-go"
	"github.com/aaronsb/mmaid-go/internal/cells"
	"github.com/aaronsb/mmaid-go/internal/diagram"
	"github.com/aaronsb/mmaid-go/internal/glyph"
	"github.com/aaronsb/mmaid-go/internal/ingest"
	"github.com/aaronsb/mmaid-go/internal/renderer"
	"github.com/aaronsb/mmaid-go/internal/textwidth"
)

const version = "0.5.0"

// cellsWidth is the width --cells renders at when -w is absent. It matches the
// golden harness so a snapshot and its reference are the same frame.
const cellsWidth = 120

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
		ascii       bool
		paddingX    int
		paddingY    int
		sharpEdges  bool
		theme       string
		showVer     bool
		listThemes  bool
		demo        string
		markdown    bool
		insert      string
		width       int
		jsonMode    string
		showTmpl    bool
		nameKey     string
		valueKey    string
		childrenKey string
		orientation string
		cellsPath   string
		cellsLint   bool
		output      string
		watch       bool
		glyphs      string
		glyphSample bool
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
	flag.BoolVar(&markdown, "markdown", false, "")
	flag.BoolVar(&markdown, "m", false, "")
	flag.StringVar(&insert, "insert", "", "")
	flag.IntVar(&width, "width", 0, "")
	flag.IntVar(&width, "w", 0, "")
	flag.StringVar(&jsonMode, "json", "", "")
	flag.BoolVar(&showTmpl, "template", false, "")
	flag.StringVar(&nameKey, "name-key", "", "")
	flag.StringVar(&valueKey, "value-key", "", "")
	flag.StringVar(&childrenKey, "children-key", "", "")
	flag.StringVar(&orientation, "orientation", "", "")
	flag.StringVar(&cellsPath, "cells", "", "")
	flag.BoolVar(&cellsLint, "cells-lint", false, "")
	flag.StringVar(&output, "output", "", "")
	flag.BoolVar(&watch, "watch", false, "")
	flag.StringVar(&glyphs, "glyphs", "", "")
	flag.BoolVar(&glyphSample, "glyphs-sample", false, "")

	flag.Usage = func() { printUsage() }
	flag.Parse()

	// The config subcommand is dispatched once the settings are applied, so
	// it measures text the way a render would; flag.Parse stops at its name
	// and leaves it and its own flags in flag.Args.
	isConfig := flag.Arg(0) == "config"

	if cellsPath != "" && (markdown || insert != "") {
		fmt.Fprintf(os.Stderr, "%smmaid:%s --cells cannot be combined with --markdown or --insert\n", ansiBold+ansiCyan, ansiReset)
		os.Exit(1)
	}
	if cellsLint && cellsPath == "" {
		fmt.Fprintf(os.Stderr, "%smmaid:%s --cells-lint has no effect without --cells\n", ansiBold+ansiCyan, ansiReset)
	}
	if output != "" && insert != "" {
		fmt.Fprintf(os.Stderr, "%smmaid:%s --output cannot be combined with --insert\n", ansiBold+ansiCyan, ansiReset)
		os.Exit(1)
	}
	if watch && (insert != "" || cellsPath != "" || output != "" || jsonMode != "" || showTmpl) {
		fmt.Fprintf(os.Stderr, "%smmaid:%s --watch cannot be combined with --insert, --cells, --output or --json\n", ansiBold+ansiCyan, ansiReset)
		os.Exit(1)
	}

	// Neither of these renders anything, so neither reads the configuration.
	if showVer {
		fmt.Printf("%smmaid%s %s%s%s\n", ansiBold+ansiCyan, ansiReset, ansiYellow, version, ansiReset)
		os.Exit(0)
	}
	if listThemes {
		printThemes()
		os.Exit(0)
	}

	// Resolution order: flag, MMAID_*, the terminal's profile, the file's
	// default section, the built-in default (ADR-500). A file that cannot be
	// read is a warning: it must not stand between the user and a render.
	// The config subcommand reports the file's problems itself.
	res, _, err := resolveSettings()
	if err != nil && !isConfig {
		fmt.Fprintf(os.Stderr, "%smmaid:%s config: %v\n", ansiBold+ansiCyan, ansiReset, err)
	}
	if !isConfig {
		for _, w := range res.Warnings {
			fmt.Fprintf(os.Stderr, "%smmaid:%s config: %s\n", ansiBold+ansiCyan, ansiReset, w)
		}
	}
	theme = res.Theme
	glyphs = glyphSet(res.Glyphs).Name
	if !isConfig {
		warnUnknownFamilies(res.Failed)
	}
	paddingX, paddingY = res.PaddingX, res.PaddingY
	sharpEdges = res.SharpEdges

	if res.Width > 0 {
		diagram.SetWidthOverride(res.Width)
	} else if cellsPath != "" {
		// A frame is a comparable artifact, so it never tracks the window the
		// command happens to run in.
		diagram.SetWidthOverride(cellsWidth)
	}
	if res.Orientation != "" && !diagram.SetOrientationOverride(res.Orientation) {
		fmt.Fprintf(os.Stderr, "%smmaid:%s unknown orientation %q (use TB or LR)\n", ansiBold+ansiCyan, ansiReset, res.Orientation)
	}
	renderer.SetTruecolor(res.Truecolor)
	textwidth.SetAmbiguousWide(res.AmbiguousWide)

	if isConfig {
		runConfig(flag.Args()[1:])
		return
	}

	if glyphSample {
		w, closeOut := openOutput(output)
		fmt.Fprintln(w, mmaid.GlyphSheet(glyphs, res.Failed))
		closeOut()
		os.Exit(0)
	}

	if demo != "" {
		if theme == "" {
			theme = "default"
		}
		w, closeOut := openOutput(output)
		runDemo(w, theme, demo, mmaid.WithGlyphs(glyphs, res.Failed))
		closeOut()
		os.Exit(0)
	}

	out := outputSpec{
		glyphs:     glyphs,
		failed:     res.Failed,
		paddingX:   paddingX,
		paddingY:   paddingY,
		sharpEdges: sharpEdges,
		theme:      theme,
		hyperlinks: res.Hyperlinks,
		markdown:   markdown,
		insert:     insert,
		cellsPath:  cellsPath,
		cellsLint:  cellsLint,
		output:     output,
	}

	// JSON ingest mode
	if jsonMode != "" || showTmpl {
		cfg := ingest.DefaultConfig()
		if nameKey != "" {
			cfg.NameKey = nameKey
		}
		if valueKey != "" {
			cfg.ValueKey = valueKey
		}
		if childrenKey != "" {
			cfg.ChildrenKey = childrenKey
		}

		if showTmpl {
			if jsonMode == "" {
				fmt.Fprintf(os.Stderr, "%smmaid:%s --template requires --json MODE\n", ansiBold+ansiCyan, ansiReset)
				os.Exit(1)
			}
			tmpl, err := ingest.Template(jsonMode, cfg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%smmaid:%s %v\n", ansiBold+ansiCyan, ansiReset, err)
				os.Exit(1)
			}
			fmt.Print(tmpl)
			os.Exit(0)
		}

		// Read JSON from stdin.
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%smmaid:%s reading stdin: %v\n", ansiBold+ansiCyan, ansiReset, err)
			os.Exit(1)
		}
		mermaidSrc, err := ingest.Convert(jsonMode, data, cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%smmaid:%s %v\n", ansiBold+ansiCyan, ansiReset, err)
			os.Exit(1)
		}

		// Render the generated Mermaid syntax.
		renderAndOutput(mermaidSrc, out)
		os.Exit(0)
	}

	if watch {
		runWatch(flag.Args(), out)
		return
	}

	input, err := readInput(flag.Args())
	if err != nil {
		fmt.Fprintf(os.Stderr, "%smmaid:%s %v\n", ansiBold+ansiCyan, ansiReset, err)
		os.Exit(1)
	}

	renderAndOutput(input, out)
}

// outputSpec is what the render pass needs after the settings are resolved.
type outputSpec struct {
	glyphs     string
	failed     []string
	paddingX   int
	paddingY   int
	sharpEdges bool
	theme      string
	hyperlinks bool
	markdown   bool
	insert     string
	cellsPath  string
	cellsLint  bool
	output     string
}

func render(source string, o outputSpec) string {
	var opts []mmaid.Option
	if o.glyphs != "" || len(o.failed) > 0 {
		opts = append(opts, mmaid.WithGlyphs(o.glyphs, o.failed))
	}
	if o.paddingX != 4 || o.paddingY != 2 {
		opts = append(opts, mmaid.WithPadding(o.paddingX, o.paddingY))
	}
	if o.sharpEdges {
		opts = append(opts, mmaid.WithSharpEdges())
	}
	if o.theme != "" {
		opts = append(opts, mmaid.WithTheme(o.theme))
	}
	if o.hyperlinks {
		opts = append(opts, mmaid.WithHyperlinks())
	}
	return mmaid.Render(source, opts...)
}

// decorate wraps the render in a fenced code block when --markdown asks for it.
func decorate(result string, o outputSpec) string {
	if o.markdown {
		return "```\n" + result + "\n```"
	}
	return result
}

func renderAndOutput(source string, o outputSpec) {
	result := render(source, o)

	if o.cellsPath != "" {
		writeCells(result, o.cellsPath, o.cellsLint)
		return
	}

	result = decorate(result, o)

	if o.insert != "" {
		if err := insertIntoFile(o.insert, result); err != nil {
			fmt.Fprintf(os.Stderr, "%smmaid:%s %v\n", ansiBold+ansiCyan, ansiReset, err)
			os.Exit(1)
		}
		return
	}

	w, closeOut := openOutput(o.output)
	fmt.Fprintln(w, result)
	closeOut()
}

// openOutput returns where stdout should go: the file --output names, created
// or truncated, or os.Stdout when it names none.
func openOutput(path string) (io.Writer, func()) {
	if path == "" {
		return os.Stdout, func() {}
	}
	f, err := os.Create(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%smmaid:%s %v\n", ansiBold+ansiCyan, ansiReset, err)
		os.Exit(1)
	}
	return f, func() {
		if err := f.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "%smmaid:%s writing %s: %v\n", ansiBold+ansiCyan, ansiReset, path, err)
			os.Exit(1)
		}
	}
}

// writeCells interprets the rendered stream as a frame and writes it to path
// ("-" is stdout), optionally printing the structural lint's findings.
func writeCells(result, path string, lint bool) {
	fail := func(err error) {
		fmt.Fprintf(os.Stderr, "%smmaid:%s cells: %v\n", ansiBold+ansiCyan, ansiReset, err)
		os.Exit(1)
	}

	frame, err := cells.Interpret(result)
	if err != nil {
		fail(err)
	}

	out, opened := os.Stdout, false
	if path != "-" {
		f, err := os.Create(path)
		if err != nil {
			fail(err)
		}
		out, opened = f, true
	}
	if err := cells.Write(out, frame); err != nil {
		fail(err)
	}
	if opened {
		if err := out.Close(); err != nil {
			fail(err)
		}
	}

	if lint {
		for _, finding := range cells.Lint(frame) {
			fmt.Fprintln(os.Stderr, finding)
		}
	}
}

func printUsage() {
	w := os.Stderr
	fmt.Fprintf(w, "\n  %smmaid%s — render Mermaid diagrams as terminal art\n\n", ansiBold+ansiCyan, ansiReset)
	fmt.Fprintf(w, "  %sUSAGE%s\n", ansiBold+ansiWhite, ansiReset)
	fmt.Fprintf(w, "    mmaid [flags] [file]\n")
	fmt.Fprintf(w, "    cat diagram.mmd | mmaid -t blueprint\n")
	fmt.Fprintf(w, "    lsblk -Jb | mmaid --json treemap -t blueprint\n\n")
	fmt.Fprintf(w, "  %sFLAGS%s\n", ansiBold+ansiWhite, ansiReset)
	fmt.Fprintf(w, "    %s-a%s, %s--ascii%s          Use ASCII characters instead of Unicode (%s--glyphs ascii%s)\n", ansiYellow, ansiReset, ansiYellow, ansiReset, ansiYellow, ansiReset)
	fmt.Fprintf(w, "        %s--glyphs%s %sNAME%s    Glyph set: %s\n", ansiYellow, ansiReset, ansiDim, ansiReset, strings.Join(glyph.SetNames, ", "))
	fmt.Fprintf(w, "        %s--glyphs-sample%s  Print one sample line per glyph family and exit\n", ansiYellow, ansiReset)
	fmt.Fprintf(w, "    %s-t%s, %s--theme%s %sNAME%s    Color theme (use %s--themes%s to list)\n", ansiYellow, ansiReset, ansiYellow, ansiReset, ansiDim, ansiReset, ansiYellow, ansiReset)
	fmt.Fprintf(w, "    %s-v%s, %s--version%s        Print version and exit\n", ansiYellow, ansiReset, ansiYellow, ansiReset)
	fmt.Fprintf(w, "        %s--themes%s         List available color themes\n", ansiYellow, ansiReset)
	fmt.Fprintf(w, "    %s-m%s, %s--markdown%s       Wrap output in a fenced code block\n", ansiYellow, ansiReset, ansiYellow, ansiReset)
	fmt.Fprintf(w, "        %s--insert%s %sFILE:LINE%s  Insert output into file after line N\n", ansiYellow, ansiReset, ansiDim, ansiReset)
	fmt.Fprintf(w, "        %s--demo%s %sTYPE%s     Show sample diagram (use with %s-t%s for theme)\n", ansiYellow, ansiReset, ansiDim, ansiReset, ansiYellow, ansiReset)
	fmt.Fprintf(w, "        %s--padding-x%s %sN%s   Horizontal node padding (default: 4)\n", ansiYellow, ansiReset, ansiDim, ansiReset)
	fmt.Fprintf(w, "        %s--padding-y%s %sN%s   Vertical node padding (default: 2)\n", ansiYellow, ansiReset, ansiDim, ansiReset)
	fmt.Fprintf(w, "    %s-w%s, %s--width%s %sN%s      Override diagram width (columns)\n", ansiYellow, ansiReset, ansiYellow, ansiReset, ansiDim, ansiReset)
	fmt.Fprintf(w, "        %s--orientation%s %sTB|LR%s  Force layout orientation (overrides 'direction')\n", ansiYellow, ansiReset, ansiDim, ansiReset)
	fmt.Fprintf(w, "        %s--sharp-edges%s    Sharp corners on edge routing\n\n", ansiYellow, ansiReset)
	fmt.Fprintf(w, "  %sOUTPUT%s\n", ansiBold+ansiWhite, ansiReset)
	fmt.Fprintf(w, "        %s--output%s %sFILE%s    Write what would go to stdout to FILE\n", ansiYellow, ansiReset, ansiDim, ansiReset)
	fmt.Fprintf(w, "        %s--watch%s          Re-render the file whenever it changes (Ctrl-C to stop)\n", ansiYellow, ansiReset)
	fmt.Fprintf(w, "        %s--cells%s %sFILE%s     Write the rendered frame as a .cells dump (- is stdout)\n", ansiYellow, ansiReset, ansiDim, ansiReset)
	fmt.Fprintf(w, "        %s--cells-lint%s     With %s--cells%s, print structural lint findings to stderr\n\n", ansiYellow, ansiReset, ansiYellow, ansiReset)
	fmt.Fprintf(w, "  %sCONFIG%s\n", ansiBold+ansiWhite, ansiReset)
	fmt.Fprintf(w, "    %smmaid config show%s  Every setting with its resolved value and source\n", ansiYellow, ansiReset)
	fmt.Fprintf(w, "    %smmaid config init%s  Probe the terminal, ask which glyph families look wrong, write its profile\n", ansiYellow, ansiReset)
	fmt.Fprintf(w, "    %sFile%s      %s$XDG_CONFIG_HOME/mmaid/config.json%s, else ~/.config/mmaid/config.json\n", ansiDim, ansiReset, ansiDim, ansiReset)
	fmt.Fprintf(w, "    %sOrder%s     flag, %sMMAID_*%s, the terminal's profile, the file's default, built in\n", ansiDim, ansiReset, ansiDim, ansiReset)
	fmt.Fprintf(w, "    %sColour%s    %sNO_COLOR%s disables a theme that came from the file or the environment\n\n", ansiDim, ansiReset, ansiDim, ansiReset)
	fmt.Fprintf(w, "  %sJSON INGEST%s\n", ansiBold+ansiWhite, ansiReset)
	fmt.Fprintf(w, "        %s--json%s %sMODE%s     Read JSON from stdin, render as MODE (treemap, pie)\n", ansiYellow, ansiReset, ansiDim, ansiReset)
	fmt.Fprintf(w, "        %s--template%s       Print minimum valid JSON for the given --json mode\n", ansiYellow, ansiReset)
	fmt.Fprintf(w, "        %s--name-key%s %sKEY%s  JSON field for node labels (default: name)\n", ansiYellow, ansiReset, ansiDim, ansiReset)
	fmt.Fprintf(w, "        %s--value-key%s %sKEY%s JSON field for leaf weights (default: size)\n", ansiYellow, ansiReset, ansiDim, ansiReset)
	fmt.Fprintf(w, "        %s--children-key%s %sKEY%s JSON field for child arrays (default: children)\n\n", ansiYellow, ansiReset, ansiDim, ansiReset)
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
		{"treeView-beta", "Indented file and folder trees"},
		{"journey", "User journey maps"},
		{"packet-beta", "Network packet layouts"},
		{"eventmodeling", "Event-model swimlanes over time"},
		{"ishikawa-beta", "Fishbone cause-and-effect"},
		{"requirementDiagram", "SysML requirements and elements"},
		{"C4Context", "C4 context, container, component, deployment"},
		{"usecase-beta", "UML actors, use cases and boundaries"},
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
	"timeline": `timeline
    title Project Milestones
    2024 Q1 : Requirements
    2024 Q2 : Design : Prototype
    2024 Q3 : Development
    2024 Q4 : Launch`,
	"journey": `journey
    title Ship a Feature
    section Build
        Write code   : 4: Dev
        Run tests    : 3: Dev, CI
    section Release
        Code review  : 2: Dev, Lead
        Deploy       : 5: Dev`,
	"packet": `packet-beta
    0-15: "Source Port"
    16-31: "Destination Port"
    32-63: "Sequence Number"
    64-95: "Acknowledgment Number"
    96-99: "Data Offset"
    100-111: "Flags"
    112-127: "Window"`,
	"requirement": `requirementDiagram
    requirement checkout_req {
    id: 1
    text: Orders must be payable online.
    risk: high
    verifymethod: test
    }

    functionalRequirement payment_req {
    id: 1.1
    text: Card payments must be authorised.
    risk: high
    verifymethod: test
    }

    element checkout_service {
    type: service
    docref: docs/checkout.md
    }

    checkout_req - contains -> payment_req
    checkout_service - satisfies -> payment_req`,
	"c4": `C4Context
    title Internet Banking
    Enterprise_Boundary(b0, "Bank") {
        Person(customer, "Banking Customer", "A personal account holder.")
        Person(staff, "Support Staff", "Answers customer queries.")
        System(banking, "Internet Banking", "Accounts and payments.")
    }
    System_Ext(email, "E-mail System", "Microsoft Exchange.")

    Rel(customer, banking, "Uses")
    Rel(staff, banking, "Administers")
    Rel(banking, email, "Sends mail", "SMTP")`,
	"usecase": `usecase-beta
    actor Customer("Customer")
    actor Agent("Support Agent")
    systemBoundary Storefront
        Browse("Browse catalogue")
        Checkout("Place order")
        Pay("Take payment")
    end
    Customer --> Browse
    Customer --> Checkout
    Agent --> Checkout
    Checkout ..> : include Pay`,
	"quadrant": `quadrantChart
    title Priority Matrix
    x-axis Low Effort --> High Effort
    y-axis Low Impact --> High Impact
    quadrant-1 Do First
    quadrant-2 Schedule
    quadrant-3 Delegate
    quadrant-4 Eliminate
    Feature A: [0.2, 0.8]
    Feature B: [0.7, 0.9]
    Feature C: [0.8, 0.3]
    Feature D: [0.3, 0.4]`,
	"xychart": `xychart-beta
    title "Monthly Revenue"
    x-axis [Jan, Feb, Mar, Apr, May]
    y-axis "Revenue ($K)" 0 --> 100
    bar [45, 52, 68, 73, 91]
    line [45, 52, 68, 73, 91]`,
	"class": `classDiagram
    class Animal {
        +String name
        +int age
        +makeSound()
    }
    class Dog {
        +fetch()
    }
    class Cat {
        +purr()
    }
    Animal <|-- Dog
    Animal <|-- Cat`,
	"er": `erDiagram
    CUSTOMER ||--o{ ORDER : places
    ORDER ||--|{ LINE_ITEM : contains
    PRODUCT ||--o{ LINE_ITEM : "ordered in"`,
	"state": `stateDiagram-v2
    [*] --> Idle
    Idle --> Processing : submit
    Processing --> Review : complete
    Review --> Idle : reject
    Review --> Done : approve
    Done --> [*]`,
	"gitgraph": `gitGraph
    commit
    commit
    branch develop
    checkout develop
    commit
    commit
    checkout main
    merge develop
    commit`,
	"block": `block-beta
    columns 3
    A["Frontend"] B["API"] C["Database"]
    D["Cache"]:2 E["Queue"]`,
	"treeview": `treeView-beta
    mmaid-go/
        cmd/
            mmaid/
                main.go
        internal/
            diagram/  ## one file per type
            renderer/
        README.md`,
	"eventmodeling": `eventmodeling
    tf 01 ui CartUI
    tf 02 cmd AddItem ->> 01
    tf 03 evt ItemAdded ->> 02
    tf 04 rmo CartView ->> 03
    tf 05 ui CartScreen ->> 04`,
	"ishikawa": `ishikawa-beta
    Late Delivery
        Process
            Slow handoffs
            Manual steps
        People
            Understaffed
        Tooling
            Flaky CI
            No cache
        Environment
            Remote timezones`,
}

var demoTypes = []struct{ name, key string }{
	{"Flowchart", "flowchart"},
	{"Sequence Diagram", "sequence"},
	{"Class Diagram", "class"},
	{"ER Diagram", "er"},
	{"State Diagram", "state"},
	{"Git Graph", "gitgraph"},
	{"Block Diagram", "block"},
	{"Pie Chart", "pie"},
	{"Gantt Chart", "gantt"},
	{"Timeline", "timeline"},
	{"Kanban Board", "kanban"},
	{"Mindmap", "mindmap"},
	{"Quadrant Chart", "quadrant"},
	{"XY Chart", "xychart"},
	{"Treemap", "treemap"},
	{"Tree View", "treeview"},
	{"User Journey", "journey"},
	{"Packet Diagram", "packet"},
	{"Event Modeling", "eventmodeling"},
	{"Ishikawa Diagram", "ishikawa"},
	{"Requirement Diagram", "requirement"},
	{"C4 Diagram", "c4"},
	{"Use Case Diagram", "usecase"},
}

func runDemo(w io.Writer, themeName, diagramType string, opts ...mmaid.Option) {
	if _, ok := renderer.Themes[themeName]; !ok {
		fmt.Fprintf(os.Stderr, "%smmaid:%s unknown theme %q (use --themes to list)\n", ansiBold+ansiCyan, ansiReset, themeName)
		os.Exit(1)
	}

	fmt.Fprintf(w, "\n  %sTheme: %s%s\n", ansiBold+ansiCyan, themeName, ansiReset)

	if diagramType == "all" {
		for _, s := range demoTypes {
			fmt.Fprintf(w, "\n  %s%s%s\n\n", ansiBold+ansiWhite, s.name, ansiReset)
			result := mmaid.Render(demoSamples[s.key], append(opts, mmaid.WithTheme(themeName))...)
			fmt.Fprintln(w, result)
		}
		return
	}

	// Find matching sample
	source, ok := demoSamples[diagramType]
	if !ok {
		// Try matching by demo key aliases
		aliases := map[string]string{
			"flow": "flowchart", "seq": "sequence", "sequencediagram": "sequence",
			"classdiagram": "class", "erdiagram": "er",
			"statediagram": "state", "statediagram-v2": "state",
			"block-beta": "block", "git": "gitgraph",
			"xy": "xychart", "xychart-beta": "xychart",
			"quadrantchart": "quadrant",
			"treeview-beta": "treeview", "tree": "treeview",
			"eventmodel": "eventmodeling", "em": "eventmodeling",
			"ishikawa-beta": "ishikawa", "fishbone": "ishikawa",
		}
		if mapped, ok2 := aliases[strings.ToLower(diagramType)]; ok2 {
			source = demoSamples[mapped]
		} else {
			fmt.Fprintf(os.Stderr, "%smmaid:%s unknown demo type %q\n", ansiBold+ansiCyan, ansiReset, diagramType)
			fmt.Fprintf(os.Stderr, "  available: all, %s\n", strings.Join(demoKeys(), ", "))
			os.Exit(1)
		}
	}

	result := mmaid.Render(source, append(opts, mmaid.WithTheme(themeName))...)
	fmt.Fprintln(w, result)
}

func demoKeys() []string {
	keys := make([]string, 0, len(demoSamples))
	for k := range demoSamples {
		keys = append(keys, k)
	}
	return keys
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

// insertIntoFile inserts text into a file after a specific line.
// Format: "FILE:LINE" (e.g., "README.md:42")
func insertIntoFile(spec, content string) error {
	parts := strings.SplitN(spec, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("--insert format: FILE:LINE (e.g., README.md:42)")
	}
	filename := parts[0]
	lineNum, err := strconv.Atoi(parts[1])
	if err != nil {
		return fmt.Errorf("invalid line number %q", parts[1])
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("reading %s: %w", filename, err)
	}

	lines := strings.Split(string(data), "\n")
	if lineNum < 0 || lineNum > len(lines) {
		return fmt.Errorf("line %d out of range (file has %d lines)", lineNum, len(lines))
	}

	// Insert content after line N
	contentLines := strings.Split(content, "\n")
	result := make([]string, 0, len(lines)+len(contentLines))
	result = append(result, lines[:lineNum]...)
	result = append(result, contentLines...)
	result = append(result, lines[lineNum:]...)

	return os.WriteFile(filename, []byte(strings.Join(result, "\n")), 0644)
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
