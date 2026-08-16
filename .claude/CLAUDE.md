# mmaid-go

## Releasing

`aaronsb/arch-repo` publishes this project. It reads `PKGBUILD` from the default
branch, builds it in a clean container, lints with namcap, signs, and pushes to
the AUR (\`mmaid\`) and the `[aaronsb]` pacman repository.

```bash
make package                 # check it builds and lints
make release                 # cross-compile, tag, cut the GitHub release
```

Nothing here talks to the AUR. There is no `aur` target and no publish script:
two writers to one AUR ref is how a PKGBUILD and its `.SRCINFO` drift apart.

### Fields arch-repo owns

It overwrites all four before publishing, so a value set here is only wrong
until it does. Do not maintain them, and do not commit a `.SRCINFO`.

| Field | Where it really comes from |
|---|---|
| `pkgver` | the newest published GitHub release |
| `pkgrel` | arch-repo's count of how many times it packaged that release |
| `sha256sums` | computed from the release artifact |
| `.SRCINFO` | regenerated at publish |

This project's own version lives in `cmd/mmaid/main.go` — `const version`.

### A packaging fix needs no release

Change the recipe on the default branch and push. arch-repo compares the
rendered recipe against what it last published and ships the difference as a
`pkgrel` bump — `1.2.0-1` becomes `1.2.0-2`, resetting to `-1` at the next
real release. Do not cut a version for a change to packaging alone.

### Check before you tag

`make package` builds the recipe in a clean chroot and runs namcap. It builds
from `HEAD` rather than the published archive, so it works before the release
it precedes, and it fails on a namcap error — namcap exits 0 whether or not it
found one.

`make release` also attaches the darwin and windows binaries to the release.
Those are for people not on Arch; the Arch packaging reads the source tarball
GitHub generates.

`_repo=mmaid-go` in the recipe names the repository, because the package is
`mmaid` but the tarball extracts to `mmaid-go-$pkgver`.

The full contract: https://github.com/aaronsb/arch-repo/blob/main/docs/packaging-contract.md
