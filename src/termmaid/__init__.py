"""termmaid - Render Mermaid diagram syntax as beautiful Unicode art in the terminal."""
from __future__ import annotations

from .graph.model import Graph
from .parser.flowchart import parse_flowchart
from .parser.statediagram import parse_state_diagram


__version__ = "0.1.0"


def parse(source: str) -> Graph:
    """Parse mermaid syntax and return a Graph model.

    Auto-detects diagram type (flowchart or state diagram).

    Args:
        source: Mermaid diagram source text

    Returns:
        Parsed Graph model
    """
    text = source.strip()
    if text.startswith("stateDiagram"):
        return parse_state_diagram(text)
    return parse_flowchart(text)


def render(
    source: str,
    *,
    use_ascii: bool = False,
    padding_x: int = 4,
    padding_y: int = 2,
    rounded_edges: bool = True,
) -> str:
    """Render mermaid syntax as Unicode (or ASCII) art.

    Args:
        source: Mermaid diagram source text
        use_ascii: Use ASCII characters instead of Unicode box-drawing
        padding_x: Horizontal padding inside node boxes
        padding_y: Vertical padding inside node boxes

    Returns:
        Rendered diagram as a string

    Example:
        >>> from termmaid import render
        >>> print(render("graph LR\\n  A --> B --> C"))
    """
    graph = parse(source)
    from .output.text import render_text
    return render_text(graph, use_ascii=use_ascii, padding_x=padding_x, padding_y=padding_y, rounded_edges=rounded_edges)


def render_rich(
    source: str,
    *,
    use_ascii: bool = False,
    padding_x: int = 4,
    padding_y: int = 2,
    rounded_edges: bool = True,
    theme: str = "default",
):
    """Render mermaid syntax as a Rich Text object with colors.

    Requires: pip install termmaid[rich]

    Args:
        source: Mermaid diagram source text
        use_ascii: Use ASCII characters instead of Unicode
        padding_x: Horizontal padding inside node boxes
        padding_y: Vertical padding inside node boxes
        theme: Color theme name (default, terra, neon, mono, amber, phosphor)

    Returns:
        rich.text.Text object
    """
    graph = parse(source)
    from .output.rich import render_rich as _render_rich
    return _render_rich(graph, use_ascii=use_ascii, padding_x=padding_x, padding_y=padding_y, rounded_edges=rounded_edges, theme=theme)


# Lazy import for MermaidWidget
def __getattr__(name: str):
    if name == "MermaidWidget":
        from .output.widget import _get_widget_class
        return _get_widget_class()
    raise AttributeError(f"module 'termmaid' has no attribute {name!r}")
