package ingest

import (
	"encoding/json"
	"strings"
)

func templateTreemap(cfg Config) string {
	// Build the template using the configured field names so it adapts to overrides.
	// Build using configured field names so template adapts to overrides.
	root := map[string]any{
		cfg.NameKey: "Root",
		cfg.ChildrenKey: []any{
			map[string]any{
				cfg.NameKey: "Group A",
				cfg.ChildrenKey: []any{
					map[string]any{cfg.NameKey: "Item 1", cfg.ValueKey: 100},
					map[string]any{cfg.NameKey: "Item 2", cfg.ValueKey: 200},
				},
			},
			map[string]any{
				cfg.NameKey: "Group B",
				cfg.ChildrenKey: []any{
					map[string]any{cfg.NameKey: "Item 3", cfg.ValueKey: 150},
				},
			},
		},
	}

	return prettyJSON(root)
}

func templatePie(cfg Config) string {
	obj := map[string]any{
		"title": "Distribution",
		"data": map[string]any{
			"Category A": 45,
			"Category B": 30,
			"Category C": 25,
		},
	}
	return prettyJSON(obj)
}

func prettyJSON(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "{}"
	}
	// Ensure trailing newline.
	s := string(b)
	if !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	return s
}
