package ingest

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// convertPie transforms flat key/value JSON into pie chart Mermaid syntax.
//
// Accepted shapes:
//
//	{"key": number, ...}
//	{"title": "...", "data": {"key": number, ...}}
func convertPie(data []byte, cfg Config) (string, error) {
	// Try the {title, data} shape first.
	var wrapper struct {
		Title string                     `json:"title"`
		Data  map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(data, &wrapper); err == nil && len(wrapper.Data) > 0 {
		return pieFromMap(wrapper.Title, wrapper.Data)
	}

	// Fall back to flat key/value.
	var flat map[string]json.RawMessage
	if err := json.Unmarshal(data, &flat); err != nil {
		return "", fmt.Errorf("pie ingest: expected JSON object: %w", err)
	}

	// Filter to only numeric values, validate types.
	numeric := make(map[string]json.RawMessage)
	for k, v := range flat {
		if validateType(v, k, typeFloat) == nil {
			numeric[k] = v
		}
	}
	if len(numeric) == 0 {
		return "", fmt.Errorf("pie ingest: no numeric key/value pairs found")
	}

	return pieFromMap("", numeric)
}

func pieFromMap(title string, data map[string]json.RawMessage) (string, error) {
	var b strings.Builder

	if title != "" {
		b.WriteString(fmt.Sprintf("pie title %s\n", sanitizeLabel(title)))
	} else {
		b.WriteString("pie\n")
	}

	// Sort keys for deterministic output.
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		var v float64
		if err := json.Unmarshal(data[k], &v); err != nil {
			continue
		}
		b.WriteString(fmt.Sprintf("    %q : %.6g\n", sanitizeLabel(k), v))
	}

	return b.String(), nil
}
