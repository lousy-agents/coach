package codesignalcli

import (
	"testing"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func body_projectConfigSuggestionPart3Test_133(t *testing.T, tt struct {
	name     string
	result   projectmodel.RootDiscoveryResult
	wantCode string
	wantOK   bool
	wantPath string
}) {
	code, path, message, ok := suggestPrimaryRootDiagnostic(tt.result)
	if ok != tt.wantOK {
		t.Fatalf("ok = %v, want %v (code=%q message=%q)", ok, tt.wantOK, code, message)
	}
	if tt.wantOK {
		return
	}
	if code != tt.wantCode {
		t.Errorf("code = %q, want %q", code, tt.wantCode)
	}
	if path != tt.wantPath {
		t.Errorf("path = %q, want %q", path, tt.wantPath)
	}
	if message == "" {
		t.Errorf("message must be non-empty and deterministic, got empty string")
	}
}
