---
status: Draft
date: 2026-03-23
deciders:
  - aaronsb
related: []
---

# ADR-300: Opinionated JSON ingest modes

## Context

mmaid renders Mermaid syntax into terminal art. When users want to visualize
system data (disk layout, process stats, resource allocation), they must write
jq pipelines to transform JSON into Mermaid syntax before piping to mmaid:

```bash
lsblk -Jb -o NAME,SIZE,TYPE | jq -r '
  def human: ...30 lines of jq...
  "treemap-beta", ...
' | mmaid -t blueprint
```

The jq step is the hardest part — it requires knowing both jq and Mermaid
syntax. This friction undermines mmaid's value as a pipe-friendly tool.

## Decision

Add a `--json DIAGRAM_TYPE` flag that reads JSON from stdin and converts it to
the target diagram type using opinionated field mapping conventions.

### Initial ingest modes

| Mode | JSON shape | Mermaid target | Field defaults |
|------|-----------|----------------|----------------|
| `treemap` | Nested objects with children arrays | `treemap-beta` | name=`name`, value=`size`, children=`children` |
| `pie` | Flat `{"key": number}` | `pie` | keys=labels, values=weights |

### Interface

```bash
# Tree JSON -> treemap (discovers name/size/children fields)
lsblk -Jb -o NAME,SIZE,TYPE | mmaid --json treemap -t blueprint

# Flat key/value -> pie
echo '{"Go":45,"Rust":30,"Python":20}' | mmaid --json pie -t monokai
```

### Field override flags

When JSON fields don't match the defaults:

```bash
# Custom field names
some-tool --json | mmaid --json treemap --name-key label --value-key bytes --children-key nodes
```

### Tree walker behavior (treemap mode)

1. Walk the root object to find the first array of objects
2. Use `name` field (or `--name-key`) for node labels
3. Use `size` field (or `--value-key`) for leaf weights
4. Use `children` field (or `--children-key`) for nesting
5. Auto-format large values with human-readable suffixes (G/M/K) in labels
6. Floor leaf values to integers, minimum 1 (treemap requires positive ints)

### Scope boundary

This is NOT a general-purpose data transformation tool. We support:
- A small number of opinionated modes (not arbitrary JSON-to-Mermaid)
- Common CLI tool output shapes (lsblk, du, df patterns)
- Sensible defaults that work without flags for the 80% case

We do NOT support:
- Arbitrary JSON path expressions
- Complex aggregation or grouping
- CSV, YAML, or other input formats (yet)

## Consequences

### Positive

- `lsblk -Jb | mmaid --json treemap -t blueprint` — one pipe, no jq
- Makes the asciinema demo compelling: real system data, simple command
- Opens the door for more ingest modes later (xychart, gantt) without redesign

### Negative

- Adds JSON parsing dependency to a Mermaid renderer
- Each ingest mode is its own code path to maintain
- Opinionated defaults won't match every JSON shape

### Neutral

- `--json` flag is orthogonal to existing Mermaid stdin input — no breaking changes
- Field override flags add CLI surface area but are optional
- Go's `encoding/json` handles this natively, no external deps needed

## Alternatives Considered

- **Do nothing, let users write jq**: Works but kills the pipe-friendly pitch.
  The jq step is 30 lines for a treemap. Most users won't bother.
- **Ship jq wrapper scripts**: Fragile, requires jq installed, not discoverable.
- **Full JSONPath query language**: Over-engineered. jq exists for that.
  We want opinionated convenience, not generality.
- **`--from json --as treemap` (two flags)**: More explicit but verbose.
  `--json treemap` is shorter and reads naturally.
