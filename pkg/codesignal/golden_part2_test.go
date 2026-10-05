package codesignal

import (
	"context"
	"encoding/json"

	"reflect"

	"testing"

	"github.com/lousy-agents/coach/pkg/domain"
	"github.com/lousy-agents/coach/pkg/semantics"
)

// TestFrozenSchema_FieldNames locks Report's JSON field names (across every
// nested type it can reach) against accidental rename, independent of which
// fields any golden fixture above happens to populate. It walks Go struct
// tags via reflection rather than any single marshaled Report, so the same
// assertion holds regardless of whether a field is present-but-empty or
// omitted in a given fixture. Issue #269 (#308) made Report's own
// slice/map/pointer fields always-present rather than `omitempty`; this
// test needed no changes when that landed, exactly as designed -- a
// presence/absence change is not a rename.
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
	(&jsonFieldWalker{seen: map[reflect.Type]bool{}, out: got}).walk(reflect.TypeOf(Report{}))

	t.Run("NoFrozenNameMissing", func(t *testing.T) {
		body_goldenPart2Test_NoFrozenNameMissing_35(t, got)
	})
	t.Run("NoUnexpectedNameAppeared", func(t *testing.T) {
		body_goldenPart2Test_NoUnexpectedNameAppeared_42(t, got)
	})
}

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

// projectLifecycleChange returns a minimal architecture.layer_bypass
// ProjectChange tagged with the source language that produced it (via
// Provenance.Language), standing in for
// EvaluateGoLayerBypass/EvaluateTypeScriptLayerBypass output. It exists to
// prove (AC-VER-2, Task 11 of issue #334) that classifyProjectChanges
// (project_lifecycle.go) assigns the same five frozen lifecycle wire values
// -- introduced, existing, resolved, baseline, unknown -- regardless of
// which language produced the observation; language is carried only as
// provenance, never folded into lifecycle identity.
func projectLifecycleChange(key, language string) ProjectChange {
	return ProjectChange{
		SemanticKey: key,
		RuleID:      "architecture.layer_bypass",
		RuleVersion: "1",
		Kind:        "architecture.layer_bypass",
		Category:    Category("architecture"),
		Severity:    Severity("advisory"),
		Confidence:  Confidence("high"),
		PrimaryAnchor: ProjectLocation{
			Path:     "pkg/handlers/handlers.go",
			Location: semantics.Location{StartRow: 3},
		},
		Evidence:   "handler reaches sink via a statically resolved path that never passes through required layer \"service\"",
		Provenance: Provenance{Producer: "projectmodel", FindingKind: "architecture.layer_bypass", Language: language},
	}
}

// projectLifecycleDiffInput exercises existing/introduced/resolved together:
// a Go-sourced key present on both sides (existing), a TypeScript-sourced
// key present only on head (introduced), and a Go-sourced key present only
// on base (resolved).
func projectLifecycleDiffInput() Input {
	return Input{
		Scope: Scope{Repository: "example/repo", Revision: "pqr678", Base: "main"},
		ProjectChanges: []ProjectChange{
			projectLifecycleChange("bypass:service:go-A", "go"),
			projectLifecycleChange("bypass:service:ts-B", "typescript"),
		},
		BaseProjectChanges: []ProjectChange{
			projectLifecycleChange("bypass:service:go-A", "go"),
			projectLifecycleChange("bypass:service:go-C", "go"),
		},
		ProjectBaseAnalyzed: true,
		ProjectCoverage:     &domain.Coverage{Phase: "full", Complete: true},
		BaseProjectCoverage: &domain.Coverage{Phase: "full", Complete: true},
	}
}

// projectLifecycleBaselineInput exercises baseline: a TypeScript-sourced,
// head-only key with complete coverage in Options.Baseline mode.
func projectLifecycleBaselineInput() Input {
	return Input{
		Scope:           Scope{Repository: "example/repo", Revision: "stu901"},
		ProjectChanges:  []ProjectChange{projectLifecycleChange("bypass:service:ts-D", "typescript")},
		ProjectCoverage: &domain.Coverage{Phase: "full", Complete: true},
	}
}

// projectLifecycleUnknownInput exercises unknown: a Go-sourced, head-only
// key degraded to indeterminate lifecycle by incomplete project coverage.
func projectLifecycleUnknownInput() Input {
	return Input{
		Scope:           Scope{Repository: "example/repo", Revision: "vwx234"},
		ProjectChanges:  []ProjectChange{projectLifecycleChange("bypass:service:go-E", "go")},
		ProjectCoverage: &domain.Coverage{Phase: "full", Complete: false},
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
		{"AddedFile", addedFileInput(), Options{}, "testdata/golden/added_file.json"},
		{"LifecycleExcludingResolved", lifecycleScenarioInput(), Options{IncludeResolved: false}, "testdata/golden/lifecycle_excluding_resolved.json"},
		{"LifecycleIncludingResolved", lifecycleScenarioInput(), Options{IncludeResolved: true}, "testdata/golden/lifecycle_including_resolved.json"},
		{"Diagnostics", diagnosticsInput(), Options{}, "testdata/golden/diagnostics.json"},
		{"Baseline", baselineScenarioInput(), Options{Baseline: true}, "testdata/golden/baseline_report.json"},
		{"MultiRule", multiRuleInput(), Options{}, "testdata/golden/multi_rule.json"},
		{"MetricsRules", metricsRulesInput(), Options{}, "testdata/golden/metrics_rules.json"},
		{"ProjectLifecycleDiffGoAndTS", projectLifecycleDiffInput(), Options{ProjectEnabled: true, IncludeResolved: true}, "testdata/golden/project_lifecycle_diff.json"},
		{"ProjectLifecycleBaselineTS", projectLifecycleBaselineInput(), Options{ProjectEnabled: true, Baseline: true}, "testdata/golden/project_lifecycle_baseline.json"},
		{"ProjectLifecycleUnknownGo", projectLifecycleUnknownInput(), Options{ProjectEnabled: true, Baseline: true}, "testdata/golden/project_lifecycle_unknown.json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildAndMarshal(t, tt.input, tt.options)
			assertMatchesGolden(t, tt.goldenPath, got)
		})
	}
}
