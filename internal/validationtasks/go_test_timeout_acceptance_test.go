package validationtasks

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("the go test timeout of the test task (#605)", func() {
	var toml, yml string

	BeforeEach(func() {
		raw, err := os.ReadFile(filepath.Join("..", "..", "mise.toml"))
		Expect(err).NotTo(HaveOccurred())
		toml = string(raw)
		raw, err = os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "ci.yml"))
		Expect(err).NotTo(HaveOccurred())
		yml = string(raw)
	})

	It("runs the whole module under the race detector with a timeout of 25 minutes, so a loaded machine's slow pass is not reported as a failure", func() {
		Expect(taskRunCommands(toml, "test")).To(ContainElement("go test -race -timeout 25m ./..."))
	})

	It("leaves the verify job more time than the go test timeout, so go test panics with its goroutine dump before Actions cancels the job", func() {
		goTimeout, _ := taskGoTestTimeout(toml, "test")
		jobLimit, _ := jobTimeoutMinutes(yml, "verify")

		Expect(jobLimit).To(BeNumerically(">=", goTimeout+5), "the other steps of the job need the margin")
	})

	DescribeTable("does not take a flag go test never receives for the test task's timeout",
		func(step string) {
			mutant := strings.Replace(toml, `run = "go test -race -timeout 25m ./..."`, `run = "`+step+`"`, 1)
			Expect(mutant).NotTo(Equal(toml))

			_, named := taskGoTestTimeout(mutant, "test")
			Expect(named).To(BeFalse())
		},
		Entry("a trailing shell comment inside the step", "go test -race ./... # -timeout 25m"),
		Entry("a second command in the step", "go test -race ./...; echo -timeout 25m"),
	)

	It("reads the verify job limit from the key, not from a comment that mentions it", func() {
		mutant := strings.Replace(yml, "    timeout-minutes: 30\n    steps:", "    # timeout-minutes: 30\n    timeout-minutes: 28\n    steps:", 1)
		Expect(mutant).NotTo(Equal(yml))

		limit, _ := jobTimeoutMinutes(mutant, "verify")
		Expect(limit).To(Equal(28))
	})
})
