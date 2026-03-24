// Package ingest converts structured data (JSON) into Mermaid diagram syntax.
//
// Each ingest mode defines an opinionated mapping from a common JSON shape
// to a specific Mermaid diagram type. Modes are registered in the modes map.
package ingest

import "fmt"

// Config holds field-name overrides for JSON-to-Mermaid conversion.
type Config struct {
	NameKey     string // JSON field for node labels (default: "name")
	ValueKey    string // JSON field for leaf weights (default: "size")
	ChildrenKey string // JSON field for child arrays (default: "children")
}

// DefaultConfig returns a Config with default field names.
func DefaultConfig() Config {
	return Config{
		NameKey:     "name",
		ValueKey:    "size",
		ChildrenKey: "children",
	}
}

// converter transforms JSON bytes into Mermaid source text.
type converter func(data []byte, cfg Config) (string, error)

// templater returns a minimum valid JSON example for a mode.
type templater func(cfg Config) string

type mode struct {
	convert  converter
	template templater
}

var modes = map[string]mode{
	"treemap": {convert: convertTreemap, template: templateTreemap},
	"pie":     {convert: convertPie, template: templatePie},
}

// Modes returns the list of supported ingest mode names.
func Modes() []string {
	out := make([]string, 0, len(modes))
	for k := range modes {
		out = append(out, k)
	}
	return out
}

// Convert reads JSON bytes and returns Mermaid syntax for the given mode.
func Convert(modeName string, data []byte, cfg Config) (string, error) {
	m, ok := modes[modeName]
	if !ok {
		return "", fmt.Errorf("unknown ingest mode %q (available: %s)", modeName, modeList())
	}
	return m.convert(data, cfg)
}

// Template returns a minimum valid JSON example for the given mode.
func Template(modeName string, cfg Config) (string, error) {
	m, ok := modes[modeName]
	if !ok {
		return "", fmt.Errorf("unknown ingest mode %q (available: %s)", modeName, modeList())
	}
	return m.template(cfg), nil
}

func modeList() string {
	s := ""
	for k := range modes {
		if s != "" {
			s += ", "
		}
		s += k
	}
	return s
}
