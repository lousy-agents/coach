package baseline

import (
	"fmt"

	"github.com/lousy-agents/coach/internal/agentloop"
)

// Analyze wall time never consumes the judgment budget: judgment uses a
// fresh loop of its own.
func newAnalyzeLoop(cfg ScanConfig, fileCount int) (*agentloop.Loop, error) {
	analyzeMaxTools := fileCount + 1 + 10
	if analyzeMaxTools < agentloop.DefaultMaxToolCalls {
		analyzeMaxTools = agentloop.DefaultMaxToolCalls
	}
	analyzeLoop, err := agentloop.New(agentloop.Options{
		Budget: agentloop.Budget{MaxToolCalls: analyzeMaxTools},
	})
	if err != nil {
		return nil, fmt.Errorf("coachapi: constructing analyze agent loop: %w", err)
	}
	if cfg.ConfigureLoop != nil {
		cfg.ConfigureLoop(analyzeLoop)
	}
	return analyzeLoop, nil
}
