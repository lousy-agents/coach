package codesignalcli

import (
	"context"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

func countDiagnosticsOfKind(diagnostics []codesignal.Diagnostic, kind string) int {
	count := 0
	for _, d := range diagnostics {
		if d.Kind == kind {
			count++
		}
	}
	return count
}

// recordingProjectBackend records every request it is handed and answers
// each with the same result.
type recordingProjectBackend struct {
	requests []ProjectBackendRequest
	result   *ProjectBackendResult
}

func (b *recordingProjectBackend) Analyze(_ context.Context, req ProjectBackendRequest) (*ProjectBackendResult, error) {
	b.requests = append(b.requests, req)
	return b.result, nil
}
