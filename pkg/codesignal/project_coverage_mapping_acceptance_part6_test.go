package codesignal_test

import (
	"github.com/lousy-agents/coach/pkg/domain"
)

// backendUnavailableCoverage returns a Coverage containing DiagBackendUnavailable,
// as produced when the TS sidecar cannot be reached.
func backendUnavailableCoverage() *domain.Coverage {
	return &domain.Coverage{
		Phase:    "model",
		Complete: false,
		Diagnostics: []domain.Diagnostic{
			{Code: domain.DiagBackendUnavailable, Message: "sidecar unavailable"},
		},
	}
}
