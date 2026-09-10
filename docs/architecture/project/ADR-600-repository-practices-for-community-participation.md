---
status: Accepted
date: 2026-09-09
deciders:
  - aaronsb
  - claude
related:
  - ADR-101
  - ADR-402
---

# ADR-600: Repository practices for community participation

## Context

mmaid-go began as a Go rewrite of termaid and carried that repository's
scaffolding: a Python CI workflow, a PyPI publish workflow, a LICENSE
naming termaid's author alone, no contributing guide, no templates, no
owners file, the default label set, and an unprotected default branch.
The fork link was severed on 2026-09-09. The project now has 31 diagram
types, nine accepted ADRs, and a test loop that lets a contributor verify
a rendering change without a human eye; it is ready for contributors.

## Decision

### ADR domains

Six bands: core (100), cli (200), ingest (300), renderer (400), config
(500), and project (600, this band). Every architectural change lands
with an ADR in its band as the first commit of its PR, `Proposed` until
the PR merges.

### Ownership

`.github/CODEOWNERS` names the maintainer for everything, with explicit
lines for the decision records, the rendering core, and the reference
frames, so a change to a golden or a gallery image asks for a look.

### Contribution flow

`CONTRIBUTING.md` states the loop (`make snap`, `make golden`,
`make golden-record`, `make check`), the rules the code keeps (ADR-400,
ADR-401, ADR-402, ADR-500, zero dependencies), and the steps for a new
diagram type. The pull request template asks for the decision, the gate,
and the PNGs read. Three issue forms: rendering or CLI bug, feature
request, diagram type gap. Security reports go through GitHub advisories.
Discussions are enabled for questions.

### Labels

`area:` by package (renderer, layout, routing, parser, cli, config, docs),
`diagram-type`, `adr`, `needs-fixture`, and `effort:` small, medium, large,
beside GitHub's defaults.

### CI

One workflow: gofmt, `make check`, `make golden` (goldens and the
structural lint), an empty `known-bad.txt`, and a cross-compile. It runs
on pushes to main and on every pull request. The PyPI workflow is gone;
arch-repo publishes the AUR package from GitHub releases.

### Branch protection

`main` requires the CI check to pass and refuses force pushes. It does not
require a review, since the maintainer is the only committer; that flips
to one required review when a second regular contributor appears.

### Project ways

`.claude/ways/render/snapshot-loop` fires when an agent edits the
renderer, layout, routing, or a diagram, and repeats the loop and the
rules in `.claude/CLAUDE.md`.

### License

MIT, copyright the maintainer, with the journey and packet renderers
credited to termaid's author under the same license.

## Consequences

### Positive

- A contributor can verify a rendering change alone, through the loop,
  and CI holds the same gate.
- Every inherited artifact that named the wrong project is gone.

### Negative

- Branch protection blocks a direct push to main, including the
  maintainer's.
- Issue forms add friction for a one-line report.

### Neutral

- This ADR is the scaffold record: `/project-audit` compares the
  repository against it and reports drift.

## Alternatives Considered

- **Required reviews from the start.** Rejected: one committer cannot
  review their own PR, and a rule that is always bypassed teaches nothing.
- **Keep the default labels only.** Rejected: `area:` and `effort:` are
  what triage sorts by.
- **A CHANGELOG file.** Rejected for now: GitHub release notes are
  generated from PR titles, and every PR title is a conventional commit.
