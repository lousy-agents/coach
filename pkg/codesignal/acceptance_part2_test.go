package codesignal_test

import (
	"github.com/lousy-agents/coach/pkg/semantics"
)

func cleanResult(path string, findings ...semantics.Finding) *semantics.Result {
	return &semantics.Result{Path: path, Language: semantics.LanguageGo, ParseStatus: "ok", Findings: findings}
}
