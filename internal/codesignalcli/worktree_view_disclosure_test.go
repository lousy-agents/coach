package codesignalcli

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

func TestPerPathDisclosureNamesAPathOnceHoweverManyCategoriesItFallsIn(t *testing.T) {
	for _, code := range []string{"MM", "AM", "UU", "MT"} {
		t.Run(code, func(t *testing.T) {
			assertOneDiagnosticForOnePath(t, code)
		})
	}
}

func assertOneDiagnosticForOnePath(t *testing.T, code string) {
	t.Helper()
	entries := []gitrepo.WorktreeEntry{{Code: code, Path: "pkg/x.go"}}

	diagnostics := perPathDisclosure(collectWorktreePaths(entries), nil)

	if len(diagnostics) != 1 {
		t.Errorf("status %q gave %d diagnostics for one path, want 1: %+v", code, len(diagnostics), diagnostics)
	}
}
