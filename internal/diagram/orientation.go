package diagram

import (
	"regexp"
	"strings"
)

// ── Orientation Control ──────────────────────────────────────────────────────
//
// Orientation is authorial intent ("lay this out vertically"), distinct from
// terminal width, which is environmental ("the terminal is this wide"). They
// are resolved independently.
//
// Two inputs, in precedence order:
//  1. CLI override (--orientation), set via SetOrientationOverride. Wins.
//  2. An in-diagram `direction TB|LR|...` directive line (Mermaid already uses
//     `direction` for flowcharts/state; here it is a port-wide convention
//     usable by every diagram type).
//
// If neither is present, each renderer falls back to its own natural default.
//
// This deliberately does NOT consider terminal width: a width-triggered
// auto-switch conflates the two axes and cannot fire reliably when terminal
// detection is unavailable (e.g. inside some embedded shells).

// orientationOverride is the CLI-forced orientation: "" (none), "v", or "h".
var orientationOverride string

var reDirectionLine = regexp.MustCompile(`(?im)^\s*direction\s+(TB|TD|BT|LR|RL)\s*$`)

// SetOrientationOverride forces orientation from the CLI, overriding any
// in-diagram directive. Accepts Mermaid direction tokens (TB/TD/BT/LR/RL,
// case-insensitive). An empty or unrecognized value clears the override.
func SetOrientationOverride(token string) {
	orientationOverride = directionToken(token)
}

// directionToken maps a Mermaid direction token to "v" (vertical), "h"
// (horizontal), or "" (unrecognized).
func directionToken(token string) string {
	switch strings.ToUpper(strings.TrimSpace(token)) {
	case "TB", "TD", "BT":
		return "v"
	case "LR", "RL":
		return "h"
	default:
		return ""
	}
}

// resolveVertical reports whether a diagram should render vertically.
// Precedence: CLI override > in-diagram `direction` directive > defaultVertical.
func resolveVertical(source string, defaultVertical bool) bool {
	if orientationOverride != "" {
		return orientationOverride == "v"
	}
	if m := reDirectionLine.FindStringSubmatch(source); m != nil {
		if v := directionToken(m[1]); v != "" {
			return v == "v"
		}
	}
	return defaultVertical
}
