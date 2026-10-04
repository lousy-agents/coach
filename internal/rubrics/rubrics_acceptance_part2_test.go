package rubrics_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/modelgateway"
	"github.com/lousy-agents/coach/internal/rubrics"
)

func expectJudgmentRequest(req modelgateway.JudgmentRequest, def rubrics.Definition, evidenceSubstrings ...string) {
	GinkgoHelper()
	Expect(req.RubricID).To(Equal(def.ID))
	Expect(req.RubricVersion).To(Equal(def.Version))
	Expect(req.OutputSchema).NotTo(BeEmpty(),
		"Gateway.Judge must receive OutputSchema so production validation enforces rubric enums")
	Expect(canonicalJSON(req.OutputSchema)).To(Equal(canonicalJSON(def.OutputSchema)),
		"Gateway.Judge OutputSchema must match seed definition for %s", def.ID)
	Expect(req.Messages).NotTo(BeEmpty(), "Gateway.Judge must receive evidence-bearing Messages")
	joined := joinedMessageContent(req.Messages)
	Expect(joined).NotTo(BeEmpty())
	for _, s := range evidenceSubstrings {
		Expect(joined).To(ContainSubstring(s), "Messages must carry deterministic evidence %q", s)
	}
}

func newRecordingGateway(inner modelgateway.Gateway) *recordingGateway {
	return &recordingGateway{inner: inner}
}
