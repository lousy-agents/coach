package codesignalcli

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

func TestMapHostNodeResolveErrorIsNotACompilerUnresolvedError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		code string
	}{
		{name: "node missing", err: errHostNodeNotFound, code: projectreadiness.GapNodeMissing},
		{name: "node unsupported", err: errHostNodeMajorDisallowed, code: projectreadiness.GapNodeUnsupported},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body_projectTsRuntimeTest_18(t, tc)
		})
	}
}
