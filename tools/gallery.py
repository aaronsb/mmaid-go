"""Build docs/gallery from the golden dump: one PNG per fixture and an index.

Usage: gallery.py DUMP_DIR FIXTURES_DIR OUT_DIR

DUMP_DIR holds <stem>.cells for every fixture, as written by
GOLDEN_DUMP=<dir> go test ./ -run TestGolden. Each becomes OUT_DIR/<stem>.png
through cells2png.py, and OUT_DIR/README.md lists every fixture with its
source and image.
"""
import os
import subprocess
import sys

if len(sys.argv) != 4:
    sys.exit("usage: gallery.py DUMP_DIR FIXTURES_DIR OUT_DIR")
dump, fixtures, out = sys.argv[1:4]
here = os.path.dirname(os.path.abspath(__file__))
os.makedirs(out, exist_ok=True)

stems = sorted(f[:-4] for f in os.listdir(fixtures) if f.endswith(".mmd"))
lines = [
    "# Gallery",
    "",
    "Every fixture in `testdata/fixtures`, rendered from its reference frame in",
    "`testdata/golden`. `make gallery` rebuilds this page; `make golden-record`",
    "rebuilds it after re-recording the references. A fixture's first line may",
    "carry a `%% mmaid:` directive naming the flags it renders with; the default",
    "is `-t default -w 120`.",
    "",
]
for stem in stems:
    cells = os.path.join(dump, stem + ".cells")
    if not os.path.exists(cells):
        sys.exit(f"no frame for {stem} in {dump}")
    subprocess.run(
        [sys.executable, os.path.join(here, "cells2png.py"), cells, os.path.join(out, stem + ".png")],
        check=True,
    )
    with open(os.path.join(fixtures, stem + ".mmd")) as f:
        source = f.read().rstrip("\n")
    flags = "-t default -w 120"
    body = source
    first, _, rest = source.partition("\n")
    if first.startswith("%% mmaid:"):
        flags = first[len("%% mmaid:"):].strip()
        body = rest
    lines += [
        f"## {stem}",
        "",
        f"Rendered with `{flags}`.",
        "",
        "```mermaid",
        body,
        "```",
        "",
        f"![{stem}]({stem}.png)",
        "",
    ]
with open(os.path.join(out, "README.md"), "w") as f:
    f.write("\n".join(lines))
print(os.path.join(out, "README.md"))
