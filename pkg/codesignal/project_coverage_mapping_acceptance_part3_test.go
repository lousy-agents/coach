package codesignal_test

import (
	"github.com/lousy-agents/coach/pkg/domain"
)

// completeCoverage returns a complete Coverage for a given phase name.
func completeCoverage(phase string) *domain.Coverage {
	return &domain.Coverage{Phase: phase, Complete: true}
}
