---
description: Seeing a render through the snapshot loop when editing the renderer, layout, routing, or a diagram
files: internal/(renderer|layout|routing|diagram|parser|glyph)/.*\.go$|testdata/fixtures/.*\.mmd$
pattern: make snap|golden|known-bad|lint finding|re-record
---
# Snapshot Loop

You are editing something that draws. Do not judge the result from ANSI in
a tool result.

```bash
make snap FILE=testdata/fixtures/<name>.mmd ARGS="-t blueprint"
```

Read `.snap/<name>.png`. It carries rulers, so name cells as (row, col) the
way the lint findings do.

Before committing: `make golden`. If frames moved on purpose,
`make golden-record` and list each changed frame and why in the commit
body. `testdata/fixtures/known-bad.txt` stays empty; fix the renderer
rather than listing the fixture.

Lines go through `Arm`, `Segment`, or `PutBox` (ADR-400). Every line end
meets an arm, a fed arrowhead, or text (ADR-402). Glyphs come from the
`CharSet` (ADR-500); widths from `textwidth` (ADR-401).
