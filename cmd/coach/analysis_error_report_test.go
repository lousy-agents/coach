package main

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
)

func TestAnalysisErrorReportForRuntimeUnresolvedError(t *testing.T) {
	err := &tstoolchain.RuntimeUnresolvedError{Code: projectreadiness.GapNodeMissing, Policy: policyref.Named("project.json")}
	got := analysisErrorReportFor(err, "typescript", policyref.Ref{}, false)
	if got.exitCode != 2 {
		t.Fatalf("analysisErrorReportFor(RuntimeUnresolvedError).exitCode = %d, want 2", got.exitCode)
	}
	if len(got.lines) != 1 {
		t.Fatalf("analysisErrorReportFor(RuntimeUnresolvedError).lines = %q, want exactly the gap line", got.lines)
	}
	if got.lines[0] != err.RemediationLine() {
		t.Fatalf("analysisErrorReportFor(RuntimeUnresolvedError).lines[0] = %q, want %q", got.lines[0], err.RemediationLine())
	}
}

func TestAuthoringRemediationNamesTheOutputOnlyWhereTheNamedPathIsAbsent(t *testing.T) {
	const named = "cfg/new.json"
	const withOutput = "--output " + named
	cases := []struct {
		name       string
		kind       projectconfig.ErrorKind
		wantOutput bool
	}{
		{"a policy missing on disk", projectconfig.KindMissing, true},
		{"a policy found at neither the revision nor the worktree", projectconfig.KindNotFound, true},
		{"a policy that exists and is invalid, which --output would refuse to overwrite", projectconfig.KindInvalid, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			configErr := &projectconfig.ConfigError{Kind: c.kind, Remedy: projectreadiness.RemedyAuthor}

			got := authoringRemediation(configErr, policyref.Named(named), false, "go")

			if has := strings.Contains(got, withOutput); has != c.wantOutput {
				t.Errorf("authoringRemediation(kind %v) = %q; contains %q = %v, want %v", c.kind, got, withOutput, has, c.wantOutput)
			}
			if got == "" {
				t.Errorf("authoringRemediation(kind %v) offered no command; the remedy is to author", c.kind)
			}
		})
	}
}
