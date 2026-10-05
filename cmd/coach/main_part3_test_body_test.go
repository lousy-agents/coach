package main

import (
	"testing"
)

func body_mainPart3Test_20(t *testing.T, tc struct {
	name   string
	result optionalPreparationResult
	want   bool
}) {
	if got := shouldRenderAfterOptionalPreparation(tc.result); got != tc.want {
		t.Fatalf("shouldRenderAfterOptionalPreparation(%+v) = %v, want %v", tc.result, got, tc.want)
	}
}
