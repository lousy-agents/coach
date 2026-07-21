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
		downCmd.CombinedOutput() //nolint:errcheck // best-effort cleanup
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

	if result.GuardResult.Rejected() {
		t.Errorf("expected an empty guard_result inside the clean container, got %+v", result.GuardResult)
	}

	if len(result.BlockedRequests) != 0 {
		t.Errorf("expected no blocked_requests, got %+v", result.BlockedRequests)
	}

	var sawInstallationContentsRead bool
	for _, rec := range result.FakeGitHubRecords {
		if rec.AuthMode == acceptanceharness.AuthModeInstallation {
			sawInstallationContentsRead = true
		}
	}
	if !sawInstallationContentsRead {
		t.Errorf("expected at least one fake_github_records entry with auth_mode %q, got %+v", acceptanceharness.AuthModeInstallation, result.FakeGitHubRecords)
	}

	goldenPath := filepath.Join("testdata", "golden", "report_v1.json")
	goldenBytes, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("reading golden %s: %v", goldenPath, err)
	}

	gotReport, err := json.MarshalIndent(result.Report, "", "  ")
	if err != nil {
		t.Fatalf("marshaling result.Report for golden comparison: %v", err)
	}
	gotReport = append(gotReport, '\n')

	if string(gotReport) != string(goldenBytes) {
		t.Errorf("report did not match golden %s:\n--- got ---\n%s\n--- want ---\n%s", goldenPath, gotReport, goldenBytes)
	}
}
