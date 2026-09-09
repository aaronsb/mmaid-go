package glyph

// Family is a group of runes a font tends to cover or miss together. A
// profile marks a family failed and the renderer draws its roles from the
// family's fallback instead (ADR-500).
type Family string

const (
	BoxLight    Family = "box-light"
	BoxRounded  Family = "box-rounded"
	BoxHeavy    Family = "box-heavy"
	BoxDouble   Family = "box-double"
	Diagonals   Family = "diagonals"
	Arrows      Family = "arrows"
	Blocks      Family = "blocks"
	Braille     Family = "braille"
	Sextants    Family = "sextants"
	Octants     Family = "octants"
	LegacyFills Family = "legacy-fills"
	ASCIIFamily Family = "ascii"
)

// Families lists every family in the order the ADR's table and the sample
// sheet use.
var Families = []Family{
	BoxLight, BoxRounded, BoxHeavy, BoxDouble,
	Diagonals, Arrows, Blocks, Braille,
	Sextants, Octants, LegacyFills, ASCIIFamily,
}

// Fallback is the family a failed family degrades to. ASCII has none.
var Fallback = map[Family]Family{
	BoxLight:    ASCIIFamily,
	BoxRounded:  BoxLight,
	BoxHeavy:    BoxLight,
	BoxDouble:   BoxLight,
	Diagonals:   ASCIIFamily,
	Arrows:      ASCIIFamily,
	Blocks:      ASCIIFamily,
	Braille:     Blocks,
	Sextants:    Blocks,
	Octants:     Sextants,
	LegacyFills: Sextants,
}

// ParseFamily returns the family a profile names.
func ParseFamily(s string) (Family, bool) {
	for _, f := range Families {
		if string(f) == s {
			return f, true
		}
	}
	return "", false
}

// ParseFamilies converts a profile's failed list, returning the names it did
// not recognise so the caller can warn.
func ParseFamilies(names []string) (families []Family, unknown []string) {
	for _, n := range names {
		if f, ok := ParseFamily(n); ok {
			families = append(families, f)
		} else {
			unknown = append(unknown, n)
		}
	}
	return families, unknown
}

// Set binds one family to each role group the renderer draws from. Each
// role is named by the family that serves it natively; a set may bind
// another family there, and Resolve rebinds a failed one to its fallback.
type Set struct {
	Name string

	Light    Family // the light stroke's table
	Heavy    Family // the heavy stroke's table
	Double   Family // the double stroke's table
	Dashed   Family // the dashed stroke's table: box-light's dashed strokes, or ascii
	Corners  Family // the rounded-corner override
	Chamfers Family // diamond and hexagon chamfers and their indicators
	Arrows   Family // arrowheads, endpoints, circle markers
	Fills    Family // bars, shades, and the pie's half-cell circle
	Dots     Family // the pie's braille circle
	Sextants Family
	Octants  Family
	Legacy   Family
}

// Binding returns the family the set draws a role from, the role being named
// by the family that serves it natively.
func (s Set) Binding(role Family) Family {
	if p := s.role(role); p != nil {
		return *p
	}
	return role
}

// role maps a family to the field of the role it natively serves.
func (s *Set) role(f Family) *Family {
	switch f {
	case BoxLight:
		return &s.Light
	case BoxRounded:
		return &s.Corners
	case BoxHeavy:
		return &s.Heavy
	case BoxDouble:
		return &s.Double
	case Diagonals:
		return &s.Chamfers
	case Arrows:
		return &s.Arrows
	case Blocks:
		return &s.Fills
	case Braille:
		return &s.Dots
	case Sextants:
		return &s.Sextants
	case Octants:
		return &s.Octants
	case LegacyFills:
		return &s.Legacy
	}
	return nil
}

// unicodeSet is the default: every role drawn by its native family.
func unicodeSet(name string) Set {
	return Set{
		Name:     name,
		Light:    BoxLight,
		Heavy:    BoxHeavy,
		Double:   BoxDouble,
		Dashed:   BoxLight,
		Corners:  BoxRounded,
		Chamfers: Diagonals,
		Arrows:   Arrows,
		Fills:    Blocks,
		Dots:     Braille,
		Sextants: Sextants,
		Octants:  Octants,
		Legacy:   LegacyFills,
	}
}

// Sets holds the built-in glyph sets by name.
var Sets = map[string]Set{
	"unicode": unicodeSet("unicode"),
	"rounded": func() Set {
		s := unicodeSet("rounded")
		s.Light = BoxRounded
		return s
	}(),
	"heavy": func() Set {
		s := unicodeSet("heavy")
		s.Light, s.Corners = BoxHeavy, BoxHeavy
		return s
	}(),
	"double": func() Set {
		s := unicodeSet("double")
		s.Light, s.Corners = BoxDouble, BoxDouble
		return s
	}(),
	"legacy": func() Set {
		s := unicodeSet("legacy")
		s.Fills = LegacyFills
		return s
	}(),
	"ascii": {
		Name:  "ascii",
		Light: ASCIIFamily, Heavy: ASCIIFamily, Double: ASCIIFamily, Dashed: ASCIIFamily,
		Corners: ASCIIFamily, Chamfers: ASCIIFamily, Arrows: ASCIIFamily,
		Fills: ASCIIFamily, Dots: ASCIIFamily,
		Sextants: ASCIIFamily, Octants: ASCIIFamily, Legacy: ASCIIFamily,
	},
}

// SetNames lists the built-in sets in the order the ADR names them.
var SetNames = []string{"unicode", "rounded", "heavy", "double", "legacy", "ascii"}

// LookupSet returns the built-in set of that name.
func LookupSet(name string) (Set, bool) {
	s, ok := Sets[name]
	return s, ok
}

// DefaultSet is the set used when none is named.
func DefaultSet() Set { return Sets["unicode"] }

// Resolve rebinds every role whose family failed to the first family down
// its fallback chain that did not. A family that was not marked failed stays
// where it is, whatever happened to its fallback. Nothing else in the set
// changes.
func Resolve(set Set, failed []Family) Set {
	bad := make(map[Family]bool, len(failed))
	for _, f := range failed {
		bad[f] = true
	}
	delete(bad, ASCIIFamily)
	resolve := func(f Family) Family {
		if !bad[f] {
			return f
		}
		for cur := f; ; {
			next, ok := Fallback[cur]
			if !ok || !bad[next] {
				return next
			}
			cur = next
		}
	}
	for _, role := range Families {
		if p := set.role(role); p != nil {
			*p = resolve(*p)
		}
	}
	set.Dashed = resolve(set.Dashed)
	return set
}
