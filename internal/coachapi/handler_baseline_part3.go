package coachapi

import (
	"context"
	"encoding/json"

	"fmt"

	"github.com/lousy-agents/coach/internal/agentloop"

	"github.com/lousy-agents/coach/pkg/codesignal"

	"strings"
)

func parseBaselineParams(raw json.RawMessage) (RepoBaselineScanParams, error) {
	if len(raw) == 0 {
		return RepoBaselineScanParams{}, fmt.Errorf("coachapi: baseline params are required")
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(raw, &keys); err != nil {
		return RepoBaselineScanParams{}, fmt.Errorf("coachapi: invalid baseline params: %w", err)
	}
	for _, forbidden := range []string{"git_url", "clone_url"} {
		if _, ok := keys[forbidden]; ok {
			return RepoBaselineScanParams{}, fmt.Errorf("coachapi: client-supplied %s is not allowed in repo_baseline_scan params", forbidden)
		}
	}
	var params RepoBaselineScanParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return RepoBaselineScanParams{}, fmt.Errorf("coachapi: invalid baseline params: %w", err)
	}
	params.RepoOwner = strings.TrimSpace(params.RepoOwner)
	params.RepoName = strings.TrimSpace(params.RepoName)
	params.Ref = strings.TrimSpace(params.Ref)
	if params.RepoOwner == "" || params.RepoName == "" {
		return RepoBaselineScanParams{}, fmt.Errorf("coachapi: repo_owner and repo_name are required")
	}
	return params, nil
}
func analyzeBaselineViaLoop(ctx context.Context, loop *agentloop.Loop, files []loadedBaselineFile, repo, revision string) ([]loadedBaselineFile, *codesignal.Report, error) {

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
