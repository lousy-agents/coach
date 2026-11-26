package validationtasks

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("the guard that keeps the evaluation out of CI", func() {
	When("a workflow runs a mise eval task", func() {
		It("reports the invocation", func() {
			workflow := "on:\n  push:\njobs:\n  check:\n    steps:\n      - run: mise run eval-detection\n"

			Expect(evalWorkflowViolations("check.yml", workflow)).To(ConsistOf(ContainSubstring("eval-detection")))
		})
	})

	When("a workflow runs a leg of the eval module", func() {
		DescribeTable("it reports the leg",
			func(step string) {
				workflow := "on:\n  push:\njobs:\n  check:\n    steps:\n" + step

				Expect(evalWorkflowViolations("check.yml", workflow)).NotTo(BeEmpty())
			},
			Entry("go -C eval", "      - run: go -C eval vet ./...\n"),
			Entry("cd eval", "      - run: cd eval && go test ./...\n"),
			Entry("working-directory: eval", "      - working-directory: eval\n        run: go mod tidy\n"),
			Entry("a ./eval/... package pattern", "      - run: go test ./eval/...\n"),
		)
	})

	When("a scheduled workflow runs an eval task", func() {
		It("reports the schedule trigger as well as the invocation", func() {
			workflow := "on:\n  schedule:\n    - cron: '0 3 * * 1'\njobs:\n  study:\n    steps:\n      - run: mise run eval-agent-study\n"

			Expect(evalWorkflowViolations("nightly.yml", workflow)).To(ContainElement(ContainSubstring("schedule")))
		})
	})

	When("a workflow only mentions words that contain eval", func() {
		It("reports nothing", func() {
			workflow := "on:\n  schedule:\n    - cron: '0 3 * * 1'\njobs:\n  docs:\n    steps:\n      - run: echo retrieval-augmented evaluation notes\n      - run: mise run verify\n"

			Expect(evalWorkflowViolations("docs.yml", workflow)).To(BeEmpty())
		})
	})

	When("the repository's own workflows are read", func() {
		It("finds no file under .github/workflows that runs an eval task or an eval-module leg", func() {
			dir := filepath.Join("..", "..", ".github", "workflows")
			entries, err := os.ReadDir(dir)
			Expect(err).NotTo(HaveOccurred())
			Expect(entries).NotTo(BeEmpty())

			var violations []string
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				raw, err := os.ReadFile(filepath.Join(dir, entry.Name()))
				Expect(err).NotTo(HaveOccurred())
				violations = append(violations, evalWorkflowViolations(entry.Name(), string(raw))...)
			}

			Expect(violations).To(BeEmpty(), "the evaluation runs only as local mise tasks (docs/evaluation/agentic-eval-design.md §10)")
		})
	})
})
