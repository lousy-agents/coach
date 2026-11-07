package codesignalcli

import (
	"context"
	"errors"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

// analyzeAddedOrModifiedFile reads and analyzes one changed file. Its error is a git that did not answer in
// time (see readFailure); every other problem with the file is a diagnostic.
func analyzeAddedOrModifiedFile(ctx context.Context, analyzer *semantics.Analyzer, dir, headSHA, mergeBaseSHA string, sf gitrepo.SelectedFile) (*codesignal.FileChange, []codesignal.Diagnostic, error) {
	headBytes, err := runGit(dir, "show", headSHA+":"+sf.Path)
	if err != nil {
		diagnostics, stalled := readFailure(sf.Path, headSide, err)
		return nil, diagnostics, stalled
	}

	headResult, headErr := analyzer.AnalyzeBytes(ctx, semantics.FileInput{Path: sf.Path, Language: sf.Language, Content: headBytes})
	if headErr != nil && !errors.Is(headErr, semantics.ErrSyntax) {
		return nil, []codesignal.Diagnostic{mapSemanticsError(sf.Path, headErr)}, nil
	}

	fc := codesignal.FileChange{Path: sf.Path, Status: sf.Status, SourceScope: sf.SourceScope, Head: headResult}
	var diagnostics []codesignal.Diagnostic

	if sf.Status == "modified" {
		base, baseDiagnostics, stalled := analyzeModifiedBase(ctx, analyzer, dir, mergeBaseSHA, sf)
		if stalled != nil {
			return nil, nil, stalled
		}
		fc.Base = base
		diagnostics = append(diagnostics, baseDiagnostics...)
	}

	ranges, rangeDiagnostic, stalled := computeChangedRanges(dir, mergeBaseSHA, sf.Path)
	if stalled != nil {
		return nil, nil, stalled
	}
	if rangeDiagnostic != nil {
		diagnostics = append(diagnostics, *rangeDiagnostic)
	} else {
		fc.ChangedRanges = ranges
	}

	return &fc, diagnostics, nil
}
