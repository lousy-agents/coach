package coachapi_test

import (
	"encoding/json"

	"github.com/lousy-agents/coach/internal/agentloop"
	"github.com/lousy-agents/coach/internal/rubrics"
)

type sigbodyhandlerBaselineJudgmentPackAcceptanceTestissuesStrict struct {
	c       int
	maxPath *int
}

func (sigRecv *sigbodyhandlerBaselineJudgmentPackAcceptanceTestissuesStrict) call() {

	if sigRecv.c > *sigRecv.maxPath {
		*sigRecv.maxPath = sigRecv.c
	}
}

type sigbodyhandlerBaselineJudgmentPackAcceptanceTestissuesStrict0 struct {
	loop *agentloop.
		Loop
	sawMultiPackArgs *bool
}

func (sigRecv *sigbodyhandlerBaselineJudgmentPackAcceptanceTestissuesStrict0) call() {

	for _, c := range sigRecv.loop.Calls() {
		if c.Name != rubrics.IDHiddenMutationContextualization {
			continue
		}
		var args struct {
			Items []json.RawMessage `json:"items"`
		}
		if json.Unmarshal(c.Args, &args) == nil && len(args.Items) >= 2 {
			*sigRecv.sawMultiPackArgs = true
		}
	}
}
