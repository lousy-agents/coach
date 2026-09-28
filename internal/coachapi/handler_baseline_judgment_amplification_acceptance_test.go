package coachapi_test

import (
	"encoding/json"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"

	"github.com/lousy-agents/coach/internal/coachapi"
)

// lousyIAMStyleHiddenMutationCounts is the offline distribution shape measured on
// zpratt/lousy-iam (42 hidden_input_mutation signals across 5 paths).
var lousyIAMStyleHiddenMutationCounts = []int{22, 7, 6, 4, 3}

// lousyIAMStyleHiddenMutationFixtureRoot builds a temp tree with the 22+7+6+4+3
// hidden_mutation distribution (Task 6 / local-LLM call-amplification lock).
func lousyIAMStyleHiddenMutationFixtureRoot() string {
	GinkgoHelper()
	root := GinkgoT().TempDir()

	writeMutators := func(pkg, path string, n int) {
		body_handlerBaselineJudgmentAmplificationAcceptanceTe_31(pkg, path, n, root)
	}

	for i, n := range lousyIAMStyleHiddenMutationCounts {
		writeMutators(fmt.Sprintf("p%d", i), fmt.Sprintf("path_%d.go", i), n)
	}
	return root
}

func deterministicHiddenMutationPathCounts(findings []coachapi.JobFinding) map[string]int {
	paths := map[string]int{}
	for _, f := range findings {
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
		paths[sig.Path]++
	}
	return paths
}

var _ = Describe("repo_baseline_scan local-model judgment amplification harness (Task 6)", func() {
	When("a lousy-iam-shaped fixture has 22+7+6+4+3 hidden_mutation signals and the gateway is slow", func() {
		It("completes under a short judgment wall with ≥1 source=agent finding via packed+capped judgment where pure 1:1 cannot finish in time", func() {
			body_handlerBaselineJudgmentAmplificationAcceptanceTe_completesUnderAShortJudgmentWallWith1SourceAgent_73()
		})
	})
})
