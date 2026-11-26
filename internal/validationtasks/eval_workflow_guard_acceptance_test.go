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
			Entry("the eval module's go.mod", "      - run: go mod download -modfile eval/go.mod\n"),
			Entry("a quoted working-directory", "      - working-directory: \"eval\"\n        run: go vet ./...\n"),
			Entry("a quoted cd", "      - run: cd 'eval' && go test ./...\n"),
			Entry("a quoted go -C", "      - run: go -C \"eval\" vet ./...\n"),
		)
	})

	When("a scheduled workflow runs an eval task", func() {
		It("reports the schedule trigger as well as the invocation", func() {
			workflow := "on:\n  schedule:\n    - cron: '0 3 * * 1'\njobs:\n  study:\n    steps:\n      - run: mise run eval-agent-study\n"

			Expect(evalWorkflowViolations("nightly.yml", workflow)).To(ContainElement(ContainSubstring("schedule")))
		})
	})

	When("an eval task appears only in a comment", func() {
		It("reports nothing", func() {
			workflow := "on:\n  push:\njobs:\n  check:\n    steps:\n      # mise run eval-detection runs locally, never here\n      - run: mise run verify\n"

			Expect(evalWorkflowViolations("check.yml", workflow)).To(BeEmpty())
		})
	})

	When("a workflow only mentions words that contain eval", func() {
		It("reports nothing", func() {
			workflow := "on:\n  schedule:\n    - cron: '0 3 * * 1'\njobs:\n  docs:\n    steps:\n      - run: echo retrieval-augmented evaluation notes\n      - run: mise run verify\n"

			Expect(evalWorkflowViolations("docs.yml", workflow)).To(BeEmpty())
		})
	})

	When("the automation files under .github are listed", func() {
		It("includes composite actions and .yaml workflows, and nothing that is not YAML", func() {
			github := filepath.Join(GinkgoT().TempDir(), ".github")
			for _, path := range []string{"actions/setup/action.yml", "workflows/nightly.yaml", "workflows/README.md"} {
				full := filepath.Join(github, path)
				Expect(os.MkdirAll(filepath.Dir(full), 0o755)).To(Succeed())
				Expect(os.WriteFile(full, []byte("name: x\n"), 0o644)).To(Succeed())
			}

			Expect(githubAutomationFiles(github)).To(ConsistOf(
				filepath.Join(github, "actions", "setup", "action.yml"),
				filepath.Join(github, "workflows", "nightly.yaml"),
			))
		})
	})

	When("the repository's own workflows are read", func() {
		It("finds no workflow or composite action under .github that runs an eval task or an eval-module leg", func() {
			files := githubAutomationFiles(filepath.Join("..", "..", ".github"))
			Expect(files).To(ContainElement(HaveSuffix("ci.yml")))

			var violations []string
			for _, path := range files {
				raw, err := os.ReadFile(path)
				Expect(err).NotTo(HaveOccurred())
				violations = append(violations, evalWorkflowViolations(path, string(raw))...)
			}

			Expect(violations).To(BeEmpty(), "the evaluation runs only as local mise tasks (docs/evaluation/agentic-eval-design.md §10)")
		})
	})
})
