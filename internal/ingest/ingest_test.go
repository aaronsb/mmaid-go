package ingest

import (
	"encoding/json"
	"strings"
	"testing"
)

// ── treemap tests ────────────────────────────────────────────────

func TestConvertTreemap_RootWithChildren(t *testing.T) {
	input := `{
		"name": "Storage",
		"children": [
			{"name": "Disk A", "children": [
				{"name": "part1", "size": 500},
				{"name": "part2", "size": 1500}
			]},
			{"name": "Disk B", "children": [
				{"name": "part3", "size": 1000}
			]}
		]
	}`
	out, err := Convert("treemap", []byte(input), DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "treemap-beta\n") {
		t.Errorf("expected treemap-beta header, got: %s", out[:40])
	}
	if !strings.Contains(out, `"Storage"`) {
		t.Error("missing root label")
	}
	if !strings.Contains(out, `"Disk A"`) {
		t.Error("missing branch label")
	}
	if !strings.Contains(out, `"part1"`) {
		t.Error("missing leaf label")
	}
	if !strings.Contains(out, ": 500") {
		t.Error("missing leaf value 500")
	}
}

func TestConvertTreemap_WrappedArray(t *testing.T) {
	// lsblk-style: root has an array field containing the nodes.
	input := `{
		"blockdevices": [
			{"name": "sda", "size": 1000000000000, "children": [
				{"name": "sda1", "size": 536870912},
				{"name": "sda2", "size": 999463129088}
			]}
		]
	}`
	out, err := Convert("treemap", []byte(input), DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"sda"`) {
		t.Error("missing branch label")
	}
	// sda1 is ~512M, should get human-readable suffix.
	if !strings.Contains(out, "536.9M") {
		t.Errorf("expected human-readable suffix for sda1, got: %s", out)
	}
}

func TestConvertTreemap_CustomKeys(t *testing.T) {
	input := `{
		"label": "Root",
		"nodes": [
			{"label": "A", "bytes": 100},
			{"label": "B", "bytes": 200}
		]
	}`
	cfg := Config{NameKey: "label", ValueKey: "bytes", ChildrenKey: "nodes"}
	out, err := Convert("treemap", []byte(input), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"Root"`) {
		t.Error("missing root with custom key")
	}
	if !strings.Contains(out, `"A"`) || !strings.Contains(out, `"B"`) {
		t.Error("missing nodes with custom key")
	}
}

func TestConvertTreemap_MinValueOne(t *testing.T) {
	input := `[{"name": "tiny", "size": 0.5}]`
	out, err := Convert("treemap", []byte(input), DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, ": 1") {
		t.Errorf("expected minimum value of 1, got: %s", out)
	}
}

// ── pie tests ────────────────────────────────────────────────────

func TestConvertPie_FlatKV(t *testing.T) {
	input := `{"Go": 45, "Rust": 30, "Python": 25}`
	out, err := Convert("pie", []byte(input), DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "pie\n") {
		t.Errorf("expected pie header, got: %s", out[:20])
	}
	if !strings.Contains(out, `"Go"`) {
		t.Error("missing key")
	}
	if !strings.Contains(out, "45") {
		t.Error("missing value")
	}
}

func TestConvertPie_WithTitle(t *testing.T) {
	input := `{"title": "Languages", "data": {"Go": 45, "Rust": 30}}`
	out, err := Convert("pie", []byte(input), DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "pie title Languages") {
		t.Errorf("expected title, got: %s", out)
	}
}

func TestConvertPie_Deterministic(t *testing.T) {
	input := `{"B": 2, "A": 1, "C": 3}`
	out1, _ := Convert("pie", []byte(input), DefaultConfig())
	out2, _ := Convert("pie", []byte(input), DefaultConfig())
	if out1 != out2 {
		t.Error("pie output is not deterministic")
	}
	// A should come before B.
	if strings.Index(out1, `"A"`) > strings.Index(out1, `"B"`) {
		t.Error("expected sorted keys")
	}
}

// ── template tests ───────────────────────────────────────────────

func TestTemplate_Treemap_ValidJSON(t *testing.T) {
	tmpl, err := Template("treemap", DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	var parsed any
	if err := json.Unmarshal([]byte(tmpl), &parsed); err != nil {
		t.Errorf("template is not valid JSON: %v", err)
	}
}

func TestTemplate_Pie_ValidJSON(t *testing.T) {
	tmpl, err := Template("pie", DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	var parsed any
	if err := json.Unmarshal([]byte(tmpl), &parsed); err != nil {
		t.Errorf("template is not valid JSON: %v", err)
	}
}

func TestTemplate_Treemap_RoundTrip(t *testing.T) {
	cfg := DefaultConfig()
	tmpl, err := Template("treemap", cfg)
	if err != nil {
		t.Fatal(err)
	}
	// The template should be convertible back to Mermaid syntax.
	out, err := Convert("treemap", []byte(tmpl), cfg)
	if err != nil {
		t.Fatalf("round-trip failed: %v", err)
	}
	if !strings.HasPrefix(out, "treemap-beta\n") {
		t.Errorf("round-trip didn't produce treemap-beta: %s", out[:40])
	}
}

func TestTemplate_Pie_RoundTrip(t *testing.T) {
	cfg := DefaultConfig()
	tmpl, err := Template("pie", cfg)
	if err != nil {
		t.Fatal(err)
	}
	out, err := Convert("pie", []byte(tmpl), cfg)
	if err != nil {
		t.Fatalf("round-trip failed: %v", err)
	}
	if !strings.HasPrefix(out, "pie title") {
		t.Errorf("round-trip didn't produce pie: %s", out[:20])
	}
}

func TestTemplate_Treemap_CustomKeys(t *testing.T) {
	cfg := Config{NameKey: "label", ValueKey: "bytes", ChildrenKey: "nodes"}
	tmpl, err := Template("treemap", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(tmpl, `"label"`) {
		t.Error("template should use custom name key")
	}
	if !strings.Contains(tmpl, `"bytes"`) {
		t.Error("template should use custom value key")
	}
	if !strings.Contains(tmpl, `"nodes"`) {
		t.Error("template should use custom children key")
	}
}

// ── error tests ──────────────────────────────────────────────────

func TestConvert_UnknownMode(t *testing.T) {
	_, err := Convert("bogus", []byte("{}"), DefaultConfig())
	if err == nil {
		t.Error("expected error for unknown mode")
	}
}

func TestConvertTreemap_InvalidJSON(t *testing.T) {
	_, err := Convert("treemap", []byte("not json"), DefaultConfig())
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestConvertPie_NoNumericValues(t *testing.T) {
	_, err := Convert("pie", []byte(`{"a": "text", "b": "more"}`), DefaultConfig())
	if err == nil {
		t.Error("expected error when no numeric values")
	}
}

// ── sanitization tests ───────────────────────────────────────────

func TestSanitizeLabel_ControlChars(t *testing.T) {
	got := sanitizeLabel("hello\x00world\x07")
	if strings.ContainsAny(got, "\x00\x07") {
		t.Errorf("control chars not stripped: %q", got)
	}
	if got != "helloworld" {
		t.Errorf("expected 'helloworld', got %q", got)
	}
}

func TestSanitizeLabel_Newlines(t *testing.T) {
	got := sanitizeLabel("line1\nline2\r\nline3")
	if strings.ContainsAny(got, "\n\r") {
		t.Errorf("newlines not replaced: %q", got)
	}
	if got != "line1 line2 line3" {
		t.Errorf("expected 'line1 line2 line3', got %q", got)
	}
}

func TestSanitizeLabel_Quotes(t *testing.T) {
	got := sanitizeLabel(`say "hello" world`)
	if strings.Contains(got, `"`) {
		t.Errorf("double quotes not escaped: %q", got)
	}
	if got != "say 'hello' world" {
		t.Errorf("expected quotes replaced, got %q", got)
	}
}

func TestSanitizeLabel_TruncatesLong(t *testing.T) {
	long := strings.Repeat("x", 300)
	got := sanitizeLabel(long)
	if len(got) > maxLabelLen+3 {
		t.Errorf("label too long: %d chars", len(got))
	}
	if !strings.HasSuffix(got, "...") {
		t.Error("truncated label should end with ...")
	}
}

func TestTreemap_MaliciousLabel(t *testing.T) {
	// A name containing Mermaid syntax should be sanitized.
	input := `[{"name": "evil\": 999\n\"injected", "size": 10}]`
	out, err := Convert("treemap", []byte(input), DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	// The output should not contain raw newlines in labels.
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || trimmed == "treemap-beta" {
			continue
		}
		// Each non-empty line should be a valid treemap entry.
		if strings.Contains(trimmed, "injected") && !strings.HasPrefix(trimmed, `"`) {
			t.Errorf("injection not sanitized: %s", trimmed)
		}
	}
}

func TestPie_MaliciousKey(t *testing.T) {
	input := `{"normal": 10, "evil\ninjected": 5}`
	out, err := Convert("pie", []byte(input), DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	// No raw newlines should appear inside label quotes.
	if strings.Contains(out, "evil\ninjected") {
		t.Error("newline in pie key not sanitized")
	}
}

func TestValidateType_WrongTypes(t *testing.T) {
	tests := []struct {
		raw      string
		expected valueType
	}{
		{`"hello"`, typeFloat},
		{`42`, typeString},
		{`true`, typeString},
		{`[1,2]`, typeString},
		{`{"a":1}`, typeArray},
		{`null`, typeString},
	}
	for _, tt := range tests {
		err := validateType(json.RawMessage(tt.raw), "test", tt.expected)
		if err == nil {
			t.Errorf("validateType(%s, %s): expected error", tt.raw, tt.expected)
		}
	}
}

func TestValidateType_CorrectTypes(t *testing.T) {
	tests := []struct {
		raw      string
		expected valueType
	}{
		{`"hello"`, typeString},
		{`42`, typeFloat},
		{`-3.14`, typeFloat},
		{`true`, typeBool},
		{`[1,2]`, typeArray},
		{`{"a":1}`, typeObject},
	}
	for _, tt := range tests {
		err := validateType(json.RawMessage(tt.raw), "test", tt.expected)
		if err != nil {
			t.Errorf("validateType(%s, %s): unexpected error: %v", tt.raw, tt.expected, err)
		}
	}
}
