package validationtasks

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ts-project-backend-acceptance wiring", func() {
	var toml, yml string

	BeforeEach(func() {
		rawTOML, err := os.ReadFile(filepath.Join("..", "..", "mise.toml"))
		Expect(err).NotTo(HaveOccurred())
		toml = string(rawTOML)

		rawYML, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "ci.yml"))
		Expect(err).NotTo(HaveOccurred())
		yml = string(rawYML)
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
})
