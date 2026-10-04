package coachapi_test

import (
	"encoding/json"

	"github.com/lousy-agents/coach/internal/agentloop"
	"github.com/lousy-agents/coach/internal/rubrics"
)

type sigbodyhandlerBaselineJudgmentCapAcceptanceTestjudgesARoundR struct {
	judgedRefs *int
	loop       *agentloop.
			Loop
}

func (sigRecv *sigbodyhandlerBaselineJudgmentCapAcceptanceTestjudgesARoundR) call() {

	for _, c := range sigRecv.loop.Calls() {
		if c.Name != rubrics.IDHiddenMutationContextualization {
			continue
		}
		var args struct {
			Items []json.RawMessage `json:"items"`
		}
		if json.Unmarshal(c.Args, &args) == nil && len(args.Items) > 0 {
			*sigRecv.judgedRefs += len(args.Items)
			continue
		}
		*sigRecv.judgedRefs++
	}
}
