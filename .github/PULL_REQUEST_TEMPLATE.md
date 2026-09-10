## Summary

<!-- One to three bullets. What changed and, for a renderer change, why the new drawing is right. -->

## Decision

<!-- The ADR this implements, or "none needed" for a fix. A PR that changes an architectural rule adds or amends an ADR as its first commit. -->

## Test plan

- [ ] `make check` clean
- [ ] `make golden` clean; if frames were re-recorded, the commit body lists each and why
- [ ] `testdata/fixtures/known-bad.txt` still empty
- [ ] PNGs read for the fixtures this touches (`make snap FILE=...`)
- [ ] New diagram type: fixture, gallery, README row, `--demo`, `// Read:` / `// Skipped:` block
