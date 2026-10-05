package semantics

import (
	"context"
	"encoding/json"

	"testing"
)

func body_resultTest_63(t *testing.T, a *Analyzer, tt struct {
	name       string
	input      FileInput
	wantStatus ParseStatus
}) {
	result, _ := a.AnalyzeBytes(context.Background(), tt.input)
	if result == nil {
		t.Fatalf("AnalyzeBytes(%s): got nil result, want non-nil", tt.name)
	}
	if result.ParseStatus != tt.wantStatus {
		t.Fatalf("AnalyzeBytes(%s): ParseStatus = %q, want %q", tt.name, result.ParseStatus, tt.wantStatus)
	}
	if result.ReactComponents != nil {
		t.Errorf("AnalyzeBytes(%s): ReactComponents = %+v, want nil", tt.name, result.ReactComponents)
	}

	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("AnalyzeBytes(%s): marshaling Result must not fail: %v", tt.name, err)
	}
	var asMap map[string]json.RawMessage
	if err := json.Unmarshal(raw, &asMap); err != nil {
		t.Fatalf("AnalyzeBytes(%s): Result JSON must unmarshal into a generic map: %v", tt.name, err)
	}
	if _, present := asMap["react_components"]; present {
		t.Errorf("AnalyzeBytes(%s): Result JSON must omit key %q (omitempty), got present with value %s", tt.name, "react_components", asMap["react_components"])
	}
}
