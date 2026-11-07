package codesignalcli

import (
	"context"
	"errors"
	"fmt"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

func AnalyzeChanges(ctx context.Context, dir, headSHA, mergeBaseSHA string, files []gitrepo.SelectedFile, extraDiagnostics []codesignal.Diagnostic, continuity []codesignal.PathContinuity, appliedScope string, excluded []codesignal.CoverageGroup, project *ProjectAnalysis) (*codesignal.Report, error) {
	return analyzeChangesThrough(gitrepo.RunBytes, ctx, dir, headSHA, mergeBaseSHA, files, extraDiagnostics, continuity, appliedScope, excluded, project)
}

// analyzeChangesThrough is AnalyzeChanges with the git that answers the per-file questions of a --base scan (the content
// of a file at a revision, the changed lines of a file) given by the caller, so that a test can answer them from a git
// that is the whole of git for the test, and nothing is replaced.
func analyzeChangesThrough(run gitrepo.Runner, ctx context.Context, dir, headSHA, mergeBaseSHA string, files []gitrepo.SelectedFile, extraDiagnostics []codesignal.Diagnostic, continuity []codesignal.PathContinuity, appliedScope string, excluded []codesignal.CoverageGroup, project *ProjectAnalysis) (*codesignal.Report, error) {
	analyzer, err := semantics.NewAnalyzer(semantics.AnalyzerOptions{})
	if err != nil {
		return nil, &gitrepo.OperationalError{Message: fmt.Sprintf("coach codesignal: %s", err)}
	}

	var fileChanges []codesignal.FileChange
	diagnostics := append([]codesignal.Diagnostic(nil), extraDiagnostics...)

	for _, sf := range files {
		var fc *codesignal.FileChange
		var fileDiagnostics []codesignal.Diagnostic
		var stalled error
		if sf.Status == "removed" {
			fc, fileDiagnostics, stalled = analyzeRemovedFile(run, ctx, analyzer, dir, mergeBaseSHA, sf)
		} else {
			fc, fileDiagnostics, stalled = analyzeAddedOrModifiedFile(run, ctx, analyzer, dir, headSHA, mergeBaseSHA, sf)
		}
		if stalled != nil {
			return nil, stalled
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
