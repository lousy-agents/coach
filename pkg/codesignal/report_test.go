package codesignal

import (
	"encoding/json"
	"testing"
)

func TestReport_JSONIncludesSchemaVersionAtTopLevel(t *testing.T) {
	report := Report{SchemaVersion: "1"}

	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshaling a minimal Report must not fail: %v", err)
	}

	var asMap map[string]json.RawMessage
	if err := json.Unmarshal(raw, &asMap); err != nil {
		t.Fatalf("Report JSON must unmarshal into a generic map: %v", err)
	}

	got, ok := asMap["schema_version"]
	if !ok {
		t.Fatalf("Report JSON missing top-level %q key; got keys: %v", "schema_version", asMap)
	}
	if string(got) != `"1"` {
		t.Errorf("Report JSON %q value: got %s, want %q", "schema_version", got, `"1"`)
	}
}
