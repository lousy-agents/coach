package main

import (
	"github.com/lousy-agents/coach/internal/codesignalcli"
	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/sourcescope"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

// scopedDiff is a diff scan's selection after the scope filter: the files to
// analyze, the groups the filter excluded, and the diagnostics and rename
// continuity the selection carries.
type scopedDiff struct {
	files       []gitrepo.SelectedFile
	excluded    []codesignal.CoverageGroup
	diagnostics []codesignal.Diagnostic
	continuity  []codesignal.PathContinuity
}

// scopeDiffSelection applies the scope filter to a diff scan's committed
// selection. Under --worktree the view does it in one step with the working-tree
// files, so it learns what the filter dropped without the caller telling it.
func scopeDiffSelection(dir, headSHA string, f codesignalFlags, view *codesignalcli.WorktreeView, selection scopedDiff) (scopedDiff, error) {
	var excluded []codesignal.CoverageGroup
	scope := func(files []gitrepo.SelectedFile) ([]gitrepo.SelectedFile, error) {
		kept, groups, err := sourcescope.Apply(dir, headSHA, f.buildTarget, f.scope, files)
		excluded = groups
		return kept, err
	}
	if view == nil {
		kept, err := scope(selection.files)
		selection.files, selection.excluded = kept, excluded
		return selection, err
	}
	worktree, err := view.SelectDiff(headSHA, selection.files, selection.diagnostics, selection.continuity, scope)
	if err != nil {
		return selection, err
	}
	return scopedDiff{files: worktree.Files, excluded: excluded, diagnostics: worktree.Diagnostics, continuity: worktree.Continuity}, nil
}

// scopedBaseline is a baseline scan's files after the scope filter, with the
// coverage that describes them.
type scopedBaseline struct {
	files    []gitrepo.SelectedFile
	coverage codesignal.Coverage
}

// scopeBaselineSelection applies the scope filter to a baseline scan's
// discovered files, through the view under --worktree for the reason above.
func scopeBaselineSelection(dir, revisionSHA string, f codesignalFlags, view *codesignalcli.WorktreeView, discovered []gitrepo.SelectedFile, coverage codesignal.Coverage) (scopedBaseline, error) {
	var excluded []codesignal.CoverageGroup
	scope := func(files []gitrepo.SelectedFile) ([]gitrepo.SelectedFile, error) {
		kept, groups, err := sourcescope.ApplyBaseline(dir, revisionSHA, f.buildTarget, f.scope, files)
		excluded = groups
		return kept, err
	}
	selection := scopedBaseline{files: discovered, coverage: coverage}
	if view == nil {
		kept, err := scope(discovered)
		selection.files = kept
		selection.coverage.Excluded = excluded
		return selection, err
	}
	worktree, err := view.SelectBaseline(discovered, coverage, scope)
	if err != nil {
		return selection, err
	}
	worktree.Coverage.Excluded = excluded
	return scopedBaseline{files: worktree.Files, coverage: worktree.Coverage}, nil
}
