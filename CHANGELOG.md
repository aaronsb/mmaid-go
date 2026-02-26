# Changelog

## 0.1.0 (2025-xx-xx)

Initial release.

### Features

- Parse and render Mermaid flowcharts (`graph`/`flowchart` with `LR`, `RL`, `TD`, `BT`)
- Parse and render Mermaid state diagrams (`stateDiagram-v2`)
- 14 node shapes: rectangle, rounded, stadium, subroutine, diamond, hexagon, circle, double circle, asymmetric, cylinder, parallelogram, parallelogram alt, trapezoid, trapezoid alt
- 3 edge styles: solid, dotted, thick
- Bidirectional edges (`<-->`, `<-.->`, `<==>`)
- Edge labels (`-->|label|`)
- Subgraphs with nesting and cross-boundary edges
- `classDef`, `style`, and `linkStyle` directives
- `@{shape}` node syntax
- Link length control (`--->`, `---->`)
- Markdown labels (`**bold**`, `*italic*`)
- ASCII mode (`--ascii`) for terminals without Unicode support
- Colored output via Rich (`--color`) with 7 built-in themes
- Textual widget for TUI applications
- CLI tool with stdin/file input
