package glyph

import "testing"

func TestOfInvertsUnicode(t *testing.T) {
	for _, w := range []Weight{Light, Heavy, Double} {
		for a := Arms(1); a < 16; a++ {
			r := Unicode[w][a]
			got, gw, rounded, ok := Of(r)
			if !ok {
				t.Errorf("Of(%c) not ok", r)
				continue
			}
			if rounded {
				t.Errorf("Of(%c) reports rounded", r)
			}
			// Double has no half lines, so its single-arm entries are the
			// full line and invert to it.
			if w == Double && (a == N || a == S || a == E || a == W) {
				continue
			}
			if got != a || gw != w {
				t.Errorf("Of(%c) = %04b %d, want %04b %d", r, got, gw, a, w)
			}
		}
	}
}

func TestOfDashed(t *testing.T) {
	for _, c := range []struct {
		r rune
		a Arms
	}{{'┄', Horizontal}, {'┆', Vertical}} {
		a, w, _, ok := Of(c.r)
		if !ok || a != c.a || w != Dashed {
			t.Errorf("Of(%c) = %04b %d %v", c.r, a, w, ok)
		}
	}
	// Dashed borrows light's corners, which stay light.
	if _, w, _, _ := Of('┌'); w != Light {
		t.Errorf("Of(┌) weight = %d, want Light", w)
	}
}

func TestOfRounded(t *testing.T) {
	for i, r := range UnicodeRounded {
		a, w, rounded, ok := Of(r)
		if !ok || !rounded || w != Light || RoundedIndex(a) != i {
			t.Errorf("Of(%c) = %04b %d rounded=%v ok=%v", r, a, w, rounded, ok)
		}
	}
	for a := Arms(1); a < 16; a++ {
		i := RoundedIndex(a)
		isCorner := a == TopLeft || a == TopRight || a == BottomLeft || a == BottomRight
		if isCorner != (i >= 0) {
			t.Errorf("RoundedIndex(%04b) = %d", a, i)
		}
	}
}

func TestOfRejectsText(t *testing.T) {
	for _, r := range "A-|+ ►╱╲◇" {
		if _, _, _, ok := Of(r); ok {
			t.Errorf("Of(%c) ok", r)
		}
	}
}

func TestASCIIIsThreeRunes(t *testing.T) {
	for w, t2 := range ASCII {
		for a := Arms(1); a < 16; a++ {
			switch t2[a] {
			case '-', '|', '+':
			default:
				t.Errorf("ASCII[%d][%04b] = %c", w, a, t2[a])
			}
		}
	}
}

func TestHeavier(t *testing.T) {
	if !Heavier(Light, Dashed) || Heavier(Dashed, Light) {
		t.Error("dashed must yield to light")
	}
	if !Heavier(Heavy, Light) || !Heavier(Double, Heavy) {
		t.Error("heavy over light, double over heavy")
	}
	if Heavier(Light, Light) {
		t.Error("a weight is not heavier than itself")
	}
}

func TestTail(t *testing.T) {
	if a, ok := Tail('►'); !ok || a != W {
		t.Errorf("Tail(►) = %04b %v", a, ok)
	}
	if _, ok := Tail('>'); ok {
		t.Error("Tail reads Unicode only")
	}
	if ASCIITails['>'] != W {
		t.Error("ASCIITails['>'] != W")
	}
}
