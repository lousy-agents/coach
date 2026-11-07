package codesignalcli

import (
	"context"
	"errors"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

// analyzeBaselineFile reads and analyzes one file of a --baseline scan. It returns the file's change, or
// none and the diagnostic that says why the file was not analyzed. Its error is a git that did not answer in
// time (see readFailure), which ends the scan.
func analyzeBaselineFile(ctx context.Context, analyzer *semantics.Analyzer, reader *gitrepo.RevisionFileReader, sf gitrepo.SelectedFile) (*codesignal.FileChange, []codesignal.Diagnostic, error) {
	headBytes, err := reader.Next(sf.Path)
	if err != nil {
		failure, stalled := baselineReadFailure(sf.Path, err)
		return nil, failure, stalled
	}

	headResult, headErr := analyzer.AnalyzeBytes(ctx, semantics.FileInput{Path: sf.Path, Language: sf.Language, Content: headBytes})
	if headErr != nil && !errors.Is(headErr, semantics.ErrSyntax) {
		return nil, []codesignal.Diagnostic{mapSemanticsError(sf.Path, headErr)}, nil
	}
	return &codesignal.FileChange{Path: sf.Path, SourceScope: sf.SourceScope, Head: headResult}, nil, nil
}

// countedBaselineFile is the coverage after one more file: analyzed when its parse was clean, and
// unanalyzable when it was not analyzed at all or parsed with errors.
func countedBaselineFile(coverage codesignal.Coverage, fileChange *codesignal.FileChange) codesignal.Coverage {
	if fileChange != nil && fileChange.Head.ParseStatus == "ok" {
		coverage.FilesAnalyzed++
		return coverage
	}
	coverage.FilesUnanalyzable++
	return coverage
}
