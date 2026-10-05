package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func body_projectTsBackendAcceptanceTest_doesNotChangeTheScanResultWhenTMPDIRPackageJsonD_792() {
	repo := newTempGitRepo()
	version := realTypescriptVersion()
	commitRealTSLayerFixture(repo, version)
	installRealTypescriptCompiler(repo, true)

	pkg := filepath.Join(os.TempDir(), "package.json")
	_, existed := os.Stat(pkg)
	if existed == nil {
		prev, err := os.ReadFile(pkg)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { _ = os.WriteFile(pkg, prev, 0o644) })
	} else {
		DeferCleanup(os.Remove, pkg)
	}
	Expect(os.WriteFile(pkg, []byte(`{"type":"commonjs"}`+"\n"), 0o644)).To(Succeed())

	stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
	Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)
	report := decodeCoachReport(stdout)
	Expect(report.ProjectCoverage.Complete).To(BeTrue(), "%+v", report.ProjectCoverage)
	Expect(report.ProjectChanges).To(HaveLen(1))
}

func body_projectTsBackendAcceptanceTest_889() {
	if reason := ensureRealTypeScriptCompilerAvailable(); reason != "" {
		Skip(reason)
	}
}

func body_projectTsBackendAcceptanceTest_degradesHeadCoverageToIncompleteAndEveryProjectC_1020() {
	repo := newTempGitRepo()
	version := realTypescriptVersion()
	commitFile(repo, "package.json", tsRealCompilerPackageJSON(version))
	commitFile(repo, "tsconfig.json", tsProjectTSConfigJSON)
	commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
	commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersImportingDB)
	commitFile(repo, "vendor/prisma-client/package.json", tsPrismaClientPackageJSON)
	commitFile(repo, "vendor/prisma-client/index.ts", tsPrismaClientIndexTS)

	commitFile(repo, "pkg/handlers/bypass.ts", tsHandlersBypassFile)
	commitFile(repo, "project.json", tsLayerBypassAmbiguousConfigJSON)
	installRealTypescriptCompiler(repo, true)

	stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
	Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)

	report := decodeCoachReport(stdout)
	Expect(report.ProjectCoverage).NotTo(BeNil())
	Expect(report.ProjectCoverage.Complete).To(BeFalse(), "an ambiguous, forced-incomplete bypass search must degrade the reported project coverage")

	Expect(report.ProjectChanges).NotTo(BeEmpty())
	for _, change := range report.ProjectChanges {
		Expect(string(change.Lifecycle)).To(Equal("unknown"), "a requested but incomplete bypass search must degrade every project-change lifecycle to unknown, got %+v", change)
	}

	Expect(report.Diagnostics).To(ContainElement(HaveField("Kind", "project_lifecycle_indeterminate")))
	Expect(report.Diagnostics).To(ContainElement(HaveField("Kind", "project_coverage_incomplete")))
	Expect(report.Diagnostics).To(ContainElement(HaveField("Kind", "project_layer_bypass_coverage_incomplete")))

	jsonRuleIDs := projectChangeRuleIDs(report.ProjectChanges)
	Expect(jsonRuleIDs).NotTo(HaveKey("architecture.layer_bypass"), "an unresolved bypass search must stay suppressed in JSON, got %+v", report.ProjectChanges)

	textStdout, textStderr, textExitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--project-language", "typescript", "--format=text")
	Expect(textExitCode).To(Equal(0), "stderr: %s stdout: %s", textStderr, textStdout)
	Expect(string(textStdout)).NotTo(ContainSubstring("architecture.layer_bypass"), "an unresolved bypass search must stay suppressed in text too, got %q", textStdout)
}

func body_projectTsBackendAcceptanceTest_keepsProjectCoverageCompleteAndTheLayerViolation_1095() {
	repo := newTempGitRepo()
	version := realTypescriptVersion()
	commitFile(repo, "package.json", tsRealCompilerPackageJSON(version))
	commitFile(repo, "tsconfig.json", tsProjectTSConfigJSON)
	commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
	commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersImportingDB)
	commitFile(repo, "pkg/service/svc.ts", tsServiceNoopFile)
	commitFile(repo, "vendor/prisma-client/package.json", tsPrismaClientPackageJSON)
	commitFile(repo, "vendor/prisma-client/index.ts", tsPrismaClientIndexTS)
	commitFile(repo, "pkg/handlers/reach.ts", tsHandlersReachabilityFile)
	commitFile(repo, "pkg/handlers/helper.ts", tsHandlersLocalGapHelperFile)
	commitFile(repo, "pkg/handlers/gap.ts", tsHandlersLocalGapFile)
	commitFile(repo, "project.json", tsLayerBypassRequiredConfigJSON)
	installRealTypescriptCompiler(repo, true)

	stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
	Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)

	report := decodeCoachReport(stdout)
	Expect(report.ProjectCoverage).NotTo(BeNil())
	Expect(report.ProjectCoverage.Complete).To(BeTrue(), "a routine reachability gap must never mark project coverage incomplete even when a bypass search ran, got %+v", report.ProjectCoverage)
	Expect(countProjectModelDiagnosticCode(report.ProjectCoverage.Diagnostics, "ts_reachability_local_call_not_followed_gap")).To(Equal(1), "expected the routine reachability gap diagnostic to be folded into ProjectCoverage exactly once, got %+v", report.ProjectCoverage.Diagnostics)

	ruleIDs := projectChangeRuleIDs(report.ProjectChanges)
	Expect(ruleIDs).To(HaveKey("architecture.layer_violation"), "got %+v", report.ProjectChanges)
	for _, change := range report.ProjectChanges {
		if change.RuleID != "architecture.layer_violation" {
			continue
		}
		Expect(string(change.Lifecycle)).To(Equal("baseline"), "an unrelated reachability gap must never degrade an otherwise complete layer-violation finding's lifecycle when a bypass search also ran, got %+v", change)
	}
}

func body_projectTsBackendAcceptanceTest_1239() {
	if reason := ensureRealTypeScriptCompilerAvailable(); reason != "" {
		Skip(reason)
	}
}

func body_projectTsBackendAcceptanceTest_derivesHeadProjectScopeWithIndependentPerRootCan_1246() {
	repo := newTempGitRepo()
	version := realTypescriptVersion()
	commitFile(repo, "package.json", tsRealCompilerPackageJSON(version))
	commitFile(repo, "tsconfig.json", tsProjectTSConfigJSON)
	commitFile(repo, "pkg/handlers/tsconfig.json", tsProjectTSConfigJSON)
	commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
	commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersWithoutImport)
	commitFile(repo, "pkg/handlers/extra.ts", tsHandlersExtraFile)
	headSHA := commitFile(repo, "project.json", tsNestedRootsScopeConfigJSON)
	installRealTypescriptCompiler(repo, true)

	sampler := startAnalyzerEnvironSampler()
	result, err := analyzeTSProjectBackend(repo, headSHA, "", true, tsNestedRootsScopeConfigJSON)
	environs := sampler.halt()
	Expect(err).NotTo(HaveOccurred())
	Expect(sampler.invocations()).To(Equal(1), "expected exactly one analyzer invocation for a baseline analysis (AC-RUN-5), observed pids: %+v", environs)

	Expect(result.HeadProjectScope).NotTo(BeNil())
	scope := *result.HeadProjectScope
	Expect(scope.InclusionRule).To(Equal(projectmodel.InclusionRuleTSConfigIncludesNoTestClassification))
	Expect(scope.PatternSet).To(Equal(projectmodel.TSReachabilityAlgorithm))
	Expect(scope.Roots).To(HaveLen(2), "got %+v", scope.Roots)

	byRoot := map[string]projectmodel.ProjectScopeRoot{}
	for _, r := range scope.Roots {
		byRoot[r.Root] = r
	}
	rootDot, ok := byRoot["."]
	Expect(ok).To(BeTrue(), "expected a root_scope entry for \".\", got %+v", scope.Roots)
	Expect(rootDot.CandidateFiles).To(Equal(3), "expected d.ts, h.ts, extra.ts under \".\", got %+v", rootDot)
	Expect(rootDot.AnalyzedFiles).To(Equal(3), "got %+v", rootDot)

	rootHandlers, ok := byRoot["pkg/handlers"]
	Expect(ok).To(BeTrue(), "expected a root_scope entry for pkg/handlers, got %+v", scope.Roots)
	Expect(rootHandlers.CandidateFiles).To(Equal(2), "expected h.ts, extra.ts under pkg/handlers, counted independently from \".\", got %+v", rootHandlers)
	Expect(rootHandlers.AnalyzedFiles).To(Equal(2), "got %+v", rootHandlers)

	Expect(scope.MatchedLayers).To(ConsistOf("handlers", "db"), "got %+v", scope.MatchedLayers)
	Expect(scope.UnmatchedLayers).To(ConsistOf("unused"), "a layer whose prefix matches no analyzed file must land in unmatched_layers, got %+v", scope.UnmatchedLayers)
}

func body_projectTsBackendAcceptanceTest_countsTheUnanalyzableCandidateFileInCandidateFil_1317() {
	repo := newTempGitRepo()
	version := realTypescriptVersion()
	commitFile(repo, "package.json", tsRealCompilerPackageJSON(version))
	commitFile(repo, "tsconfig.json", tsRootScopeGapTSConfigJSON)
	commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
	commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersImportingDB)
	headSHA := commitFile(repo, "project.json", goLayerPolicyConfigJSON)
	installRealTypescriptCompiler(repo, true)

	result, err := analyzeTSProjectBackend(repo, headSHA, "", true, goLayerPolicyConfigJSON)
	Expect(err).NotTo(HaveOccurred())

	Expect(result.HeadProjectScope).NotTo(BeNil())
	Expect(result.HeadProjectScope.Roots).To(HaveLen(1))
	root := result.HeadProjectScope.Roots[0]
	Expect(root.Root).To(Equal("."))
	Expect(root.CandidateFiles).To(Equal(3), "expected package.json, d.ts, and h.ts as candidates, got %+v", root)
	Expect(root.AnalyzedFiles).To(Equal(2), "expected package.json to be counted as a candidate but never actually analyzed, got %+v", root)

	Expect(result.HeadCoverage).NotTo(BeNil())
	var found bool
	var message string
	for _, diag := range result.HeadCoverage.Diagnostics {
		if diag.Code == projectmodel.DiagRootScopeIncomplete {
			found = true
			message = diag.Message
		}
	}
	Expect(found).To(BeTrue(), "expected a %s diagnostic, got %+v", projectmodel.DiagRootScopeIncomplete, result.HeadCoverage.Diagnostics)
	Expect(message).To(ContainSubstring("package.json"), "the unanalyzable candidate file must be named in its own diagnostic, got %q", message)
}

func body_projectTsBackendAcceptanceTest_1385() {
	if reason := ensureRealTypeScriptCompilerAvailable(); reason != "" {
		Skip(reason)
	}
}
