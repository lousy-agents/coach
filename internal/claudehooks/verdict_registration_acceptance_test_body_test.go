package claudehooks

import (
	. "github.com/onsi/gomega"
)

func body_verdictRegistrationAcceptanceTest_registersEveryOneOfThemWithTheVerdictHook_78() {
	matchers := verdictRegistrationMatchers()
	Expect(matchers).NotTo(BeEmpty())
	for _, agent := range reviewerAgents() {
		Expect(matchers).To(ContainElement(agent),
			"agent %q mandates the PASS/FINDINGS contract but no SubagentStop registration enforces it", agent)
	}
}
