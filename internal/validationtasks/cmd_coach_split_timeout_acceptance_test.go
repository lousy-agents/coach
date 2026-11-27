package validationtasks

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func expectRaceAndTwentyFiveMinutes(command string) {
	GinkgoHelper()
	minutes, named := goTestTimeoutMinutes(command)
	Expect(named).To(BeTrue(), command)
	Expect(minutes).To(Equal(25), command)
	Expect(strings.Fields(command)).To(ContainElement("-race"), command)
}

var _ = Describe("the timeouts of the split test tasks and their jobs (#920, S4)", func() {
	var toml, yml string

	BeforeEach(func() {
		toml = readRepoText("mise.toml")
		yml = readRepoText(".github/workflows/ci.yml")
	})

	DescribeTable("gives every go test command of the three test tasks the race detector and a 25 minute timeout",
		func(task string) {
			commands := taskRunCommands(toml, task)
			Expect(commands).NotTo(BeEmpty())
			for _, command := range commands {
				expectRaceAndTwentyFiveMinutes(command)
			}
		},
		Entry("test, the whole run", "test"),
		Entry("test-rest", "test-rest"),
		Entry("test-policy", "test-policy"),
	)

	DescribeTable("leaves the job that runs the task five minutes more than go test's own timeout, so go test panics with its goroutine dump before Actions cancels the job",
		func(job, task string) {
			timeout, named := taskGoTestTimeout(toml, task)
			Expect(named).To(BeTrue())
			limit, named := jobTimeoutMinutes(yml, job)
			Expect(named).To(BeTrue())

			Expect(limit).To(BeNumerically(">=", timeout+5))
		},
		Entry("verify", "verify", "test-rest"),
		Entry("verify-policy", "verify-policy", "test-policy"),
	)
})
