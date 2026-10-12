package main

import (
	"testing"
)

func TestShouldRenderAfterOptionalPreparation(t *testing.T) {
	cases := []struct {
		name   string
		result optionalPreparationResult
		want   bool
	}{
		{name: "zero value (nothing offered) still renders", result: optionalPreparationResult{}, want: true},
		{name: "declining an optional action still renders", result: optionalPreparationResult{Declined: true}, want: true},
		{name: "a succeeded optional action still renders", result: optionalPreparationResult{Succeeded: true}, want: true},
		{name: "cancelling an optional action does not render", result: optionalPreparationResult{Cancelled: true}, want: false},
		{name: "a failed optional action does not render", result: optionalPreparationResult{Failed: true}, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			expectRenderAfterOptionalPreparation(t, tc.result, tc.want)
		})
	}
}

func expectRenderAfterOptionalPreparation(t *testing.T, result optionalPreparationResult, want bool) {
	t.Helper()
	if got := shouldRenderAfterOptionalPreparation(result); got != want {
		t.Fatalf("shouldRenderAfterOptionalPreparation(%+v) = %v, want %v", result, got, want)
	}
}
