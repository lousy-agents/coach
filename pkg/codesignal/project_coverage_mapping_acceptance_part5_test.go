package codesignal_test

import (
	"github.com/lousy-agents/coach/pkg/domain"
)

// notRequestedCoverage returns a Coverage with Phase="not_requested" and Complete=true,
// as the TypeScript backend produces when no required_layer is configured.
func notRequestedCoverage() *domain.Coverage {
	return &domain.Coverage{Phase: "not_requested", Complete: true}
}
