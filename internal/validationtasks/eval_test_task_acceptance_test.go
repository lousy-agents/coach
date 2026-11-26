package validationtasks

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("the eval-test mise task", func() {
	var toml string

	BeforeEach(func() {
		raw, err := os.ReadFile(filepath.Join("..", "..", "mise.toml"))
		Expect(err).NotTo(HaveOccurred())
		toml = string(raw)
	})

	It("runs the eval module's specs with the race detector and a timeout that holds a full run (AC-21)", func() {
		body := taskBody(toml, "eval-test")

		Expect(body).NotTo(BeEmpty(), "mise.toml defines [tasks.eval-test]")
		Expect(body).To(ContainSubstring("go -C eval test -race -timeout 30m ./..."))
	})

	It("says it is local only (AC-21)", func() {
		body := taskBody(toml, "eval-test")

		Expect(body).To(ContainSubstring(`description = "Run the eval module's specs (local only; CI never runs them)"`))
	})

	It("is a step of no other mise task, so no composite such as ci or verify runs the eval specs (AC-21)", func() {
		for _, name := range miseTaskNames(toml) {
			if name == "eval-test" {
				continue
			}
			Expect(taskBody(toml, name)).NotTo(ContainSubstring("eval-test"), "[tasks.%s] must not run eval-test", name)
		}
	})

	It("is reported when a workflow runs it (AC-21)", func() {
		workflow := "on:\n  push:\njobs:\n  check:\n    steps:\n      - run: mise run eval-test\n"

		Expect(evalWorkflowViolations("check.yml", workflow)).To(ConsistOf(ContainSubstring("eval-test")))
	})
})

func miseTaskNames(toml string) []string {
	var names []string
	for _, match := range regexp.MustCompile(`(?m)^\[tasks\.([A-Za-z0-9_-]+)\]`).FindAllStringSubmatch(toml, -1) {
		names = append(names, strings.TrimSpace(match[1]))
	}
	return names
}
