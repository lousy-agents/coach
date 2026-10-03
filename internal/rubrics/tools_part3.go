package rubrics

import (
	"fmt"

	"github.com/lousy-agents/coach/internal/agentloop"
	"github.com/lousy-agents/coach/internal/modelgateway"
)

// ToolSpecs returns agentloop.ToolSpec values for the two seed rubrics.
func ToolSpecs(gw modelgateway.Gateway) ([]agentloop.ToolSpec, error) {
	if gw == nil {
		return nil, fmt.Errorf("rubrics: gateway is required")
	}
	out := make([]agentloop.ToolSpec, 0, 2)
	for _, cfg := range seedToolConfigs() {
		spec, err := seedJudgmentTool(gw, cfg)
		if err != nil {
			return nil, err
		}
		out = append(out, spec)
	}
	return out, nil
}
