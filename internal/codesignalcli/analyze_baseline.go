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
	defer reader.Close()

	scanned := baselineFiles{
		analyzer:      analyzer,
		view:          project.worktreeView(),
		dir:           dir,
		revision:      revisionSHA,
		readCommitted: reader.Next,
		diagnostics:   append([]codesignal.Diagnostic(nil), extraDiagnostics...),
		coverage:      coverage,
	}
	for _, sf := range files {
		if err := scanned.add(ctx, sf); err != nil {
			return nil, err
		}
	}

	opts := codesignal.Options{Baseline: true}
	input := codesignal.Input{
		Scope:       codesignal.Scope{Revision: revisionSHA, AppliedScope: appliedScope},
		Files:       scanned.changes,
		Diagnostics: scanned.diagnostics,
		Coverage:    &scanned.coverage,
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
