package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

var _ = Describe("coach codesignal --project-language typescript stamps side/revision onto promoted model-coverage diagnostics (coach#334 Task 11 T2, AC-VER-3)", func() {
	BeforeEach(func() {
		if reason := ensureRealTypeScriptCompilerAvailable(); reason != "" {
			Skip(reason)
		}
	})

	When("the SA-280-025 root-scope mismatch is on the head revision only, in a baseline (single-revision) run", Label("ts-project-backend"), func() {
		It("identifies the unanalyzed path with side=head and the head revision's resolved SHA, named in the diagnostic text", func() {
			repo := newTempGitRepo()
			version := realTypescriptVersion()
			commitFile(repo, "package.json", tsRealCompilerPackageJSON(version))
			commitFile(repo, "tsconfig.json", tsRootScopeGapTSConfigJSON)
			commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
			commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersImportingDB)
			commitFile(repo, "project.json", goLayerPolicyConfigJSON)
			installRealTypescriptCompiler(repo, true)

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)

			report := decodeCoachReport(stdout)
			Expect(report.Scope.Revision).NotTo(BeEmpty(), "sanity: the CLI must have resolved a head revision for this spec to check Revision against")

			diag := findDiagnosticWithPath(report.Diagnostics, "package.json")
			Expect(diag).NotTo(BeNil(), "expected an unanalyzed-path diagnostic naming package.json, got %+v", report.Diagnostics)
			Expect(diag.Kind).To(Equal(projectmodel.DiagRootScopeIncomplete), "a head-side promoted model-coverage Kind must stay unprefixed, got %+v", diag)
			Expect(diag.Side).To(Equal("head"), "got %+v", diag)
			Expect(diag.Revision).To(Equal(report.Scope.Revision), "got %+v", diag)
			Expect(diag.Message).To(ContainSubstring("head"), "the diagnostic text must name the side, got %q", diag.Message)
			Expect(diag.Message).To(ContainSubstring(report.Scope.Revision), "the diagnostic text must name the revision, got %q", diag.Message)
		})
	})

	When("the SA-280-025 root-scope mismatch is on the base revision only, in a --base diff run", Label("ts-project-backend"), func() {
		It("identifies the unanalyzed path with side=base and the base revision's resolved SHA, named in the diagnostic text", func() {
			repo := newTempGitRepo()
			version := realTypescriptVersion()
			commitFile(repo, "package.json", tsRealCompilerPackageJSON(version))
			commitFile(repo, "tsconfig.json", tsRootScopeGapTSConfigJSON)
			commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
			commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersImportingDB)
			baseSHA := commitFile(repo, "project.json", goLayerPolicyConfigJSON)
			commitFile(repo, "tsconfig.json", tsProjectTSConfigJSON)
			installRealTypescriptCompiler(repo, true)

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, baseSHA, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)

			report := decodeCoachReport(stdout)
			Expect(report.Scope.Base).To(Equal(baseSHA))

			diag := findDiagnosticWithPath(report.Diagnostics, "package.json")
			Expect(diag).NotTo(BeNil(), "expected an unanalyzed-path diagnostic naming package.json, got %+v", report.Diagnostics)
			Expect(diag.Kind).To(Equal("base_"+projectmodel.DiagRootScopeIncomplete), "a base-side promoted model-coverage Kind must carry the base_ prefix, matching baseProjectDiagnostics' convention, got %+v", diag)
			Expect(diag.Side).To(Equal("base"), "got %+v", diag)
			Expect(diag.Revision).To(Equal(baseSHA), "got %+v", diag)
			Expect(diag.Message).To(ContainSubstring("base"), "the diagnostic text must name the side, got %q", diag.Message)
			Expect(diag.Message).To(ContainSubstring(baseSHA), "the diagnostic text must name the revision, got %q", diag.Message)
		})
	})

	When("the analyzed repository has a routine, per-hop reachability gap but no root-scope incompleteness", Label("ts-project-backend"), func() {
		It("promotes no report.Diagnostics entry for the reachability gap", func() {
			repo := newTempGitRepo()
			version := realTypescriptVersion()
			commitFile(repo, "package.json", tsRealCompilerPackageJSON(version))
			commitFile(repo, "tsconfig.json", tsProjectTSConfigJSON)
			commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
			commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersImportingDB)
			commitFile(repo, "vendor/prisma-client/package.json", tsPrismaClientPackageJSON)
			commitFile(repo, "vendor/prisma-client/index.ts", tsPrismaClientIndexTS)
			commitFile(repo, "pkg/handlers/reach.ts", tsHandlersReachabilityFile)
			commitFile(repo, "pkg/handlers/helper.ts", tsHandlersLocalGapHelperFile)
			commitFile(repo, "pkg/handlers/gap.ts", tsHandlersLocalGapFile)
			commitFile(repo, "project.json", goLayerPolicyConfigJSON)
			installRealTypescriptCompiler(repo, true)

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)

			report := decodeCoachReport(stdout)
			Expect(report.ProjectCoverage).NotTo(BeNil())
			Expect(report.ProjectCoverage.Complete).To(BeTrue(), "sanity: a routine reachability gap must never mark project coverage incomplete, got %+v", report.ProjectCoverage)
			Expect(containsProjectModelDiagnosticCode(report.ProjectCoverage.Diagnostics, "ts_reachability_local_call_not_followed_gap")).To(BeTrue(), "sanity: expected the routine reachability gap to be present on the underlying model coverage this spec means to exclude from promotion, got %+v", report.ProjectCoverage.Diagnostics)

			for _, diag := range report.Diagnostics {
				Expect(diag.Kind).NotTo(HavePrefix("ts_reachability_"), "a routine reachability gap must never be promoted as a side-attributed coverage-failure diagnostic, got %+v", report.Diagnostics)
			}
		})
	})
})

func findDiagnosticWithPath(diagnostics []codesignal.Diagnostic, path string) *codesignal.Diagnostic {
	for i := range diagnostics {
		if diagnostics[i].Path == path {
			return &diagnostics[i]
		}
	}
	return nil
}
