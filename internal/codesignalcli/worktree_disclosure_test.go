package codesignalcli

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

func TestWorktreeDisclosureDiagnosticsBoundsTheSample(t *testing.T) {
	var records []string
	for i := 0; i < 20; i++ {
		records = append(records, "?? bulk"+twoDigits(i)+".go")
	}
	withWorktreeStatusOutput(t, porcelainZ(records...), nil)

	got := WorkingTreeDisclosureDiagnostics("repo")
	if len(got) != 1 {
		t.Fatalf("diagnostics: got %d (%+v), want 1 grouped diagnostic", len(got), got)
	}
	if got[0].Kind != codesignal.DiagKindWorktreeChangesNotAnalyzed {
		t.Fatalf("kind: got %q, want %q", got[0].Kind, codesignal.DiagKindWorktreeChangesNotAnalyzed)
	}
	message := got[0].Message
	if !strings.HasPrefix(message, "20 untracked files were not analyzed: ") {
		t.Fatalf("message: got %q, want a total count of 20 and an untracked classification", message)
	}
	if strings.Contains(message, "bulk05.go") {
		t.Fatalf("message listed a path past the sample: %q", message)
	}
	for _, name := range []string{"bulk00.go", "bulk04.go"} {
		if !strings.Contains(message, name) {
			t.Errorf("message missing sample path %s: %q", name, message)
		}
	}
	if strings.Contains(message, "staged") || strings.Contains(message, "modified") {
		t.Fatalf("untracked-only message named another classification: %q", message)
	}
}

func porcelainZ(records ...string) []byte {
	var b strings.Builder
	for _, record := range records {
		b.WriteString(record)
		b.WriteByte(0)
	}
	return []byte(b.String())
}

func withWorktreeStatusOutput(t *testing.T, output []byte, err error) {
	t.Helper()
	original := listWorktreeStatus
	listWorktreeStatus = func(string) ([]worktreeStatusEntry, error) {
		if err != nil {
			return nil, err
		}
		return parseWorktreeStatus(output), nil
	}
	t.Cleanup(func() { listWorktreeStatus = original })
}

func twoDigits(n int) string {
	return string([]byte{byte('0' + n/10), byte('0' + n%10)})
}
