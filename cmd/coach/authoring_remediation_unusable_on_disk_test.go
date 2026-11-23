package main

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

func TestAFileOnDiskThatCannotBeUsedForWhatItHoldsGetsTheWordsOnlyInTypeScript(t *testing.T) {
	unusable := &projectconfig.ConfigError{Kind: projectconfig.KindInvalid, Remedy: projectreadiness.RemedyNone, UnusableOnDisk: true}
	refused := &projectconfig.ConfigError{Kind: projectconfig.KindInvalid, Remedy: projectreadiness.RemedyNone}
	want := "coach codesignal: " + unusablePolicyStep + "."
	tests := []struct {
		name     string
		err      *projectconfig.ConfigError
		language string
		want     string
	}{
		{"TypeScript, a file unusable for what it holds", unusable, "typescript", want},
		{"TypeScript, a refusal that is not about what the file holds", refused, "typescript", ""},
		{"Go, a file unusable for what it holds", unusable, "go", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { expectRemediation(t, tt.err, tt.language, tt.want) })
	}
}

func expectRemediation(t *testing.T, err *projectconfig.ConfigError, language, want string) {
	t.Helper()
	if got := authoringRemediation(err, policyref.Named("pol.json"), language, false, ""); got != want {
		t.Fatalf("remediation = %q, want %q", got, want)
	}
}
