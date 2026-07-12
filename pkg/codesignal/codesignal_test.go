package codesignal

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

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

func TestBuild_CleanInputProducesSchemaVersion1(t *testing.T) {
	b, err := New(Options{})
	if err != nil {
		t.Fatalf("New returned unexpected error: %v", err)
	}

	report, err := b.Build(context.Background(), Input{})
	if err != nil {
		t.Fatalf("Build returned unexpected error: %v", err)
	}
	if report == nil {
		t.Fatalf("Build returned nil Report")
	}
	if report.SchemaVersion != "1" {
		t.Errorf("Report.SchemaVersion: got %q, want %q", report.SchemaVersion, "1")
	}
	if len(report.Signals) != 0 {
		t.Errorf("Report.Signals: got %d, want 0", len(report.Signals))
	}
	if len(report.Diagnostics) != 0 {
		t.Errorf("Report.Diagnostics: got %d, want 0", len(report.Diagnostics))
	}
	if report.Summary.FilesAnalyzed != 0 {
		t.Errorf("Report.Summary.FilesAnalyzed: got %d, want 0", report.Summary.FilesAnalyzed)
	}
}

func TestBuild_InvalidFileChangeEmitsDiagnostic(t *testing.T) {
	b, err := New(Options{})
	if err != nil {
		t.Fatalf("New returned unexpected error: %v", err)
	}

	input := Input{
		Files: []FileChange{
			{
				Path: "b.go",
				Base: &semantics.Result{Path: "b-base.go"},
			},
			{
				Path: "a.go",
				Head: &semantics.Result{Path: "a-head.go", ParseStatus: semantics.ParseStatus("ok")},
			},
			{
				Path: "c.go",
				Base: &semantics.Result{Path: "c.go"}, // matches, no diagnostic
			},
		},
	}

	report, err := b.Build(context.Background(), input)
	if err != nil {
		t.Fatalf("Build returned unexpected error: %v", err)
	}

	if len(report.Diagnostics) != 2 {
		t.Fatalf("Report.Diagnostics length: got %d, want 2: %+v", len(report.Diagnostics), report.Diagnostics)
	}

	// Sorted by Path: "a.go" before "b.go".
	if report.Diagnostics[0].Path != "a.go" {
		t.Errorf("Diagnostics[0].Path: got %q, want %q", report.Diagnostics[0].Path, "a.go")
	}
	if report.Diagnostics[1].Path != "b.go" {
		t.Errorf("Diagnostics[1].Path: got %q, want %q", report.Diagnostics[1].Path, "b.go")
	}
	for _, d := range report.Diagnostics {
		if d.Kind != "invalid_file_change" {
			t.Errorf("Diagnostic.Kind: got %q, want %q", d.Kind, "invalid_file_change")
		}
		if d.Message == "" {
			t.Errorf("Diagnostic.Message must not be empty for %+v", d)
		}
	}

	if report.Summary.FilesAnalyzed != 3 {
		t.Errorf("Report.Summary.FilesAnalyzed: got %d, want 3", report.Summary.FilesAnalyzed)
	}
	if report.Summary.FilesWithDiagnostics != 2 {
		t.Errorf("Report.Summary.FilesWithDiagnostics: got %d, want 2", report.Summary.FilesWithDiagnostics)
	}
}

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

func TestReport_JSONIncludesSchemaVersionAtTopLevel(t *testing.T) {
	report := Report{SchemaVersion: "1"}

	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshaling a minimal Report must not fail: %v", err)
	}

	var asMap map[string]json.RawMessage
	if err := json.Unmarshal(raw, &asMap); err != nil {
		t.Fatalf("Report JSON must unmarshal into a generic map: %v", err)
	}

	got, ok := asMap["schema_version"]
	if !ok {
		t.Fatalf("Report JSON missing top-level %q key; got keys: %v", "schema_version", asMap)
	}
	if string(got) != `"1"` {
		t.Errorf("Report JSON %q value: got %s, want %q", "schema_version", got, `"1"`)
	}
}
