package baseline

import (
	"context"

	"github.com/lousy-agents/coach/internal/agentloop"
	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

type loadedBaselineFile struct {
	Path     string
	Language semantics.Language
	Content  string
	Result   *semantics.Result
}

func analyzeBaselineViaLoop(ctx context.Context, loop *agentloop.Loop, files []loadedBaselineFile, repo, revision string) ([]loadedBaselineFile, *codesignal.Report, error) {
	// Copy: do not mutate the caller's slice elements in place.
	loaded := make([]loadedBaselineFile, 0, len(files))
	fileChanges := make([]codesignal.FileChange, 0, len(files))
	for _, f := range files {
		entry, fc, err := analyzeOneBaselineFile(ctx, loop, f)
		if err != nil {
			return nil, nil, err
		}
		loaded = append(loaded, entry)
		if fc != nil {
			fileChanges = append(fileChanges, *fc)
		}
	}

	report, err := codesignalReportViaLoop(ctx, loop, fileChanges, repo, revision)
	if err != nil {
		return nil, nil, err
	}
	return loaded, report, nil
}
