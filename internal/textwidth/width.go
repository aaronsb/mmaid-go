// Package textwidth measures strings in terminal columns.
//
// The range tables in tables.go are generated from the Unicode character
// database. Download the two inputs from
// https://www.unicode.org/Public/16.0.0/ucd/EastAsianWidth.txt and
// https://www.unicode.org/Public/16.0.0/ucd/emoji/emoji-data.txt, then set
// UCD_DIR to the directory holding them and run go generate.
package textwidth

//go:generate go run ../../tools/gen-widths $UCD_DIR/EastAsianWidth.txt $UCD_DIR/emoji-data.txt

import (
	"os"
	"sync"
	"unicode"
)

var ambigCfg struct {
	mu     sync.RWMutex
	loaded bool
	wide   bool
}

// AmbiguousWide reports whether East Asian Ambiguous characters count as two
// columns. It reads MMAID_AMBIGUOUS_WIDE once, unless SetAmbiguousWide has
// already set it.
func AmbiguousWide() bool {
	ambigCfg.mu.RLock()
	if ambigCfg.loaded {
		defer ambigCfg.mu.RUnlock()
		return ambigCfg.wide
	}
	ambigCfg.mu.RUnlock()

	ambigCfg.mu.Lock()
	defer ambigCfg.mu.Unlock()
	if !ambigCfg.loaded {
		ambigCfg.wide = os.Getenv("MMAID_AMBIGUOUS_WIDE") == "1"
		ambigCfg.loaded = true
	}
	return ambigCfg.wide
}

// SetAmbiguousWide overrides the ambiguous-width setting.
func SetAmbiguousWide(wide bool) {
	ambigCfg.mu.Lock()
	ambigCfg.wide = wide
	ambigCfg.loaded = true
	ambigCfg.mu.Unlock()
}

// Rune returns the number of terminal columns a rune occupies.
func Rune(r rune) int {
	if r == 0 {
		return 0
	}
	if r < 0x20 || (r >= 0x7F && r < 0xA0) {
		return 0
	}
	if r < 0xA0 {
		return 1
	}
	if inTable(zero, r) || unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) {
		return 0
	}
	if inTable(wide, r) {
		return 2
	}
	if AmbiguousWide() && inTable(ambiguous, r) {
		return 2
	}
	return 1
}

// String returns the number of terminal columns a string occupies.
func String(s string) int {
	w := 0
	for _, r := range s {
		w += Rune(r)
	}
	return w
}

// Truncate cuts s to at most max columns, never splitting a wide rune.
func Truncate(s string, max int) string {
	w := 0
	for i, r := range s {
		rw := Rune(r)
		if w+rw > max {
			return s[:i]
		}
		w += rw
	}
	return s
}

// inTable reports whether r falls in one of the sorted, non-overlapping ranges.
func inTable(table [][2]rune, r rune) bool {
	lo, hi := 0, len(table)-1
	for lo <= hi {
		mid := (lo + hi) / 2
		switch {
		case r < table[mid][0]:
			hi = mid - 1
		case r > table[mid][1]:
			lo = mid + 1
		default:
			return true
		}
	}
	return false
}
