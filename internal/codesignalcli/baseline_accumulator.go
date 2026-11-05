package codesignalcli

import (
	"context"
	"errors"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

// baselineFiles accumulates what the baseline's files contribute to the report:
// their analyzed heads, the diagnostics for those that could not be analyzed,
// and the coverage accounting for both.
type baselineFiles struct {
	analyzer      *semantics.Analyzer
	view          *WorktreeView
	dir           string
	revision      string
	readCommitted func(name string) ([]byte, error)

	changes     []codesignal.FileChange
	diagnostics []codesignal.Diagnostic
	coverage    codesignal.Coverage
}

// add reads and analyzes one file. Its error is a git that did not answer in time (see baselineReadFailure), which
// ends the scan; every other problem with the file is a diagnostic.
func (f *baselineFiles) add(ctx context.Context, sf gitrepo.SelectedFile) error {
	headBytes, err := headFileBytes(f.view, f.dir, f.revision, sf.Path, f.readCommitted)
	if errors.Is(err, errAbsentFromWorktree) {
		f.coverage = f.view.coverageForAbsentSource(f.coverage, sf.Path)
		return nil
	}
	if err != nil {
		failure, stalled := baselineReadFailure(sf.Path, err)
		if stalled != nil {
			return stalled
		}
		f.skip(failure...)
		return nil
	}

	headResult, headErr := f.analyzer.AnalyzeBytes(ctx, semantics.FileInput{Path: sf.Path, Language: sf.Language, Content: headBytes})
	if headErr != nil && !errors.Is(headErr, semantics.ErrSyntax) {
		f.skip(mapSemanticsError(sf.Path, headErr))
		return nil
	}

	f.changes = append(f.changes, codesignal.FileChange{Path: sf.Path, SourceScope: sf.SourceScope, Head: headResult})
	if headResult.ParseStatus == "ok" {
		f.coverage.FilesAnalyzed++
	} else {
		f.coverage.FilesUnanalyzable++
	}
	return nil
}

func (f *baselineFiles) skip(reasons ...codesignal.Diagnostic) {
	f.diagnostics = append(f.diagnostics, reasons...)
	f.coverage.FilesUnanalyzable++
}
