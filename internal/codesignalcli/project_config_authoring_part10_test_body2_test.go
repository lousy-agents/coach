package codesignalcli

import (
	"testing"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func body_projectConfigAuthoringPart10Test_24(t *testing.T, discovered projectmodel.TSRootDiscoveryResult, tc struct {
	name   string
	answer string
}) {
	result, _ := runAuthoring(discovered,
		"1",
		"api", "apps/api",
		"",
		"",
		"",
		tc.answer,
	)
	if result.Approved {
		t.Fatalf("expected Approved = false for answer %q, got true", tc.answer)
	}
	t.Run("declining is not a cancellation", func(t *testing.T) {
		body_projectConfigAuthoringPart10Test_decliningIsNotACancellation_36(t, tc, result)
	})
}
