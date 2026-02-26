# termmaid

Render [Mermaid](https://mermaid.js.org/) diagrams as Unicode art in the terminal. Pure Python, zero dependencies.

```
┌─────────┐    ┌──────◇─────┐    ┌────────┐
│         │    │            │Yes │        │
│  Start  ├───►│  Decision  ├──╮►│   OK   │
│         │    │            │  │ │        │
└─────────┘    └──────◇─────┘  │ └────────┘
                               │
                               │No
                               │
                               │ ┌────────┐
                               │ │        │
                               ╰►│  Fail  │
                                 │        │
                                 └────────┘
```

## Why?

I needed Mermaid rendering for a Python project and couldn't find a library that worked
without a browser, Node.js, or external services. The existing tools in this space are
great, specially [mermaid-ascii](https://github.com/AlexanderGrooff/mermaid-ascii) (Go) and
[beautiful-mermaid](https://github.com/lukilabs/beautiful-mermaid) (TypeScript), but
neither offered a native Python library I could import and call directly. So I built
termmaid: a pure Python implementation with no runtime dependecies that works anywhere
Python runs.

## Install

```bash
pip install termmaid
```

## Usage

### CLI

```bash
termmaid diagram.mmd
echo "graph LR; A-->B-->C" | termmaid
termmaid diagram.mmd --color --theme terra
termmaid diagram.mmd --ascii
```

### Python

```python
from termmaid import render

print(render("graph LR\n  A --> B --> C"))
```

```python
# Colored output (requires: pip install termmaid[rich])
from termmaid import render_rich
from rich import print as rprint

rprint(render_rich("graph LR\n  A --> B", theme="terra"))
```

## Supported diagram types

### Flowcharts

All four directions: `LR`, `RL`, `TD`/`TB`, `BT`

```mermaid
graph TD
    A[Start] --> B{Is valid?}
    B -->|Yes| C(Process)
    C --> D([Done])
    B -->|No| E[Error]
```

```
┌─────────────┐
│             │
│    Start    │
│             │
└──────┬──────┘
       │
       ▼
┌──────◇──────┐
│             │
│  Is valid?  │
│             │
└──────◇──────┘
       │No
       ╰Yes─────────────╮
       ▼                ▼
╭─────────────╮    ┌─────────┐
│             │    │         │
│   Process   │    │  Error  │
│             │    │         │
╰──────┬──────╯    └─────────┘
       │
       ▼
╭─────────────╮
(             )
(    Done     )
(             )
╰─────────────╯
```

**Node shapes:** rectangle `[text]`, rounded `(text)`, diamond `{text}`, stadium `([text])`, subroutine `[[text]]`, circle `((text))`, hexagon `{{text}}`, cylinder `[(text)]`, and more.

**Edge styles:** solid `-->`, dotted `-.->`, thick `==>`, bidirectional `<-->`, labeled `-->|text|`

**Subgraphs:** Nesting, cross-boundary edges, labels

### State diagrams

```mermaid
stateDiagram-v2
    [*] --> Idle
    Idle --> Processing : start
    Processing --> Done : complete
    Done --> [*]
```

## CLI options

| Flag | Description |
|------|-------------|
| `--ascii` | ASCII-only output (no Unicode box-drawing) |
| `--color` | Colored output (requires `pip install termmaid[rich]`) |
| `--theme NAME` | Color theme: default, terra, neon, mono, amber, phosphor |
| `--padding-x N` | Horizontal padding inside boxes (default: 4) |
| `--padding-y N` | Vertical padding inside boxes (default: 2) |
| `--sharp-edges` | Sharp corners on edge turns instead of rounded |

## Optional extras

```bash
pip install termmaid[rich]      # Colored terminal output
pip install termmaid[textual]   # Textual TUI widget
```

## Acknowledgements

Inspired by [mermaid-ascii](https://github.com/AlexanderGrooff/mermaid-ascii) by Alexander Grooff
and [beautiful-mermaid](https://github.com/lukilabs/beautiful-mermaid) by Lukilabs.

## License

MIT
