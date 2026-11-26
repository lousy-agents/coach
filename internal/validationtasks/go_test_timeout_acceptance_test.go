package validationtasks

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"

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

	minutes := func(pattern *regexp.Regexp, text string) int {
		match := pattern.FindStringSubmatch(text)
		Expect(match).NotTo(BeNil(), "no match for %s", pattern)
		value, err := strconv.Atoi(match[1])
		Expect(err).NotTo(HaveOccurred())
		return value
	}

	It("runs the whole module under the race detector with a timeout of 25 minutes, so a loaded machine's slow pass is not reported as a failure", func() {
		Expect(taskSteps(toml, "test")).To(ContainSubstring("go test -race -timeout 25m ./..."))
	})

	It("leaves the verify job more time than the go test timeout, so go test panics with its goroutine dump before Actions cancels the job", func() {
		goTimeout := minutes(regexp.MustCompile(`-timeout (\d+)m`), taskSteps(toml, "test"))
		jobLimit := minutes(regexp.MustCompile(`timeout-minutes:\s*(\d+)`), jobBody(yml, "verify"))

		Expect(jobLimit).To(BeNumerically(">=", goTimeout+5), "the other steps of the job need the margin")
	})
})
