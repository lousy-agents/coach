package codesignalcli

import (
	"context"
	"errors"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

func AnalyzeChanges(ctx context.Context, dir, headSHA, mergeBaseSHA string, files []gitrepo.SelectedFile, extraDiagnostics []codesignal.Diagnostic, continuity []codesignal.PathContinuity, appliedScope string, excluded []codesignal.CoverageGroup, project *ProjectAnalysis) (*codesignal.Report, error) {
	return analyzeChangesThrough(gitrepo.RunBytes, ctx, dir, headSHA, mergeBaseSHA, files, extraDiagnostics, continuity, appliedScope, excluded, project)
}

func mapSemanticsError(path string, err error) codesignal.Diagnostic {
	kind := "analysis_failed"
	switch {
	case errors.Is(err, semantics.ErrEmptyContent):
		kind = "empty_content"
	case errors.Is(err, semantics.ErrBinaryContent):
		kind = "binary_content"
	case errors.Is(err, semantics.ErrFileTooLarge):
		kind = "file_too_large"
	case errors.Is(err, semantics.ErrUnsupportedLanguage):
		kind = "unsupported_language"
	}
	return codesignal.Diagnostic{Path: path, Kind: kind, Message: err.Error()}
}
