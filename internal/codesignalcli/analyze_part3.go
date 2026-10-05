package codesignalcli

import (
	"context"
	"errors"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

func analyzeAddedOrModifiedFile(ctx context.Context, analyzer *semantics.Analyzer, dir, headSHA, mergeBaseSHA string, sf SelectedFile) (*codesignal.FileChange, []codesignal.Diagnostic) {
	headBytes, err := runGitBytes(dir, "show", headSHA+":"+sf.Path)
	if err != nil {
		return nil, []codesignal.Diagnostic{readFailedDiagnostic(sf.Path, "head", err)}
	}

	headResult, headErr := analyzer.AnalyzeBytes(ctx, semantics.FileInput{Path: sf.Path, Language: sf.Language, Content: headBytes})
	if headErr != nil && !errors.Is(headErr, semantics.ErrSyntax) {
		return nil, []codesignal.Diagnostic{mapSemanticsError(sf.Path, headErr)}
	}

	fc := codesignal.FileChange{Path: sf.Path, Status: sf.Status, SourceScope: sf.SourceScope, Head: headResult}
	var diagnostics []codesignal.Diagnostic

	if sf.Status == "modified" {
		baseBytes, err := runGitBytes(dir, "show", mergeBaseSHA+":"+sf.Path)
		if err != nil {
			diagnostics = append(diagnostics, readFailedDiagnostic(sf.Path, "base", err))
		} else {
			baseResult, baseErr := analyzer.AnalyzeBytes(ctx, semantics.FileInput{Path: sf.Path, Language: sf.Language, Content: baseBytes})
			switch {
			case baseErr == nil:
				fc.Base = baseResult
			case errors.Is(baseErr, semantics.ErrSyntax):
				diagnostics = append(diagnostics, baseSyntaxDiagnostics(sf.Path, baseErr)...)
			default:
				diagnostics = append(diagnostics, codesignal.Diagnostic{
					Path:    sf.Path,
					Kind:    "base_analysis_failed",
					Message: baseErr.Error(),
				})
			}
		}
	}

	ranges, rangeDiagnostic := computeChangedRanges(dir, mergeBaseSHA, sf.Path)
	if rangeDiagnostic != nil {
		diagnostics = append(diagnostics, *rangeDiagnostic)
	} else {
		fc.ChangedRanges = ranges
	}

	return &fc, diagnostics
}
