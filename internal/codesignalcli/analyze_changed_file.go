package codesignalcli

import (
	"context"
	"errors"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

func analyzeAddedOrModifiedFile(ctx context.Context, analyzer *semantics.Analyzer, dir, headSHA, mergeBaseSHA string, sf gitrepo.SelectedFile, view *WorktreeView) (*codesignal.FileChange, []codesignal.Diagnostic) {
	headBytes, err := headFileBytes(view, dir, headSHA, sf.Path, func(name string) ([]byte, error) {
		return gitrepo.RunBytes(dir, "show", headSHA+":"+name)
	})
	if errors.Is(err, errAbsentFromWorktree) {
		return nil, nil
	}
	if err != nil {
		return nil, []codesignal.Diagnostic{readFailedDiagnostic(sf.Path, "head", err)}
	}

	headResult, headErr := analyzer.AnalyzeBytes(ctx, semantics.FileInput{Path: sf.Path, Language: sf.Language, Content: headBytes})
	if headErr != nil && !errors.Is(headErr, semantics.ErrSyntax) {
		return nil, []codesignal.Diagnostic{mapSemanticsError(sf.Path, headErr)}
	}

	fc := codesignal.FileChange{Path: sf.Path, Status: sf.Status, SourceScope: sf.SourceScope, Head: headResult}
	base := analyzeBaseSide(ctx, analyzer, dir, mergeBaseSHA, sf)
	fc.Base = base.result

	ranges, rangeDiagnostics := changedRangesFor(view, dir, mergeBaseSHA, sf, base.content, headBytes)
	fc.ChangedRanges = ranges
	if view != nil && sf.Status == "added" && len(fc.ChangedRanges) == 0 {
		fc.ChangedRanges = wholeFileRange(headBytes)
	}

	return &fc, append(base.diagnostics, rangeDiagnostics...)
}

// baseSide is a modified file as it stood at the merge base. A base that
// cannot be read or analyzed leaves result nil and says why in diagnostics.
type baseSide struct {
	content     []byte
	result      *semantics.Result
	diagnostics []codesignal.Diagnostic
}

func analyzeBaseSide(ctx context.Context, analyzer *semantics.Analyzer, dir, mergeBaseSHA string, sf gitrepo.SelectedFile) baseSide {
	if sf.Status != "modified" {
		return baseSide{}
	}
	content, err := gitrepo.RunBytes(dir, "show", mergeBaseSHA+":"+sf.Path)
	if err != nil {
		return baseSide{content: content, diagnostics: []codesignal.Diagnostic{readFailedDiagnostic(sf.Path, "base", err)}}
	}

	result, analysisErr := analyzer.AnalyzeBytes(ctx, semantics.FileInput{Path: sf.Path, Language: sf.Language, Content: content})
	switch {
	case analysisErr == nil:
		return baseSide{content: content, result: result}
	case errors.Is(analysisErr, semantics.ErrSyntax):
		return baseSide{content: content, diagnostics: baseSyntaxDiagnostics(sf.Path, analysisErr)}
	default:
		return baseSide{content: content, diagnostics: []codesignal.Diagnostic{{
			Path:    sf.Path,
			Kind:    "base_analysis_failed",
			Message: analysisErr.Error(),
		}}}
	}
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
func changedRangesFor(view *WorktreeView, dir, mergeBaseSHA string, sf gitrepo.SelectedFile, baseBytes, headBytes []byte) ([]codesignal.LineRange, []codesignal.Diagnostic) {
	switch {
	case !view.ReadFromDisk(sf.Path):
		return committedChangedRanges(dir, mergeBaseSHA, sf.Path)
	case sf.Status == "modified":
		return changedRangesBetween(baseBytes, headBytes), nil
	default:
		return wholeFileRange(headBytes), nil
	}
}

func committedChangedRanges(dir, mergeBaseSHA, path string) ([]codesignal.LineRange, []codesignal.Diagnostic) {
	ranges, diagnostic := computeChangedRanges(dir, mergeBaseSHA, path)
	if diagnostic != nil {
		return nil, []codesignal.Diagnostic{*diagnostic}
	}
	return ranges, nil
}
