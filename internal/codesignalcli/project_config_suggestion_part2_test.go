package codesignalcli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

// TestSuggestProjectConfigIncompleteBudget locks the discovery → primary
// diagnostic → exit-2 glue for budget exhaustion through SuggestProjectConfig
// (issue #220). pkg/projectmodel covers incomplete discovery in isolation and
// TestSuggestPrimaryRootDiagnostic maps Complete=false in isolation; this is
// the package-level path that lowers suggestGoBudgets over a real multi-file
// Git snapshot so a regression in either hop fails here.
func TestSuggestProjectConfigIncompleteBudget(t *testing.T) {
	prev := suggestGoBudgets
	suggestGoBudgets = projectmodel.GoBudgets{MaxInputFiles: 1}
	t.Cleanup(func() { suggestGoBudgets = prev })

	dir := gitfixture.Init(t)

	for _, mod := range []string{"mod1", "mod2", "mod3"} {
		if err := os.MkdirAll(filepath.Join(dir, mod), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", mod, err)
		}
		gitfixture.CommitFile(t, dir, filepath.Join(mod, "go.mod"), "module example.com/"+mod+"\n\ngo 1.25\n")
	}

	const outRel = "suggested-project.json"
	result := SuggestProjectConfig(dir, outRel, true)

	if result.ExitCode != 2 {
		t.Fatalf("ExitCode = %d, want 2 (stderr envelope: %s)", result.ExitCode, result.Envelope)
	}
	if len(result.Candidate) != 0 {
		t.Fatalf("Candidate must be empty on incomplete discovery, got %q", result.Candidate)
	}

	var envelope struct {
		Diagnostics []struct {
			Code string `json:"code"`
		} `json:"diagnostics"`
	}
	if err := json.Unmarshal(result.Envelope, &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v\nraw: %s", err, result.Envelope)
	}
	if len(envelope.Diagnostics) != 1 {
		t.Fatalf("diagnostics len = %d, want 1; envelope: %s", len(envelope.Diagnostics), result.Envelope)
	}
	if got := envelope.Diagnostics[0].Code; got != SuggestDiagIncomplete {
		t.Fatalf("primary diagnostic code = %q, want %q; envelope: %s", got, SuggestDiagIncomplete, result.Envelope)
	}

	if _, err := os.Stat(filepath.Join(dir, outRel)); !os.IsNotExist(err) {
		t.Fatalf("--output target %q must not be created on incomplete discovery; Stat err = %v", outRel, err)
	}
}
