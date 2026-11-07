package codesignalcli

import (
	"context"
	"errors"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

// analyzeModifiedBase reads and analyzes the merge-base side of a modified file. A base side that cannot be
// read or analyzed is a diagnostic and no result; the head side is still reported.
func analyzeModifiedBase(ctx context.Context, analyzer *semantics.Analyzer, dir, mergeBaseSHA string, sf gitrepo.SelectedFile) (*semantics.Result, []codesignal.Diagnostic, error) {
	baseBytes, err := runGit(dir, "show", mergeBaseSHA+":"+sf.Path)
	if err != nil {
		diagnostics, stalled := readFailure(sf.Path, modifiedBaseSide, err)
		return nil, diagnostics, stalled
	}

	baseResult, baseErr := analyzer.AnalyzeBytes(ctx, semantics.FileInput{Path: sf.Path, Language: sf.Language, Content: baseBytes})
	switch {
	case baseErr == nil:
		return baseResult, nil, nil
	case errors.Is(baseErr, semantics.ErrSyntax):
		return nil, baseSyntaxDiagnostics(sf.Path, baseErr), nil
	default:
		return nil, []codesignal.Diagnostic{{
			Path:    sf.Path,
			Kind:    "base_analysis_failed",
			Message: baseErr.Error(),
		}}, nil
	}
}
