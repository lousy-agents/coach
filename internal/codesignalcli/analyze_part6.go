package codesignalcli

import (
	"context"
	"errors"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

func analyzeRemovedFile(ctx context.Context, analyzer *semantics.Analyzer, dir, mergeBaseSHA string, sf SelectedFile) (*codesignal.FileChange, []codesignal.Diagnostic) {
	baseBytes, err := runGitBytes(dir, "show", mergeBaseSHA+":"+sf.Path)
	if err != nil {
		return nil, []codesignal.Diagnostic{readFailedDiagnostic(sf.Path, "base", err)}
	}

	baseResult, baseErr := analyzer.AnalyzeBytes(ctx, semantics.FileInput{Path: sf.Path, Language: sf.Language, Content: baseBytes})
	switch {
	case baseErr == nil:
		return &codesignal.FileChange{Path: sf.Path, Status: sf.Status, SourceScope: sf.SourceScope, Base: baseResult}, nil
	case errors.Is(baseErr, semantics.ErrSyntax):
		return nil, baseSyntaxDiagnostics(sf.Path, baseErr)
	default:
		return nil, []codesignal.Diagnostic{mapSemanticsError(sf.Path, baseErr)}
	}
}
