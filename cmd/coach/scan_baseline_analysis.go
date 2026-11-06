package main

import (
	"context"
	"os"

	"github.com/lousy-agents/coach/internal/codesignalcli"
	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

func runBaselineAnalysis(dir string, f codesignalFlags, stdin, stderr *os.File) (*codesignal.Report, error) {
	revisionSHA, err := gitrepo.ResolveBaselineRevision(dir)
	if err != nil {
		return nil, err
	}
	discovered, coverage, err := gitrepo.DiscoverTrackedFiles(dir, revisionSHA)
	if err != nil {
		return nil, err
	}
	status := codesignalcli.ReadWorktreeStatus(dir, f.worktree)
	view := worktreeViewFor(status, f)
	scoped, err := scopeBaselineSelection(dir, revisionSHA, f, view, discovered, coverage)
	if err != nil {
		return nil, err
	}

	project, diags, opErr := prepareScanProjectAnalysis(dir, revisionSHA, f, status, view, stdin, stderr)
	if opErr != nil {
		return nil, opErr
	}
	disclosure := committedWorkingTreeDisclosure(status, f)
	report, err := codesignalcli.AnalyzeBaseline(context.Background(), dir, revisionSHA, scoped.files, disclosure, f.scope, scoped.coverage, project)
	if err != nil {
		return nil, wrapScanAnalysisError(err, dir, revisionSHA, f.projectConfig, stderr)
	}
	return withProjectDiagnostic(report, nil, diags...), nil
}
