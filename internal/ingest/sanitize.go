package ingest

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
)

// Allowed JSON value types at the ingest boundary.
// Each field in the input schema declares what type it expects.
type valueType int

const (
	typeString valueType = iota
	typeFloat
	typeInt
	typeBool
	typeArray
	typeObject
)

func (t valueType) String() string {
	switch t {
	case typeString:
		return "string"
	case typeFloat:
		return "float"
	case typeInt:
		return "int"
	case typeBool:
		return "bool"
	case typeArray:
		return "array"
	case typeObject:
		return "object"
	default:
		return "unknown"
	}
}

// validateType checks that a JSON raw value matches the expected type.
// Returns the value or a descriptive error.
func validateType(raw json.RawMessage, field string, expected valueType) error {
	if len(raw) == 0 {
		return fmt.Errorf("field %q is empty", field)
	}

	first := raw[0]
	switch expected {
	case typeString:
		if first != '"' {
			return fmt.Errorf("field %q: expected string, got %s", field, describeJSON(first))
		}
	case typeFloat, typeInt:
		if first != '-' && (first < '0' || first > '9') {
			return fmt.Errorf("field %q: expected number, got %s", field, describeJSON(first))
		}
	case typeBool:
		if first != 't' && first != 'f' {
			return fmt.Errorf("field %q: expected boolean, got %s", field, describeJSON(first))
		}
	case typeArray:
		if first != '[' {
			return fmt.Errorf("field %q: expected array, got %s", field, describeJSON(first))
		}
	case typeObject:
		if first != '{' {
			return fmt.Errorf("field %q: expected object, got %s", field, describeJSON(first))
		}
	}
	return nil
}

func describeJSON(first byte) string {
	switch {
	case first == '"':
		return "string"
	case first == '{':
		return "object"
	case first == '[':
		return "array"
	case first == 't' || first == 'f':
		return "boolean"
	case first == 'n':
		return "null"
	case first == '-' || (first >= '0' && first <= '9'):
		return "number"
	default:
		return "unknown"
	}
}

// sanitizeLabel cleans a string for safe use as a Mermaid node label.
// Strips control characters, collapses whitespace, and limits length.
const maxLabelLen = 200

func sanitizeLabel(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	prevSpace := false
	for _, r := range s {
		switch {
		case r == '\n' || r == '\r' || r == '\t':
			// Replace whitespace control chars with space.
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
		case unicode.IsControl(r):
			// Strip other control characters entirely.
			continue
		case r == '"':
			// Escape embedded quotes so they don't break Mermaid %q formatting.
			// Go's %q handles this, but defense in depth.
			b.WriteRune('\'')
			prevSpace = false
		default:
			b.WriteRune(r)
			prevSpace = r == ' '
		}
	}

	result := strings.TrimSpace(b.String())
	runes := []rune(result)
	if len(runes) > maxLabelLen {
		result = string(runes[:maxLabelLen]) + "..."
	}
	return result
}

// extractString unmarshals a JSON field as a string with type validation and sanitization.
func extractString(node map[string]json.RawMessage, key string) (string, error) {
	raw, ok := node[key]
	if !ok {
		return "", fmt.Errorf("missing field %q", key)
	}
	if err := validateType(raw, key, typeString); err != nil {
		return "", err
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", fmt.Errorf("field %q: %w", key, err)
	}
	return sanitizeLabel(s), nil
}

// extractNumber unmarshals a JSON field as a float64 with type validation.
func extractNumber(node map[string]json.RawMessage, key string) (float64, error) {
	raw, ok := node[key]
	if !ok {
		return 0, fmt.Errorf("missing field %q", key)
	}
	if err := validateType(raw, key, typeFloat); err != nil {
		return 0, err
	}
	var v float64
	if err := json.Unmarshal(raw, &v); err != nil {
		return 0, fmt.Errorf("field %q: %w", key, err)
	}
	return v, nil
}

// extractChildren unmarshals a JSON field as an array of objects with type validation.
func extractChildren(node map[string]json.RawMessage, key string) ([]map[string]json.RawMessage, error) {
	raw, ok := node[key]
	if !ok {
		return nil, nil // children are optional
	}
	if err := validateType(raw, key, typeArray); err != nil {
		return nil, err
	}
	var children []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &children); err != nil {
		return nil, fmt.Errorf("field %q: %w", key, err)
	}
	return children, nil
}
