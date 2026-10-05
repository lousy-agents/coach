package coachapi_test

import (
	"encoding/json"
	"strings"

	"github.com/lousy-agents/coach/internal/coachapi"
)

type sigagentHiddenPathsViaJudgedRefsS6 struct {
	body struct {
		FindingRef string "json:\"finding_ref\""

		Path string "json:\"path\""
	}
	detByHash map[string]string
	paths     map[string]int
}

func (sigRecv *sigagentHiddenPathsViaJudgedRefsS6) call() {

	if p, ok := sigRecv.detByHash[sigRecv.body.FindingRef]; ok {
		sigRecv.paths[p]++
	}
}

type sigagentHiddenPathsViaJudgedRefsS1 struct {
	detByHash map[string]string
	findings  []coachapi.
			JobFinding
}

func (sigRecv *sigagentHiddenPathsViaJudgedRefsS1) call() {

	for _, f := range sigRecv.findings {
		if f.Source != coachapi.FindingSourceDeterministic {
			continue
		}
		if !strings.Contains(string(f.Payload), "hidden_input_mutation") {
			continue
		}
		var sig struct {
			Path string `json:"path"`
		}
		if json.Unmarshal(f.Payload, &sig) != nil || sig.Path == "" {
			continue
		}
		sigRecv.detByHash[f.PayloadHash] = sig.Path
	}
}
