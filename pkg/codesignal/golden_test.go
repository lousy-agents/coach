package codesignal

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

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

func TestGolden_MinimalReport(t *testing.T) {
	got := buildAndMarshal(t, Input{}, Options{})
	assertMatchesGolden(t, "testdata/golden/minimal_report.json", got)
}

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

func TestGolden_HiddenMutation(t *testing.T) {
	got := buildAndMarshal(t, hiddenMutationInput(), Options{})
	assertMatchesGolden(t, "testdata/golden/hidden_mutation.json", got)
}

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

func TestGolden_LifecycleExcludingResolved(t *testing.T) {
	got := buildAndMarshal(t, lifecycleScenarioInput(), Options{IncludeResolved: false})
	assertMatchesGolden(t, "testdata/golden/lifecycle_excluding_resolved.json", got)
}

func TestGolden_LifecycleIncludingResolved(t *testing.T) {
	got := buildAndMarshal(t, lifecycleScenarioInput(), Options{IncludeResolved: true})
	assertMatchesGolden(t, "testdata/golden/lifecycle_including_resolved.json", got)
}

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

func TestGolden_Diagnostics(t *testing.T) {
	got := buildAndMarshal(t, diagnosticsInput(), Options{})
	assertMatchesGolden(t, "testdata/golden/diagnostics.json", got)
}

func baselineScenarioInput() Input {
	return Input{
		Scope: Scope{Revision: "abc123"},
		Coverage: &Coverage{
			TrackedFilesDiscovered: 12,
			FilesAnalyzed:          10,
			FilesUnanalyzable:      2,
			Unsupported: []CoverageGroup{
				{Reason: "unsupported_language", Language: "python", Count: 1},
			},
			Excluded: []CoverageGroup{
				{Reason: "vendored", Count: 1},
			},
		},
		Files: []FileChange{
			{
				Path:   "pkg/example/service.go",
				Status: "added",
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

func TestGolden_Baseline(t *testing.T) {
	got := buildAndMarshal(t, baselineScenarioInput(), Options{Baseline: true})
	assertMatchesGolden(t, "testdata/golden/baseline_report.json", got)
}

func multiRuleInput() Input {
	return Input{
		Scope: Scope{Repository: "example/repo", Revision: "jkl012", Base: "main"},
		Files: []FileChange{
			{
				Path:   "pkg/example/factory.go",
				Status: "modified",
				Head: &semantics.Result{
					Path:        "pkg/example/factory.go",
					Language:    semantics.LanguageGo,
					ParseStatus: semantics.ParseStatus("ok"),
					Findings: []semantics.Finding{
						{
							Kind:     "tight_coupling",
							Name:     "NewService",
							Location: semantics.Location{StartRow: 1, StartCol: 0, EndRow: 3, EndCol: 1},
						},
						{
							Kind:     "constructor_func",
							Name:     "NewService",
							Location: semantics.Location{StartRow: 1, StartCol: 0, EndRow: 3, EndCol: 1},
						},
						{
							Kind:     "constructor_func",
							Name:     "NewWidget",
							Location: semantics.Location{StartRow: 5, StartCol: 0, EndRow: 7, EndCol: 1},
						},
						{
							Kind: "mutates_input",
							Name: "ApplyDefaults",
							Location: semantics.Location{
								StartRow: 10, StartCol: 0,
								EndRow: 12, EndCol: 1,
							},
							Confidence: "high",
							Evidence:   "cfg.Timeout = defaultTimeout",
						},
					},
				},
				ChangedRanges: []LineRange{{StartRow: 0, EndRow: 20}},
			},
		},
	}
}

func TestGolden_MultiRule(t *testing.T) {
	got := buildAndMarshal(t, multiRuleInput(), Options{})
	assertMatchesGolden(t, "testdata/golden/multi_rule.json", got)
}
