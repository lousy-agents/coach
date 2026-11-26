package validationtasks

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("the go test timeout of the other full-module runs (#909)", func() {
	const floorMinutes = 25 // what `mise run test` uses (#605)

	var toml, yml string

	BeforeEach(func() {
		raw, err := os.ReadFile(filepath.Join("..", "..", "mise.toml"))
		Expect(err).NotTo(HaveOccurred())
		toml = string(raw)
		raw, err = os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "ci.yml"))
		Expect(err).NotTo(HaveOccurred())
		yml = string(raw)
	})

	timeoutMinutes := func(pattern *regexp.Regexp, text string) int {
		match := pattern.FindStringSubmatch(text)
		ExpectWithOffset(1, match).NotTo(BeNil(), "no match for %s in:\n%s", pattern, text)
		value, err := strconv.Atoi(match[1])
		ExpectWithOffset(1, err).NotTo(HaveOccurred())
		return value
	}

	goTimeout := regexp.MustCompile(`-timeout[ =](\d+)m`)
	jobLimit := regexp.MustCompile(`timeout-minutes:\s*(\d+)`)

	It("names a go test timeout on test-acceptance-fast, which runs cmd/coach under -race and would otherwise stop at go test's 10-minute default", func() {
		Expect(taskSteps(toml, "test-acceptance-fast")).To(MatchRegexp(`go test -race[^\n"]*-timeout[ =]\d+m`))
	})

	It("gives test-acceptance-fast a go test timeout of at least as many minutes as `mise run test` uses", func() {
		Expect(timeoutMinutes(goTimeout, taskSteps(toml, "test-acceptance-fast"))).To(BeNumerically(">=", floorMinutes))
	})

	It("names a go test timeout on the distro-git leg's acceptance-ci-env run, which passes its arguments to go test", func() {
		Expect(jobBody(yml, "distro-git")).To(MatchRegexp(`acceptance-ci-env[^\n]*-timeout[ =]\d+m`))
	})

	It("gives the distro-git leg a go test timeout of at least as many minutes as `mise run test` uses", func() {
		Expect(timeoutMinutes(goTimeout, jobBody(yml, "distro-git"))).To(BeNumerically(">=", floorMinutes))
	})

	It("leaves the distro-git job more time than its go test timeout, so go test panics with its goroutine dump before Actions cancels the job", func() {
		body := jobBody(yml, "distro-git")

		Expect(timeoutMinutes(jobLimit, body)).To(BeNumerically(">=", timeoutMinutes(goTimeout, body)+5), "the other steps of the job need the margin")
	})
})
