package main

import (
	"context"
	"os"

	"github.com/lousy-agents/coach/internal/codesignalcli"
	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/sourcescope"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

func runBaselineAnalysis(dir string, f codesignalFlags, stderr *os.File) (*codesignal.Report, error) {
	revisionSHA, err := gitrepo.ResolveBaselineRevision(dir)
	if err != nil {
		return nil, err
	}
	discovered, coverage, err := gitrepo.DiscoverTrackedFiles(dir, revisionSHA)
	if err != nil {
		return nil, err
	}
	kept, excluded, err := sourcescope.ApplyBaseline(dir, revisionSHA, f.buildTarget, f.scope, discovered)
	if err != nil {
		return nil, err
	}
	coverage.Excluded = excluded

	project, diag, opErr := prepareProjectAnalysis(dir, revisionSHA, f.projectConfigSet, f.projectConfig, f.projectLanguage)
	if opErr != nil {
		return nil, opErr
	}
	disclosure := codesignalcli.WorkingTreeDisclosureDiagnostics(dir)
	report, err := codesignalcli.AnalyzeBaseline(context.Background(), dir, revisionSHA, kept, disclosure, f.scope, coverage, project)
	if err != nil {
		return nil, wrapScanAnalysisError(err, dir, revisionSHA, f.projectConfig, stderr)
	}
	return withProjectDiagnostic(report, diag), nil
}
