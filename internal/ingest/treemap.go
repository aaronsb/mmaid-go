package ingest

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

// convertTreemap transforms a nested JSON tree into treemap-beta Mermaid syntax.
//
// It walks the root object to find the first array of objects, then recursively
// emits indented node labels and leaf values.
func convertTreemap(data []byte, cfg Config) (string, error) {
	// Try to parse as a single root object first.
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		// Try as an array of objects.
		var arr []map[string]json.RawMessage
		if err2 := json.Unmarshal(data, &arr); err2 != nil {
			return "", fmt.Errorf("treemap ingest: expected JSON object or array: %w", err)
		}
		return treemapFromArray(arr, cfg, 1)
	}

	return treemapFromRoot(root, cfg)
}

// treemapFromRoot handles a single root object. It looks for:
// 1. A root with name+children (the root itself is the top node)
// 2. A root whose first array-valued field contains the tree nodes
func treemapFromRoot(root map[string]json.RawMessage, cfg Config) (string, error) {
	var b strings.Builder
	b.WriteString("treemap-beta\n")

	// Check if root has a name field — treat it as the top-level node.
	if _, ok := root[cfg.NameKey]; ok {
		name, err := extractString(root, cfg.NameKey)
		if err == nil {
			b.WriteString(fmt.Sprintf("  %q\n", name))
			children, err := extractChildren(root, cfg.ChildrenKey)
			if err == nil && len(children) > 0 {
				s, err := treemapNodes(children, cfg, 2)
				if err != nil {
					return "", err
				}
				b.WriteString(s)
				return b.String(), nil
			}
			return b.String(), nil
		}
	}

	// No name field — find the first array field and use it as the node list.
	for _, v := range root {
		var arr []map[string]json.RawMessage
		if err := json.Unmarshal(v, &arr); err == nil && len(arr) > 0 {
			// Check if these objects have the expected fields.
			if _, hasName := arr[0][cfg.NameKey]; hasName {
				s, err := treemapFromArray(arr, cfg, 1)
				if err != nil {
					return "", err
				}
				b.WriteString(s)
				return b.String(), nil
			}
		}
	}

	return "", fmt.Errorf("treemap ingest: no array of objects with %q field found", cfg.NameKey)
}

func treemapFromArray(arr []map[string]json.RawMessage, cfg Config, depth int) (string, error) {
	var b strings.Builder
	if depth == 1 {
		b.WriteString("treemap-beta\n")
	}
	s, err := treemapNodes(arr, cfg, depth)
	if err != nil {
		return "", err
	}
	b.WriteString(s)
	return b.String(), nil
}

func treemapNodes(nodes []map[string]json.RawMessage, cfg Config, depth int) (string, error) {
	// First pass: collect all leaf values to determine scale factor.
	leaves := collectLeafValues(nodes, cfg)
	scale := scaleFor(leaves)

	// Second pass: emit the tree.
	return emitNodes(nodes, cfg, depth, scale)
}

// collectLeafValues recursively gathers all leaf numeric values.
func collectLeafValues(nodes []map[string]json.RawMessage, cfg Config) []float64 {
	var vals []float64
	for _, node := range nodes {
		children, _ := extractChildren(node, cfg.ChildrenKey)
		if len(children) > 0 {
			vals = append(vals, collectLeafValues(children, cfg)...)
		} else {
			v, err := extractNumber(node, cfg.ValueKey)
			if err != nil {
				v = 1
			}
			vals = append(vals, v)
		}
	}
	return vals
}

// scaleFor picks a divisor so the largest value fits in a reasonable integer range.
func scaleFor(values []float64) float64 {
	if len(values) == 0 {
		return 1
	}
	maxVal := 0.0
	for _, v := range values {
		if v > maxVal {
			maxVal = v
		}
	}
	// Scale so max value is ~1000-10000 range for nice treemap proportions.
	switch {
	case maxVal >= 1e12:
		return 1e9 // values become GB
	case maxVal >= 1e9:
		return 1e6 // values become MB
	case maxVal >= 1e6:
		return 1e3 // values become KB
	case maxVal >= 1e4:
		return 10
	default:
		return 1
	}
}

func emitNodes(nodes []map[string]json.RawMessage, cfg Config, depth int, scale float64) (string, error) {
	var b strings.Builder
	indent := strings.Repeat("  ", depth)

	for _, node := range nodes {
		// Extract and sanitize name.
		name, err := extractString(node, cfg.NameKey)
		if err != nil {
			continue // skip nodes without a valid name
		}

		// Check for children.
		children, _ := extractChildren(node, cfg.ChildrenKey)

		if len(children) > 0 {
			// Branch node — label only, recurse into children.
			b.WriteString(fmt.Sprintf("%s%q\n", indent, name))
			s, err := emitNodes(children, cfg, depth+1, scale)
			if err != nil {
				return "", err
			}
			b.WriteString(s)
		} else {
			// Leaf node — label with human-readable suffix, weight scaled.
			value, verr := extractNumber(node, cfg.ValueKey)
			if verr != nil {
				value = 1
			}
			label := formatLabel(name, value)
			weight := max(int(math.Floor(value/scale)), 1)
			b.WriteString(fmt.Sprintf("%s%q: %d\n", indent, label, weight))
		}
	}
	return b.String(), nil
}

// formatLabel appends a human-readable size suffix for large values.
func formatLabel(name string, value float64) string {
	switch {
	case value >= 1e12:
		return fmt.Sprintf("%s (%.1fT)", name, value/1e12)
	case value >= 1e9:
		return fmt.Sprintf("%s (%.1fG)", name, value/1e9)
	case value >= 1e6:
		return fmt.Sprintf("%s (%.1fM)", name, value/1e6)
	case value >= 1e3:
		return fmt.Sprintf("%s (%.1fK)", name, value/1e3)
	default:
		return name
	}
}
