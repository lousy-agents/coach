package codesignalcli

import (
	"errors"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
)

func TestMapHostNodeResolveErrorIsNotACompilerUnresolvedError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		code string
	}{
		{name: "node missing", err: tstoolchain.ErrHostNodeNotFound, code: projectreadiness.GapNodeMissing},
		{name: "node unsupported", err: tstoolchain.ErrHostNodeMajorDisallowed, code: projectreadiness.GapNodeUnsupported},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body_projectTsRuntimeTest_18(t, tc)
		})
	}
}

func body_projectTsRuntimeTest_18(t *testing.T, tc struct {
	name string
	err  error
	code string
}) {
	got := mapHostNodeResolveError(tc.err)
	var compilerErr *tstoolchain.CompilerUnresolvedError
	if errors.As(got, &compilerErr) {
		t.Fatalf("mapHostNodeResolveError(%v) satisfied errors.As(*CompilerUnresolvedError) with Code=%q; a host-runtime miss must not enter the compiler-setup offer", tc.err, compilerErr.Code)
	}
	var runtimeErr *tstoolchain.RuntimeUnresolvedError
	if !errors.As(got, &runtimeErr) {
		t.Fatalf("mapHostNodeResolveError(%v) = %T %v, want *RuntimeUnresolvedError", tc.err, got, got)
	}
	if runtimeErr.Code != tc.code {
		t.Fatalf("mapHostNodeResolveError(%v).Code = %q, want %q", tc.err, runtimeErr.Code, tc.code)
	}
}
