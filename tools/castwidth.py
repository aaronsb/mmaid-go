"""Report the widest line an asciinema cast draws, against its declared columns.

Usage: castwidth.py demo/mmaid-demo.cast

Reads a v2 or v3 cast, replays every output event through a cursor model
that skips escape sequences and counts display columns, and prints the
maximum column reached, the declared width, and the first ten lines that
exceed it. Exit status 1 when any line overflows.
"""
import json
import re
import sys
import unicodedata

ESC = re.compile(r"\x1b(?:\[[0-?]*[ -/]*[@-~]|\][^\x07\x1b]*(?:\x07|\x1b\\)|[@-Z\\-_])")


def width(ch):
    if unicodedata.combining(ch):
        return 0
    return 2 if unicodedata.east_asian_width(ch) in ("W", "F") else 1


def main(path):
    with open(path) as f:
        header = json.loads(f.readline())
        cols = header.get("width") or header.get("term", {}).get("cols")
        events = [json.loads(line) for line in f if line.strip()]
    col = 0
    line = []
    widest = 0
    overflow = []
    text = "".join(ev[2] for ev in events if len(ev) >= 3 and ev[1] == "o")
    for ch in ESC.sub("", text):
        if ch in "\n\r":
            widest = max(widest, col)
            if col > cols and len(overflow) < 10:
                overflow.append((col, "".join(line)[:60]))
            col, line = 0, []
        elif ch >= " ":
            col += width(ch)
            line.append(ch)
    widest = max(widest, col)
    print(f"declared {cols} columns, widest line {widest}")
    for w, sample in overflow:
        print(f"  {w} cols  {sample!r}")
    return 1 if widest > cols else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1]))
