package main

import (
	"os"
	"path/filepath"
	"regexp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/domain"
)

func rootsEmptyDiagnostic(coverage *domain.Coverage) (domain.Diagnostic, bool) {
	for _, diagnostic := range coverage.Diagnostics {
		if diagnostic.Code == rootsEmptyDiagnosticCode {
			return diagnostic, true
		}
	}
	return domain.Diagnostic{}, false
}

var _ = Describe("A Go project analysis whose policy roots hold no Go source", func() {
	When("the repository is TypeScript only and the Go analysis is the default", func() {
		It("reports project coverage incomplete with the cause and exits 3 under --worktree (AC-1.1, AC-2.1, AC-4.1)", func() {
			repo := typescriptOnlyRepo()
			Expect(os.WriteFile(filepath.Join(repo, "pkg", "handlers", "h.ts"), []byte(typescriptHandlerEdited), 0o644)).To(Succeed())

			stdout, stderr, exitCode := runCoachBinary(commandPath, repo, nil,
				"codesignal", "--worktree", "--no-interactive", "--fail-on-incomplete-coverage", "--format", "json")

			report := decodeCoachReport(stdout)
			Expect(report.ProjectCoverage).NotTo(BeNil(), "stdout: %.400s", stdout)
			Expect(report.ProjectCoverage.Phase).To(Equal("go_model_build"))
			Expect(report.ProjectCoverage.Complete).To(BeFalse())
			_, named := rootsEmptyDiagnostic(report.ProjectCoverage)
			Expect(named).To(BeTrue(), "coverage diagnostics: %+v", report.ProjectCoverage.Diagnostics)
			Expect(exitCode).To(Equal(3), "stderr: %s", stderr)
		})

		It("does the same under --baseline (AC-4.1)", func() {
			repo := typescriptOnlyRepo()

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json", "--fail-on-incomplete-coverage")

			report := decodeCoachReport(stdout)
			Expect(report.ProjectCoverage).NotTo(BeNil(), "stdout: %.400s", stdout)
			Expect(report.ProjectCoverage.Complete).To(BeFalse())
			Expect(exitCode).To(Equal(3), "stderr: %s", stderr)
		})

		It("does the same on the head side under --base (AC-4.1)", func() {
			repo := typescriptOnlyRepo()
			commitFile(repo, "pkg/handlers/h.ts", typescriptHandlerEdited)

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, "HEAD~1", "--format=json", "--fail-on-incomplete-coverage")

			report := decodeCoachReport(stdout)
			Expect(report.ProjectCoverage).NotTo(BeNil(), "stdout: %.400s", stdout)
			Expect(report.ProjectCoverage.Complete).To(BeFalse())
			Expect(exitCode).To(Equal(3), "stderr: %s", stderr)
		})

		It("names the roots and both counts in the diagnostic message (AC-1.2)", func() {
			repo := newTempGitRepo()
			commitFile(repo, "services/README.md", "services\n")
			commitFile(repo, "pkg/db/db.go", dbPackageFile)
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, ".lousy-agents/coach.json", goRootsEmptyPolicyUnderServices)

			stdout, _, _ := runCoachCodesignalBaselineRaw(repo, "--format=json")

			report := decodeCoachReport(stdout)
			Expect(report.ProjectCoverage).NotTo(BeNil(), "stdout: %.400s", stdout)
			diagnostic, named := rootsEmptyDiagnostic(report.ProjectCoverage)
			Expect(named).To(BeTrue(), "coverage diagnostics: %+v", report.ProjectCoverage.Diagnostics)
			Expect(diagnostic.Message).To(ContainSubstring("services"))
			Expect(diagnostic.Message).To(ContainSubstring("roots_seen=0"))
			Expect(diagnostic.Message).To(ContainSubstring("files_seen=0"))
		})

		It("names the cause in the text report's project coverage section (AC-2.2)", func() {
			repo := typescriptOnlyRepo()

			stdout, _, _ := runCoachCodesignalBaselineRaw(repo)

			Expect(string(stdout)).To(ContainSubstring(rootsEmptyDiagnosticCode))
		})

		It("still exits 0 without the flag, because coach is advisory by default", func() {
			repo := typescriptOnlyRepo()

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json")

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(decodeCoachReport(stdout).ProjectCoverage.Complete).To(BeFalse())
		})
	})

	When("the roots hold a Go module with no Go file", func() {
		It("is incomplete although roots_seen is 1, because files_seen is 0 (AC-1.1)", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "README.md", "docs only\n")
			commitFile(repo, ".lousy-agents/coach.json", goLayerPolicyConfigJSON)

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json", "--fail-on-incomplete-coverage")

			report := decodeCoachReport(stdout)
			Expect(report.ProjectCoverage).NotTo(BeNil(), "stdout: %.400s", stdout)
			Expect(report.ProjectCoverage.Counts["roots_seen"]).To(Equal(1), "the fixture must hold a module, or this row proves nothing beyond the TypeScript one")
			Expect(report.ProjectCoverage.Complete).To(BeFalse())
			Expect(exitCode).To(Equal(3), "stderr: %s", stderr)
		})
	})

	When("the roots hold Go source", func() {
		It("reports project coverage as it does today (AC-3.1, regression row)", func() {
			repo := goLayeredRepo()

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo,
				"--project-config", "project.json", "--format=json", "--fail-on-incomplete-coverage")

			report := decodeCoachReport(stdout)
			Expect(report.ProjectCoverage.Complete).To(BeTrue())
			_, named := rootsEmptyDiagnostic(report.ProjectCoverage)
			Expect(named).To(BeFalse())
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		})
	})

	When("only the base side holds no Go source, because the diff adds the project", func() {
		It("keeps the base coverage as it is today and exits 0 (AC-3.2, regression row)", func() {
			repo := newTempGitRepo()
			commitFile(repo, ".lousy-agents/coach.json", goLayerPolicyConfigJSON)
			commitFile(repo, "README.md", "before the project\n")
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/db/db.go", dbPackageFile)
			commitFile(repo, "pkg/handlers/handlers.go", handlersWithoutImport)

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, "HEAD~3", "--format=json", "--fail-on-incomplete-coverage")

			report := decodeCoachReport(stdout)
			Expect(report.ProjectCoverage.Counts["files_seen"]).To(BeNumerically(">", 0), "the head side must hold Go source")
			Expect(report.ProjectCoverage.Complete).To(BeTrue())
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		})
	})

	It("docs/cli-codesignal.md names project_roots_empty in the --fail-on-incomplete-coverage row (AC-4.2)", func() {
		flagRow := regexp.MustCompile("\\| `--fail-on-incomplete-coverage` \\|[^|]*\\|[^|]*\\|").FindString(documentText("docs/cli-codesignal.md"))

		Expect(flagRow).To(ContainSubstring(rootsEmptyDiagnosticCode))
	})
})
