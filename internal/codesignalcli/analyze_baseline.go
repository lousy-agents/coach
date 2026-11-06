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
		scanned.add(ctx, sf)
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

// baselineFiles accumulates what the baseline's files contribute to the report:
// their analyzed heads, the diagnostics for those that could not be analyzed,
// and the coverage accounting for both.
type baselineFiles struct {
	analyzer      *semantics.Analyzer
	view          *WorktreeView
	dir           string
	revision      string
	readCommitted func(name string) ([]byte, error)

	changes     []codesignal.FileChange
	diagnostics []codesignal.Diagnostic
	coverage    codesignal.Coverage
}

func (f *baselineFiles) add(ctx context.Context, sf gitrepo.SelectedFile) {
	headBytes, err := headFileBytes(f.view, f.dir, f.revision, sf.Path, f.readCommitted)
	if errors.Is(err, errAbsentFromWorktree) {
		f.coverage = f.view.coverageForAbsentSource(f.coverage, sf.Path)
		return
	}
	if err != nil {
		f.skip(readFailedDiagnostic(sf.Path, "head", err))
		return
	}

	headResult, headErr := f.analyzer.AnalyzeBytes(ctx, semantics.FileInput{Path: sf.Path, Language: sf.Language, Content: headBytes})
	if headErr != nil && !errors.Is(headErr, semantics.ErrSyntax) {
		f.skip(mapSemanticsError(sf.Path, headErr))
		return
	}

	f.changes = append(f.changes, codesignal.FileChange{Path: sf.Path, SourceScope: sf.SourceScope, Head: headResult})
	if headResult.ParseStatus == "ok" {
		f.coverage.FilesAnalyzed++
	} else {
		f.coverage.FilesUnanalyzable++
	}
}

func (f *baselineFiles) skip(reason codesignal.Diagnostic) {
	f.diagnostics = append(f.diagnostics, reason)
	f.coverage.FilesUnanalyzable++
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
