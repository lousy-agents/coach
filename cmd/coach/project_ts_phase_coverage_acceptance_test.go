package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

// T2 (issue #332 Task 9): ProjectBackendResult carries each analyzed
// revision's model/bypass/reachability Coverage independently of
// HeadCoverage/BaseCoverage's existing combined fold (PR #386), which stays
// unchanged. analyzeTSProjectBackend is reused from T1
// (project_ts_project_scope_acceptance_test.go): these fields
// are not yet rendered through codesignal.Input/Report either, so
// ProjectBackendResult remains the most meaningful boundary to observe them.
var _ = Describe("coach codesignal --project-language typescript carries per-phase (model, bypass, reachability) coverage per revision on ProjectBackendResult, additive to the existing fold (coach#332 Task 9 T2)", func() {
	BeforeEach(func() {
		skipWithoutRealTypeScriptCompiler()
	})

	When("the tsRootScopeGapTSConfigJSON fixture accepts a candidate file into the compiler's Program that is never actually analyzed, with no bypass configured", Label("ts-project-backend"), func() {
		It("marks model-phase coverage incomplete while leaving the existing folded HeadCoverage exactly as it already was (SA-280-025, AC-9/AC-13/AC-18/AC-28)", func() {
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

			Expect(result.HeadModelCoverage).NotTo(BeNil())
			Expect(result.HeadModelCoverage.Complete).To(BeFalse(), "an unanalyzable candidate file must never be reported as complete model-phase coverage, got %+v", result.HeadModelCoverage)
			Expect(containsProjectModelDiagnosticCode(result.HeadModelCoverage.Diagnostics, projectmodel.DiagRootScopeIncomplete)).To(BeTrue(), "got %+v", result.HeadModelCoverage.Diagnostics)

			Expect(result.HeadCoverage).NotTo(BeNil())
			Expect(result.HeadCoverage.Complete).To(BeFalse(), "the existing folded HeadCoverage must stay incomplete exactly as PR #386 already produces it")
			Expect(*result.HeadCoverage).To(Equal(*result.HeadModelCoverage), "with no bypass configured the existing fold is exactly the model coverage, unchanged by this task's additive fields")
		})
	})

	When("a required_layer is configured and its bypass search finds a genuine witness, with an unrelated routine reachability gap elsewhere in the snapshot", Label("ts-project-backend"), func() {
		It("carries the bypass search's own reachability-gap-excluded completeness (tsBypassCoverageForFold), not BuildTypeScriptLayerBypassFromModel's raw gap-folded Coverage, as a phase distinct from the existing folded HeadCoverage (AC-4/AC-16)", func() {
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
			headSHA := commitFile(repo, "project.json", tsLayerBypassRequiredConfigJSON)
			installRealTypescriptCompiler(repo, true)

			result, err := analyzeTSProjectBackend(repo, headSHA, "", true, tsLayerBypassRequiredConfigJSON)
			Expect(err).NotTo(HaveOccurred())

			Expect(result.HeadModelCoverage).NotTo(BeNil())
			Expect(result.HeadBypassCoverage).NotTo(BeNil())
			Expect(result.HeadCoverage).NotTo(BeNil())

			Expect(result.HeadBypassCoverage.Phase).To(Equal("ts_layer_bypass"), "the bypass-phase coverage must be the bypass search's own Coverage (tsBypassCoverageForFold), not the folded model Coverage, got %+v", result.HeadBypassCoverage)
			Expect(result.HeadBypassCoverage.Complete).To(BeTrue(), "tsBypassCoverageForFold must exclude the routine reachability-gap term from the bypass search's own completeness; BuildTypeScriptLayerBypassFromModel's raw Coverage folds the gap in via tsReachabilityHasGap and would report false here, got %+v", result.HeadBypassCoverage)
			Expect(result.HeadCoverage.Phase).To(Equal(result.HeadModelCoverage.Phase), "the existing folded HeadCoverage keeps the model's own Phase unchanged, per combineProjectCoverage's documented convention")
			Expect(*result.HeadBypassCoverage).NotTo(Equal(*result.HeadCoverage), "the bypass-phase coverage and the existing folded model+bypass HeadCoverage are different values")
		})
	})

	When("no required_layer is configured, and the analyzed repository has a routine, per-hop reachability gap but no model incompleteness", Label("ts-project-backend"), func() {
		It("reports bypass-phase coverage as exactly not_requested and lets reachability-phase coverage go incomplete independently of model-phase and the existing folded HeadCoverage, both of which stay complete (AC-4/AC-16)", func() {
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
			headSHA := commitFile(repo, "project.json", goLayerPolicyConfigJSON)
			installRealTypescriptCompiler(repo, true)

			result, err := analyzeTSProjectBackend(repo, headSHA, "", true, goLayerPolicyConfigJSON)
			Expect(err).NotTo(HaveOccurred())

			Expect(result.HeadBypassCoverage).NotTo(BeNil())
			Expect(result.HeadBypassCoverage.Phase).To(Equal("not_requested"), "no required_layer is configured, so the bypass phase must never claim it ran, got %+v", result.HeadBypassCoverage)
			Expect(result.HeadBypassCoverage.Complete).To(BeTrue(), "a phase that was never requested is not itself an incompleteness")

			Expect(result.HeadModelCoverage).NotTo(BeNil())
			Expect(result.HeadModelCoverage.Complete).To(BeTrue(), "a routine reachability gap must never mark model-phase coverage incomplete, got %+v", result.HeadModelCoverage)

			Expect(result.HeadCoverage).NotTo(BeNil())
			Expect(result.HeadCoverage.Complete).To(BeTrue(), "a routine reachability gap must never mark the existing folded HeadCoverage incomplete, got %+v", result.HeadCoverage)

			Expect(result.HeadReachabilityCoverage).NotTo(BeNil())
			Expect(result.HeadReachabilityCoverage.Complete).To(BeFalse(), "BuildTypeScriptReachabilityFromModel folds the routine gap into its own Coverage.Complete, independently of model-phase and the existing folded HeadCoverage, got %+v", result.HeadReachabilityCoverage)
		})
	})

	When("a --base diff has the SA-280-025 root-scope mismatch only on the base revision, with no bypass configured", Label("ts-project-backend"), func() {
		It("carries base-revision-specific model/bypass/reachability coverage independently of the head revision's own values (AC-4/AC-16)", func() {
			repo := newTempGitRepo()
			version := realTypescriptVersion()
			commitFile(repo, "package.json", tsRealCompilerPackageJSON(version))
			commitFile(repo, "tsconfig.json", tsRootScopeGapTSConfigJSON)
			commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
			commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersImportingDB)
			baseSHA := commitFile(repo, "project.json", goLayerPolicyConfigJSON)
			headSHA := commitFile(repo, "tsconfig.json", tsProjectTSConfigJSON)
			installRealTypescriptCompiler(repo, true)

			result, err := analyzeTSProjectBackend(repo, headSHA, baseSHA, false, goLayerPolicyConfigJSON)
			Expect(err).NotTo(HaveOccurred())

			Expect(result.BaseModelCoverage).NotTo(BeNil())
			Expect(result.HeadModelCoverage).NotTo(BeNil())
			Expect(result.BaseModelCoverage.Complete).To(BeFalse(), "the base revision's own unanalyzable candidate file must degrade base-phase model coverage independently of the head revision, got %+v", result.BaseModelCoverage)
			Expect(result.HeadModelCoverage.Complete).To(BeTrue(), "sanity: the head revision's own model coverage must be complete, or this spec is not isolating the base-side failure it claims to, got %+v", result.HeadModelCoverage)

			Expect(result.BaseBypassCoverage).NotTo(BeNil())
			Expect(result.HeadBypassCoverage).NotTo(BeNil())
			Expect(result.BaseBypassCoverage.Phase).To(Equal("not_requested"), "no required_layer is configured, so the base-side bypass phase must never claim it ran either, got %+v", result.BaseBypassCoverage)
			Expect(result.HeadBypassCoverage.Phase).To(Equal("not_requested"), "got %+v", result.HeadBypassCoverage)

			Expect(result.BaseReachabilityCoverage).NotTo(BeNil())
			Expect(result.HeadReachabilityCoverage).NotTo(BeNil())
			Expect(result.BaseReachabilityCoverage.Complete).To(BeFalse(), "tsReachabilityCoverage folds the base revision's own model.Coverage.Complete term into reachability-phase coverage, so the base-only root-scope mismatch must degrade it independently of the head revision, got %+v", result.BaseReachabilityCoverage)
			Expect(result.HeadReachabilityCoverage.Complete).To(BeTrue(), "sanity: the head revision's own reachability-phase coverage must be complete, or this spec is not isolating the base-side failure it claims to, got %+v", result.HeadReachabilityCoverage)
		})
	})
})
