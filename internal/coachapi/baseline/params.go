package baseline

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lousy-agents/coach/internal/coachapi"
)

func parseBaselineParams(raw json.RawMessage) (coachapi.RepoBaselineScanParams, error) {
	if len(raw) == 0 {
		return coachapi.RepoBaselineScanParams{}, fmt.Errorf("coachapi: baseline params are required")
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(raw, &keys); err != nil {
		return coachapi.RepoBaselineScanParams{}, fmt.Errorf("coachapi: invalid baseline params: %w", err)
	}
	for _, forbidden := range []string{"git_url", "clone_url"} {
		if _, ok := keys[forbidden]; ok {
			return coachapi.RepoBaselineScanParams{}, fmt.Errorf("coachapi: client-supplied %s is not allowed in repo_baseline_scan params", forbidden)
		}
	}
	var params coachapi.RepoBaselineScanParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return coachapi.RepoBaselineScanParams{}, fmt.Errorf("coachapi: invalid baseline params: %w", err)
	}
	params.RepoOwner = strings.TrimSpace(params.RepoOwner)
	params.RepoName = strings.TrimSpace(params.RepoName)
	params.Ref = strings.TrimSpace(params.Ref)
	if params.RepoOwner == "" || params.RepoName == "" {
		return coachapi.RepoBaselineScanParams{}, fmt.Errorf("coachapi: repo_owner and repo_name are required")
	}
	return params, nil
}
