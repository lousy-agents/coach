package main

import (
	"context"
	"os"

	"github.com/lousy-agents/coach/internal/codesignalcli"
	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/sourcescope"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

func runDiffAnalysis(dir string, f codesignalFlags, stdin, stderr *os.File) (*codesignal.Report, error) {
	headSHA, mergeBaseSHA, err := gitrepo.ResolveRevisions(dir, f.base)
	if err != nil {
		return nil, err
	}

	selected, diagnostics, continuity, err := gitrepo.SelectChangedFiles(dir, mergeBaseSHA, headSHA)
	if err != nil {
		return nil, err
	}
	selected, excluded, err := sourcescope.Apply(dir, headSHA, f.buildTarget, f.scope, selected)
	if err != nil {
		return nil, err
	}
	diagnostics = append(diagnostics, codesignalcli.WorkingTreeDisclosureDiagnostics(dir)...)

	project, diags, restoreStatus, opErr := prepareScanProjectAnalysis(dir, headSHA, f, stdin, stderr)
	defer restoreStatus()
	if opErr != nil {
		return nil, opErr
	}
	report, err := codesignalcli.AnalyzeChanges(context.Background(), dir, headSHA, mergeBaseSHA, selected, diagnostics, continuity, f.scope, excluded, project)
	if err != nil {
		return nil, wrapScanAnalysisError(err, dir, headSHA, f.projectConfig, stderr)
	}
	return withProjectDiagnostic(report, nil, diags...), nil
}
