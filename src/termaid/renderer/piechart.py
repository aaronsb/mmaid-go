"""Renderer for pie chart diagrams as horizontal bar charts.

Draws a horizontal stacked bar and per-slice bars with labels,
percentages, and optional raw values.
"""
from __future__ import annotations

from ..model.piechart import PieChart
from .canvas import Canvas
from .charset import ASCII, UNICODE, CharSet

_FILL_CHARS = ["█", "▓", "░", "▒", "▞", "▚", "▖", "▗"]
_FILL_CHARS_ASCII = ["#", "*", "+", "~", ":", ".", "o", "="]

_BAR_WIDTH = 40
_MARGIN = 2


def render_pie_chart(
    diagram: PieChart,
    *,
    use_ascii: bool = False,
) -> Canvas:
    """Render a PieChart as a horizontal bar chart on a Canvas."""
    cs = ASCII if use_ascii else UNICODE

    if not diagram.slices:
        canvas = Canvas(1, 1)
        return canvas

    total = sum(s.value for s in diagram.slices)
    fills = _FILL_CHARS_ASCII if use_ascii else _FILL_CHARS

    # Compute label column width
    max_label_len = max(len(s.label) for s in diagram.slices)
    label_col_w = max_label_len + _MARGIN

    # Compute suffix (percentage + optional value)
    suffixes: list[str] = []
    for s in diagram.slices:
        pct = s.value / total * 100
        if diagram.show_data:
            suffixes.append(f" {pct:5.1f}%  [{s.value:g}]")
        else:
            suffixes.append(f" {pct:5.1f}%")
    max_suffix_len = max(len(sf) for sf in suffixes)

    bar_left = label_col_w
    canvas_w = bar_left + _BAR_WIDTH + max_suffix_len + _MARGIN
    title_rows = 2 if diagram.title else 0

    # Stacked bar: 3 rows (border, bar, border)
    # Per-slice bars: 1 row each
    # Spacing
    stacked_top = _MARGIN + title_rows
    detail_top = stacked_top + 4  # border + bar + border + blank
    canvas_h = detail_top + len(diagram.slices) + _MARGIN

    canvas = Canvas(canvas_w, canvas_h)

    # Title
    if diagram.title:
        title_col = max(0, (canvas_w - len(diagram.title)) // 2)
        canvas.put_text(_MARGIN, title_col, diagram.title, style="label")

    # ── Stacked overview bar ──────────────────────────────────────────────
    # Top border
    canvas.put(stacked_top, bar_left, cs.top_left, style="edge")
    for c in range(1, _BAR_WIDTH + 1):
        canvas.put(stacked_top, bar_left + c, cs.horizontal, style="edge")
    canvas.put(stacked_top, bar_left + _BAR_WIDTH + 1, cs.top_right, style="edge")

    # Bar content
    bar_row = stacked_top + 1
    canvas.put(bar_row, bar_left, cs.vertical, style="edge")
    col = bar_left + 1
    for i, s in enumerate(diagram.slices):
        width = max(1, round(s.value / total * _BAR_WIDTH))
        # Clamp so we don't overflow
        width = min(width, bar_left + _BAR_WIDTH + 1 - col)
        if width <= 0:
            continue
        fill = fills[i % len(fills)]
        for c in range(width):
            canvas.put(bar_row, col + c, fill, merge=False, style="node")
        col += width
    canvas.put(bar_row, bar_left + _BAR_WIDTH + 1, cs.vertical, style="edge")

    # Bottom border
    canvas.put(stacked_top + 2, bar_left, cs.bottom_left, style="edge")
    for c in range(1, _BAR_WIDTH + 1):
        canvas.put(stacked_top + 2, bar_left + c, cs.horizontal, style="edge")
    canvas.put(stacked_top + 2, bar_left + _BAR_WIDTH + 1, cs.bottom_right, style="edge")

    # ── Per-slice detail bars ─────────────────────────────────────────────
    for i, s in enumerate(diagram.slices):
        row = detail_top + i
        pct = s.value / total * 100
        fill = fills[i % len(fills)]
        bar_len = max(1, round(s.value / total * _BAR_WIDTH))

        # Label (right-aligned)
        label_text = s.label.rjust(max_label_len)
        canvas.put_text(row, _MARGIN, label_text, style="label")

        # Bar
        if use_ascii:
            canvas.put(row, bar_left, "|", merge=False, style="edge")
        else:
            canvas.put(row, bar_left, "┃", merge=False, style="edge")
        for c in range(bar_len):
            canvas.put(row, bar_left + 1 + c, fill, merge=False, style="node")

        # Suffix
        canvas.put_text(row, bar_left + 1 + bar_len, suffixes[i], style="label")

    return canvas
