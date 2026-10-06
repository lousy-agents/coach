package codesignal

import (
	"context"
	"testing"
)

func mustBuild(t *testing.T, options Options, input Input) *Report {
	t.Helper()
	b, err := New(options)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	report, err := b.Build(context.Background(), input)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return report
}
