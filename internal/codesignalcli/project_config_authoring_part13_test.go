package codesignalcli

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func TestAuthorProjectConfig_ApprovalGateTerminatesPromptlyOnExhaustedOrErroringInput(t *testing.T) {
	discovered := projectmodel.TSRootDiscoveryResult{Complete: true}
	const watchdog = 3 * time.Second

	assertNotApprovedBeforeWatchdog := func(t *testing.T, in io.Reader) {
		body_projectConfigAuthoringPart13Test_20(t, in, discovered, watchdog)
	}

	t.Run("persistent non-EOF read error does not approve and returns before watchdog", func(t *testing.T) {
		in := &persistentErrorReader{data: []byte("root\napiLayer\napps/api\n\n\n\n"), err: errors.New("simulated non-EOF read error (e.g. EBADF/EIO)")}
		assertNotApprovedBeforeWatchdog(t, in)
	})

	t.Run("clean EOF at the approval prompt does not approve and returns before watchdog", func(t *testing.T) {
		in := strings.NewReader("root\napiLayer\napps/api\n\n\n\n")
		assertNotApprovedBeforeWatchdog(t, in)
	})
}

// approvedCandidateBytes builds the expected schema-1 project-config
// document for config independently of buildApprovedCandidate, so a test
// comparing against it actually checks AuthorProjectConfig's write-time
// serialization rather than merely confirming two identical code paths
// agree with each other.
func approvedCandidateBytes(t *testing.T, config projectconfig.Config) []byte {
	t.Helper()
	config.SchemaVersion = "1"
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal expected project config: %v", err)
	}
	return append(data, '\n')
}

// tailAfterLastPrompt returns whatever was written to out after the last
// "> " prompt marker -- everything the approval gate's own prompt/read
// writes, up to and including that marker, is excluded, isolating the
// approved candidate document (or nothing, if none was ever emitted) from
// the preceding human-readable interactive text.
func tailAfterLastPrompt(out string) string {
	const marker = "> "
	idx := strings.LastIndex(out, marker)
	if idx == -1 {
		return out
	}
	return out[idx+len(marker):]
}

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
