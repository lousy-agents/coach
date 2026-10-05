package codesignalcli

import (
	"context"
	"errors"
	"fmt"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

func AnalyzeChanges(ctx context.Context, dir, headSHA, mergeBaseSHA string, files []SelectedFile, extraDiagnostics []codesignal.Diagnostic, continuity []codesignal.PathContinuity, appliedScope string, excluded []codesignal.CoverageGroup, project *ProjectAnalysis) (*codesignal.Report, error) {
	analyzer, err := semantics.NewAnalyzer(semantics.AnalyzerOptions{})
	if err != nil {
		return nil, &OperationalError{Message: fmt.Sprintf("coach codesignal: %s", err)}
	}

	var fileChanges []codesignal.FileChange
	diagnostics := append([]codesignal.Diagnostic(nil), extraDiagnostics...)

	for _, sf := range files {
		var fc *codesignal.FileChange
		var fileDiagnostics []codesignal.Diagnostic
		if sf.Status == "removed" {
			fc, fileDiagnostics = analyzeRemovedFile(ctx, analyzer, dir, mergeBaseSHA, sf)
		} else {
			fc, fileDiagnostics = analyzeAddedOrModifiedFile(ctx, analyzer, dir, headSHA, mergeBaseSHA, sf)
		}
		diagnostics = append(diagnostics, fileDiagnostics...)
		if fc != nil {
			fileChanges = append(fileChanges, *fc)
		}
	}

	var coverage *codesignal.Coverage
	if len(excluded) > 0 {
		coverage = &codesignal.Coverage{Excluded: excluded}
	}

	opts := codesignal.Options{IncludeResolved: true}
	input := codesignal.Input{
		Scope:       codesignal.Scope{Repository: "", Revision: headSHA, Base: mergeBaseSHA, AppliedScope: appliedScope},
		Files:       fileChanges,
		Diagnostics: diagnostics,
		Coverage:    coverage,

		UndeterminedContinuity: continuity,
	}
	input, opts, err = applyProjectBackend(ctx, input, opts, project, dir, headSHA, mergeBaseSHA, false)
	if err != nil {
		return nil, err
	}
	builder, err := codesignal.New(opts)
	if err != nil {
		return nil, err
	}
	return builder.Build(ctx, input)
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
