<h1 align="center">termaid</h1>

<p align="center">Render Mermaid diagrams in your terminal. Single binary, zero dependencies.</p>

## Features

- **16 diagram types:** flowcharts, sequence, class, ER, state, block, git graphs, pie charts, treemaps, gantt, timeline, kanban, mindmap, quadrant, XY charts
- **Zero dependencies:** pure Go, single portable binary
- **11 color themes:** including 5 solid-background themes with depth-based region coloring
- **Anti-aliased pie charts:** circular rendering with half-block characters and supersampled edges
- **Braille fallback:** pie charts use distinct dot patterns when no color theme is active
- **ASCII mode:** works on any terminal
- **Pipe-friendly CLI:** `echo "graph LR; A-->B" | termaid` just works

## Install

```bash
go install github.com/aaronsb/termaid-go/cmd/termaid@latest
```

Or build from source:

```bash
git clone https://github.com/aaronsb/termaid-go
cd termaid-go
make build
```

## Quick start

```bash
# Render a diagram file
termaid diagram.mmd

# Pipe from stdin
echo "graph LR; A[Start] --> B{Check} --> C[Done]" | termaid

# With a color theme
echo "graph LR; A --> B --> C" | termaid -t blueprint

# Preview a theme
termaid --demo all -t monokai
```

## Use cases

```bash
# Visualize disk usage as a treemap
du -d1 -k /var/log | awk 'NR>1{printf "        \"%s\": %d\n",$2,$1}' | \
  (echo 'treemap-beta'; echo '    "disk usage"'; cat) | termaid -t blueprint

# Show Docker image layers
docker history --no-trunc --format '{{.Size}}\t{{.CreatedBy}}' myimage | \
  head -8 | awk -F'\t' '{gsub(/[^0-9]/,"",$1); if($1+0>0) printf "        \"%s\": %s\n",substr($2,1,30),$1}' | \
  (echo 'treemap-beta'; echo '    "layers"'; cat) | termaid -t gruvbox

# Quick architecture sketch
cat <<'EOF' | termaid -t blueprint
graph LR
    subgraph Frontend
        A[React App] --> B[API Client]
    end
    subgraph Backend
        C[REST API] --> D[(PostgreSQL)]
        C --> E[(Redis)]
    end
    B --> C
EOF

# Inline in Claude Code sessions
termaid -t blueprint <<'EOF'
sequenceDiagram
    participant User
    participant Claude
    participant Tool
    User->>Claude: Request
    Claude->>Tool: termaid render
    Tool-->>Claude: Diagram output
    Claude-->>User: Visual response
EOF
```

## Go API

```go
import termaid "github.com/aaronsb/termaid-go"

// Plain text
result := termaid.Render("graph LR\n  A --> B --> C")

// With options
result := termaid.Render(source,
    termaid.WithTheme("blueprint"),
    termaid.WithASCII(),
    termaid.WithPadding(6, 3),
)
```

## Supported diagram types

| Type | Keyword | Description |
|------|---------|-------------|
| Flowchart | `graph` / `flowchart` | Directed graphs with shapes, subgraphs, styling |
| Sequence | `sequenceDiagram` | Interaction sequences with lifelines and blocks |
| Class | `classDiagram` | UML class relationships and members |
| ER | `erDiagram` | Entity-relationship schemas with cardinality |
| State | `stateDiagram-v2` | State machines with transitions |
| Pie | `pie` | Circular charts (anti-aliased color or braille) |
| Git Graph | `gitGraph` | Branch, commit, merge, cherry-pick flows |
| Block | `block-beta` | Grid-based block layouts |
| Gantt | `gantt` | Project schedules with sections and today marker |
| Timeline | `timeline` | Chronological event sequences |
| Kanban | `kanban` | Column-based task boards |
| Mindmap | `mindmap` | Hierarchical concept maps |
| Quadrant | `quadrantChart` | 2x2 matrix plots with data points |
| XY Chart | `xychart-beta` | Bar and line charts on axes |
| Treemap | `treemap-beta` | Proportional area treemaps |

### Node shapes

Shapes are visually distinct and carry a small indicator in the upper-left corner:

| Syntax | Shape | Indicator |
|--------|-------|-----------|
| `[text]` | Rectangle (sharp corners) | — |
| `(text)` | Rounded rectangle | `◦` |
| `{text}` | Diamond (chamfered `⟋⟍`) | `◇` |
| `((text))` | Circle | `○` |
| `([text])` | Stadium | `⊂` |
| `{{text}}` | Hexagon | `⬡` |
| `[[text]]` | Subroutine | `‖` |

## CLI

```
termaid [flags] [file]

FLAGS
  -a, --ascii          ASCII-only output
  -t, --theme NAME     Color theme (use --themes to list)
  -v, --version        Print version
      --themes         List available themes
      --demo TYPE      Preview diagrams (all, pie, gantt, flowchart, ...)
      --padding-x N    Horizontal node padding (default: 4)
      --padding-y N    Vertical node padding (default: 2)
      --sharp-edges    Sharp corners on edge routing
```

## Themes

| Theme | Type | Description |
|-------|------|-------------|
| `default` | text | Cyan nodes, yellow arrows, white labels |
| `terra` | text | Warm earth tones |
| `neon` | text | Magenta, green, cyan |
| `mono` | text | White/gray monochrome |
| `amber` | text | Amber CRT-style |
| `phosphor` | text | Green phosphor terminal |
| `blueprint` | **solid** | Deep blue backgrounds, depth-based region colors |
| `slate` | **solid** | Dark gray backgrounds, orange accents |
| `sunset` | **solid** | Deep rose backgrounds, gold arrows |
| `gruvbox` | **solid** | Gruvbox dark palette |
| `monokai` | **solid** | Monokai dark with pink/green accents |

**Solid themes** include:
- Wallpaper fills behind chart-type diagrams
- Per-section hue with per-depth shade stepping
- Fill layer compositing (foreground elements inherit region backgrounds)
- Section-colored text labels for visual grouping

## Rendering modes

Pie charts render in three modes depending on context:

| Mode | When | Rendering |
|------|------|-----------|
| Color circle | Any `--theme` | Half-block chars with 4x4 supersampled anti-aliasing |
| Braille circle | No theme | Braille dot patterns per slice, bordered legend |
| Bar chart | `--ascii` | Horizontal bars with fill characters |

## Acknowledgements

Originally inspired by [termaid](https://github.com/saikocat/termaid) (Python) by saikocat. Rewritten in Go for portability and single-binary distribution.

Also inspired by [mermaid-ascii](https://github.com/AlexanderGrooff/mermaid-ascii) by Alexander Grooff.

## License

MIT
