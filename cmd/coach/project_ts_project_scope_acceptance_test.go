package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

// T1 (issue #332 Task 9): ProjectScopeFromModel is wired into
// tsProjectBackend.evaluateRevision and its result carried per analyzed
// revision on ProjectBackendResult, derived from the same single analyzer
// response the layer-violation/layer-bypass/reachability evidence families
// (project_ts_backend_acceptance_test.go) already share (AC-RUN-5's one-invocation-per-revision instrumentation
// applies here unchanged).
var _ = Describe("coach codesignal --project-language typescript carries project_scope on ProjectBackendResult, derived from the same analyzer response as the other evidence families (coach#332 Task 9 T1)", func() {
	BeforeEach(func() {
		skipWithoutRealTypeScriptCompiler()
	})

	When("a baseline analysis runs against a multi-root policy with a nested root", Label("ts-project-backend"), func() {
		It("derives HeadProjectScope with independent per-root candidate/analyzed counts, matched_layers, unmatched_layers, inclusion_rule, and pattern_set from one analyzer response (AC-3/AC-15/AC-25/AC-26)", func() {
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
		})
	})

	When("a --base diff analyzes two revisions under the same multi-root policy", Label("ts-project-backend"), func() {
		It("invokes the analyzer exactly twice, once per revision, and carries both HeadProjectScope and BaseProjectScope (AC-3/AC-RUN-5)", func() {
			repo := newTempGitRepo()
			version := realTypescriptVersion()
			commitFile(repo, "package.json", tsRealCompilerPackageJSON(version))
			commitFile(repo, "tsconfig.json", tsProjectTSConfigJSON)
			commitFile(repo, "pkg/handlers/tsconfig.json", tsProjectTSConfigJSON)
			commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
			commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersWithoutImport)
			commitFile(repo, "pkg/handlers/extra.ts", tsHandlersExtraFile)
			baseSHA := commitFile(repo, "project.json", tsNestedRootsScopeConfigJSON)
			headSHA := commitFile(repo, "pkg/handlers/extra.ts", tsHandlersExtraFile+"export const more = 2;\n")
			installRealTypescriptCompiler(repo, true)

			sampler := startAnalyzerEnvironSampler()
			result, err := analyzeTSProjectBackend(repo, headSHA, baseSHA, false, tsNestedRootsScopeConfigJSON)
			environs := sampler.halt()
			Expect(err).NotTo(HaveOccurred())
			Expect(sampler.invocations()).To(Equal(2), "expected exactly one analyzer invocation per revision (head + base), no second analyzer pass just to derive project_scope, observed pids: %+v", environs)

			Expect(result.HeadProjectScope).NotTo(BeNil(), "head-side project_scope must be carried")
			Expect(result.BaseProjectScope).NotTo(BeNil(), "base-side project_scope must be carried under --base")
			Expect(result.HeadProjectScope.Roots).To(HaveLen(2))
			Expect(result.BaseProjectScope.Roots).To(HaveLen(2))
		})
	})

	When("the tsRootScopeGapTSConfigJSON fixture accepts a candidate file into the compiler's Program that is never actually analyzed", Label("ts-project-backend"), func() {
		It("counts the unanalyzable candidate file in candidate_files but not analyzed_files, and names it in its own diagnostic (AC-5/AC-25)", func() {
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
		})
	})

	When("a fixture file matches none of the configured layers' prefixes", Label("ts-project-backend"), func() {
		It("counts the file as an ordinary candidate/analyzed file without it appearing in any layer's matched set (AC-5)", func() {
			repo := newTempGitRepo()
			version := realTypescriptVersion()
			commitFile(repo, "package.json", tsRealCompilerPackageJSON(version))
			commitFile(repo, "tsconfig.json", tsProjectTSConfigJSON)
			commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
			commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersImportingDB)
			commitFile(repo, "pkg/util/misc.ts", tsUtilMiscFile)
			headSHA := commitFile(repo, "project.json", goLayerPolicyConfigJSON)
			installRealTypescriptCompiler(repo, true)

			result, err := analyzeTSProjectBackend(repo, headSHA, "", true, goLayerPolicyConfigJSON)
			Expect(err).NotTo(HaveOccurred())

			Expect(result.HeadProjectScope).NotTo(BeNil())
			Expect(result.HeadProjectScope.Roots).To(HaveLen(1))
			root := result.HeadProjectScope.Roots[0]
			Expect(root.CandidateFiles).To(Equal(3), "expected d.ts, h.ts, and misc.ts (outside every configured layer) as candidates, got %+v", root)
			Expect(root.AnalyzedFiles).To(Equal(3), "got %+v", root)

			Expect(result.HeadProjectScope.MatchedLayers).To(ConsistOf("handlers", "db"), "a file outside every configured layer must not spuriously create or expand a layer match, got %+v", result.HeadProjectScope.MatchedLayers)
			Expect(result.HeadProjectScope.UnmatchedLayers).To(BeEmpty())
		})
	})
})
