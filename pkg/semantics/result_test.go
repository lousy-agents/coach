package semantics

import (
	"encoding/json"
	"testing"
)

// AC-4.1: the Result type (and its nested types) must carry explicit json
// struct tags in snake_case.
func TestResult_JSONUsesSnakeCaseTags(t *testing.T) {
	r := Result{
		Path:        "main.go",
		Language:    LanguageGo,
		ParseStatus: ParseStatus("ok"),
		Metrics:     StructuralMetrics{Ifs: 1},
	}

	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("AC-4.1: marshaling a minimal Result must not fail: %v", err)
	}

	var asMap map[string]json.RawMessage
	if err := json.Unmarshal(raw, &asMap); err != nil {
		t.Fatalf("AC-4.1: Result JSON must unmarshal into a generic map: %v", err)
	}

	wantKeys := []string{"path", "language", "parse_status", "metrics"}
	for _, key := range wantKeys {
		if _, ok := asMap[key]; !ok {
			t.Errorf("AC-4.1: Result JSON missing expected snake_case key %q; got keys: %v", key, asMap)
		}
	}

	badKeys := []string{"Path", "Language", "ParseStatus", "Metrics"}
	for _, key := range badKeys {
		if _, ok := asMap[key]; ok {
			t.Errorf("AC-4.1: Result JSON must not use PascalCase key %q; got keys: %v", key, asMap)
		}
	}
}

// AC-4.1 (omitempty specifics): marshaling a Result with nil SyntaxErrors,
// Imports, and Findings must omit those keys entirely from the JSON output,
// not emit them as null or [].
func TestResult_OmitsEmptyOptionalSlices(t *testing.T) {
	r := Result{
		Path:        "empty.go",
		Language:    LanguageGo,
		ParseStatus: ParseStatus("ok"),
		Metrics:     StructuralMetrics{},
	}

	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshaling a Result with nil optional slices must not fail: %v", err)
	}

	var asMap map[string]json.RawMessage
	if err := json.Unmarshal(raw, &asMap); err != nil {
		t.Fatalf("Result JSON must unmarshal into a generic map: %v", err)
	}

	for _, key := range []string{"syntax_errors", "imports", "findings"} {
		if raw, present := asMap[key]; present {
			t.Errorf("Result JSON must omit key %q for nil slice (omitempty), got present with value %s", key, raw)
		}
	}
}
