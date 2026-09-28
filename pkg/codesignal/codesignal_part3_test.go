package codesignal

import (
	"context"

	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestBuild_DoesNotMutateInput(t *testing.T) {
	b, err := New(Options{})
	if err != nil {
		t.Fatalf("New returned unexpected error: %v", err)
	}

	callerDiagnostics := []Diagnostic{
		{Path: "z.go", Kind: "custom", Message: "hello"},
	}
	input := Input{
		Files: []FileChange{
			{Path: "m.go", Base: &semantics.Result{Path: "wrong.go"}},
		},
		Diagnostics: callerDiagnostics,
	}

	if _, err := b.Build(context.Background(), input); err != nil {
		t.Fatalf("Build returned unexpected error: %v", err)
	}

	if len(callerDiagnostics) != 1 {
		t.Errorf("caller's Diagnostics slice must not be mutated in place; got length %d, want 1", len(callerDiagnostics))
	}
	if callerDiagnostics[0].Path != "z.go" || callerDiagnostics[0].Kind != "custom" {
		t.Errorf("caller's Diagnostics slice contents must not be mutated; got %+v", callerDiagnostics[0])
	}
}

func TestBuild_RespectsContextCancellation(t *testing.T) {
	b, err := New(Options{})
	if err != nil {
		t.Fatalf("New returned unexpected error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	report, err := b.Build(ctx, Input{})
	if err == nil {
		t.Fatalf("Build with a canceled context must return an error")
	}
	if err != context.Canceled {
		t.Errorf("Build error: got %v, want %v", err, context.Canceled)
	}
	if report != nil {
		t.Errorf("Build with a canceled context must return a nil Report, got %+v", report)
	}
}

func TestNew_DoesNotAliasOrMutateOptions(t *testing.T) {
	opts := Options{IncludeResolved: true}

	b, err := New(opts)
	if err != nil {
		t.Fatalf("New(%+v) returned unexpected error: %v", opts, err)
	}
	if b == nil {
		t.Fatalf("New(%+v) returned nil Builder", opts)
	}

	opts.IncludeResolved = false
	if !b.options.IncludeResolved {
		t.Errorf("Builder.options must be a copy of the passed Options, not an alias; mutating caller's Options after New changed b.options")
	}
}
