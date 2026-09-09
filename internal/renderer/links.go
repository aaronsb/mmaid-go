package renderer

import (
	"sort"
	"strings"
	"sync"
)

// OSC 8 wraps a run of cells in a hyperlink. The ADR-101 interpreter skips OSC
// sequences, so a frame is the same with links on or off.
const (
	oscClose = "\033]8;;\033\\"
)

func oscOpen(url string) string { return "\033]8;;" + url + "\033\\" }

var links struct {
	mu sync.RWMutex
	// urls maps one line of a node's label to the node's URL.
	urls map[string]string
}

// SetLinks registers the node labels that carry a `click` URL. Serialization
// wraps each label it finds in the grid in OSC 8. Passing nil turns hyperlinks
// off again.
//
// The map is keyed by label because the canvas records a style per cell and not
// which node drew it, and the placements that would give the answer live inside
// the draw pass.
func SetLinks(labelToURL map[string]string) {
	links.mu.Lock()
	defer links.mu.Unlock()
	if len(labelToURL) == 0 {
		links.urls = nil
		return
	}
	m := make(map[string]string, len(labelToURL))
	for label, url := range labelToURL {
		if url == "" {
			continue
		}
		for _, line := range labelLines(label) {
			if line = strings.TrimSpace(line); line != "" {
				m[line] = url
			}
		}
	}
	links.urls = m
}

// labelLines splits a label the way drawLabel does.
func labelLines(label string) []string {
	switch {
	case strings.Contains(label, "\n"):
		return strings.Split(label, "\n")
	case strings.Contains(label, `\n`):
		return strings.Split(label, `\n`)
	}
	return []string{label}
}

// linkSpans locates every registered label in the canvas and returns, per cell,
// the URL to open before it and whether to close after it.
func linkSpans(c *Canvas) (open map[[2]int]string, closed map[[2]int]bool) {
	links.mu.RLock()
	urls := links.urls
	links.mu.RUnlock()
	if len(urls) == 0 {
		return nil, nil
	}

	// Longest label first, so a label that contains another wins the cells.
	labels := make([]string, 0, len(urls))
	for label := range urls {
		labels = append(labels, label)
	}
	sort.Slice(labels, func(i, j int) bool {
		if len(labels[i]) != len(labels[j]) {
			return len(labels[i]) > len(labels[j])
		}
		return labels[i] < labels[j]
	})

	open = make(map[[2]int]string)
	closed = make(map[[2]int]bool)
	taken := make(map[[2]int]bool)

	for y := range c.Height {
		text, columns := rowText(c, y)
		if text == "" {
			continue
		}
		for _, label := range labels {
			for at := 0; ; {
				i := strings.Index(text[at:], label)
				if i < 0 {
					break
				}
				i += at
				at = i + len(label)                 // bytes, not columns
				if !isolated(text, i, len(label)) { // bytes, not columns
					continue
				}
				start := columns[i]
				end := columns[at-1]
				if taken[[2]int{y, start}] || taken[[2]int{y, end}] {
					continue
				}
				for x := start; x <= end; x++ {
					taken[[2]int{y, x}] = true
				}
				open[[2]int{y, start}] = urls[label]
				closed[[2]int{y, end}] = true
			}
		}
	}
	if len(open) == 0 {
		return nil, nil
	}
	return open, closed
}

// rowText returns a row's runes as a string along with the column each byte
// belongs to, skipping the continuation cells of wide runes.
func rowText(c *Canvas, y int) (string, []int) {
	var b strings.Builder
	columns := make([]int, 0, c.Width)
	for x := range c.Width {
		ch := c.grid[y][x]
		if ch == Continuation {
			continue
		}
		n := b.Len()
		b.WriteRune(ch)
		for range b.Len() - n {
			columns = append(columns, x)
		}
	}
	return b.String(), columns
}

// isolated reports whether the match at i is bounded by something other than
// label text, so that a short label does not link inside a longer word.
func isolated(row string, i, n int) bool {
	edge := func(b byte) bool {
		return !(b >= '0' && b <= '9' || b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z')
	}
	if i > 0 && !edge(row[i-1]) {
		return false
	}
	if i+n < len(row) && !edge(row[i+n]) {
		return false
	}
	return true
}
