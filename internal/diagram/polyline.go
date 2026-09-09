package diagram

import (
	"github.com/aaronsb/mmaid-go/internal/glyph"
	"github.com/aaronsb/mmaid-go/internal/renderer"
)

// A cell path is a run of cells each adjacent to the next. Charts that draw
// shapes no axis-aligned segment can express — a radar's rings and curves, a
// Wardley dependency — rasterise their vertices into one and hand it here.
//
// Two rasterisations. An orthogonal path steps one side at a time and every
// cell resolves from its arms, so an arc draws as a box-drawing staircase. A
// diagonal path steps corner to corner and its cells carry the charset's
// chamfers. `cellPath` picks per segment: a run within half an octant of the
// cell diagonal is drawn with chamfers, everything else orthogonally.
//
// Either way every arm meets an arm pointing back or a literal, which is what
// ADR-101's lint asks of one.

// orthoPath rasterises a straight run as cells that touch on a side only,
// stepping along the major axis and taking each minor-axis step in the
// column or row it leaves.
func orthoPath(r1, c1, r2, c2 int) [][2]int {
	dr, dc := r2-r1, c2-c1
	out := [][2]int{{r1, c1}}
	if abs(dc) >= abs(dr) {
		n := abs(dc)
		if n == 0 {
			return out
		}
		sc, prev := sign(dc), r1
		for i := 1; i <= n; i++ {
			r := r1 + roundDiv(dr*i, n)
			c := c1 + sc*i
			for sr := sign(r - prev); prev != r; prev += sr {
				out = append(out, [2]int{prev + sr, c - sc})
			}
			out = append(out, [2]int{r, c})
		}
		return out
	}
	n := abs(dr)
	sr, prev := sign(dr), c1
	for i := 1; i <= n; i++ {
		c := c1 + roundDiv(dc*i, n)
		r := r1 + sr*i
		for sc := sign(c - prev); prev != c; prev += sc {
			out = append(out, [2]int{r - sr, prev + sc})
		}
		out = append(out, [2]int{r, c})
	}
	return out
}

// diagPath rasterises a straight run as cells that may touch at a corner,
// Bresenham's.
func diagPath(r1, c1, r2, c2 int) [][2]int {
	dr, dc := abs(r2-r1), abs(c2-c1)
	sr, sc := sign(r2-r1), sign(c2-c1)
	err := dc - dr
	r, c := r1, c1
	out := make([][2]int, 0, max(dr, dc)+1)
	for {
		out = append(out, [2]int{r, c})
		if r == r2 && c == c2 {
			return out
		}
		e2 := 2 * err
		if e2 > -dr {
			err -= dr
			c += sc
		}
		if e2 < dc {
			err += dc
			r += sr
		}
	}
}

// nearDiagonal reports whether a run is close enough to the cell diagonal to
// read better as chamfers than as a staircase.
func nearDiagonal(dr, dc int) bool {
	dr, dc = abs(dr), abs(dc)
	return dr > 0 && dc > 0 && 4*dr >= 3*dc && 4*dc >= 3*dr
}

// cellPath joins consecutive vertices into one path, dropping the repeats a
// join leaves behind. A closed path returns to its first vertex. With
// diagonals false every segment is orthogonal, which suits a densely sampled
// curve whose segments are too short to have a direction of their own.
func cellPath(vertices [][2]int, closed, diagonals bool) [][2]int {
	if len(vertices) < 2 {
		return append([][2]int(nil), vertices...)
	}
	ends := vertices
	if closed {
		ends = append(append([][2]int(nil), vertices...), vertices[0])
	}
	var out [][2]int
	for i := 0; i+1 < len(ends); i++ {
		a, b := ends[i], ends[i+1]
		run := orthoPath(a[0], a[1], b[0], b[1])
		if diagonals && nearDiagonal(b[0]-a[0], b[1]-a[1]) {
			run = diagPath(a[0], a[1], b[0], b[1])
		}
		for _, p := range run {
			if len(out) > 0 && out[len(out)-1] == p {
				continue
			}
			out = append(out, p)
		}
	}
	if closed && len(out) > 1 && out[0] == out[len(out)-1] {
		out = out[:len(out)-1]
	}
	return out
}

// armToward returns the arm bit pointing from a at b when the two are
// orthogonally adjacent, and zero otherwise.
func armToward(a, b [2]int) glyph.Arms {
	switch {
	case a[0] == b[0] && b[1] == a[1]+1:
		return glyph.E
	case a[0] == b[0] && b[1] == a[1]-1:
		return glyph.W
	case a[1] == b[1] && b[0] == a[0]+1:
		return glyph.S
	case a[1] == b[1] && b[0] == a[0]-1:
		return glyph.N
	}
	return 0
}

// pathPen carries the literals a path falls back on where arms cannot draw it.
type pathPen struct {
	slash     rune
	backslash rune
	dot       rune
	weight    glyph.Weight
	rounded   bool
	style     string
}

// drawCellPath draws a path, arming the cells whose neighbours both lie on a
// side and drawing a chamfer wherever one lies on a corner. A closed path
// treats its two ends as neighbours.
func drawCellPath(c *renderer.Canvas, path [][2]int, closed bool, pen pathPen) {
	n := len(path)
	if n == 0 {
		return
	}
	if n == 1 {
		c.Put(path[0][0], path[0][1], pen.dot, pen.style)
		return
	}
	for i, p := range path {
		prev, hasPrev := path[(i-1+n)%n], closed || i > 0
		next, hasNext := path[(i+1)%n], closed || i < n-1
		var arms glyph.Arms
		var corner bool
		if hasPrev {
			if a := armToward(p, prev); a != 0 {
				arms |= a
			} else {
				corner = true
			}
		}
		if hasNext {
			if a := armToward(p, next); a != 0 {
				arms |= a
			} else {
				corner = true
			}
		}
		if !corner && arms != 0 {
			c.Arm(p[0], p[1], arms, pen.weight, pen.rounded, pen.style)
			c.SetStyle(p[0], p[1], pen.style)
			continue
		}
		from, to := p, p
		if hasPrev {
			from = prev
		}
		if hasNext {
			to = next
		}
		ch := pen.dot
		switch dr, dc := to[0]-from[0], to[1]-from[1]; {
		case dr*dc < 0:
			ch = pen.slash
		case dr*dc > 0:
			ch = pen.backslash
		}
		c.Put(p[0], p[1], ch, pen.style)
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func sign(v int) int {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	}
	return 0
}

// roundDiv divides rounding half away from zero.
func roundDiv(a, b int) int {
	if b == 0 {
		return 0
	}
	if (a < 0) != (b < 0) {
		return -((abs(a)*2 + abs(b)) / (2 * abs(b)))
	}
	return (abs(a)*2 + abs(b)) / (2 * abs(b))
}
