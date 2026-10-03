package codesignal

import (
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func body_sortTest_66(t *testing.T, ranges []LineRange, tc struct {
	name string
	loc  semantics.Location
	want bool
}) {
	if got := overlapsAny(tc.loc, ranges); got != tc.want {
		t.Errorf("overlapsAny(%+v, %+v): got %v, want %v", tc.loc, ranges, got, tc.want)
	}
}
