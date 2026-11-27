package validationtasks

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("the distro-git-policy leaf (#925)", func() {
	var yml string

	BeforeEach(func() {
		yml = readRepoText(".github/workflows/ci.yml")
	})

	It("is its own job, not a matrix of distro-git, so a failing half is a failing job that status reads as it reads every leaf (D7)", func() {
		policy := workflowJob(yml, "distro-git-policy")

		Expect(policy).NotTo(BeNil())
		Expect(policy).NotTo(HaveKey("strategy"))
		Expect(workflowJob(yml, "distro-git")).NotTo(HaveKey("strategy"))
	})

	It("runs in distro-git's container, under distro-git's git pin and env, and with distro-git's setup steps, so the two halves run alike (D7)", func() {
		distro, policy := workflowJob(yml, "distro-git"), workflowJob(yml, "distro-git-policy")
		Expect(policy).NotTo(BeNil())

		Expect(policy["runs-on"]).To(Equal(distro["runs-on"]))
		Expect(policy["container"]).To(Equal(distro["container"]))
		Expect(policy["env"]).To(Equal(distro["env"]))
		Expect(distroSetupSteps(policy)).To(Equal(distroSetupSteps(distro)))
	})

	It("is in status.needs, as every leaf must be, and the workflow never lets a step or job continue on error (D8)", func() {
		Expect(needsList(jobBody(yml, "status"))).To(ContainElement("distro-git-policy"))
		Expect(yml).NotTo(ContainSubstring("continue-on-error"))
	})

	It("is named by AGENTS.md and by the CI document (D9)", func() {
		Expect(readRepoText("AGENTS.md")).To(ContainSubstring("`distro-git-policy`"))
		Expect(readRepoText("docs/development/validation-and-ci.md")).To(ContainSubstring("`distro-git-policy`"))
	})
})
