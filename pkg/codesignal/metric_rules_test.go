package codesignal

import (
	"context"
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func metricRuleHead(t *testing.T, ruleID string, metric int) *semantics.Result {
	t.Helper()
	result := &semantics.Result{Path: "f.go", Language: semantics.LanguageGo, ParseStatus: "ok"}
	switch ruleID {
	case cognitiveComplexityRule.ruleID:
		result.CognitiveComplexity = []semantics.FunctionCognitiveComplexity{{Name: "fn", Kind: "function", Score: metric}}
	case branchDensityRule.ruleID:
		result.Metrics.Ifs = metric
	case maxNestingDepthRule.ruleID:
		result.Metrics.MaxNestingDepth = metric
	default:
		t.Fatalf("metric rule %q has no input builder in this guard test", ruleID)
	}
	return result
}

func TestMetricRulesTableDrivesSeverityAndMagnitudeThroughBuild(t *testing.T) {
	seen := map[string]bool{}
	for _, rule := range metricRules {
		if seen[rule.ruleID] {
			t.Fatalf("metric rule %q listed twice", rule.ruleID)
		}
		seen[rule.ruleID] = true

		for _, metric := range []int{rule.threshold, 2*rule.threshold - 1, 2 * rule.threshold} {
			builder, err := New(Options{})
			if err != nil {
				t.Fatal(err)
			}
			report, err := builder.Build(context.Background(), Input{Files: []FileChange{{
				Path: "f.go", Status: "modified", Head: metricRuleHead(t, rule.ruleID, metric),
			}}})
			if err != nil {
				t.Fatal(err)
			}
			var signals []Signal
			for _, sig := range report.Signals {
				if sig.RuleID == rule.ruleID {
					signals = append(signals, sig)
				}
			}
			if len(signals) != 1 {
				t.Fatalf("%s metric=%d: got %d signals, want 1", rule.ruleID, metric, len(signals))
			}
			sig := signals[0]

			ratio, ok := signalMagnitude(sig)
			if want := float64(metric) / float64(rule.threshold); !ok || ratio != want {
				t.Fatalf("%s metric=%d: magnitude = %v, %t from evidence %q, want %v", rule.ruleID, metric, ratio, ok, sig.Evidence, want)
			}
			wantSeverity := Severity("medium")
			if rule.escalates && metric >= highSeverityThresholdMultiple*rule.threshold {
				wantSeverity = "high"
			}
			if sig.Severity != wantSeverity {
				t.Fatalf("%s metric=%d: severity = %q, want %q", rule.ruleID, metric, sig.Severity, wantSeverity)
			}
		}
	}
	if len(seen) == 0 {
		t.Fatal("metric rule table is empty")
	}
}
