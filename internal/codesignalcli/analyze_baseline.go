package codesignalcli

import (
	"context"
	"errors"
	"fmt"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

func AnalyzeBaseline(ctx context.Context, dir, revisionSHA string, files []gitrepo.SelectedFile, extraDiagnostics []codesignal.Diagnostic, appliedScope string, coverage codesignal.Coverage, project *ProjectAnalysis) (*codesignal.Report, error) {
	analyzer, err := semantics.NewAnalyzer(semantics.AnalyzerOptions{})
	if err != nil {
		return nil, &gitrepo.OperationalError{Message: fmt.Sprintf("coach codesignal: %s", err)}
	}

	reader, err := gitrepo.NewRevisionFileReader(dir, revisionSHA)
	if err != nil {
		return nil, &gitrepo.OperationalError{Message: fmt.Sprintf("coach codesignal: starting git cat-file --batch failed: %s", err)}
	}
	defer func() { _ = reader.Close() }()

	var fileChanges []codesignal.FileChange
	diagnostics := append([]codesignal.Diagnostic(nil), extraDiagnostics...)

	for _, sf := range files {
		headBytes, err := reader.Next(sf.Path)
		if err != nil {
			diagnostics = append(diagnostics, readFailedDiagnostic(sf.Path, "head", err))
			coverage.FilesUnanalyzable++
			continue
		}

		headResult, headErr := analyzer.AnalyzeBytes(ctx, semantics.FileInput{Path: sf.Path, Language: sf.Language, Content: headBytes})
		if headErr != nil && !errors.Is(headErr, semantics.ErrSyntax) {
			diagnostics = append(diagnostics, mapSemanticsError(sf.Path, headErr))
			coverage.FilesUnanalyzable++
			continue
		}

		fileChanges = append(fileChanges, codesignal.FileChange{Path: sf.Path, SourceScope: sf.SourceScope, Head: headResult})
		if headResult.ParseStatus == "ok" {
			coverage.FilesAnalyzed++
		} else {
			coverage.FilesUnanalyzable++
		}
	}

	opts := codesignal.Options{Baseline: true}
	input := codesignal.Input{
		Scope:       codesignal.Scope{Revision: revisionSHA, AppliedScope: appliedScope},
		Files:       fileChanges,
		Diagnostics: diagnostics,
		Coverage:    &coverage,
	}
	input, opts, err = applyProjectBackend(ctx, input, opts, project, dir, revisionSHA, "", true)
	if err != nil {
		return nil, err
	}
	builder, err := codesignal.New(opts)
	if err != nil {
		return nil, err
	}
	return builder.Build(ctx, input)
}

func readFailedDiagnostic(path, side string, err error) codesignal.Diagnostic {
	return codesignal.Diagnostic{
		Path:    path,
		Kind:    side + "_read_failed",
		Message: fmt.Sprintf("reading %s content for %q: %s", side, path, err),
	}
}

func baseSyntaxDiagnostics(path string, baseErr error) []codesignal.Diagnostic {
	var syntaxErr *semantics.SyntaxError
	if !errors.As(baseErr, &syntaxErr) {
		return []codesignal.Diagnostic{{
			Path:    path,
			Kind:    "base_syntax_errors",
			Message: baseErr.Error(),
		}}
	}

	diagnostics := make([]codesignal.Diagnostic, 0, len(syntaxErr.Issues))
	for _, issue := range syntaxErr.Issues {
		location := issue.Location
		diagnostics = append(diagnostics, codesignal.Diagnostic{
			Path:     path,
			Kind:     "base_syntax_errors",
			Location: &location,
			Message:  fmt.Sprintf("base analysis found a syntax issue of kind %q", issue.Kind),
		})
	}
	return diagnostics
}
