package codesignalcli

import (
	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

// ScopeFilter narrows files to the scope a scan covers and returns those that
// stay. The caller supplies it, because the scope rules live outside the view.
type ScopeFilter func(files []gitrepo.SelectedFile) ([]gitrepo.SelectedFile, error)

// DiffSelection is what a --base --worktree scan analyzes: the files that stay
// after the scope filter, and the diagnostics and rename continuity of the
// committed selection and the working tree together.
type DiffSelection struct {
	Files       []gitrepo.SelectedFile
	Diagnostics []codesignal.Diagnostic
	Continuity  []codesignal.PathContinuity
}

// SelectDiff lays the working tree over the committed selection and applies the
// scope filter. It records which files the filter dropped and which paths an
// unsupported_language diagnostic already names, because the disclosure must not
// report either a second time. Doing both here, in one call, means no caller can
// get the disclosure wrong by skipping or reordering a step. A committed rename
// whose new path the working tree deleted loses its continuity pair and its
// undetermined-continuity diagnostic: nothing is left to carry a finding over to.
func (v *WorktreeView) SelectDiff(headSHA string, committed []gitrepo.SelectedFile, committedDiagnostics []codesignal.Diagnostic, committedContinuity []codesignal.PathContinuity, scope ScopeFilter) (DiffSelection, error) {
	files, diagnostics, continuity, vanished, err := v.diffFiles(headSHA, committed)
	if err != nil {
		return DiffSelection{}, err
	}
	kept, err := scope(files)
	if err != nil {
		return DiffSelection{}, err
	}
	v.scopeExcluded = pathSet(droppedBy(files, kept))
	v.languageNamed = pathSet(unsupportedLanguagePaths(append(append([]codesignal.Diagnostic(nil), committedDiagnostics...), diagnostics...)))
	return DiffSelection{
		Files:       kept,
		Diagnostics: appendUnreported(withoutVanished(committedDiagnostics, vanished), diagnostics),
		Continuity:  append(withoutVanishedPairs(committedContinuity, vanished), continuity...),
	}, nil
}

// BaselineSelection is the files a --baseline --worktree scan analyzes and the
// coverage that describes the working tree's files.
type BaselineSelection struct {
	Files    []gitrepo.SelectedFile
	Coverage codesignal.Coverage
}

// SelectBaseline turns the committed revision's discovered files into the
// working tree's and applies the scope filter, recording the files it dropped so
// they are counted as excluded and not also reported as unanalyzed.
func (v *WorktreeView) SelectBaseline(discovered []gitrepo.SelectedFile, coverage codesignal.Coverage, scope ScopeFilter) (BaselineSelection, error) {
	files, coverage, err := v.baselineFiles(discovered, coverage)
	if err != nil {
		return BaselineSelection{}, err
	}
	kept, err := scope(files)
	if err != nil {
		return BaselineSelection{}, err
	}
	v.scopeExcluded = pathSet(droppedBy(files, kept))
	return BaselineSelection{Files: kept, Coverage: coverage}, nil
}

// droppedBy lists the paths in before that kept no longer holds.
func droppedBy(before, kept []gitrepo.SelectedFile) []string {
	retained := make(map[string]bool, len(kept))
	for _, file := range kept {
		retained[file.Path] = true
	}
	var dropped []string
	for _, file := range before {
		if !retained[file.Path] {
			dropped = append(dropped, file.Path)
		}
	}
	return dropped
}

func unsupportedLanguagePaths(diagnostics []codesignal.Diagnostic) []string {
	var paths []string
	for _, diagnostic := range diagnostics {
		if diagnostic.Kind == "unsupported_language" && diagnostic.Path != "" {
			paths = append(paths, diagnostic.Path)
		}
	}
	return paths
}
