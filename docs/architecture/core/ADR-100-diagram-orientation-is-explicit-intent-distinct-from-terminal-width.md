---
status: Accepted
date: 2026-05-15
deciders:
  - aaronsb
  - claude
related: []
---

# ADR-100: Diagram orientation is explicit intent, distinct from terminal width

## Context

Several diagram types (user journey, packet, gantt, timeline) lay out along
one axis and sprawl past the terminal on wide content. `timeline` already
copes by auto-switching to a vertical layout when its computed horizontal
width exceeds `usableWidth()`.

Generalising that width-triggered auto-switch to other diagrams is the wrong
model, for two reasons:

1. **It conflates two independent things.** Terminal width is *environmental*
   ("the terminal is this many columns"). Orientation is *authorial intent*
   ("I want this laid out vertically"). Coupling them makes "lay this out
   vertically but let it be as wide as it needs" unexpressible, and turns
   `--width` into a side-effect lever for something it does not name.
2. **The trigger is unreliable.** Terminal detection is unavailable in some
   embedded shells (e.g. inside Claude Code), so `getTerminalWidth()` returns
   a value that does not reflect reality and a width-keyed auto-switch never
   fires. Intent must not depend on a signal that can be silently absent.

## Decision

Orientation is controlled explicitly, by two inputs resolved in precedence
order (CLI wins):

1. **`--orientation TB|LR` CLI flag** — a render-time override.
2. **`direction TB|LR` directive line** in the diagram source — the authored
   default. Mermaid already uses `direction` for flowcharts and state
   subgraphs; this extends it as a port-wide convention available to every
   diagram type.

If neither is present, each renderer keeps its own natural default
(journey: horizontal).

Resolution is centralised in `diagram.resolveVertical(source, defaultVertical)`,
mirroring the existing `SetWidthOverride` / `usableWidth()` pattern, so no
render-function signatures change and diagrams adopt it incrementally
(journey first).

The directive is recognised only at **nesting depth 0**, matching Mermaid's
own scoping of `direction`: a `direction` inside a flowchart `subgraph` or a
composite-state `{ … }` block governs that block, not the whole diagram, so
it must not flip overall orientation. This keeps the convention consistent
with Mermaid semantics rather than requiring a documented exception when
flowchart/state adopt the helper.

Terminal width plays no part in orientation. `timeline`'s existing
width-based auto-switch is grandfathered and may migrate to this mechanism
later, but width-triggered switching is not extended to new diagrams.

## Consequences

### Positive

- Orientation and width are independently controllable and independently
  named — each does exactly what it says.
- Works in environments without terminal detection, because intent is
  declared, not inferred.
- One Mermaid-consistent keyword (`direction`) spans all diagram types;
  authored intent travels with the `.mmd` file.

### Negative

- Each sprawl-prone renderer must grow and maintain a second (vertical)
  layout path.
- Two orientation inputs plus the legacy timeline auto-switch is three
  code paths until timeline is migrated.

### Neutral

- New shared surface: `diagram.SetOrientationOverride` and the
  `--orientation` flag, to document and test like other CLI overrides.
- No automatic relayout: a diagram that overflows without an explicit
  `direction`/`--orientation` stays overflowed (acceptable — overflow is
  visible and the fix is one keyword).
- An unrecognised `--orientation` value warns on stderr and is ignored
  (no override applied), consistent with how `--theme` reports unknowns.

## Alternatives Considered

- **Generalise timeline's width-triggered auto-switch to all diagrams.**
  Rejected: conflates environmental width with authorial intent, and is
  inert when terminal detection is unavailable — the exact case that
  motivated this.
- **CLI flag only (`--orientation`, no directive).** Rejected: orientation
  is a property of the diagram and should travel with the source file, not
  live only in an invocation.
- **Directive only (no CLI override).** Rejected: a one-off "show me this
  the other way" should not require editing the source.
- **Mermaid `%%{init}%%` / config frontmatter.** Rejected for now: heavier
  to parse and less legible than a single `direction` line, for no added
  expressiveness here.
