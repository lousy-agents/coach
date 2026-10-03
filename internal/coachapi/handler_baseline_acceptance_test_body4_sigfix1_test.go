package coachapi_test

import (
	"github.com/lousy-agents/coach/internal/agentloop"
)

type sigbodyhandlerBaselineAcceptanceTestcompletesDeterministicFi struct {
	analyzeCalls *int
	analyzeLoop  **agentloop.
			Loop
	loop *agentloop.
		Loop
}

func (sigRecv *sigbodyhandlerBaselineAcceptanceTestcompletesDeterministicFi) call() {

	for _, c := range sigRecv.loop.Calls() {
		if c.Source == agentloop.CallSourceHandler && c.Name == agentloop.ToolSemanticsAnalyze {
			*sigRecv.analyzeCalls++
			*sigRecv.analyzeLoop = sigRecv.loop
		}
	}
}
