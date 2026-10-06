package codesignal

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"testing"
)

var update = flag.Bool("update", false, "regenerate testdata/golden files")

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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildAndMarshal(t, tt.input, tt.options)
			assertMatchesGolden(t, tt.goldenPath, got)
		})
	}
}
