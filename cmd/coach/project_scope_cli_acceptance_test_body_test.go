package main

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func body_projectScopeCliAcceptanceTest_33() {
	if reason := ensureRealTypeScriptCompilerAvailable(); reason != "" {
		Skip(reason)
	}
}

func body_projectScopeCliAcceptanceTest_JSONProjectNextActionsIsAbsentOrEmptyBecauseMode_133(jsonOut []byte) {
	report := decodeCoachReport(jsonOut)
	Expect(report.ProjectChanges).To(BeEmpty(), "SA-280-025 must be proved without an active layer finding that would also suppress next_actions")
	var doc map[string]json.RawMessage
	Expect(json.Unmarshal(jsonOut, &doc)).To(Succeed())
	if raw, ok := doc["project_next_actions"]; ok {
		var actions []json.RawMessage
		Expect(json.Unmarshal(raw, &actions)).To(Succeed())
		Expect(actions).To(BeEmpty(), "project_next_actions must be absent or empty when model coverage is incomplete")
	}
}

func body_projectScopeCliAcceptanceTest_JSONProjectNextActionsIsPresentContainsReviewPol_275(jsonOut []byte) {
	var doc map[string]json.RawMessage
	Expect(json.Unmarshal(jsonOut, &doc)).To(Succeed())
	Expect(doc).To(HaveKey("project_next_actions"), "project_next_actions must be present in diff mode")
	var actions []struct {
		Kind string `json:"kind"`
	}
	Expect(json.Unmarshal(doc["project_next_actions"], &actions)).To(Succeed())
	kinds := make([]string, len(actions))
	for i, a := range actions {
		kinds[i] = a.Kind
	}
	Expect(kinds).To(Equal([]string{"review_policy_coverage"}),
		"diff mode must emit exactly [review_policy_coverage], not record_baseline")
}
