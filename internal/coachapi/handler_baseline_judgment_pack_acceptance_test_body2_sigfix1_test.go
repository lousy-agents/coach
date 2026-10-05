package coachapi_test

import (
	"encoding/json"

	"github.com/lousy-agents/coach/internal/agentloop"
	"github.com/lousy-agents/coach/internal/rubrics"
	. "github.com/onsi/gomega"
)

type sigbodyhandlerBaselineJudgmentPackAcceptanceTestembedsSpanWi struct {
	checked *int
	loop    *agentloop.
		Loop
}

func (sigRecv *sigbodyhandlerBaselineJudgmentPackAcceptanceTestembedsSpanWi) call() {

	for _, c := range sigRecv.loop.Calls() {
		if c.Name != rubrics.IDHiddenMutationContextualization {
			continue
		}
		var args struct {
			Items []struct {
				File struct {
					Content string `json:"content"`
				} `json:"file"`
			} `json:"items"`
			File struct {
				Content string `json:"content"`
			} `json:"file"`
		}
		Expect(json.Unmarshal(c.Args, &args)).To(Succeed())
		contents := []string{}
		for _, it := range args.Items {
			contents = append(contents, it.File.Content)
		}
		if args.File.Content != "" {
			contents = append(contents, args.File.Content)
		}
		(&sigcallS6{contents: contents, v61725567: sigRecv}).call()

	}
}
