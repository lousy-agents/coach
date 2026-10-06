//go:build thinproof

package thinproof_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
)

func assertThinProofResult(t *testing.T, result thinproofResult) {
	t.Helper()
	if result.GuardResult.Rejected() {
		t.Errorf("expected an empty guard_result inside the clean container, got %+v", result.GuardResult)
	}
	if len(result.BlockedRequests) != 0 {
		t.Errorf("expected no blocked_requests, got %+v", result.BlockedRequests)
	}
	if !sawInstallationRead(result.FakeGitHubRecords) {
		t.Errorf("expected at least one fake_github_records entry with auth_mode %q, got %+v", acceptanceharness.AuthModeInstallation, result.FakeGitHubRecords)
	}
	assertReportMatchesGolden(t, result)
}

func sawInstallationRead(records []acceptanceharness.RequestRecord) bool {
	for _, rec := range records {
		if rec.AuthMode == acceptanceharness.AuthModeInstallation {
			return true
		}
	}
	return false
}

func assertReportMatchesGolden(t *testing.T, result thinproofResult) {
	t.Helper()
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
