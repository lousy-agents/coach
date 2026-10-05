package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

// tsHandlersBypassFile calls the pinned Prisma sink directly from
// pkg/handlers, never passing through pkg/service, reproducing a genuine
// architecture.layer_bypass witness under tsLayerBypassRequiredConfigJSON's
// required_layer.
const tsHandlersBypassFile = "import { PrismaClient } from \"@prisma/client\";\n\nconst prisma = new PrismaClient();\n\ninterface App {\n  get(path: string, handler: (req: unknown, res: unknown) => void): void;\n}\ndeclare const app: App;\n\nexport async function getUsersBypass(req: unknown, res: unknown): Promise<void> {\n  const users = await prisma.user.findMany();\n  console.log(users, req, res);\n}\napp.get(\"/users-bypass\", getUsersBypass);\n"

// tsLayerBypassAmbiguousConfigJSON names required_layer "service" with a
// prefix that matches no file anywhere in the snapshot, forcing
// BuildTypeScriptLayerBypassFromModel's ambiguous-layer guard (mirroring
// pkg/projectmodel/ts_layer_bypass_acceptance_test.go's own "the required
// layer's prefixes match no node anywhere in the snapshot" spec) regardless
// of the call graph.
const tsLayerBypassAmbiguousConfigJSON = `{"schema_version":"1","roots":["."],"layers":[{"name":"handlers","prefixes":["pkg/handlers"]},{"name":"db","prefixes":["pkg/db"]},{"name":"service","prefixes":["nonexistent_service_dir"]}],"forbidden_imports":[{"from":"handlers","to":"db"}],"required_layer":"service"}`

// T7 (issue #331 Task 8): one analyzer response per revision must feed
// layer-violation, layer-bypass, and reachability-facts derivation alike,
// and incompleteness in each must fold into (or, for reachability, stay out
// of) the project-change lifecycle exactly as documented on
// tsProjectBackend.evaluateRevision (internal/codesignalcli/ts_project_revision.go).
var _ = Describe("coach codesignal --project-language typescript derives layer violations, layer bypass, and reachability facts from one analyzer response per revision (issue #331 Task 8 T7)", func() {
	BeforeEach(func() {
		skipWithoutRealTypeScriptCompiler()
	})

	When("a baseline (single-revision) analysis runs", Label("ts-project-backend"), func() {
		It("invokes the analyzer exactly once while still deriving layer-violation, layer-bypass, and reachability facts from that one response (AC-1/AC-4/AC-11/AC-12)", func() {
			repo := newTempGitRepo()
			version := realTypescriptVersion()
			commitFile(repo, "package.json", tsRealCompilerPackageJSON(version))
			commitFile(repo, "tsconfig.json", tsProjectTSConfigJSON)
			commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
			commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersImportingDB)
			commitFile(repo, "pkg/service/svc.ts", tsServiceNoopFile)
			commitFile(repo, "vendor/prisma-client/package.json", tsPrismaClientPackageJSON)
			commitFile(repo, "vendor/prisma-client/index.ts", tsPrismaClientIndexTS)
			commitFile(repo, "pkg/handlers/bypass.ts", tsHandlersBypassFile)
			headSHA := commitFile(repo, "project.json", tsLayerBypassRequiredConfigJSON)
			installRealTypescriptCompiler(repo, true)

			sampler := startAnalyzerEnvironSampler()
			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			environs := sampler.halt()
			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)
			Expect(sampler.invocations()).To(Equal(1), "expected exactly one analyzer invocation for a baseline analysis, observed pids: %+v", environs)

			report := decodeCoachReport(stdout)
			ruleIDs := projectChangeRuleIDs(report.ProjectChanges)
			Expect(ruleIDs).To(HaveKey("architecture.layer_violation"), "got %+v", report.ProjectChanges)
			Expect(ruleIDs).To(HaveKey("architecture.layer_bypass"), "expected a layer-bypass ProjectChange derived from the same single analyzer response, got %+v", report.ProjectChanges)
			Expect(report.ProjectFacts).NotTo(BeEmpty(), "expected reachability facts derived from the same single analyzer response")
			Expect(report.ProjectFacts[0].Kind).To(Equal("possible_call_reachability"))
			assertReachabilityNeverSignalOrChange(report)

			textSampler := startAnalyzerEnvironSampler()
			textStdout, textStderr, textExitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--project-language", "typescript", "--format=text")
			textEnvirons := textSampler.halt()
			Expect(textExitCode).To(Equal(0), "stderr: %s stdout: %s", textStderr, textStdout)
			Expect(textSampler.invocations()).To(Equal(1), "expected exactly one analyzer invocation for the text-format rendering of the same baseline analysis, observed pids: %+v", textEnvirons)

			findingsSection, factsSection := splitTextFindingsAndFacts(string(textStdout))
			Expect(findingsSection).To(ContainSubstring("rule_id: architecture.layer_violation"), "got %q", findingsSection)
			Expect(findingsSection).To(ContainSubstring("rule_id: architecture.layer_bypass"), "got %q", findingsSection)
			Expect(findingsSection).NotTo(ContainSubstring("possible_call_reachability"), "reachability must never appear in the Signals/ProjectChanges findings section, got %q", findingsSection)
			Expect(factsSection).To(ContainSubstring("kind: possible_call_reachability"), "got %q", factsSection)
			Expect(factsSection).NotTo(ContainSubstring("rule_id:"), "the Facts section must never carry a rule_id, which would make a fact indistinguishable from a Signal/ProjectChange, got %q", factsSection)

			scopeSampler := startAnalyzerEnvironSampler()
			result, err := analyzeTSProjectBackend(repo, headSHA, "", true, tsLayerBypassRequiredConfigJSON)
			scopeEnvirons := scopeSampler.halt()
			Expect(err).NotTo(HaveOccurred())
			Expect(scopeSampler.invocations()).To(Equal(1), "expected exactly one analyzer invocation for the direct-result inspection of the same baseline analysis, observed pids: %+v", scopeEnvirons)

			resultRuleIDs := projectChangeRuleIDs(result.HeadChanges)
			Expect(resultRuleIDs).To(HaveKey("architecture.layer_violation"), "the same single-invocation result must carry the layer-violation change, got %+v", result.HeadChanges)
			Expect(resultRuleIDs).To(HaveKey("architecture.layer_bypass"), "the same single-invocation result must carry the layer-bypass change, got %+v", result.HeadChanges)
			Expect(result.Facts).NotTo(BeEmpty(), "the same single-invocation result must carry the reachability fact")
			Expect(result.Facts[0].Kind).To(Equal("possible_call_reachability"))
			Expect(result.HeadProjectScope).NotTo(BeNil(), "the same single-invocation result must carry project_scope (AC-11)")
			Expect(result.HeadModelCoverage).NotTo(BeNil(), "the same single-invocation result must carry model-phase coverage (AC-11)")
			Expect(result.HeadBypassCoverage).NotTo(BeNil(), "the same single-invocation result must carry bypass-phase coverage (AC-11)")
			Expect(result.HeadReachabilityCoverage).NotTo(BeNil(), "the same single-invocation result must carry reachability-phase coverage (AC-11)")
		})
	})

	When("a --base diff analyzes two revisions", Label("ts-project-backend"), func() {
		It("invokes the analyzer exactly twice, once per revision, each still deriving all three observation kinds from its own single response", func() {
			repo := newTempGitRepo()
			version := realTypescriptVersion()
			commitFile(repo, "package.json", tsRealCompilerPackageJSON(version))
			commitFile(repo, "tsconfig.json", tsProjectTSConfigJSON)
			commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
			commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersWithoutImport)
			commitFile(repo, "pkg/service/svc.ts", tsServiceNoopFile)
			commitFile(repo, "vendor/prisma-client/package.json", tsPrismaClientPackageJSON)
			commitFile(repo, "vendor/prisma-client/index.ts", tsPrismaClientIndexTS)
			commitFile(repo, "pkg/handlers/bypass.ts", tsHandlersBypassFile)
			baseSHA := commitFile(repo, "project.json", tsLayerBypassRequiredConfigJSON)
			headSHA := commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersImportingDB)
			installRealTypescriptCompiler(repo, true)

			sampler := startAnalyzerEnvironSampler()
			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, baseSHA, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			environs := sampler.halt()
			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)
			Expect(sampler.invocations()).To(Equal(2), "expected exactly one analyzer invocation per revision (head + base), observed pids: %+v", environs)

			report := decodeCoachReport(stdout)
			ruleIDs := projectChangeRuleIDs(report.ProjectChanges)
			Expect(ruleIDs).To(HaveKey("architecture.layer_violation"), "got %+v", report.ProjectChanges)
			Expect(ruleIDs).To(HaveKey("architecture.layer_bypass"), "expected a layer-bypass ProjectChange present on both revisions, got %+v", report.ProjectChanges)
			Expect(report.ProjectFacts).NotTo(BeEmpty())
			assertReachabilityNeverSignalOrChange(report)

			textSampler := startAnalyzerEnvironSampler()
			textStdout, textStderr, textExitCode := runCoachCodesignalRaw(repo, baseSHA, "--project-config", "project.json", "--project-language", "typescript", "--format=text")
			textEnvirons := textSampler.halt()
			Expect(textExitCode).To(Equal(0), "stderr: %s stdout: %s", textStderr, textStdout)
			Expect(textSampler.invocations()).To(Equal(2), "expected exactly one analyzer invocation per revision for the text-format rendering of the same diff, observed pids: %+v", textEnvirons)

			findingsSection, factsSection := splitTextFindingsAndFacts(string(textStdout))
			Expect(findingsSection).To(ContainSubstring("rule_id: architecture.layer_violation"), "got %q", findingsSection)
			Expect(findingsSection).To(ContainSubstring("rule_id: architecture.layer_bypass"), "got %q", findingsSection)
			Expect(findingsSection).NotTo(ContainSubstring("possible_call_reachability"), "reachability must never appear in the Signals/ProjectChanges findings section, got %q", findingsSection)
			Expect(factsSection).To(ContainSubstring("kind: possible_call_reachability"), "got %q", factsSection)
			Expect(factsSection).NotTo(ContainSubstring("rule_id:"), "the Facts section must never carry a rule_id, which would make a fact indistinguishable from a Signal/ProjectChange, got %q", factsSection)

			scopeSampler := startAnalyzerEnvironSampler()
			result, err := analyzeTSProjectBackend(repo, headSHA, baseSHA, false, tsLayerBypassRequiredConfigJSON)
			scopeEnvirons := scopeSampler.halt()
			Expect(err).NotTo(HaveOccurred())
			Expect(scopeSampler.invocations()).To(Equal(2), "expected exactly one analyzer invocation per revision for the direct-result inspection of the same diff, observed pids: %+v", scopeEnvirons)

			resultRuleIDs := projectChangeRuleIDs(result.HeadChanges)
			Expect(resultRuleIDs).To(HaveKey("architecture.layer_violation"), "the same single Analyze() result must carry the head-side layer-violation change, got %+v", result.HeadChanges)
			Expect(resultRuleIDs).To(HaveKey("architecture.layer_bypass"), "the same single Analyze() result must carry the head-side layer-bypass change, got %+v", result.HeadChanges)
			Expect(result.Facts).NotTo(BeEmpty(), "the same single Analyze() result must carry the reachability fact")
			Expect(result.Facts[0].Kind).To(Equal("possible_call_reachability"))
			Expect(result.HeadProjectScope).NotTo(BeNil(), "the same single Analyze() result must carry head-side project_scope (AC-11)")
			Expect(result.BaseProjectScope).NotTo(BeNil(), "the same single Analyze() result must carry base-side project_scope (AC-11)")
			Expect(result.HeadModelCoverage).NotTo(BeNil(), "the same single Analyze() result must carry head-side model-phase coverage (AC-11)")
			Expect(result.BaseModelCoverage).NotTo(BeNil(), "the same single Analyze() result must carry base-side model-phase coverage (AC-11)")
			Expect(result.HeadBypassCoverage).NotTo(BeNil(), "the same single Analyze() result must carry head-side bypass-phase coverage (AC-11)")
			Expect(result.BaseBypassCoverage).NotTo(BeNil(), "the same single Analyze() result must carry base-side bypass-phase coverage (AC-11)")
			Expect(result.HeadReachabilityCoverage).NotTo(BeNil(), "the same single Analyze() result must carry head-side reachability-phase coverage (AC-11)")
			Expect(result.BaseReachabilityCoverage).NotTo(BeNil(), "the same single Analyze() result must carry base-side reachability-phase coverage (AC-11)")
		})
	})

	When("a required_layer is configured but its bypass search cannot resolve any file under that layer (an ambiguous, forced-incomplete search)", Label("ts-project-backend"), func() {
		It("degrades HeadCoverage to incomplete and every project-change lifecycle to unknown (AC-2/AC-14)", func() {
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
		})
	})

	When("the analyzed repository has a routine, per-hop reachability gap but no model or bypass incompleteness", Label("ts-project-backend"), func() {
		It("keeps HeadCoverage complete and every layer-violation lifecycle determinate, surfacing the gap only via reachability facts/diagnostics (AC-3)", func() {
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
			Expect(report.ProjectCoverage.Complete).To(BeTrue(), "a routine reachability gap must never mark project coverage incomplete, got %+v", report.ProjectCoverage)
			Expect(containsProjectModelDiagnosticCode(report.ProjectCoverage.Diagnostics, "ts_reachability_local_call_not_followed_gap")).To(BeTrue(), "expected the routine reachability gap to surface on ProjectCoverage.Diagnostics, got %+v", report.ProjectCoverage.Diagnostics)

			Expect(report.ProjectChanges).To(HaveLen(1))
			Expect(report.ProjectChanges[0].RuleID).To(Equal("architecture.layer_violation"))
			Expect(string(report.ProjectChanges[0].Lifecycle)).To(Equal("baseline"), "an unrelated reachability gap must never degrade an otherwise complete layer-violation finding's lifecycle")

			Expect(report.ProjectFacts).NotTo(BeEmpty(), "expected the resolved reachability edge to still surface as a fact")
			Expect(report.ProjectFacts[0].Kind).To(Equal("possible_call_reachability"))

			Expect(report.Diagnostics).NotTo(ContainElement(HaveField("Kind", "project_lifecycle_indeterminate")), "a routine reachability gap alone must never make the project-change lifecycle indeterminate")
		})
	})

	When("a required_layer is configured and the analyzed repository has a routine, per-hop reachability gap but no bypass-search incompleteness", Label("ts-project-backend"), func() {
		It("keeps ProjectCoverage complete and the layer-violation lifecycle determinate, and folds the gap diagnostic in exactly once (AC-3)", func() {
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
		})
	})

	When("a candidate file is accepted into the compiler's Program but never actually analyzed on the head revision (SA-280-025), with no bypass configured", Label("ts-project-backend"), func() {
		It("degrades HeadCoverage to incomplete and the layer-violation ProjectChange's lifecycle to unknown", func() {
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
			Expect(report.ProjectCoverage).NotTo(BeNil())
			Expect(report.ProjectCoverage.Complete).To(BeFalse(), "expected a real candidate/analyzed root-scope mismatch to mark project coverage incomplete, got %+v", report.ProjectCoverage)
			Expect(containsProjectModelDiagnosticCode(report.ProjectCoverage.Diagnostics, projectmodel.DiagRootScopeIncomplete)).To(BeTrue(), "got %+v", report.ProjectCoverage.Diagnostics)

			Expect(report.ProjectChanges).To(HaveLen(1))
			Expect(string(report.ProjectChanges[0].Lifecycle)).To(Equal("unknown"), "model incompleteness on the analyzed revision must degrade the layer-violation lifecycle to unknown")

			Expect(report.Diagnostics).To(ContainElement(HaveField("Kind", "project_lifecycle_indeterminate")))
		})
	})

	When("a --base diff has the SA-280-025 root-scope mismatch only on the base revision", Label("ts-project-backend"), func() {
		It("degrades the diff's project-change lifecycle to unknown even though the head revision's own coverage is complete", func() {
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
			Expect(report.ProjectCoverage).NotTo(BeNil())
			Expect(report.ProjectCoverage.Complete).To(BeTrue(), "sanity: the head revision's own coverage must be complete, or this spec is not isolating the base-side failure it claims to")

			Expect(report.ProjectChanges).To(HaveLen(1))
			Expect(string(report.ProjectChanges[0].Lifecycle)).To(Equal("unknown"), "base-side model incompleteness must still degrade the diff's project-change lifecycle to unknown")

			Expect(report.Diagnostics).To(ContainElement(HaveField("Kind", "project_lifecycle_indeterminate")))
		})
	})

	When("a --base diff has the SA-280-025 root-scope mismatch only on the head revision, with the base revision fully complete", Label("ts-project-backend"), func() {
		It("degrades the diff's project-change lifecycle to unknown even though the base revision's own coverage is complete", func() {
			repo := newTempGitRepo()
			version := realTypescriptVersion()
			commitFile(repo, "package.json", tsRealCompilerPackageJSON(version))
			commitFile(repo, "tsconfig.json", tsProjectTSConfigJSON)
			commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
			commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersWithoutImport)
			baseSHA := commitFile(repo, "project.json", goLayerPolicyConfigJSON)
			commitFile(repo, "tsconfig.json", tsRootScopeGapTSConfigJSON)
			commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersImportingDB)
			installRealTypescriptCompiler(repo, true)

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, baseSHA, "--project-config", "project.json", "--project-language", "typescript", "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)

			report := decodeCoachReport(stdout)
			Expect(report.ProjectCoverage).NotTo(BeNil())
			Expect(report.ProjectCoverage.Complete).To(BeFalse(), "expected the head-side root-scope mismatch to mark head coverage incomplete, got %+v", report.ProjectCoverage)

			Expect(report.ProjectChanges).To(HaveLen(1))
			Expect(string(report.ProjectChanges[0].Lifecycle)).To(Equal("unknown"), "head-side model incompleteness must degrade the diff's project-change lifecycle to unknown even though base coverage is complete")

			lifecycleMessage := diagnosticMessageForKind(report.Diagnostics, "project_lifecycle_indeterminate")
			Expect(lifecycleMessage).To(ContainSubstring("head coverage incomplete"), "expected the indeterminacy reason to name head coverage, got %q", lifecycleMessage)
			Expect(lifecycleMessage).NotTo(ContainSubstring("base coverage incomplete"), "the base revision is fully complete in this fixture; the indeterminacy reason must not blame it too, got %q", lifecycleMessage)
		})
	})
})
