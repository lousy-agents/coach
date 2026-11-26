package main

import (
	"os"
	"path/filepath"
	"regexp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const tsRootsEmptyPolicyUnderSvc = `{"schema_version":"1","roots":["svc"]}` + "\n"

const tsRootsEmptyPolicyUnderSrc = `{"schema_version":"1","roots":["src"]}` + "\n"

const tsRootsEmptyPolicyUnderLib = `{"schema_version":"1","roots":["lib"]}` + "\n"

var tsMatchesNothingTSConfigJSON = `{"compilerOptions":{},"include":["nothing/**/*"]}` + "\n"

func tsRepoWithPolicy(policy string, files map[string]string) string {
	repo := newTempGitRepo()
	commitFile(repo, "package.json", tsRealCompilerPackageJSON(realTypescriptVersion()))
	installRealTypescriptCompiler(repo, true)
	for name, contents := range files {
		commitFile(repo, name, contents)
	}
	commitFile(repo, coachConfigPath, policy)
	return repo
}

func tsBaselineWithFlag(repo string, extra ...string) (stdout, stderr []byte, exitCode int) {
	args := append([]string{"--project-language", "typescript", "--no-interactive", "--format=json", "--fail-on-incomplete-coverage"}, extra...)
	return runCoachCodesignalBaselineRaw(repo, args...)
}

var _ = Describe("A TypeScript project analysis whose policy roots open no TypeScript project", Label("ts-project-backend"), func() {
	BeforeEach(func() {
		skipWithoutRealTypeScriptCompiler()
	})

	When("the root holds no tsconfig.json and no TypeScript", func() {
		It("reports project coverage incomplete with the cause, model incomplete, and exits 3 under --baseline (AC-1.1, AC-2.1, AC-4.1)", func() {
			repo := tsRepoWithPolicy(tsRootsEmptyPolicyUnderSvc, map[string]string{"svc/README.md": "docs\n", "tsconfig.json": tsProjectTSConfigJSON})

			stdout, stderr, exitCode := tsBaselineWithFlag(repo, "--baseline")

			report := decodeCoachReport(stdout)
			Expect(report.ProjectCoverage).NotTo(BeNil(), "stdout: %.400s", stdout)
			Expect(report.ProjectCoverage.Phase).To(Equal("ts_sidecar_build"))
			Expect(report.ProjectCoverage.Counts["tsconfig_count"]).To(Equal(0), "the fixture must open no project, or the rows below prove nothing")
			Expect(report.ProjectCoverage.Complete).To(BeFalse())
			_, named := rootsEmptyDiagnostic(report.ProjectCoverage)
			Expect(named).To(BeTrue(), "coverage diagnostics: %+v", report.ProjectCoverage.Diagnostics)
			Expect(report.ProjectProvenance).NotTo(BeNil())
			Expect(report.ProjectProvenance.Head.Coverage.Model).To(Equal("incomplete"))
			Expect(exitCode).To(Equal(3), "stderr: %s", stderr)
		})

		It("does the same under --worktree (AC-4.1)", func() {
			repo := tsRepoWithPolicy(tsRootsEmptyPolicyUnderSvc, map[string]string{"svc/README.md": "docs\n"})
			Expect(os.WriteFile(filepath.Join(repo, "svc", "README.md"), []byte("docs, edited\n"), 0o644)).To(Succeed())

			stdout, stderr, exitCode := runCoachBinary(commandPath, repo, nil,
				"codesignal", "--worktree", "--project-language", "typescript", "--no-interactive", "--format=json", "--fail-on-incomplete-coverage")

			report := decodeCoachReport(stdout)
			Expect(report.ProjectCoverage).NotTo(BeNil(), "stdout: %.400s", stdout)
			Expect(report.ProjectCoverage.Complete).To(BeFalse())
			Expect(exitCode).To(Equal(3), "stderr: %s", stderr)
		})

		It("does the same on the head side under --base (AC-4.1)", func() {
			repo := tsRepoWithPolicy(tsRootsEmptyPolicyUnderSvc, map[string]string{"svc/README.md": "docs\n"})
			commitFile(repo, "svc/README.md", "docs, edited\n")

			stdout, stderr, exitCode := runCoachBinary(commandPath, repo, nil,
				"codesignal", "--base", "HEAD~1", "--project-language", "typescript", "--no-interactive", "--format=json", "--fail-on-incomplete-coverage")

			report := decodeCoachReport(stdout)
			Expect(report.ProjectCoverage).NotTo(BeNil(), "stdout: %.400s", stdout)
			Expect(report.ProjectCoverage.Complete).To(BeFalse())
			Expect(exitCode).To(Equal(3), "stderr: %s", stderr)
		})

		It("names the language, the roots and both counts in the diagnostic message (AC-1.2)", func() {
			repo := tsRepoWithPolicy(tsRootsEmptyPolicyUnderSvc, map[string]string{"svc/README.md": "docs\n"})

			stdout, _, _ := tsBaselineWithFlag(repo, "--baseline")

			report := decodeCoachReport(stdout)
			Expect(report.ProjectCoverage).NotTo(BeNil(), "stdout: %.400s", stdout)
			diagnostic, named := rootsEmptyDiagnostic(report.ProjectCoverage)
			Expect(named).To(BeTrue(), "coverage diagnostics: %+v", report.ProjectCoverage.Diagnostics)
			Expect(diagnostic.Message).To(ContainSubstring("TypeScript"))
			Expect(diagnostic.Message).To(ContainSubstring("svc"))
			Expect(diagnostic.Message).To(ContainSubstring("tsconfig_count=0"))
			Expect(diagnostic.Message).To(ContainSubstring("files_seen=0"))
		})

		It("names the cause in the text report's project coverage section (AC-2.2)", func() {
			repo := tsRepoWithPolicy(tsRootsEmptyPolicyUnderSvc, map[string]string{"svc/README.md": "docs\n"})

			stdout, _, _ := runCoachCodesignalBaselineRaw(repo, "--project-language", "typescript", "--no-interactive")

			Expect(string(stdout)).To(ContainSubstring(rootsEmptyDiagnosticCode))
		})

		It("still exits 0 without the flag, because coach is advisory by default", func() {
			repo := tsRepoWithPolicy(tsRootsEmptyPolicyUnderSvc, map[string]string{"svc/README.md": "docs\n"})

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-language", "typescript", "--no-interactive", "--format=json")

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(decodeCoachReport(stdout).ProjectCoverage.Complete).To(BeFalse())
		})
	})

	When("the root holds a package.json and no tsconfig.json", func() {
		It("is incomplete although files_seen is 1, because tsconfig_count is 0 (AC-1.1)", func() {
			repo := tsRepoWithPolicy(tsRootsEmptyPolicyUnderLib, map[string]string{"lib/package.json": tsRealCompilerPackageJSON(realTypescriptVersion())})
			installRealTypescriptCompiler(filepath.Join(repo, "lib"), true)

			stdout, stderr, exitCode := tsBaselineWithFlag(repo, "--baseline")

			report := decodeCoachReport(stdout)
			Expect(report.ProjectCoverage).NotTo(BeNil(), "stdout: %.400s", stdout)
			Expect(report.ProjectCoverage.Counts["files_seen"]).To(BeNumerically(">", 0), "the fixture must forward a file, or this row proves nothing beyond the one above")
			Expect(report.ProjectCoverage.Complete).To(BeFalse())
			Expect(exitCode).To(Equal(3), "stderr: %s", stderr)
		})
	})

	When("the root holds a tsconfig.json that opens a project", func() {
		It("reports project coverage as it does today (AC-3.1, regression row)", func() {
			repo := tsRepoWithPolicy(singleRootPolicyJSON, map[string]string{"tsconfig.json": tsProjectTSConfigJSON, "pkg/db/d.ts": tsRealDbFile})

			stdout, stderr, exitCode := tsBaselineWithFlag(repo, "--baseline")

			report := decodeCoachReport(stdout)
			Expect(report.ProjectCoverage.Counts["tsconfig_count"]).To(BeNumerically(">", 0))
			Expect(report.ProjectCoverage.Complete).To(BeTrue())
			_, named := rootsEmptyDiagnostic(report.ProjectCoverage)
			Expect(named).To(BeFalse())
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		})
	})

	When("the root holds TypeScript source and no tsconfig.json", func() {
		It("keeps its own incomplete verdict and does not report the empty-roots cause (AC-3.1, regression row)", func() {
			repo := tsRepoWithPolicy(tsRootsEmptyPolicyUnderSrc, map[string]string{"src/a.ts": tsRealDbFile})

			stdout, stderr, exitCode := tsBaselineWithFlag(repo, "--baseline")

			report := decodeCoachReport(stdout)
			Expect(report.ProjectCoverage.Complete).To(BeFalse())
			Expect(report.ProjectCoverage.Counts["tsconfig_count"]).To(Equal(0), "the fixture must open no project, or the clause that excludes this row proves nothing")
			_, named := rootsEmptyDiagnostic(report.ProjectCoverage)
			Expect(named).To(BeFalse(), "the source is there, so the cause is ts_no_project_config")
			Expect(exitCode).To(Equal(3), "stderr: %s", stderr)
		})
	})

	When("the root holds a tsconfig.json that matches no input", func() {
		It("keeps its own incomplete verdict and does not report the empty-roots cause (AC-3.1, regression row)", func() {
			repo := tsRepoWithPolicy(singleRootPolicyJSON, map[string]string{"tsconfig.json": tsMatchesNothingTSConfigJSON})

			stdout, stderr, exitCode := tsBaselineWithFlag(repo, "--baseline")

			report := decodeCoachReport(stdout)
			Expect(report.ProjectCoverage.Complete).To(BeFalse())
			_, named := rootsEmptyDiagnostic(report.ProjectCoverage)
			Expect(named).To(BeFalse(), "a project was opened, so the cause is the compiler's own diagnostic")
			Expect(exitCode).To(Equal(3), "stderr: %s", stderr)
		})
	})

	When("only the base side opens no project, because the diff adds the first one", func() {
		It("keeps the base coverage as it is today and exits 0 (AC-3.2, regression row)", func() {
			repo := tsRepoWithPolicy(singleRootPolicyJSON, map[string]string{"README.md": "before the project\n"})
			commitFile(repo, "tsconfig.json", tsProjectTSConfigJSON)
			commitFile(repo, "pkg/db/d.ts", tsRealDbFile)

			stdout, stderr, exitCode := runCoachBinary(commandPath, repo, nil,
				"codesignal", "--base", "HEAD~2", "--project-language", "typescript", "--no-interactive", "--format=json", "--fail-on-incomplete-coverage")

			report := decodeCoachReport(stdout)
			Expect(report.ProjectCoverage.Counts["tsconfig_count"]).To(BeNumerically(">", 0), "the head side must open a project")
			Expect(report.ProjectCoverage.Complete).To(BeTrue())
			Expect(exitCode).To(Equal(0), "stdout: %.400s stderr: %s", stdout, stderr)
		})
	})

	It("docs/cli-codesignal.md names the TypeScript case in the --fail-on-incomplete-coverage row (AC-4.2)", func() {
		flagRow := regexp.MustCompile("\\| `--fail-on-incomplete-coverage` \\|[^|]*\\|[^|]*\\|").FindString(documentText("docs/cli-codesignal.md"))

		Expect(flagRow).To(ContainSubstring("TypeScript analysis whose head side opened no project"))
	})
})
