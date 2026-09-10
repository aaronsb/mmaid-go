# Security

mmaid reads a text file or stdin and writes a text stream. It opens no
network connections, executes nothing it reads, and touches the filesystem
only where a flag names a path: `--output`, `--cells`, `--insert`,
`--watch`, and its own config file under `$XDG_CONFIG_HOME/mmaid`.

Terminal escape output is the one surface worth care. `click` URLs are
emitted as OSC 8 hyperlinks only when `hyperlinks` is enabled in the
profile, and only after control bytes are rejected and the scheme is
allow-listed (`http`, `https`, `mailto`, `file`). Labels are sanitised
before rendering. If you find a way to make mmaid emit an escape sequence
it did not intend from diagram text, that is a security bug.

## Reporting

Report privately through GitHub's
[security advisories](https://github.com/aaronsb/mmaid-go/security/advisories/new)
for this repository. Expect an acknowledgement within a week. Fixes ship
as a patch release; the advisory is published after the release.

## Supported versions

The latest release. Older versions receive no patches.
