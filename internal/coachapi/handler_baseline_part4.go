package coachapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/lousy-agents/coach/internal/agentloop"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/githubingest"
	"github.com/lousy-agents/coach/pkg/semantics"

	"strings"
)

func analyzeOneBaselineFile(ctx context.Context, loop *agentloop.Loop, f loadedBaselineFile) (loadedBaselineFile, *codesignal.FileChange, error) {
	args, err := json.Marshal(map[string]string{
		"path":     f.Path,
		"language": string(f.Language),
		"content":  f.Content,
	})
	if err != nil {
		return loadedBaselineFile{}, nil, err
	}
	raw, err := loop.Call(ctx, agentloop.CallSourceHandler, agentloop.ToolSemanticsAnalyze, args)
	if err != nil && raw == nil {
		return loadedBaselineFile{}, nil, fmt.Errorf("coachapi: semantics_analyze %s: %w", f.Path, err)
	}
	out := loadedBaselineFile{Path: f.Path, Language: f.Language, Content: f.Content}
	if len(raw) == 0 {
		return out, nil, nil
	}
	var result semantics.Result
	if uerr := json.Unmarshal(raw, &result); uerr != nil {
		return loadedBaselineFile{}, nil, fmt.Errorf("coachapi: decoding semantics_analyze result for %s: %w", f.Path, uerr)
	}
	res := result
	out.Result = &res
	return out, &codesignal.FileChange{Path: out.Path, Head: &res}, nil
}
func countHiddenMutationFindings(detFindings []JobFinding) int {
	n := 0
	for _, f := range detFindings {
		if f.Source != FindingSourceDeterministic {
			continue
		}
		if _, ok := hiddenMutationSignal(f.Payload); ok {
			n++
		}
	}
	return n
}

// mapBaselineFetchError keeps errors.Is on githubingest sentinels and adds a
// stable coachapi: prefix for FailJob messages when missing.
func mapBaselineFetchError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, githubingest.ErrNotFound) ||
		errors.Is(err, githubingest.ErrAuth) ||
		errors.Is(err, githubingest.ErrTooLarge) ||
		errors.Is(err, githubingest.ErrUnsupportedContent) ||
		errors.Is(err, githubingest.ErrEmptyContent) {
		if strings.HasPrefix(err.Error(), "coachapi:") {
			return err
		}
		return fmt.Errorf("coachapi: baseline fetch failed: %w", err)
	}
	return fmt.Errorf("coachapi: baseline fetch failed: %w", err)
}
