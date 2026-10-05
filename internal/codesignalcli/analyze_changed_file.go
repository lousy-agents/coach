package codesignalcli

import (
	"context"
	"errors"
	"fmt"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

func analyzeAddedOrModifiedFile(ctx context.Context, analyzer *semantics.Analyzer, dir, headSHA, mergeBaseSHA string, sf gitrepo.SelectedFile) (*codesignal.FileChange, []codesignal.Diagnostic) {
	headBytes, err := gitrepo.RunBytes(dir, "show", headSHA+":"+sf.Path)
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
		baseBytes, err := gitrepo.RunBytes(dir, "show", mergeBaseSHA+":"+sf.Path)
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

func computeChangedRanges(dir, mergeBaseSHA, path string) ([]codesignal.LineRange, *codesignal.Diagnostic) {
	output, err := gitrepo.RunBytes(dir, "diff", "--unified=0", "--no-ext-diff", mergeBaseSHA, "HEAD", "--", path)
	if err != nil {
		return nil, &codesignal.Diagnostic{
			Path:    path,
			Kind:    "diff_analysis_failed",
			Message: fmt.Sprintf("computing changed ranges for %q: %s", path, err),
		}
	}

	ranges, err := parseChangedRanges(output)
	if err != nil {
		return nil, &codesignal.Diagnostic{
			Path:    path,
			Kind:    "diff_analysis_failed",
			Message: fmt.Sprintf("parsing diff for %q: %s", path, err),
		}
	}
	return ranges, nil
}
