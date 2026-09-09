---
status: Proposed
date: 2026-09-09
deciders:
  - aaronsb
  - claude
related:
  - ADR-101
  - ADR-400
  - ADR-401
---

# ADR-500: Glyph families with fallback, guided terminal tester, XDG profiles

## Context

`CharSet` in `internal/renderer/charset.go` is one flat struct of 40 runes
with two instances, `UNICODE` and `ASCII`, chosen by `--ascii`. A
terminal whose font lacks one glyph (the U+27CB diamond chamfer, the
U+2B21 hexagon, the heavy box set in some monospace fonts) shows tofu
for that glyph, and the only remedy is to drop to ASCII for everything.

Nothing about the terminal is remembered between runs. Theme, width, and
glyph choice are flags on every invocation. Whether the terminal renders
truecolor, whether Symbols for Legacy Computing advance one column, and
whether OSC 8 hyperlinks are understood are facts the user knows after
looking once and mmaid never learns.

Some of those facts are probeable. Terminal identity is in
`TERM_PROGRAM` or the DA1 response; truecolor in `COLORTERM`; the column
a glyph advances by in a cursor position report. Whether a glyph is
drawn as the right shape is not probeable: a missing glyph advances one
column and draws a box.

## Decision

### Families

`CharSet` splits into families, each a table over the roles the renderer
uses from it:

| Family | Roles | Fallback |
|---|---|---|
| `box-light` | ADR-400 light table, half lines | `ascii` |
| `box-rounded` | four corner overrides | `box-light` |
| `box-heavy` | ADR-400 heavy table | `box-light` |
| `box-double` | ADR-400 double table | `box-light` |
| `diagonals` | `╱ ╲ ╳` chamfers | `ascii` |
| `arrows` | `► ◄ ▲ ▼`, endpoints `○ ● ◉ ×` | `ascii` |
| `blocks` | `█ ▀ ▄ ░ ▒ ▓` fills and bars | `ascii` |
| `braille` | U+2800 dot patterns | `blocks` |
| `sextants` | U+1FB00 to U+1FB3B | `blocks` |
| `octants` | U+1CD00 to U+1CDE5 (Unicode 16) | `sextants` |
| `legacy-fills` | PETSCII and other U+1FB00 fills | `sextants` |
| `ascii` | the seven-bit set | none |

A glyph set is a name bound to one family per role group. Built-in sets
are `unicode` (default), `rounded`, `heavy`, `double`, `legacy`, and
`ascii`. `--glyphs NAME` selects one; `--ascii` stays as an alias for
`--glyphs ascii`. A profile may mark any family failed, and a failed
family resolves to its fallback and nothing else changes.

### The tester

`mmaid config init` runs in two phases and the boundary between them is
fixed: anything a terminal can answer is probed; anything only eyes can
answer is asked.

Probed, with a 200 ms timeout on each query:

- terminal identity: `TERM_PROGRAM` when set, else DA1 (`ESC [ c`)
  mapped to a known name, else `TERM`;
- truecolor: `COLORTERM` equal to `truecolor` or `24bit`;
- advance width per family: print the family's sample with the cursor
  at a known column, request a cursor position report (`ESC [ 6 n`),
  and compare. A family whose sample advances by a different count than
  its ADR-401 width is marked failed with the cause `advance`.

Raw mode for the probes uses `ioctl` through the `syscall` package on
Linux and macOS. On Windows and on a non-tty, the probes are skipped and
every item is asked.

Asked: the tester prints one numbered line per family, the family's
sample beside a reference of the same figure drawn in `ascii` and
`box-light`, and one question: which line numbers look wrong. Each
failure gets one line naming the family, the cause (`missing`,
`advance`, `shape`), and the remedy (the fallback that will be used, and
the font that would fix it). Nothing is asked that was probed.

The sample lines are produced by `renderer.GlyphSamples(set)` and
`mmaid --glyphs-sample` prints them, so the sheet is an ADR-101 fixture.

### Profiles

Config is JSON at `$XDG_CONFIG_HOME/mmaid/config.json`, defaulting to
`~/.config/mmaid/config.json`:

```json
{
  "default": { "theme": "blueprint", "glyphs": "unicode", "width": 0 },
  "profiles": {
    "WezTerm": { "truecolor": true, "hyperlinks": true },
    "Apple_Terminal": { "truecolor": false, "failed": ["box-heavy", "octants"] }
  }
}
```

Profile keys are terminal identities as the probe reports them. Every
key accepted in a profile is accepted in `default`: `theme`, `glyphs`,
`width`, `orientation`, `padding_x`, `padding_y`, `sharp_edges`,
`truecolor`, `hyperlinks`, `ambiguous_wide`, `failed`.

Resolution order for every setting, first match wins:

1. a command-line flag;
2. `MMAID_<SETTING>` in the environment, upper-cased with underscores;
3. the profile whose key is the current terminal identity;
4. `default` in the file;
5. the built-in default.

`mmaid config show` prints every setting with its resolved value and the
source that supplied it. `mmaid config init` writes the current
terminal's profile and asks before replacing one that exists.

### Colour and links

`NO_COLOR` set and non-empty disables colour from a profile or the
environment; an explicit `-t` flag on the command line still colours.
When `truecolor` resolves false, themes emit 256-colour approximations.

`click ID "url"` lines, which the flowchart parser recognises and drops
at `internal/parser/flowchart.go:270`, are kept on the node, and when
`hyperlinks` resolves true the node's label cells are wrapped in OSC 8.
The ADR-101 interpreter skips OSC, so frames are unchanged.

### Output

`--output FILE` writes what would go to stdout to FILE. `--watch` renders
the input file, then re-renders when its modification time changes,
clearing the screen between renders, polling at 250 ms.

## Consequences

### Positive

- A font gap costs one family, not the whole Unicode set.
- The terminal is measured once; every later run uses the answer.
- Everything probeable is probed and the user answers only the visual
  question.
- The tester's sheet is a golden.

### Negative

- Raw-mode probing is platform code in a project that had none; Windows
  gets the asked path only.
- A wrong `TERM_PROGRAM` (an ssh session forwarding the client's value)
  keys the profile to the wrong terminal. `mmaid config show` makes
  that visible.
- Five resolution sources for each setting is a debugging surface.

### Neutral

- The decision lands in two pull requests: the profiles, colour, links and
  output half first, then the glyph families and the tester.
- The CLI grows a `config` subcommand beside its flags.
- Theme colours are unchanged by this ADR; base16 scheme files are a
  separate decision.

## Alternatives Considered

- **Probe glyph presence by drawing and reading back.** Rejected:
  terminals do not report what they drew, and a missing glyph advances
  correctly.
- **Ask everything.** Rejected: identity, truecolor, and advance width
  are answered by the terminal in under a second and a human answers
  them worse.
- **TOML or YAML config.** Rejected: JSON needs no dependency.
- **Key profiles by `TERM`.** Rejected: `xterm-256color` is every
  terminal.
- **Detect the terminal on every run and skip the file.** Rejected: the
  visual answers cannot be re-derived on every run.
