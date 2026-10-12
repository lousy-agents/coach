package codesignal_test

import (
	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

func cleanResult(path string, findings ...semantics.Finding) *semantics.Result {
	return &semantics.Result{Path: path, Language: semantics.LanguageGo, ParseStatus: "ok", Findings: findings}
}

func mutation(name string, row uint) semantics.Finding {
	return semantics.Finding{Kind: "mutates_input", Name: name, Location: semantics.Location{StartRow: row, EndRow: row}, Evidence: "input.value = 1"}
}

func projectChange(key, ruleID string) codesignal.ProjectChange {
	return codesignal.ProjectChange{
		SemanticKey: key,
		RuleID:      ruleID,
		RuleVersion: "1",
		Kind:        "cycle",
		Category:    codesignal.Category("architecture"),
		Severity:    codesignal.Severity("medium"),
		Confidence:  codesignal.Confidence("high"),
		PrimaryAnchor: codesignal.ProjectLocation{
			Path:     "pkg/a/a.go",
			Location: semantics.Location{StartRow: 1},
		},
		Evidence:   "pkg/a -> pkg/b -> pkg/a",
		Provenance: codesignal.Provenance{Producer: "projectmodel"},
	}
}
