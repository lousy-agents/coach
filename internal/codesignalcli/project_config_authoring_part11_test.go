package codesignalcli

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func TestAuthorProjectConfig_LayerPrefixesRejectOversizedBudget(t *testing.T) {
	discovered := projectmodel.TSRootDiscoveryResult{Complete: true}

	prefixes := make([]string, maxProjectConfigLayerPrefixes+1)
	for i := range prefixes {
		prefixes[i] = fmt.Sprintf("dir%d", i)
	}

	result, out := runAuthoringWithTimeout(t, 3*time.Second, discovered,
		".",
		"domain",
		strings.Join(prefixes, ","),
		"cancel",
	)

	if !result.Cancelled {
		t.Fatalf("expected Cancelled = true after an oversized prefix list is rejected and cancelled, got false")
	}
	if len(result.Layers) != 0 {
		t.Fatalf("expected no layer to be recorded for a rejected oversized prefix list, got %+v", result.Layers)
	}
	if !strings.Contains(out, fmt.Sprintf("%d", maxProjectConfigLayerPrefixes)) {
		t.Fatalf("expected the rejection explanation to reference the %d-entry budget, got:\n%s", maxProjectConfigLayerPrefixes, out)
	}
}

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

	result := AuthorProjectConfig(t.TempDir(), in, transcript, candidateOut, discovered, "", false)

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

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
