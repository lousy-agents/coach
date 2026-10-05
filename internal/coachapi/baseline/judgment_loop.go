package baseline

import (
	"fmt"

	"github.com/lousy-agents/coach/internal/agentloop"
	"github.com/lousy-agents/coach/internal/modelgateway"
	"github.com/lousy-agents/coach/internal/rubrics"
)

// newJudgmentLoop builds the rubric-tool loop for hiddenCount
// hidden-mutation findings, with its own wall budget so analyze wall time
// never consumes it. MaxToolCalls scales for packs + cohesion + slack rather
// than 1:1 with findings: the ceiling is one Call per finding (worst pack
// size 1) plus cohesion plus slack.
func newJudgmentLoop(cfg ScanConfig, hiddenCount int) (*agentloop.Loop, error) {
	gw := cfg.Gateway
	if gw == nil {
		gw = modelgateway.NewStubGateway()
	}

	judgmentWall := cfg.JudgmentMaxWallTime
	if judgmentWall <= 0 {
		judgmentWall = DefaultJudgmentMaxWallTime
	}
	judgmentMaxTools := hiddenCount + 1 + 10
	if judgmentMaxTools < agentloop.DefaultMaxToolCalls {
		judgmentMaxTools = agentloop.DefaultMaxToolCalls
	}

	judgmentLoop, err := agentloop.New(agentloop.Options{
		Budget: agentloop.Budget{
			MaxToolCalls: judgmentMaxTools,
			MaxWallTime:  judgmentWall,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("coachapi: constructing judgment agent loop: %w", err)
	}
	if err := rubrics.RegisterTools(judgmentLoop, gw); err != nil {
		return nil, fmt.Errorf("coachapi: registering rubric tools: %w", err)
	}
	if cfg.ConfigureLoop != nil {
		cfg.ConfigureLoop(judgmentLoop)
	}
	return judgmentLoop, nil
}
