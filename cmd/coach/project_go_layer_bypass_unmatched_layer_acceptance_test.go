package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const handlerQueryingTheDatabaseDirectly = "package handlers\n\nimport (\n\t\"database/sql\"\n\t\"net/http\"\n)\n\nfunc Handler(w http.ResponseWriter, r *http.Request) {\n\tvar db *sql.DB\n\tdb.Query(\"SELECT 1\")\n}\n"

const requiredLayerUnmatchedKind = "project_layer_bypass_required_layer_unmatched"

var _ = Describe("coach codesignal --project-config names why architecture.layer_bypass could not run", func() {
	unmatchedRepo := func() string {
		repo := newTempGitRepo()
		commitFile(repo, "go.mod", goModuleFile)
		commitFile(repo, "pkg/handlers/handlers.go", handlerQueryingTheDatabaseDirectly)
		commitFile(repo, "project.json", goLayerBypassPolicyConfigJSON)
		return repo
	}

	When("the policy's required_layer matches no package in the analysed program", func() {
		It("adds a diagnostic that names the required_layer and says what to do, and keeps the incomplete-coverage signals", func() {
			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(unmatchedRepo(), "--project-config", "project.json", "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)

			report := decodeCoachReport(stdout)
			Expect(report.Diagnostics).To(ContainElement(And(
				HaveField("Kind", requiredLayerUnmatchedKind),
				HaveField("Message", And(
					ContainSubstring(`required_layer "service"`),
					ContainSubstring("matched no package"),
					ContainSubstring("correct its prefixes in the policy, or add the layer"),
				)),
			)))
			for _, kind := range []string{"project_coverage_incomplete", "project_layer_bypass_coverage_incomplete", "project_lifecycle_indeterminate"} {
				Expect(report.Diagnostics).To(ContainElement(HaveField("Kind", kind)), "the existing signal %s stays", kind)
			}
			Expect(report.ProjectChanges).To(BeEmpty())
		})

		It("prints the same cause in the text report, and still exits 3 under --fail-on-incomplete-coverage", func() {
			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(unmatchedRepo(), "--project-config", "project.json", "--fail-on-incomplete-coverage")

			Expect(exitCode).To(Equal(3), "stderr: %s stdout: %s", stderr, stdout)
			text := string(stdout)
			Expect(text).To(ContainSubstring(requiredLayerUnmatchedKind))
			Expect(text).To(ContainSubstring(`required_layer "service" matched no package`))
		})
	})

	When("the policy's required_layer matches a package", func() {
		It("does not emit that diagnostic, whether or not a bypass is found", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/service/service.go", servicePackageFile)
			commitFile(repo, "pkg/handlers/handlers.go", handlersCompliantAndBypass)
			commitFile(repo, "project.json", goLayerBypassPolicyConfigJSON)

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)

			report := decodeCoachReport(stdout)
			Expect(report.ProjectChanges).To(HaveLen(1))
			for _, diagnostic := range report.Diagnostics {
				Expect(diagnostic.Kind).NotTo(Equal(requiredLayerUnmatchedKind))
			}
		})
	})
})
