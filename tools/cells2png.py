"""Render a .cells dump to PNG using the Unscii 16 font.

Usage: cells2png.py IN.cells OUT.png
Font: $MMAID_SNAP_FONT, or /usr/share/fonts/OTF/unscii-16-full.otf.
"""
import os
import sys
from PIL import Image, ImageDraw, ImageFont

if len(sys.argv) != 3:
    sys.exit("usage: cells2png.py IN.cells OUT.png")
src, dst = sys.argv[1], sys.argv[2]
CW, CH = 8, 16
font = ImageFont.truetype(
    os.environ.get("MMAID_SNAP_FONT", "/usr/share/fonts/OTF/unscii-16-full.otf"), 16
)
with open(src) as f:
    w, h = map(int, f.readline().split())
    cells = [f.readline().split() for _ in range(w * h)]
img = Image.new("RGB", (w * CW, h * CH), (0, 0, 0))
d = ImageDraw.Draw(img)
for i, c in enumerate(cells):
    cp, fr, fg, fb, br, bg, bb = map(int, c)
    x, y = (i % w) * CW, (i // w) * CH
    d.rectangle([x, y, x + CW - 1, y + CH - 1], fill=(br, bg, bb))
    if cp not in (0, 32):
        d.text((x, y), chr(cp), font=font, fill=(fr, fg, fb))
img.save(dst)
