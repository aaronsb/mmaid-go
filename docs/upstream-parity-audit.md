# Upstream Parity Audit — mmaid-go vs. fasouto/termaid

**Date:** 2026-05-15
**Upstream reference:** [fasouto/termaid](https://github.com/fasouto/termaid) v0.6.1 (2026-03-30)
**This port at audit time:** 0.4.1 → 0.5.0 (this branch)

mmaid-go is a Go reimplementation, not a vendored port — it has independently
grown features (themes, JSON ingest, demo) that don't map 1:1 to upstream
version numbers. This audit compares *capabilities*, not version strings.

## Diagram-type parity

| Type | Upstream | mmaid-go | Status |
|---|---|---|---|
| flowchart, sequence, class, ER, state, block, git, pie, treemap, gantt, timeline, kanban, mindmap, quadrant, XY | ✅ | ✅ | Parity |
| **user journey** (`journey`) | ✅ | ✅ | **Ported this branch** |
| **packet** (`packet-beta`) | ✅ | ✅ | **Ported this branch** |
| **architecture** (`architecture-beta`) | ✅ | ❌ | **Deferred — see below** |

Result: 17/18 upstream diagram types. Architecture is the only gap.

## Architecture diagram — why deferred (not "skipped")

Upstream's `architecture.py` (332 lines) does not have its own renderer. It
builds a standard `Graph` and sets **`graph.grid_positions`** — explicit
(col,row) coordinates derived from `db:R -- L:server` direction hints — then
relies on the shared layout engine to honor those positions.

mmaid-go's layout engine cannot do this today:

- `internal/graph/model.go` `Graph` has **no explicit-position field**.
- `internal/layout/grid.go` (1455 lines, on the priority-split list) runs an
  unconditional Sugiyama auto-layout (`assignLayers` → `orderLayers` →
  `placeNodes`) with no hook to accept pre-set positions.

A reduced-fidelity port (build the graph, let auto-layout place it) would
**break the one feature the `architecture-beta` syntax exists for** — spatial
placement. Upstream lists architecture with no caveat, so shipping degraded
placement would be dishonest parity. Absence is better than a diagram users
will read as "broken."

### Scoped follow-up (when picked up)

This is its own PR, and because it touches the priority-flagged `grid.go`, it
**warrants an ADR** ("explicit-position layout path") rather than being
smuggled under a diagram port.

1. `internal/graph/model.go`: add `GridPositions map[string]GridCoord` to
   `Graph`; add `ShapeJunction` to `NodeShape`. (~15 lines)
2. `internal/layout/grid.go`: a *parallel* entry path in `ComputeLayout` — if
   `GridPositions` is set, skip layering/ordering/placement and feed positions
   straight into `computeSizes`/`computeSubgraphBounds`/`GridToDraw`.
   Addition, not edit, to the existing pipeline. (~200 lines)
3. `internal/parser/architecture.go`: port the parser — junction elimination,
   direction-hint resolver, nested subgraphs. (~350 lines Go)
4. Renderer: `ShapeJunction` renders as a no-op in `drawNodes`.

## Non-diagram drift (upstream 0.3.0 → 0.6.1)

Candidate gaps for a future pass. **Confidence** = how sure the gap is real
without deeper verification. None auto-ported, per scope.

| Item | Upstream | mmaid-go | Confidence | Notes |
|---|---|---|---|---|
| **Wide-char / CJK / emoji display width** | ✅ 0.5.0 + 0.6.1 | ❌ | **High** | No `runewidth` equivalent; canvas is strict 1-rune/cell. Hit directly while porting journey (emoji faces → ASCII). Affects any CJK/emoji label in **every** diagram. Highest-value fix. |
| `--tui` interactive viewer | ✅ 0.1.3 | ❌ | High | Significant feature. Arguably out of scope for a single-binary renderer — a deliberate design call, not just a gap. |
| `-o` / `--output FILE` | ✅ 0.3.0 | ❌ | High | Low impact (shell redirection works). |
| `--show-ids` (debug) | ✅ 0.3.0 | ❌ | High | Minor debugging aid. |
| `NO_COLOR` env var | ✅ 0.3.0 | ❌ | High | No reference in code. Easy, well-known convention — worth doing. |
| `--gap` flag | ✅ 0.2.1 | ❌ | Medium | Port has `--padding-x/y` but no `--gap`. Verify whether padding covers the use case. |
| Auto-fit / `--no-auto-fit` | ✅ 0.3.0 | Partial | Medium | timeline self-switches H/V on width. No global auto-fit-with-compaction + opt-out. **Superseded in part by ADR-100**: orientation is now explicit (`direction` directive / `--orientation`), decoupled from width — the journey-sprawl case is addressed without a width trigger. |
| Engine behaviors: gap expansion, endpoint spreading, subgraph separation, routing bias, label separation | ✅ 0.3.0 | Independent impl | Low | Port has its own routing/draw with a junction table (different approach). Needs visual regression vs. upstream fixtures to confirm parity — can't assert from code read alone. |

### Recommended priority if a drift pass happens

1. **Wide-character display width** — broad correctness impact, evidence-backed.
2. **`NO_COLOR`** — trivial, standard, expected by users.
3. Architecture diagram (separate ADR + PR per above).
4. Everything else as discrete, low-urgency issues.

## What shipped on this branch

- Ported `journey` and `packet` (parser + renderer + tests + demo).
- README diagram count corrected (claimed 16, listed 15 → now an accurate 17).
- Version 0.4.1 → 0.5.0 (code constant; release tag / PKGBUILD bump is a
  separate release step, owner's call).
- This audit.
- Diagram orientation control (`direction` directive + `--orientation`
  override), per ADR-100. journey is the first consumer (horizontal default,
  vertical on request); the `resolveVertical` helper lets other diagrams
  adopt it incrementally.
