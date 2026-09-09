package textwidth

import "testing"

func TestRuneWidths(t *testing.T) {
	cases := []struct {
		r    rune
		want int
		name string
	}{
		{'日', 2, "CJK ideograph"},
		{'🚀', 2, "emoji presentation"},
		{'a', 1, "ASCII"},
		{'─', 1, "box horizontal"},
		{'│', 1, "box vertical"},
		{'►', 1, "arrow head"},
		{'◇', 1, "diamond"},
		{'⎔', 1, "software function symbol"},
		{'╱', 1, "diagonal"},
		{0x1FB00, 1, "sextant"},
		{0x0301, 0, "combining acute"},
		{0x200D, 0, "zero width joiner"},
		{0x200C, 0, "zero width non-joiner"},
		{0xE0020, 0, "tag space"},
		{0x1161, 0, "hangul jungseong A"},
		{0x11A8, 0, "hangul jongseong kiyeok"},
	}
	for _, c := range cases {
		if got := Rune(c.r); got != c.want {
			t.Errorf("Rune(%U) [%s] = %d, want %d", c.r, c.name, got, c.want)
		}
	}
}

func TestStringWidth(t *testing.T) {
	cases := []struct {
		s    string
		want int
	}{
		{"日本語テキスト", 14},
		{"emoji 🚀 ok", 11},
		{"각", 2}, // 각 decomposed into conjoining jamo
		{"", 0},
		{"abc", 3},
	}
	for _, c := range cases {
		if got := String(c.s); got != c.want {
			t.Errorf("String(%q) = %d, want %d", c.s, got, c.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	cases := []struct {
		s    string
		max  int
		want string
	}{
		{"日本語", 3, "日"},
		{"日本語", 4, "日本"},
		{"日本語", 0, ""},
		{"abcd", 3, "abc"},
		{"abcd", 9, "abcd"},
	}
	for _, c := range cases {
		if got := Truncate(c.s, c.max); got != c.want {
			t.Errorf("Truncate(%q, %d) = %q, want %q", c.s, c.max, got, c.want)
		}
	}
}

func TestControlCharactersAreZero(t *testing.T) {
	for _, r := range []rune{0x00, 0x07, 0x1B, 0x7F, 0x9F} {
		if got := Rune(r); got != 0 {
			t.Errorf("Rune(%U) = %d, want 0", r, got)
		}
	}
}

func TestAmbiguousWideOverride(t *testing.T) {
	before := AmbiguousWide()
	t.Cleanup(func() { SetAmbiguousWide(before) })

	const r = '±' // East Asian Ambiguous
	SetAmbiguousWide(false)
	if got := Rune(r); got != 1 {
		t.Errorf("Rune(%U) with narrow ambiguous = %d, want 1", r, got)
	}
	SetAmbiguousWide(true)
	if got := Rune(r); got != 2 {
		t.Errorf("Rune(%U) with wide ambiguous = %d, want 2", r, got)
	}
}

func TestTablesSortedAndDisjoint(t *testing.T) {
	tables := map[string][][2]rune{"wide": wide, "ambiguous": ambiguous, "zero": zero}
	for name, table := range tables {
		if len(table) == 0 {
			t.Errorf("%s is empty", name)
			continue
		}
		for i, r := range table {
			if r[0] > r[1] {
				t.Errorf("%s[%d] = %U..%U is inverted", name, i, r[0], r[1])
			}
			if i > 0 && r[0] <= table[i-1][1] {
				t.Errorf("%s[%d] = %U..%U overlaps or misorders %U..%U",
					name, i, r[0], r[1], table[i-1][0], table[i-1][1])
			}
		}
	}
}
