package codesignalcli

import (
	"context"
	"errors"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

// analyzeRemovedFile reads and analyzes the merge-base side of a removed file. Its error is a git that did not
// answer in time (see readFailure); every other problem with the file is a diagnostic.
func analyzeRemovedFile(ctx context.Context, analyzer *semantics.Analyzer, dir, mergeBaseSHA string, sf gitrepo.SelectedFile) (*codesignal.FileChange, []codesignal.Diagnostic, error) {
	baseBytes, err := gitrepo.RunBytes(dir, "show", mergeBaseSHA+":"+sf.Path)
	if err != nil {
		diagnostics, stalled := readFailure(sf.Path, "base", err)
		return nil, diagnostics, stalled
	}

	baseResult, baseErr := analyzer.AnalyzeBytes(ctx, semantics.FileInput{Path: sf.Path, Language: sf.Language, Content: baseBytes})
	switch {
	case baseErr == nil:
		return &codesignal.FileChange{Path: sf.Path, Status: sf.Status, SourceScope: sf.SourceScope, Base: baseResult}, nil, nil
	case errors.Is(baseErr, semantics.ErrSyntax):
		return nil, baseSyntaxDiagnostics(sf.Path, baseErr), nil
	default:
		return nil, []codesignal.Diagnostic{mapSemanticsError(sf.Path, baseErr)}, nil
	}
}
