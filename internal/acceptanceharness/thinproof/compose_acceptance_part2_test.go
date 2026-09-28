//go:build thinproof

package thinproof_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

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
