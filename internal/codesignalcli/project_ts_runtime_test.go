package codesignalcli

import (
	"errors"
	"testing"
)

func TestMapHostNodeResolveErrorIsNotACompilerUnresolvedError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		code string
	}{
		{name: "node missing", err: errHostNodeNotFound, code: GapNodeMissing},
		{name: "node unsupported", err: errHostNodeMajorDisallowed, code: GapNodeUnsupported},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mapHostNodeResolveError(tc.err)
			var compilerErr *CompilerUnresolvedError
			if errors.As(got, &compilerErr) {
				t.Fatalf("mapHostNodeResolveError(%v) satisfied errors.As(*CompilerUnresolvedError) with Code=%q; a host-runtime miss must not enter the compiler-setup offer", tc.err, compilerErr.Code)
			}
			var runtimeErr *RuntimeUnresolvedError
			if !errors.As(got, &runtimeErr) {
				t.Fatalf("mapHostNodeResolveError(%v) = %T %v, want *RuntimeUnresolvedError", tc.err, got, got)
			}
			if runtimeErr.Code != tc.code {
				t.Fatalf("mapHostNodeResolveError(%v).Code = %q, want %q", tc.err, runtimeErr.Code, tc.code)
			}
		})
	}
}
