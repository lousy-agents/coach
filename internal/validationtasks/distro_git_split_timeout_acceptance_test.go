package validationtasks

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func expectTwentyFiveMinutes(command string) {
	GinkgoHelper()
	minutes, named := goTestTimeoutMinutes(command)
	Expect(named).To(BeTrue(), command)
	Expect(minutes).To(Equal(25), command)
}

var _ = Describe("the timeouts of the distro-git legs (#925, #909)", func() {
	var yml string

	BeforeEach(func() {
		yml = readRepoText(".github/workflows/ci.yml")
	})

	DescribeTable("gives every acceptance-ci-env command of the leg a 25 minute go test timeout (D6)",
		func(job string) {
			commands := distroCommands(yml, job)
			Expect(commands).NotTo(BeEmpty())
			for _, command := range commands {
				expectTwentyFiveMinutes(command)
			}
		},
		Entry("distro-git", "distro-git"),
		Entry("distro-git-policy", "distro-git-policy"),
	)

	DescribeTable("leaves the job that runs the leg five minutes more than go test's own timeout, so go test panics with its goroutine dump before Actions cancels the job (D6)",
		func(job string) {
			limit, named := jobTimeoutMinutes(yml, job)

			Expect(named).To(BeTrue())
			Expect(limit).To(BeNumerically(">=", 25+5))
		},
		Entry("distro-git", "distro-git"),
		Entry("distro-git-policy", "distro-git-policy"),
	)
})
