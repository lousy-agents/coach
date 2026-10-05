package codesignal_test

import (
	"github.com/lousy-agents/coach/pkg/domain"
)

// incompleteCoverage returns an incomplete Coverage for a given phase name.
func incompleteCoverage(phase string) *domain.Coverage {
	return &domain.Coverage{Phase: phase, Complete: false}
}
