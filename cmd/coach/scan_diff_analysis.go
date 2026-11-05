package main

import (
	"context"
	"os"

	"github.com/lousy-agents/coach/internal/codesignalcli"
	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
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
	status := codesignalcli.ReadWorktreeStatus(dir, f.worktree)
	view := worktreeViewFor(status, f)
	scoped, err := scopeDiffSelection(dir, headSHA, f, view, scopedDiff{files: selected, diagnostics: diagnostics, continuity: continuity})
	if err != nil {
		return nil, err
	}
	diagnostics = append(scoped.diagnostics, committedWorkingTreeDisclosure(status, f)...)

	project, diags, opErr := prepareScanProjectAnalysis(dir, headSHA, f, status, view, stdin, stderr)
	if opErr != nil {
		return nil, opErr
	}
	report, err := codesignalcli.AnalyzeChanges(context.Background(), dir, headSHA, mergeBaseSHA, scoped.files, diagnostics, scoped.continuity, f.scope, scoped.excluded, project)
	if err != nil {
		return nil, wrapScanAnalysisError(err, dir, headSHA, f.projectConfig, stderr)
	}
	return withProjectDiagnostic(report, nil, diags...), nil
}
