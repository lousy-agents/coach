package validationtasks

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("the CI jobs that run the split test tasks (#920, S5, S6)", func() {
	var yml string

	BeforeEach(func() {
		yml = readRepoText(".github/workflows/ci.yml")
	})

	It("has verify run test-rest, and neither job run the whole test task, so no package runs twice (S5)", func() {
		verify, policy := jobRunCommands(yml, "verify"), jobRunCommands(yml, "verify-policy")

		Expect(verify).To(ContainElement("mise run test-rest"))
		Expect(verify).NotTo(ContainElement("mise run test"))
		Expect(policy).To(Equal([]string{"mise run test-policy"}))
	})

	It("gives verify-policy the setup verify has, so the two halves run in one environment (S5)", func() {
		verify, policy := workflowJob(yml, "verify"), workflowJob(yml, "verify-policy")
		Expect(policy).NotTo(BeNil())

		Expect(policy["runs-on"]).To(Equal(verify["runs-on"]))
		Expect(setupSteps(policy)).To(Equal(setupSteps(verify)))
		Expect(stepEnv(policy, "mise run test-policy")).To(Equal(stepEnv(verify, "mise run test-rest")))
	})

	It("lists verify-policy in status.needs, as every leaf must be (S6)", func() {
		Expect(needsList(jobBody(yml, "status"))).To(ContainElement("verify-policy"))
	})
})
