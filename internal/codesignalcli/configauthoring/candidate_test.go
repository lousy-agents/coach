package configauthoring

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
)

func TestBuildApprovedCandidate_RejectsEmptyRoots(t *testing.T) {
	_, err := buildApprovedCandidate(nil, nil, nil, "")
	if err == nil || !strings.Contains(err.Error(), "roots must contain at least one") {
		t.Fatalf("expected buildApprovedCandidate to reject an empty roots slice (\"roots must contain at least one\"), got %v", err)
	}
}

func TestBuildApprovedCandidate_RejectsForbiddenPairNamingUndeclaredLayer(t *testing.T) {
	forbidden := []projectconfig.ForbiddenImport{{From: "domain", To: "unknown-layer"}}

	_, err := buildApprovedCandidate([]string{"apps/api"}, nil, forbidden, "")
	if err == nil {
		t.Fatalf("expected buildApprovedCandidate to reject a forbidden-import pair naming an undeclared layer, got nil error")
	}
}
