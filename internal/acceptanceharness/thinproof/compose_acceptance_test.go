//go:build thinproof

package thinproof_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/githubingest"
)

// composeDir is deploy/compose/thinproof relative to this test file's
// package directory (internal/acceptanceharness/thinproof).
const composeDir = "../../../deploy/compose/thinproof"

// thinproofResult mirrors cmd/thinproof-runner's on-disk result.json shape.
// It is redefined here (rather than imported) because cmd/thinproof-runner
// is package main and not importable; every field type is the same public
// type the runner itself uses, so this is not a competing schema.
type thinproofResult struct {
	SchemaVersion     int                                     `json:"schema_version"`
	Report            *codesignal.Report                      `json:"report"`
	FileMetadata      githubingest.FileMetadata               `json:"file_metadata"`
	GuardResult       acceptanceharness.CredentialGuardResult `json:"guard_result"`
	BlockedRequests   []string                                `json:"blocked_requests"`
	FakeGitHubRecords []acceptanceharness.RequestRecord       `json:"fake_github_records"`
}

// TestThinProofComposeAcceptance drives issue #79's Task 0.3 thin offline
// Compose proof end to end: fake GitHub as a Compose service on an internal
// (no-egress) network, read through pkg/githubingest, analyzed through
// pkg/semantics and pkg/codesignal, all from an external runner container,
// with no image implicitly pulled. See docs/architecture/acceptance-harness.md
// section 2 for the binding no-pull/offline preflight contract this test's
// companion, cmd/thinproof-preflight, implements.
func TestThinProofComposeAcceptance(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not found on PATH; skipping the offline thin Compose proof")
	}

	composeFile := filepath.Join(composeDir, "docker-compose.yml")
	outputPath := filepath.Join(composeDir, "output", "result.json")

	// A stale result.json from a previous failed run must not produce a
	// false pass if this run's runner container never actually writes one.
	if err := os.Remove(outputPath); err != nil && !os.IsNotExist(err) {
		t.Fatalf("removing stale %s: %v", outputPath, err)
	}

	t.Cleanup(func() {
		downCmd := exec.Command("docker", "compose", "-f", composeFile, "down")
		downCmd.CombinedOutput()
	})

	upCmd := exec.Command("docker", "compose", "-f", composeFile, "up",
		"--pull", "never",
		"--abort-on-container-exit",
		"--exit-code-from", "runner",
	)
	output, err := upCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("docker compose up failed: %v\n--- compose output ---\n%s", err, output)
	}

	resultBytes, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("reading %s: %v\n--- compose output ---\n%s", outputPath, err, output)
	}

	var result thinproofResult
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		t.Fatalf("unmarshaling %s: %v", outputPath, err)
	}
	assertThinProofResult(t, result)
}
