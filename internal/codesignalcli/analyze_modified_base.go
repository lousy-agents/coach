package codesignalcli

import (
	"context"
	"errors"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

// baseSide is a modified file as it stood at the merge base. A base that
// cannot be read or analyzed leaves result nil and says why in diagnostics.
type baseSide struct {
	content     []byte
	result      *semantics.Result
	diagnostics []codesignal.Diagnostic
}

// analyzeModifiedBase reads and analyzes the merge-base side of a modified file. A base side that cannot be
// read or analyzed is a diagnostic and no result; the head side is still reported. Its error is a git that
// did not answer in time (see readFailure).
func analyzeModifiedBase(run gitrepo.Runner, ctx context.Context, analyzer *semantics.Analyzer, dir, mergeBaseSHA string, sf gitrepo.SelectedFile) (baseSide, error) {
	baseBytes, err := run(dir, "show", mergeBaseSHA+":"+sf.Path)
	if err != nil {
		diagnostics, stalled := readFailure(sf.Path, modifiedBaseSide, err)
		return baseSide{content: baseBytes, diagnostics: diagnostics}, stalled
	}

	baseResult, baseErr := analyzer.AnalyzeBytes(ctx, semantics.FileInput{Path: sf.Path, Language: sf.Language, Content: baseBytes})
	switch {
	case baseErr == nil:
		return baseSide{content: baseBytes, result: baseResult}, nil
	case errors.Is(baseErr, semantics.ErrSyntax):
		return baseSide{content: baseBytes, diagnostics: baseSyntaxDiagnostics(sf.Path, baseErr)}, nil
	default:
		return baseSide{content: baseBytes, diagnostics: []codesignal.Diagnostic{{
			Path:    sf.Path,
			Kind:    "base_analysis_failed",
			Message: baseErr.Error(),
		}}}, nil
	}
}
