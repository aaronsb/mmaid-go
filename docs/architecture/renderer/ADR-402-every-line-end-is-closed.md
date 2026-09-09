---
status: Proposed
date: 2026-09-09
deciders:
  - aaronsb
  - claude
related:
  - ADR-101
  - ADR-400
---

# ADR-402: Every line end is closed

## Context

After ADR-400 and the fixes for issue #18, eight fixtures remain in
`testdata/fixtures/known-bad.txt`, each for a line whose end meets
nothing: chart axes stopping in open air (quadrant, xychart), separator
rules running to the margin (gantt, eventmodeling), lifelines cut by
messages and arrowheads landing on the lifeline column (sequence, zenuml),
an inheritance marker drawn as `◁│▷` with neither triangle fed (class),
and section brackets whose corners point at an empty row (journey).

Each is a renderer's design rather than a defect the arm model can settle
on its own. The choice for each was between changing the drawing so the
arm tables produce a closed end, or teaching the lint an exception.

## Decision

A line end is closed by what it meets: another arm, an arrowhead fed from
its tail, or text. No lint exception is added for any renderer.

- **Axes** (quadrant, xychart): the origin is a corner; each axis ends in
  an arrowhead from the arrows family, `▸` and `▴`, fed by the axis line.
- **Separator rules** (gantt section and day rules, eventmodeling lane
  rules): a rule runs from the row's label text to the right-hand text
  when there is one, and otherwise ends one cell short at a `·` marker.
- **Lifelines** (sequence, zenuml): a message crossing an intermediate
  lifeline resolves to `┼` through the tables; at its target the
  arrowhead sits in the cell before the lifeline, which continues
  unbroken.
- **Inheritance** (class): the relationship line ends in one hollow
  arrowhead, `△ ▽ ◁ ▷` by direction, fed from its tail. The `◁│▷`
  composite goes.
- **Journey sections**: a section is a light box around its task cards
  with the title in the top border, as subgraphs and lanes draw.

### Gate

`known-bad.txt` is empty and the file stays, so a future regression has
somewhere to be recorded with its cause. The lint has no renderer
exceptions.

## Consequences

### Positive

- The structural lint holds over every fixture with no allowlist.
- Axes, sections, and lifeline crossings use the same tables and glyph
  families as everything else, so ASCII and family fallback apply.

### Negative

- A sequence arrowhead no longer touches its target lifeline.
- Journey and the axis charts each gain a row or column of chrome.

### Neutral

- Each choice was the option the arm tables produce unaided; the
  alternatives that kept the current drawing all needed a lint exception.

## Alternatives Considered

- **A "rule line" class the lint exempts.** Rejected: every renderer
  could widen the hole, and the lint's value is that it has none.
- **Lifelines as a glyph outside the box tables.** Rejected: it hides
  the crossing from the tables that resolve every other crossing.
- **Message interrupts the lifeline.** Rejected: an interrupted line is a
  rule 1 finding by definition.
- **Filled arrowhead for inheritance.** Rejected: UML's hollow head
  distinguishes inheritance from composition.
- **Journey title as text only.** Rejected: the grouping is the point of
  a section.
