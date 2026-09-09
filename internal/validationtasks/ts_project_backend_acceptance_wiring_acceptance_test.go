package validationtasks

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ts-project-backend-acceptance wiring", func() {
	var toml, yml, gitignore string

	BeforeEach(func() {
		rawTOML, err := os.ReadFile(filepath.Join("..", "..", "mise.toml"))
		Expect(err).NotTo(HaveOccurred())
		toml = string(rawTOML)

		rawYML, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "ci.yml"))
		Expect(err).NotTo(HaveOccurred())
		yml = string(rawYML)

		rawGitignore, err := os.ReadFile(filepath.Join("..", "..", ".gitignore"))
		Expect(err).NotTo(HaveOccurred())
		gitignore = string(rawGitignore)
	})

	When("the TS-dependent project-backend acceptance specs must run somewhere with real Node dependencies installed", func() {
		It("has a mise task that installs js/semantics dependencies before running them", func() {
			body := taskBody(toml, "ts-project-backend-acceptance")
			Expect(body).NotTo(BeEmpty(),
				"without this task, nothing in the repository installs Node deps and runs the ts-project-backend-labeled specs together")
			Expect(body).To(ContainSubstring(`"js-install"`),
				"the specs copy js/semantics/node_modules/typescript on disk; the task must depend on the task that installs it")
		})

		It("scopes the run to the labeled specs across both packages that need the real compiler", func() {
			body := taskBody(toml, "ts-project-backend-acceptance")
			Expect(body).NotTo(BeEmpty())
			Expect(body).To(ContainSubstring("./cmd/coach/..."))
			Expect(body).To(ContainSubstring("./internal/codesignalcli/..."))
			Expect(body).To(ContainSubstring("ginkgo.label-filter=ts-project-backend"),
				"-run Acceptance alone would also run the specs that already skip gracefully without Node -- fail-on-empty below is what proves this filter still matches something")
			Expect(body).To(ContainSubstring("ginkgo.fail-on-empty"),
				"a typo'd or removed Label would otherwise run zero specs and pass")
		})

		It("is reachable from a CI job that installs both Go and Node", func() {
			jobYML := jobBody(yml, "ts-project-backend")
			Expect(jobYML).NotTo(BeEmpty(),
				"a task no job runs is a task that never executes in CI")
			Expect(jobYML).To(ContainSubstring("mise run ts-project-backend-acceptance"))
			Expect(jobYML).NotTo(ContainSubstring("install_args: go"),
				"this job needs Node for js-install too; install_args: go would leave npm absent")
		})

		It("is a required leaf in the status aggregator", func() {
			statusBody := jobBody(yml, "status")
			Expect(statusBody).NotTo(BeEmpty())
			Expect(needsList(statusBody)).To(ContainElement("ts-project-backend"),
				"a leaf job missing from status.needs can fail while the required check stays green")
		})
	})

	// coach#355 Task 14 (AC-3/AC-7): "both promised majors carry execution
	// evidence" is only true if a spec ties CI wiring to
	// codesignalcli.SupportedNodeMajors -- otherwise deleting the node26 job,
	// or its printf override step, leaves every other spec in this suite
	// green.
	When("binding both promised Node majors to real CI execution evidence", func() {
		It("has exactly the two legs this spec's mapping covers: mise.toml's shared pin, and one dedicated override job", func() {
			Expect(codesignalcli.SupportedNodeMajors).To(HaveLen(2),
				"a third supported major needs a third leg (and a third spec here) before it can claim coverage; this spec cannot silently keep passing for a set it was not written against")
		})

		It("pins mise.toml's shared Node major to the first supported major, which ts-project-backend already exercises", func() {
			first := fmt.Sprintf(`node = "%d"`, codesignalcli.SupportedNodeMajors[0])
			Expect(toml).To(ContainSubstring(first),
				"ts-project-backend runs under mise.toml's shared pin with no override, so that pin is the first major's only execution evidence")
		})

		It("is reachable from a CI job that installs both Go and Node", func() {
			jobYML := jobBody(yml, "ts-project-backend-node26")
			Expect(jobYML).NotTo(BeEmpty(),
				"a job absent from ci.yml is evidence for nothing")
			Expect(jobYML).To(ContainSubstring("mise run ts-project-backend-acceptance"))
			Expect(jobYML).NotTo(ContainSubstring("install_args: go"),
				"this job needs Node for js-install too; install_args: go would leave npm absent")
		})

		It("overrides the Node major to the second supported major via a git-ignored mise.local.toml", func() {
			jobYML := jobBody(yml, "ts-project-backend-node26")
			Expect(jobYML).NotTo(BeEmpty())
			second := fmt.Sprintf(`node = "%d"`, codesignalcli.SupportedNodeMajors[len(codesignalcli.SupportedNodeMajors)-1])
			Expect(jobYML).To(ContainSubstring(second),
				"without this override the job would just re-run mise.toml's pinned major, and AC-7's two-major evidence would collapse to one")
			Expect(jobYML).To(ContainSubstring("mise.local.toml"))
		})

		It("proves the resolved runtime Node major is actually 26 before running the acceptance step", func() {
			jobYML := jobBody(yml, "ts-project-backend-node26")
			Expect(jobYML).NotTo(BeEmpty())
			secondMajor := fmt.Sprintf("%d", codesignalcli.SupportedNodeMajors[len(codesignalcli.SupportedNodeMajors)-1])
			Expect(jobYML).To(ContainSubstring(`mise exec -- node -p 'process.versions.node.split(".")[0]'`),
				"without observing the resolved Node major at runtime, a silently-ignored mise.local.toml override would leave this job re-running the Node 24 leg while staying green")
			Expect(jobYML).To(ContainSubstring(`test "$major" = "`+secondMajor+`"`),
				"the guard must fail closed when the resolved major is not the second supported major")

			guardIdx := strings.Index(jobYML, `mise exec -- node -p`)
			acceptanceIdx := strings.Index(jobYML, "mise run ts-project-backend-acceptance")
			Expect(guardIdx).To(BeNumerically(">", 0), "guard step must be present")
			Expect(acceptanceIdx).To(BeNumerically(">", guardIdx),
				"the guard must run before the acceptance step it is protecting, not after")
		})

		It("keeps mise.local.toml out of version control, or the override would leak into every other job's Node major", func() {
			Expect(gitignore).To(MatchRegexp(`(?m)^mise\.local\.toml$`),
				"mise.local.toml has higher precedence than mise.toml; a committed copy would override node for every job, not just this one")
			Expect(gitignore).To(MatchRegexp(`(?m)^\.mise\.local\.toml$`))
		})

		It("installs the same Linux file-syscall control ts-project-backend runs", func() {
			Expect(jobBody(yml, "ts-project-backend-node26")).To(ContainSubstring("strace"))
		})

		It("is a required leaf in the status aggregator", func() {
			statusBody := jobBody(yml, "status")
			Expect(statusBody).NotTo(BeEmpty())
			Expect(needsList(statusBody)).To(ContainElement("ts-project-backend-node26"),
				"a leaf job missing from status.needs can fail while the required check stays green")
		})
	})

	When("checking the ts-project-backend Label the mise task's filter selects", func() {
		It("is actually applied to the cmd/coach specs the task's filter selects", func() {
			src, err := os.ReadFile(filepath.Join("..", "..", "cmd", "coach", "project_ts_backend_acceptance_test.go"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(src)).To(ContainSubstring(`Label("ts-project-backend")`),
				"the mise task's label filter is useless unless at least one Describe/When here carries this label")
		})

		It("is actually applied to the internal/codesignalcli specs the task's filter selects", func() {
			src, err := os.ReadFile(filepath.Join("..", "..", "internal", "codesignalcli", "project_acceptance_test.go"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(src)).To(ContainSubstring(`Label("ts-project-backend")`),
				"the mise task's label filter is useless unless at least one Describe here carries this label")
		})
	})

	When("checking the Linux file-syscall control the ts-project-backend job must run", func() {
		var body string

		BeforeEach(func() {
			src, err := os.ReadFile(filepath.Join("..", "..", "cmd", "coach", "project_ts_linux_confinement_acceptance_test.go"))
			Expect(err).NotTo(HaveOccurred(),
				"Linux file-syscall controls must live in cmd/coach/project_ts_linux_confinement_acceptance_test.go")
			body = string(src)
		})

		It("keeps confined and unconfined Its labeled ts-project-backend", func() {
			Expect(body).To(ContainSubstring(`Label("ts-project-backend")`))
			Expect(body).To(ContainSubstring(`It("completes a confined --baseline scan whose analyzer-subtree file syscalls stay inside the frozen allowlist and never observe the typeRoots decoy"`))
			Expect(body).To(ContainSubstring(`It("records the typeRoots decoy in the analyzer-subtree trace when the harness-built unconfined analyzer omits tsserverPath and restores listing fall-through"`))
		})

		It("keeps asserting that the decoy must appear", func() {
			Expect(body).To(ContainSubstring("decoy must appear"))
		})

		It("installs strace in the ts-project-backend job", func() {
			Expect(jobBody(yml, "ts-project-backend")).To(ContainSubstring("strace"))
		})
	})
})
