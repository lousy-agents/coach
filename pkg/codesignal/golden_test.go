package codesignal

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strings"
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

// assertMatchesGolden's byte-for-byte comparison deliberately locks key
// ORDER too, not just which keys appear: encoding/json always marshals
// struct fields in declaration order, so any field reordering in
// report.go/coverage_types.go/project_report.go changes this comparison.
// That is intentional, not incidental -- a reviewer reordering struct
// fields for readability should expect this test to fail and regenerate
// the golden file deliberately, not be surprised by it.
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

func metricsRulesInput() Input {
	return Input{
		Scope: Scope{Repository: "example/repo", Revision: "mno345", Base: "main"},
		Files: []FileChange{
			{
				Path:   "pkg/example/tangled.go",
				Status: "modified",
				Head: &semantics.Result{
					Path:        "pkg/example/tangled.go",
					Language:    semantics.LanguageGo,
					ParseStatus: semantics.ParseStatus("ok"),
					Metrics: semantics.StructuralMetrics{
						Ifs:             4,
						Fors:            3,
						ExprSwitches:    2,
						TypeSwitches:    2,
						Selects:         1,
						MaxNestingDepth: 5,
					},
				},
				ChangedRanges: []LineRange{{StartRow: 0, EndRow: 40}},
			},
		},
	}
}

func TestGolden(t *testing.T) {
	tests := []struct {
		name       string
		input      Input
		options    Options
		goldenPath string
	}{
		{"MinimalReport", Input{}, Options{}, "testdata/golden/minimal_report.json"},
		{"HiddenMutation", hiddenMutationInput(), Options{}, "testdata/golden/hidden_mutation.json"},
		{"LifecycleExcludingResolved", lifecycleScenarioInput(), Options{IncludeResolved: false}, "testdata/golden/lifecycle_excluding_resolved.json"},
		{"LifecycleIncludingResolved", lifecycleScenarioInput(), Options{IncludeResolved: true}, "testdata/golden/lifecycle_including_resolved.json"},
		{"Diagnostics", diagnosticsInput(), Options{}, "testdata/golden/diagnostics.json"},
		{"Baseline", baselineScenarioInput(), Options{Baseline: true}, "testdata/golden/baseline_report.json"},
		{"MultiRule", multiRuleInput(), Options{}, "testdata/golden/multi_rule.json"},
		{"MetricsRules", metricsRulesInput(), Options{}, "testdata/golden/metrics_rules.json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildAndMarshal(t, tt.input, tt.options)
			assertMatchesGolden(t, tt.goldenPath, got)
		})
	}
}

// reflectJSONFieldNames walks t's json struct tags, following pointers,
// slices, arrays, and map values (never map keys, which are caller data, not
// schema) across package boundaries, and records every field's tag name (the
// part before any comma) in out. It returns a flat set rather than a
// per-type map: the frozen-name assertion below only needs to know which
// names can ever appear in Report's JSON output, not which struct owns each
// one, so a struct-tag rename is caught even for a field that happens to be
// zero/empty (and therefore invisible via omitempty) in every golden
// fixture above.
func reflectJSONFieldNames(t reflect.Type, seen map[reflect.Type]bool, out map[string]struct{}) {
	switch t.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Array, reflect.Map:
		reflectJSONFieldNames(t.Elem(), seen, out)
		return
	case reflect.Struct:
	default:
		return
	}

	if seen[t] {
		return
	}
	seen[t] = true

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}

		tag, ok := field.Tag.Lookup("json")
		if !ok {
			out[field.Name] = struct{}{}
			reflectJSONFieldNames(field.Type, seen, out)
			continue
		}

		name, _, _ := strings.Cut(tag, ",")
		if name == "-" {
			continue
		}
		if name == "" {
			name = field.Name
		}

		out[name] = struct{}{}
		reflectJSONFieldNames(field.Type, seen, out)
	}
}

var frozenReportJSONFieldNames = map[string]struct{}{
	// Report
	"schema_version": {}, "scope": {}, "summary": {}, "signals": {},
	"diagnostics": {}, "coverage": {}, "project_changes": {}, "project_facts": {},
	"project_summary": {}, "project_coverage": {},
	// Scope
	"repository": {}, "revision": {}, "base": {}, "applied_scope": {}, "baseline": {},
	// Summary
	"files_analyzed": {}, "files_with_diagnostics": {}, "active_signals": {},
	"introduced_signals": {}, "existing_signals": {}, "resolved_signals": {},
	"baseline_signals": {},
	// Signal
	"id": {}, "fingerprint": {}, "rule_id": {}, "rule_version": {}, "kind": {},
	"category": {}, "severity": {}, "confidence": {}, "lifecycle": {}, "changed": {},
	"path": {}, "source_scope": {}, "subject": {}, "location": {}, "evidence": {},
	"why_it_matters": {}, "recommendation": {}, "suggested_skill": {}, "provenance": {},
	"machine_evidence": {}, "related_locations": {}, "path_steps": {}, "coverage_refs": {},
	// Provenance
	"producer": {}, "finding_kind": {}, "language": {},
	// Diagnostic
	"message": {},
	// Coverage
	"tracked_files_discovered": {}, "files_unanalyzable": {}, "unsupported": {}, "excluded": {},
	// CoverageGroup
	"reason": {}, "count": {},
	// ProjectChange
	"semantic_key": {}, "backend_version": {}, "algorithm_version": {}, "config_digest": {},
	"causal_evidence_digest": {}, "primary_anchor": {},
	// ProjectPathStep
	"node_id": {}, "display_name": {}, "resolution": {}, "source_locations": {},
	// ProjectSummary
	"active_changes": {}, "introduced_changes": {}, "existing_changes": {},
	"resolved_changes": {}, "baseline_changes": {},
	// semantics.Location
	"start_byte": {}, "end_byte": {}, "start_row": {}, "start_col": {}, "end_row": {}, "end_col": {},
	// projectmodel.Coverage
	"phase": {}, "complete": {}, "counts": {}, "budgets": {},
	// projectmodel.Diagnostic
	"code": {},
}

// TestFrozenSchema_FieldNames locks Report's JSON field names (across every
// nested type it can reach) against accidental rename, independent of which
// fields any golden fixture above happens to populate. It walks Go struct
// tags via reflection rather than any single marshaled Report, so it does
// not marshal a Report at all and therefore takes no position on whether an
// empty collection field is present (`[]`/`{}`) or omitted in JSON output --
// today Report's slice/map/pointer fields carry `omitempty`, so this test
// tolerates that omission by construction. Issue #269 proposes making empty
// collections always-present; when it lands, the field *names* enumerated in
// frozenReportJSONFieldNames do not change (a presence/absence change is not
// a rename) and this test should not need edits -- only the golden fixtures
// in testdata/golden/ regenerate. Do not read this test's silence on
// presence/absence as a decision that omission is the frozen contract.
//
// This test freezes JSON field names only, for Report's JSON rendering path.
// It says nothing about text-format output: internal/codesignalcli/render.go's
// RenderText is pinned byte-for-byte only for the single scenario in
// internal/codesignalcli/render_test.go's
// TestRenderTextSignalsPresentRenderingIsPinnedExactly; there is no
// exhaustive, field-name-level freeze of the text schema equivalent to this
// test. That narrower gap is not something this test (or #218/T7, scoped to
// golden_test.go and README.md) covers -- do not read this test's presence as
// evidence that the "text" half of AC-3 is satisfied.
func TestFrozenSchema_FieldNames(t *testing.T) {
	got := map[string]struct{}{}
	reflectJSONFieldNames(reflect.TypeOf(Report{}), map[reflect.Type]bool{}, got)

	t.Run("NoFrozenNameMissing", func(t *testing.T) {
		for name := range frozenReportJSONFieldNames {
			if _, ok := got[name]; !ok {
				t.Errorf("expected JSON field %q not found among Report's reachable struct tags (renamed or removed?)", name)
			}
		}
	})
	t.Run("NoUnexpectedNameAppeared", func(t *testing.T) {
		for name := range got {
			if _, ok := frozenReportJSONFieldNames[name]; !ok {
				t.Errorf("unexpected JSON field %q found among Report's reachable struct tags (new field, or a rename, not reflected in frozenReportJSONFieldNames)", name)
			}
		}
	})
}
