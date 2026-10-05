package coachapi_test

import (
	"github.com/lousy-agents/coach/internal/agentloop"
	"github.com/lousy-agents/coach/internal/rubrics"
)

type sigcountHiddenMutationToolCallsS1 struct {
	loop *agentloop.
		Loop
	n *int
}

func (sigRecv *sigcountHiddenMutationToolCallsS1) call() {

	for _, c := range sigRecv.loop.Calls() {
		if c.Source == agentloop.CallSourceHandler && c.Name == rubrics.IDHiddenMutationContextualization {
			*sigRecv.n++
		}
	}
}
