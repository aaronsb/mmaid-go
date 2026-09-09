"""Render a .cells dump to PNG using the Unscii 16 font.

Usage: cells2png.py IN.cells OUT.png
Font: $MMAID_SNAP_FONT, or /usr/share/fonts/OTF/unscii-16-full.otf.
Wide glyphs (a cell followed by a codepoint-0 continuation) use
$MMAID_SNAP_FONT_WIDE, or Noto Sans CJK when it is installed, drawn
across both cells.
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
wide_path = os.environ.get(
    "MMAID_SNAP_FONT_WIDE", "/usr/share/fonts/noto-cjk/NotoSansCJK-Regular.ttc"
)
wide_font = ImageFont.truetype(wide_path, 14) if os.path.exists(wide_path) else font
with open(src) as f:
    w, h = map(int, f.readline().split())
    cells = [f.readline().split() for _ in range(w * h)]
img = Image.new("RGB", (w * CW, h * CH), (0, 0, 0))
d = ImageDraw.Draw(img)
for i, c in enumerate(cells):
    _, _, _, _, br, bg, bb = map(int, c)
    x, y = (i % w) * CW, (i // w) * CH
    d.rectangle([x, y, x + CW - 1, y + CH - 1], fill=(br, bg, bb))
for i, c in enumerate(cells):
    cp, fr, fg, fb, _, _, _ = map(int, c)
    if cp in (0, 32):
        continue
    x, y = (i % w) * CW, (i // w) * CH
    wide = i + 1 < len(cells) and (i + 1) % w != 0 and cells[i + 1][0] == "0"
    if wide:
        l, t, r, b = wide_font.getbbox(chr(cp))
        d.text((x + (2 * CW - (r - l)) // 2 - l, y + (CH - (b - t)) // 2 - t),
               chr(cp), font=wide_font, fill=(fr, fg, fb))
    else:
        d.text((x, y), chr(cp), font=font, fill=(fr, fg, fb))
img.save(dst)
