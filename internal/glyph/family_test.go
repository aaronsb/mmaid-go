package glyph

import "testing"

func TestEveryBuiltInSetResolves(t *testing.T) {
	known := map[Family]bool{}
	for _, f := range Families {
		known[f] = true
	}
	for _, name := range SetNames {
		set, ok := LookupSet(name)
		if !ok {
			t.Fatalf("no built-in set %q", name)
		}
		if set.Name != name {
			t.Errorf("%s: Name is %q", name, set.Name)
		}
		got := Resolve(set, nil)
		if got != set {
			t.Errorf("%s: resolving with nothing failed changed the set", name)
		}
		for _, role := range Families {
			if b := set.Binding(role); !known[b] {
				t.Errorf("%s: role %s bound to unknown family %q", name, role, b)
			}
		}
	}
}

func TestResolveFailedHeavyLeavesLightAlone(t *testing.T) {
	got := Resolve(DefaultSet(), []Family{BoxHeavy})
	if got.Light != BoxLight {
		t.Errorf("Light became %s", got.Light)
	}
	if got.Heavy != BoxLight {
		t.Errorf("Heavy is %s, want box-light", got.Heavy)
	}
	want := DefaultSet()
	want.Heavy = BoxLight
	if got != want {
		t.Errorf("more than the heavy role changed:\n got %+v\nwant %+v", got, want)
	}
}

func TestResolveFailedBlocksCascades(t *testing.T) {
	got := Resolve(DefaultSet(), []Family{Blocks})
	for _, role := range []Family{Blocks, Braille, Sextants, Octants, LegacyFills} {
		if b := got.Binding(role); b != ASCIIFamily {
			t.Errorf("%s resolved to %s, want ascii", role, b)
		}
	}
	if got.Light != BoxLight || got.Arrows != Arrows {
		t.Error("a failed blocks family touched the box or arrow roles")
	}
}

func TestResolveWalksTheChainPastAFailedFallback(t *testing.T) {
	got := Resolve(DefaultSet(), []Family{Octants, Sextants})
	if got.Octants != Blocks {
		t.Errorf("octants resolved to %s, want blocks", got.Octants)
	}
}

func TestResolveFailedLightDropsDashedToASCII(t *testing.T) {
	got := Resolve(DefaultSet(), []Family{BoxLight})
	if got.Light != ASCIIFamily || got.Dashed != ASCIIFamily {
		t.Errorf("light %s dashed %s, want ascii for both", got.Light, got.Dashed)
	}
	if got.Corners != ASCIIFamily {
		t.Errorf("rounded corners fall through their failed fallback, got %s", got.Corners)
	}
	if got.Heavy != ASCIIFamily || got.Double != ASCIIFamily {
		t.Error("the box families all pass through box-light and fail with it")
	}
	if got.Arrows != Arrows || got.Fills != Blocks {
		t.Error("arrows or fills changed")
	}
}

func TestParseFamiliesReportsUnknownNames(t *testing.T) {
	families, unknown := ParseFamilies([]string{"box-heavy", "bogus", "braille"})
	if len(families) != 2 || families[0] != BoxHeavy || families[1] != Braille {
		t.Errorf("families = %v", families)
	}
	if len(unknown) != 1 || unknown[0] != "bogus" {
		t.Errorf("unknown = %v", unknown)
	}
}

func TestFallbackChainsEndAtASCII(t *testing.T) {
	for _, f := range Families {
		seen := map[Family]bool{}
		for cur := f; cur != ASCIIFamily; {
			if seen[cur] {
				t.Fatalf("%s: fallback chain loops at %s", f, cur)
			}
			seen[cur] = true
			next, ok := Fallback[cur]
			if !ok {
				t.Fatalf("%s: chain ends at %s, not ascii", f, cur)
			}
			cur = next
		}
	}
}
