package main

import (
	"regexp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var emptyProjectDiagnosticLine = regexp.MustCompile(`(?m)^\s*project diagnostic: \S+: *$`)

var _ = Describe("a project diagnostic always says what it found (#894)", func() {
	// The required layer's package sits outside the policy's roots, so the layer-bypass search cannot apply it and
	// reports project_layer_bypass_ambiguous_layer.
	outsideRootsPolicy := `{"schema_version":"1","roots":["pkg/handlers"],"layers":[{"name":"handlers","prefixes":["pkg/handlers"]},{"name":"service","prefixes":["pkg/service"]},{"name":"db","prefixes":["pkg/db"]}],"required_layer":"service"}`
	outsideRootsRepo := func() string {
		repo := newTempGitRepo()
		commitFile(repo, "go.mod", goModuleFile)
		commitFile(repo, "pkg/handlers/handlers.go", handlerQueryingTheDatabaseDirectly)
		commitFile(repo, "pkg/service/svc.go", "package service\n\nfunc F() {}\n")
		commitFile(repo, "project.json", outsideRootsPolicy)
		return repo
	}

	It("gives project_layer_bypass_ambiguous_layer a message in the JSON report, and says what to do", func() {
		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(outsideRootsRepo(), "--project-config", "project.json", "--format=json")
		Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)

		coverage := decodeCoachReport(stdout).ProjectCoverage
		Expect(coverage).NotTo(BeNil())
		var found bool
		for _, diagnostic := range coverage.Diagnostics {
			Expect(diagnostic.Message).NotTo(BeEmpty(), "diagnostic %s has no message", diagnostic.Code)
			if diagnostic.Code == "project_layer_bypass_ambiguous_layer" {
				found = true
				Expect(diagnostic.Message).To(ContainSubstring("required_layer"))
				Expect(diagnostic.Message).To(ContainSubstring("policy"))
			}
		}
		Expect(found).To(BeTrue(), "the scenario reports project_layer_bypass_ambiguous_layer: %s", stdout)
	})

	It("prints no project diagnostic line without its message in the text report", func() {
		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(outsideRootsRepo(), "--project-config", "project.json")
		Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)

		Expect(string(stdout)).To(ContainSubstring("project diagnostic: project_layer_bypass_ambiguous_layer: "))
		Expect(emptyProjectDiagnosticLine.FindAllString(string(stdout), -1)).To(BeEmpty())
	})
})
