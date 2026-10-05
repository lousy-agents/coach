package baseline

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lousy-agents/coach/internal/agentloop"
	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
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
