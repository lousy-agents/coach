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
	cases := map[string]struct {
		kind       projectconfig.ErrorKind
		wantOutput bool
	}{
		"a policy missing on disk":                                                      {projectconfig.KindMissing, true},
		"a policy found at neither the revision nor the worktree":                       {projectconfig.KindNotFound, true},
		"a policy that exists and is invalid, which --output would refuse to overwrite": {projectconfig.KindInvalid, false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) { assertAuthoringNamesOutput(t, c.kind, c.wantOutput) })
	}
}

func assertAuthoringNamesOutput(t *testing.T, kind projectconfig.ErrorKind, wantOutput bool) {
	t.Helper()
	const named = "cfg/new.json"
	configErr := &projectconfig.ConfigError{Kind: kind, Remedy: projectreadiness.RemedyAuthor}

	got := authoringRemediation(configErr, policyref.Named(named), false, "go")

	if has := strings.Contains(got, "--output "+named); has != wantOutput || got == "" {
		t.Errorf("authoringRemediation(kind %v) = %q; contains --output %s = %v, want %v, and the remedy is to author so a command is offered", kind, got, named, has, wantOutput)
	}
}
