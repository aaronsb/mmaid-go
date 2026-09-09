package renderer

import "strings"

// OSC 8 wraps a run of cells in a hyperlink. The ADR-101 interpreter skips OSC
// sequences, so a frame is the same with links on or off.
const oscClose = "\033]8;;\033\\"

// oscOpen returns the opening half of a hyperlink, or "" for a URL that would
// end the sequence early and let the rest of it reach the terminal as commands.
func oscOpen(url string) string {
	if url == "" || strings.ContainsAny(url, "\033\a") {
		return ""
	}
	return "\033]8;;" + url + "\033\\"
}

// writeLink emits the OSC 8 transitions for the cell at (y, x): it closes the
// link that was open when the cell belongs to another one or to none, and opens
// the cell's own. It takes the URL currently open and returns the new one.
func (c *Canvas) writeLink(b *strings.Builder, y, x int, open string) string {
	if !c.hyperlinks {
		return ""
	}
	url := c.linkGrid[y][x]
	if url == open {
		return open
	}
	if open != "" {
		b.WriteString(oscClose)
	}
	if seq := oscOpen(url); seq != "" {
		b.WriteString(seq)
		return url
	}
	return ""
}
