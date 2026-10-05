package codesignal

import (
	"flag"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

var update = flag.Bool("update", false, "regenerate testdata/golden files")

var frozenReportJSONFieldNames = map[string]struct{}{
	"schema_version": {}, "scope": {}, "summary": {}, "signals": {},
	"diagnostics": {}, "coverage": {}, "project_changes": {}, "project_facts": {},
	"project_summary": {}, "project_coverage": {},
	"repository": {}, "revision": {}, "base": {}, "applied_scope": {}, "baseline": {},
	"files_analyzed": {}, "files_with_diagnostics": {}, "files_unanalyzed": {}, "active_signals": {},
	"introduced_signals": {}, "existing_signals": {}, "resolved_signals": {},
	"baseline_signals": {}, "unknown_signals": {},
	"id": {}, "fingerprint": {}, "rule_id": {}, "rule_version": {}, "kind": {},
	"category": {}, "severity": {}, "confidence": {}, "lifecycle": {}, "changed": {},
	"path": {}, "source_scope": {}, "subject": {}, "location": {}, "evidence": {}, "side": {},
	"why_it_matters": {}, "recommendation": {}, "suggested_skill": {}, "provenance": {},
	"machine_evidence": {}, "related_locations": {}, "path_steps": {}, "coverage_refs": {},
	"producer": {}, "finding_kind": {}, "language": {},
	"message":                  {},
	"tracked_files_discovered": {}, "files_unanalyzable": {}, "unsupported": {}, "excluded": {},
	"reason": {}, "count": {},
	"semantic_key": {}, "backend_version": {}, "algorithm_version": {}, "config_digest": {},
	"causal_evidence_digest": {}, "primary_anchor": {},
	"node_id": {}, "display_name": {}, "resolution": {}, "source_locations": {},
	"active_changes": {}, "introduced_changes": {}, "existing_changes": {},
	"resolved_changes": {}, "baseline_changes": {},
	"start_byte": {}, "end_byte": {}, "start_row": {}, "start_col": {}, "end_row": {}, "end_col": {},
	"phase": {}, "complete": {}, "counts": {}, "budgets": {},
	"code": {},

	"project_provenance": {}, "project_scope": {}, "project_next_actions": {},
	"selected_roots": {}, "analyzer": {}, "runtime": {}, "package_manager": {}, "head": {},
	"version": {}, "digest": {}, "protocol_version": {},
	"origin": {}, "compiler_version": {}, "compiler_origin": {}, "declared_version": {},
	"model": {}, "bypass": {}, "reachability": {},
	"inclusion_rule": {}, "pattern_set": {},
	"roots": {}, "matched_layers": {}, "unmatched_layers": {},
	"root": {}, "candidate_files": {}, "analyzed_files": {},
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
// jsonFieldWalker owns the traversal state so field-name collection writes
// the walker's maps rather than caller-owned parameters.
type jsonFieldWalker struct {
	seen map[reflect.Type]bool
	out  map[string]struct{}
}

func (w *jsonFieldWalker) walk(t reflect.Type) {
	switch t.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Array, reflect.Map:
		w.walk(t.Elem())
		return
	case reflect.Struct:
	default:
		return
	}

	if w.seen[t] {
		return
	}
	w.seen[t] = true

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}

		tag, ok := field.Tag.Lookup("json")
		if !ok {
			w.out[field.Name] = struct{}{}
			w.walk(field.Type)
			continue
		}

		name, _, _ := strings.Cut(tag, ",")
		if name == "-" {
			continue
		}
		if name == "" {
			name = field.Name
		}

		w.out[name] = struct{}{}
		w.walk(field.Type)
	}
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

	if *update {
		if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
			t.Fatalf("write golden file %s: %v", goldenPath, err)
		}
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("reading golden file %s: %v\n\nactual output (save this as the golden file if correct, or rerun with -update):\n%s", goldenPath, err, got)
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

func addedFileInput() Input {
	input := hiddenMutationInput()
	input.Files[0].Status = "added"
	return input
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
