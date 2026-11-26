package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func coverageDiagnosticCodes(stdout []byte) []string {
	report := decodeCoachReport(stdout)
	Expect(report.ProjectCoverage).NotTo(BeNil(), "stdout: %.400s", stdout)
	codes := make([]string, 0, len(report.ProjectCoverage.Diagnostics))
	for _, diagnostic := range report.ProjectCoverage.Diagnostics {
		codes = append(codes, diagnostic.Code)
	}
	return codes
}

var _ = Describe("A Go project analysis that was already incomplete for another cause", func() {
	When("the repository holds Go source and discovery stopped at the input-file budget", func() {
		It("names only that cause and never claims the roots hold no Go source (AC-1.3)", func() {
			repo := newTempGitRepo()
			commitFile(repo, "services/app/go.mod", goModuleFile)
			commitFile(repo, "services/app/pkg/db/db.go", dbPackageFile)
			commitFile(repo, ".lousy-agents/coach.json", `{"schema_version":"1","roots":["."],"layers":[{"name":"db","prefixes":["services/app/pkg/db"]}],"forbidden_imports":[]}`)
			commitJunkFiles(repo, junkFileCount)

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json", "--fail-on-incomplete-coverage")

			codes := coverageDiagnosticCodes(stdout)
			Expect(codes).To(ContainElement("project_root_incomplete"), "the fixture must truncate discovery, or this row proves nothing")
			Expect(decodeCoachReport(stdout).ProjectCoverage.Counts["files_seen"]).To(BeZero(), "the junk sorts before the module, so discovery stops before it: the case where files_seen alone misleads")
			Expect(codes).NotTo(ContainElement(rootsEmptyDiagnosticCode))
			Expect(exitCode).To(Equal(3), "stderr: %s", stderr)
		})
	})

	When("a policy root names a directory that does not exist", func() {
		It("is incomplete and says the roots hold no Go source (AC-1.1)", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/db/db.go", dbPackageFile)
			commitFile(repo, ".lousy-agents/coach.json", `{"schema_version":"1","roots":["nodir"],"layers":[{"name":"handlers","prefixes":["nodir/handlers"]},{"name":"db","prefixes":["nodir/db"]}],"forbidden_imports":[{"from":"handlers","to":"db"}]}`)

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json", "--fail-on-incomplete-coverage")

			Expect(coverageDiagnosticCodes(stdout)).To(ContainElement(rootsEmptyDiagnosticCode))
			Expect(decodeCoachReport(stdout).ProjectCoverage.Complete).To(BeFalse())
			Expect(exitCode).To(Equal(3), "stderr: %s", stderr)
		})
	})
})
