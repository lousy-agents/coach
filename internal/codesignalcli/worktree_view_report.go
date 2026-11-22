package codesignalcli

import (
	"fmt"
	"slices"
	"sort"

	"github.com/lousy-agents/coach/internal/codesignalcli/worktreeoverlay"
	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

// Diagnostics names every path the overlay could not read from disk. Each
// carries its path, so the report counts it as unanalyzed and the verdict is
// qualified. It builds the view itself, because a scan whose analysis read
// nothing through the view has no outcome to report until it exists. A build
// failure is reported by Disclosure, which returns the same memoized error.
func (v *WorktreeView) Diagnostics(dir, revision string) []codesignal.Diagnostic {
	_, _ = v.OpenHead(dir, revision)
	var diagnostics []codesignal.Diagnostic
	for _, name := range v.outcome.SymlinkSkipped {
		diagnostics = append(diagnostics, codesignal.Diagnostic{
			Kind:    codesignal.DiagKindWorktreeSymlinkSkipped,
			Path:    name,
			Message: fmt.Sprintf("%s is a symbolic link in the working tree; coach does not follow it, so the path is absent from this scan", name),
		})
	}
	for _, unreadable := range v.outcome.Unreadable {
		diagnostics = append(diagnostics, codesignal.Diagnostic{
			Kind:    codesignal.DiagKindWorktreeChangesNotAnalyzed,
			Path:    unreadable.Path,
			Message: fmt.Sprintf("%s changed in the working tree but could not be read (%s), so the path is absent from this scan", unreadable.Path, unreadableCause(unreadable.Reason)),
		})
	}
	for _, name := range v.outcome.Changed {
		diagnostics = append(diagnostics, codesignal.Diagnostic{
			Kind:    codesignal.DiagKindWorktreeChangesNotAnalyzed,
			Path:    name,
			Message: fmt.Sprintf("%s changed in the working tree and changed again while it was being read, so the path is absent from this scan", name),
		})
	}
	return diagnostics
}

func unreadableCause(reason string) string {
	switch reason {
	case worktreeoverlay.ReasonPermissionDenied:
		return "permission denied"
	case worktreeoverlay.ReasonNonRegular:
		return "not a regular file"
	case worktreeoverlay.ReasonTooLarge:
		return "over the size limit"
	case worktreeoverlay.ReasonNameMismatch:
		return "its name on disk differs from the name Git records"
	}
	return "the read failed"
}

// ConfigFiles lists the project configuration files the analysis read from
// disk: those that differ from the committed revision and that it opened.
func (v *WorktreeView) ConfigFiles() []string {
	files := []string{}
	for _, name := range v.outcome.Overlaid {
		if projectmodel.IsConfigInput(name) && v.reads != nil && v.reads.wasOpened(name) {
			files = append(files, name)
		}
	}
	sort.Strings(files)
	return files
}

// withWorktreeDiagnostics adds the diagnostics a worktree scan owes once the
// project backend has run: the paths the overlay could not read, and the dirty
// paths nothing analyzed. They wait for the backend because a configuration file
// is accounted for only if the analysis opened it.
func withWorktreeDiagnostics(input codesignal.Input, view *WorktreeView, dir, revision, policyPath string) codesignal.Input {
	extra := append(view.Diagnostics(dir, revision), view.Disclosure(dir, revision, policyPath)...)
	input.Diagnostics = slices.Concat(input.Diagnostics, extra)
	return input
}

func withWorktreeScope(input codesignal.Input, view *WorktreeView) codesignal.Input {
	input.Scope.Worktree = &codesignal.WorktreeScope{
		Revision:    view.revision,
		Modified:    view.counts.Modified,
		Added:       view.counts.Added,
		Deleted:     view.counts.Deleted,
		Untracked:   view.counts.Untracked,
		ConfigFiles: view.ConfigFiles(),
	}
	return input
}

// withWorktree is the one place a --worktree scan adds what it owes to the
// report input: its diagnostics and its scope. A scan without --worktree has no
// view, and the input passes through. The returned input shares no slice with
// the argument.
func withWorktree(input codesignal.Input, project *ProjectAnalysis, dir, revision string) codesignal.Input {
	view := project.worktreeView()
	if view == nil {
		return input
	}
	return withWorktreeScope(withWorktreeDiagnostics(input, view, dir, revision, project.policyPath()), view)
}
