package main

import (
	"context"
	"os"

	"github.com/lousy-agents/coach/internal/codesignalcli"
	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/sourcescope"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

func runDiffAnalysis(dir string, f codesignalFlags, stderr *os.File) (*codesignal.Report, error) {
	headSHA, mergeBaseSHA, err := gitrepo.ResolveRevisions(dir, f.base)
	if err != nil {
		return nil, err
	}

	selected, diagnostics, err := gitrepo.SelectChangedFiles(dir, mergeBaseSHA)
	if err != nil {
		return nil, err
	}
	selected, excluded, err := sourcescope.Apply(dir, headSHA, f.buildTarget, f.scope, selected)
	if err != nil {
		return nil, err
	}
	diagnostics = append(diagnostics, codesignalcli.WorkingTreeDisclosureDiagnostics(dir)...)

	project, diag, opErr := prepareProjectAnalysis(dir, headSHA, f.projectConfigSet, f.projectConfig, f.projectLanguage)
	if opErr != nil {
		return nil, opErr
	}
	report, err := codesignalcli.AnalyzeChanges(context.Background(), dir, headSHA, mergeBaseSHA, selected, diagnostics, f.scope, excluded, project)
	if err != nil {
		return nil, wrapScanAnalysisError(err, dir, headSHA, f.projectConfig, stderr)
	}
	return withProjectDiagnostic(report, diag), nil
}
