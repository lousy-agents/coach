// Command thinproof-runner is the external test-runner container for issue
// #79's Task 0.3 thin offline proof: it exercises the fake-GitHub ->
// pkg/githubingest -> pkg/semantics -> pkg/codesignal path against a fake
// GitHub service reachable only at FAKE_GITHUB_BASE_URL (e.g.
// http://fake-github:8080 inside Compose), through a GuardedTransport that
// allows egress to that host only, and writes its findings to OUTPUT_PATH
// as JSON.
//
// It deliberately does not assert pass/fail against a golden itself: that
// comparison happens host-side, in
// internal/acceptanceharness/thinproof/compose_acceptance_test.go, so the
// golden stays a committed, reviewable test fixture rather than a value
// baked into this binary.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/githubingest"
)

// resultSchemaVersion versions thinproofResult's on-disk shape, independent
// of acceptanceharness.FixtureSchemaVersion and codesignal.Report's own
// SchemaVersion, per docs/architecture/acceptance-harness.md section 3's
// golden-fixture-versioning rules.
const resultSchemaVersion = 1

// thinproofResult is the JSON shape written to OUTPUT_PATH, decoded by
// compose_acceptance_test.go on the host side.
type thinproofResult struct {
	SchemaVersion     int                                     `json:"schema_version"`
	Report            *codesignal.Report                      `json:"report"`
	FileMetadata      githubingest.FileMetadata               `json:"file_metadata"`
	GuardResult       acceptanceharness.CredentialGuardResult `json:"guard_result"`
	BlockedRequests   []string                                `json:"blocked_requests"`
	FakeGitHubRecords []acceptanceharness.RequestRecord       `json:"fake_github_records"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "thinproof-runner: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("thinproof-runner: proof succeeded")
}

func run() error {
	session, err := openThinproofSession()
	if err != nil {
		return err
	}
	return scanThinproof(session)
}

func writeResult(path string, result thinproofResult) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
