package configauthoring

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func TestAuthorProjectConfig_ApprovedAndOutputUnset_WriteFailureIsReportedNotSilentlySuccessful(t *testing.T) {
	discovered := projectmodel.TSRootDiscoveryResult{Roots: []string{"apps/api"}, Complete: true}
	writeErr := errors.New("write: broken pipe")
	transcript := &bytes.Buffer{}
	candidateOut := &alwaysErrorWriter{err: writeErr}
	in := strings.NewReader(strings.Join([]string{
		"1",
		"domain", "internal/domain",
		"",
		"",
		"",
		"approve",
	}, "\n") + "\n")

	result := Author(t.TempDir(), in, transcript, candidateOut, discovered, "", false)

	if !result.Approved {
		t.Fatalf("expected Approved = true (the candidate itself is valid), got false; transcript:\n%s", transcript.String())
	}
	if result.ValidationError != nil {
		t.Fatalf("expected ValidationError = nil, got %v", result.ValidationError)
	}
	if result.WriteError == nil {
		t.Fatalf("expected WriteError to report the failed write to candidateOut, got nil")
	}
	if !errors.Is(result.WriteError, writeErr) {
		t.Fatalf("WriteError = %v, want it to wrap %v", result.WriteError, writeErr)
	}
}
