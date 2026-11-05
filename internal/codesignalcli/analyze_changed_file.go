package codesignalcli

import (
	"context"
	"errors"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

// analyzeAddedOrModifiedFile reads and analyzes one changed file. Its error is a git that did not answer in
// time (see readFailure); every other problem with the file is a diagnostic.
func analyzeAddedOrModifiedFile(run gitrepo.Runner, ctx context.Context, analyzer *semantics.Analyzer, dir, headSHA, mergeBaseSHA string, sf gitrepo.SelectedFile, view *WorktreeView) (*codesignal.FileChange, []codesignal.Diagnostic, error) {
	headBytes, err := headFileBytes(view, dir, headSHA, sf.Path, func(name string) ([]byte, error) {
		return run(dir, "show", headSHA+":"+name)
	})
	if errors.Is(err, errAbsentFromWorktree) {
		return nil, nil, nil
	}
	if err != nil {
		diagnostics, stalled := readFailure(sf.Path, headSide, err)
		return nil, diagnostics, stalled
	}

	headResult, headErr := analyzer.AnalyzeBytes(ctx, semantics.FileInput{Path: sf.Path, Language: sf.Language, Content: headBytes})
	if headErr != nil && !errors.Is(headErr, semantics.ErrSyntax) {
		return nil, []codesignal.Diagnostic{mapSemanticsError(sf.Path, headErr)}, nil
	}

	fc := codesignal.FileChange{Path: sf.Path, Status: sf.Status, SourceScope: sf.SourceScope, Head: headResult}
	var diagnostics []codesignal.Diagnostic

	var base baseSide
	if sf.Status == "modified" {
		var stalled error
		base, stalled = analyzeModifiedBase(run, ctx, analyzer, dir, mergeBaseSHA, sf)
		if stalled != nil {
			return nil, nil, stalled
		}
		fc.Base = base.result
		diagnostics = append(diagnostics, base.diagnostics...)
	}

	ranges, rangeDiagnostics, stalled := changedRangesFor(run, view, dir, mergeBaseSHA, sf, base.content, headBytes)
	if stalled != nil {
		return nil, nil, stalled
	}
	fc.ChangedRanges = ranges
	if view != nil && sf.Status == "added" && len(fc.ChangedRanges) == 0 {
		fc.ChangedRanges = wholeFileRange(headBytes)
	}

	return &fc, append(diagnostics, rangeDiagnostics...), nil
}

// wholeFileRange covers every line of an added file. git diff does not list an
// untracked file, so its changed ranges cannot come from the diff.
func wholeFileRange(content []byte) []codesignal.LineRange {
	lines := len(splitLines(content))
	if lines == 0 {
		return nil
	}
	return []codesignal.LineRange{{StartRow: 0, EndRow: uint(lines - 1)}}
}

// changedRangesFor takes the ranges from git when the head is committed, and
// from the bytes already read when the scan read the head from disk. A file that
// is new at the base has no base bytes to compare, so all of it is changed.
func changedRangesFor(run gitrepo.Runner, view *WorktreeView, dir, mergeBaseSHA string, sf gitrepo.SelectedFile, baseBytes, headBytes []byte) ([]codesignal.LineRange, []codesignal.Diagnostic, error) {
	switch {
	case !view.ReadFromDisk(sf.Path):
		return committedChangedRanges(run, dir, mergeBaseSHA, sf.Path)
	case sf.Status == "modified":
		return changedRangesBetween(baseBytes, headBytes), nil, nil
	default:
		return wholeFileRange(headBytes), nil, nil
	}
}

func committedChangedRanges(run gitrepo.Runner, dir, mergeBaseSHA, path string) ([]codesignal.LineRange, []codesignal.Diagnostic, error) {
	ranges, diagnostic, stalled := computeChangedRanges(run, dir, mergeBaseSHA, path)
	if stalled != nil {
		return nil, nil, stalled
	}
	if diagnostic != nil {
		return nil, []codesignal.Diagnostic{*diagnostic}, nil
	}
	return ranges, nil, nil
}
