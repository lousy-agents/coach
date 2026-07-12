package codesignal

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

// buildAndMarshal runs input through a Builder configured with options and
// marshals the resulting *Report the same way TestResult_MarshalMatchesGoldenFile
// does for pkg/semantics.Result: json.MarshalIndent with a trailing newline.
func buildAndMarshal(t *testing.T, input Input, options Options) []byte {
	t.Helper()

	b, err := New(options)
	if err != nil {
		t.Fatalf("New(%+v): %v", options, err)
	}

	report, err := b.Build(context.Background(), input)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	got, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatalf("marshaling Report: %v", err)
	}
	return append(got, '\n')
}

// assertMatchesGolden compares got against the checked-in fixture at
// goldenPath byte-for-byte, printing both on mismatch. If the golden file
// does not exist yet, it fails loudly with the actual output so it can be
// reviewed and saved as the golden fixture (see workflow note in the task
// description: golden files are captured from a real run, never hand-typed,
// since Fingerprint/ID are SHA-256 hashes of Signal fields).
func assertMatchesGolden(t *testing.T, goldenPath string, got []byte) {
	t.Helper()

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("reading golden file %s: %v\n\nactual output (save this as the golden file if correct):\n%s", goldenPath, err, got)
	}

	if string(got) != string(want) {
		t.Errorf("%s: Report JSON must match golden file byte-for-byte.\ngot:\n%s\nwant:\n%s", goldenPath, got, want)
	}
}

// TestGolden_MinimalReport locks the JSON shape of an empty Input's Report:
// the same scenario as TestBuild_CleanInputProducesSchemaVersion1, but
// compared byte-for-byte instead of field-by-field.
func TestGolden_MinimalReport(t *testing.T) {
	got := buildAndMarshal(t, Input{}, Options{})
	assertMatchesGolden(t, "testdata/golden/minimal_report.json", got)
}

// hiddenMutationInput builds the single Head-only mutates_input scenario
// used by TestGolden_HiddenMutation.
func hiddenMutationInput() Input {
	return Input{
		Scope: Scope{Repository: "example/repo", Revision: "abc123", Base: "main"},
		Files: []FileChange{
			{
				Path:   "pkg/example/service.go",
				Status: "modified",
				Head: &semantics.Result{
					Path:        "pkg/example/service.go",
					Language:    semantics.LanguageGo,
					ParseStatus: semantics.ParseStatus("ok"),
					Findings: []semantics.Finding{
						{
							Kind: "mutates_input",
							Name: "ApplyDefaults",
							Location: semantics.Location{
								StartByte: 120, EndByte: 180,
								StartRow: 10, StartCol: 0,
								EndRow: 12, EndCol: 1,
							},
							Confidence:     "high",
							Evidence:       "cfg.Timeout = defaultTimeout",
							Recommendation: "Return a new Config instead of mutating cfg in place.",
							SuggestedSkill: "go-testable-design",
						},
					},
				},
				ChangedRanges: []LineRange{{StartRow: 0, EndRow: 20}},
			},
		},
	}
}

// TestGolden_HiddenMutation locks the JSON shape of a single Head-only
// mutates_input Finding mapped to a hidden_input_mutation Signal. There is
// no Base, so the signal's Lifecycle is "unknown" per the pinned "base
// absent" rule (lifecycle.go's classifyFileSignals) -- that is the expected,
// intentional shape for this fixture, not an oversight.
func TestGolden_HiddenMutation(t *testing.T) {
	got := buildAndMarshal(t, hiddenMutationInput(), Options{})
	assertMatchesGolden(t, "testdata/golden/hidden_mutation.json", got)
}

// lifecycleScenarioInput builds the Base/Head scenario shared by
// TestGolden_LifecycleExcludingResolved and TestGolden_LifecycleIncludingResolved:
// a genuine mix of introduced ("NewOne"), existing ("Existing"), and
// resolved ("GoneNow") signals for one file.
func lifecycleScenarioInput() Input {
	return Input{
		Scope: Scope{Repository: "example/repo", Revision: "def456", Base: "main"},
		Files: []FileChange{
			{
				Path:   "pkg/example/state.go",
				Status: "modified",
				Base: &semantics.Result{
					Path:        "pkg/example/state.go",
					Language:    semantics.LanguageGo,
					ParseStatus: semantics.ParseStatus("ok"),
					Findings: []semantics.Finding{
						{
							Kind:       "mutates_input",
							Name:       "Existing",
							Location:   semantics.Location{StartRow: 1, StartCol: 0, EndRow: 1, EndCol: 20},
							Confidence: "medium",
							Evidence:   "s.Count = s.Count + 1",
						},
						{
							Kind:       "mutates_input",
							Name:       "GoneNow",
							Location:   semantics.Location{StartRow: 2, StartCol: 0, EndRow: 2, EndCol: 20},
							Confidence: "low",
							Evidence:   "s.Stale = true",
						},
					},
				},
				Head: &semantics.Result{
					Path:        "pkg/example/state.go",
					Language:    semantics.LanguageGo,
					ParseStatus: semantics.ParseStatus("ok"),
					Findings: []semantics.Finding{
						{
							Kind:       "mutates_input",
							Name:       "Existing",
							Location:   semantics.Location{StartRow: 10, StartCol: 0, EndRow: 10, EndCol: 20},
							Confidence: "medium",
							Evidence:   "s.Count = s.Count + 1",
						},
						{
							Kind:           "mutates_input",
							Name:           "NewOne",
							Location:       semantics.Location{StartRow: 20, StartCol: 0, EndRow: 20, EndCol: 24},
							Confidence:     "high",
							Evidence:       "s.Cache[k] = v",
							Recommendation: "Return an updated cache instead of mutating s.Cache in place.",
							SuggestedSkill: "go-testable-design",
						},
					},
				},
				ChangedRanges: []LineRange{{StartRow: 5, EndRow: 25}},
			},
		},
	}
}

// TestGolden_LifecycleExcludingResolved and TestGolden_LifecycleIncludingResolved
// render the same Base/Head scenario under both Options.IncludeResolved
// values, as two separate golden files (the issue's task text names a
// single "lifecycle.json", but explicitly requires testing both
// IncludeResolved values -- Signals and Summary.ActiveSignals genuinely
// differ between them, so two files is the only way to golden-test both
// without ambiguity about which one a single "lifecycle.json" would be).
func TestGolden_LifecycleExcludingResolved(t *testing.T) {
	got := buildAndMarshal(t, lifecycleScenarioInput(), Options{IncludeResolved: false})
	assertMatchesGolden(t, "testdata/golden/lifecycle_excluding_resolved.json", got)
}

func TestGolden_LifecycleIncludingResolved(t *testing.T) {
	got := buildAndMarshal(t, lifecycleScenarioInput(), Options{IncludeResolved: true})
	assertMatchesGolden(t, "testdata/golden/lifecycle_including_resolved.json", got)
}

// diagnosticsInput builds the every-diagnostic-kind scenario used by
// TestGolden_Diagnostics.
func diagnosticsInput() Input {
	return Input{
		Scope: Scope{Repository: "example/repo", Revision: "ghi789", Base: "main"},
		Diagnostics: []Diagnostic{
			{Path: "adapter.go", Kind: "analysis_failed", Message: "upstream GitHub API returned 500"},
		},
		Files: []FileChange{
			{
				Path:   "broken.go",
				Status: "modified",
				Head: &semantics.Result{
					Path:        "broken.go",
					Language:    semantics.LanguageGo,
					ParseStatus: semantics.ParseStatus("syntax_errors"),
					SyntaxErrors: []semantics.SyntaxIssue{
						{Kind: "error", Location: semantics.Location{StartRow: 1, StartCol: 2, EndRow: 1, EndCol: 5}},
						{Kind: "missing", Location: semantics.Location{StartRow: 3, StartCol: 0, EndRow: 3, EndCol: 1}},
					},
				},
			},
			{
				Path:   "weird.ts",
				Status: "modified",
				Head: &semantics.Result{
					Path:        "weird.ts",
					Language:    semantics.LanguageTypeScript,
					ParseStatus: semantics.ParseStatus("weird"),
				},
			},
			{
				Path:   "mismatched.go",
				Status: "modified",
				Base: &semantics.Result{
					Path:        "other.go",
					Language:    semantics.LanguageGo,
					ParseStatus: semantics.ParseStatus("ok"),
				},
				Head: &semantics.Result{
					Path:        "mismatched.go",
					Language:    semantics.LanguageGo,
					ParseStatus: semantics.ParseStatus("ok"),
				},
				ChangedRanges: []LineRange{{StartRow: 5, EndRow: 2}},
			},
			{
				Path:   "new_file.go",
				Status: "added",
				Head:   nil,
			},
		},
	}
}

// TestGolden_Diagnostics locks the JSON shape of a Report exercising every
// Diagnostic.Kind this package can produce in one pass: "invalid_file_change"
// (base path mismatch), "syntax_errors" (multi-issue),
// "unsupported_parse_status", "missing_head_result", "invalid_changed_range",
// plus one caller-supplied kind ("analysis_failed") passed through
// Input.Diagnostics as an adapter would for an upstream failure it can't
// analyze at all.
func TestGolden_Diagnostics(t *testing.T) {
	got := buildAndMarshal(t, diagnosticsInput(), Options{})
	assertMatchesGolden(t, "testdata/golden/diagnostics.json", got)
}
