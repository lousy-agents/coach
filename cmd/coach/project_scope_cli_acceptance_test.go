package main

import (
	"encoding/json"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

// hostNodeVersion returns the trimmed output of `node --version`.
func hostNodeVersion() string {
	out, err := exec.Command("node", "--version").Output()
	Expect(err).NotTo(HaveOccurred())
	return strings.TrimSpace(string(out))
}

// findScopeRoot returns a pointer to the ProjectScopeRoot whose Root field
// matches rootPath, or nil when no entry matches.
func findScopeRoot(roots []projectmodel.ProjectScopeRoot, rootPath string) *projectmodel.ProjectScopeRoot {
	for i := range roots {
		if roots[i].Root == rootPath {
			return &roots[i]
		}
	}
	return nil
}

var _ = Describe("coach codesignal project_scope, provenance, and next_actions CLI golden contracts (issue #333 Task 10 T6)", func() {
	BeforeEach(func() {
		if reason := ensureRealTypeScriptCompilerAvailable(); reason != "" {
			Skip(reason)
		}
	})

	// AC-1: Nested roots — project_scope.head.roots carries independent per-root
	// candidate/analyzed counts when the policy declares a nested root pair.
	When("a baseline analysis runs against the tsNestedRootsScopeConfigJSON multi-root policy", Label("ts-project-backend"), func() {
		var jsonOut []byte
		var textOut []byte

		BeforeEach(func() {
			repo := newTempGitRepo()
			version := realTypescriptVersion()
			commitFile(repo, ".gitignore", "node_modules/\n")
			commitFile(repo, "package.json", tsRealCompilerPackageJSON(version))
			commitFile(repo, "tsconfig.json", tsProjectTSConfigJSON)
			commitFile(repo, "pkg/handlers/tsconfig.json", tsProjectTSConfigJSON)
			commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
			commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersWithoutImport)
			commitFile(repo, "pkg/handlers/extra.ts", tsHandlersExtraFile)
			commitFile(repo, "project.json", tsNestedRootsScopeConfigJSON)
			installRealTypescriptCompiler(repo, true)

			var exitCode int
			jsonOut, _, exitCode = runCoachCodesignalBaselineRaw(repo,
				"--project-config", "project.json", "--project-language", "typescript", "--format=json")
			Expect(exitCode).To(Equal(0))
			textOut, _, exitCode = runCoachCodesignalBaselineRaw(repo,
				"--project-config", "project.json", "--project-language", "typescript")
			Expect(exitCode).To(Equal(0))
		})

		It("JSON project_scope.head.roots has '.' candidate/analyzed=3 and 'pkg/handlers' candidate/analyzed=2, independently (AC-3/AC-15/AC-25/AC-26)", func() {
			report := decodeCoachReport(jsonOut)
			Expect(report.ProjectScope).NotTo(BeNil())
			Expect(report.ProjectScope.Head.Roots).To(HaveLen(2))

			dotRoot := findScopeRoot(report.ProjectScope.Head.Roots, ".")
			Expect(dotRoot).NotTo(BeNil(), "expected a root_scope entry for '.'")
			Expect(dotRoot.CandidateFiles).To(Equal(3), "d.ts, h.ts, extra.ts under '.'; got %+v", dotRoot)
			Expect(dotRoot.AnalyzedFiles).To(Equal(3))

			handlersRoot := findScopeRoot(report.ProjectScope.Head.Roots, "pkg/handlers")
			Expect(handlersRoot).NotTo(BeNil(), "expected a root_scope entry for 'pkg/handlers'")
			Expect(handlersRoot.CandidateFiles).To(Equal(2), "h.ts and extra.ts under 'pkg/handlers', counted independently; got %+v", handlersRoot)
			Expect(handlersRoot.AnalyzedFiles).To(Equal(2))
		})

		It("JSON project_scope.head.matched_layers contains handlers and db; unmatched_layers contains unused", func() {
			report := decodeCoachReport(jsonOut)
			Expect(report.ProjectScope).NotTo(BeNil())
			Expect(report.ProjectScope.Head.MatchedLayers).To(ConsistOf("handlers", "db"))
			Expect(report.ProjectScope.Head.UnmatchedLayers).To(ConsistOf("unused"))
		})

		It("text shows both roots with their candidate and analyzed counts", func() {
			text := string(textOut)
			Expect(text).To(ContainSubstring("root: .  candidate_files: 3  analyzed_files: 3"))
			Expect(text).To(ContainSubstring("root: pkg/handlers  candidate_files: 2  analyzed_files: 2"))
		})
	})

	// AC-2: Unanalyzable file / SA-280-025 — when tsRootScopeGapTSConfigJSON
	// causes package.json to be counted as a candidate but never analyzed, model
	// coverage is incomplete, next_actions is suppressed, and the verdict is not
	// the "complete_no_match" sentence. Handlers do not import db, so an active
	// layer finding cannot explain the omitted next_actions.
	When("the tsRootScopeGapTSConfigJSON fixture causes an unanalyzable candidate file (SA-280-025)", Label("ts-project-backend"), func() {
		var repo string
		var jsonOut []byte
		var textOut []byte

		BeforeEach(func() {
			repo = newTempGitRepo()
			version := realTypescriptVersion()
			commitFile(repo, ".gitignore", "node_modules/\n")
			commitFile(repo, "package.json", tsRealCompilerPackageJSON(version))
			commitFile(repo, "tsconfig.json", tsRootScopeGapTSConfigJSON)
			commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
			commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersWithoutImport)
			commitFile(repo, "project.json", goLayerPolicyConfigJSON)
			installRealTypescriptCompiler(repo, true)

			var exitCode int
			jsonOut, _, exitCode = runCoachCodesignalBaselineRaw(repo,
				"--project-config", "project.json", "--project-language", "typescript", "--format=json")
			Expect(exitCode).To(Equal(0))
			textOut, _, exitCode = runCoachCodesignalBaselineRaw(repo,
				"--project-config", "project.json", "--project-language", "typescript")
			Expect(exitCode).To(Equal(0))
		})

		It("JSON project_provenance.head.coverage.model is 'incomplete'", func() {
			report := decodeCoachReport(jsonOut)
			Expect(report.ProjectProvenance).NotTo(BeNil())
			Expect(report.ProjectProvenance.Head.Coverage.Model).To(Equal("incomplete"))
		})

		It("JSON project_next_actions is absent or empty because model coverage is incomplete", func() {
			report := decodeCoachReport(jsonOut)
			Expect(report.ProjectChanges).To(BeEmpty(), "SA-280-025 must be proved without an active layer finding that would also suppress next_actions")
			var doc map[string]json.RawMessage
			Expect(json.Unmarshal(jsonOut, &doc)).To(Succeed())
			if raw, ok := doc["project_next_actions"]; ok {
				var actions []json.RawMessage
				Expect(json.Unmarshal(raw, &actions)).To(Succeed())
				Expect(actions).To(BeEmpty(), "project_next_actions must be absent or empty when model coverage is incomplete")
			}
		})

		It("text verdict does not contain 'Complete scan found no configured covered match'", func() {
			Expect(string(textOut)).NotTo(ContainSubstring("Complete scan found no configured covered match"))
		})

		It("exits 3 with --fail-on-incomplete-coverage and report is still on stdout", func() {
			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo,
				"--project-config", "project.json",
				"--project-language", "typescript",
				"--format=json",
				"--fail-on-incomplete-coverage",
			)
			Expect(exitCode).To(Equal(3), "stderr: %s", stderr)
			Expect(stdout).NotTo(BeEmpty(), "report must still be written to stdout when --fail-on-incomplete-coverage triggers exit 3")
			report := decodeCoachReport(stdout)
			Expect(report.ProjectProvenance).NotTo(BeNil())
			Expect(report.ProjectProvenance.Head.Coverage.Model).To(Equal("incomplete"))
		})

		// SA-280-039: incomplete coverage renders Diagnostic.Path/Message.
		It("JSON stdout contains no absolute repo path, no 'node:internal', and no 'file://'", func() {
			report := decodeCoachReport(jsonOut)
			Expect(report.Diagnostics).To(ContainElement(HaveField("Kind", "project_coverage_incomplete")))
			Expect(report.Diagnostics).To(ContainElement(HaveField("Kind", "project_lifecycle_indeterminate")))
			jsonStr := string(jsonOut)
			Expect(jsonStr).NotTo(ContainSubstring(repo), "JSON must not leak the repo absolute path")
			Expect(jsonStr).NotTo(ContainSubstring("node:internal"), "JSON must not leak node:internal module URLs")
			Expect(jsonStr).NotTo(ContainSubstring("file://"), "JSON must not leak file:// scheme URLs")
		})

		It("text stdout contains no absolute repo path, no 'node:internal', and no 'file://'", func() {
			textStr := string(textOut)
			Expect(textStr).NotTo(ContainSubstring(repo), "text must not leak the repo absolute path")
			Expect(textStr).NotTo(ContainSubstring("node:internal"), "text must not leak node:internal module URLs")
			Expect(textStr).NotTo(ContainSubstring("file://"), "text must not leak file:// scheme URLs")
		})
	})

	// AC-3: File outside every layer — pkg/util/misc.ts falls under no configured
	// layer prefix; it must be counted in candidate/analyzed but must not spuriously
	// expand any layer's matched set or create an entry in unmatched_layers.
	When("a candidate file falls outside every configured layer prefix", Label("ts-project-backend"), func() {
		var jsonOut []byte

		BeforeEach(func() {
			repo := newTempGitRepo()
			version := realTypescriptVersion()
			commitFile(repo, ".gitignore", "node_modules/\n")
			commitFile(repo, "package.json", tsRealCompilerPackageJSON(version))
			commitFile(repo, "tsconfig.json", tsProjectTSConfigJSON)
			commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
			commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersImportingDB)
			commitFile(repo, "pkg/util/misc.ts", tsUtilMiscFile)
			commitFile(repo, "project.json", goLayerPolicyConfigJSON)
			installRealTypescriptCompiler(repo, true)

			var exitCode int
			jsonOut, _, exitCode = runCoachCodesignalBaselineRaw(repo,
				"--project-config", "project.json", "--project-language", "typescript", "--format=json")
			Expect(exitCode).To(Equal(0))
		})

		It("JSON project_scope.head root '.' has candidate_files=3 and analyzed_files=3, matched_layers=[handlers,db], unmatched_layers=[] (AC-5)", func() {
			report := decodeCoachReport(jsonOut)
			Expect(report.ProjectScope).NotTo(BeNil())
			Expect(report.ProjectScope.Head.Roots).To(HaveLen(1))
			root := report.ProjectScope.Head.Roots[0]
			Expect(root.Root).To(Equal("."))
			Expect(root.CandidateFiles).To(Equal(3), "d.ts, h.ts, and misc.ts must all be counted; got %+v", root)
			Expect(root.AnalyzedFiles).To(Equal(3), "all three must be analyzed; got %+v", root)
			Expect(report.ProjectScope.Head.MatchedLayers).To(ConsistOf("handlers", "db"))
			Expect(report.ProjectScope.Head.UnmatchedLayers).To(BeEmpty(),
				"a file outside every layer must not create or expand unmatched_layers; got %v", report.ProjectScope.Head.UnmatchedLayers)
		})
	})

	// AC-4: Differing base/HEAD counts under --base — project_scope.base is present
	// with head "." 3/3 and "pkg/handlers" 2/2, base "." 2/2 and "pkg/handlers" 1/1.
	// record_baseline is absent in diff mode.
	When("a --base diff analyzes a nested-roots fixture where HEAD adds a file absent from base", Label("ts-project-backend"), func() {
		var jsonOut []byte
		var textOut []byte

		BeforeEach(func() {
			repo := newTempGitRepo()
			version := realTypescriptVersion()
			commitFile(repo, ".gitignore", "node_modules/\n")
			commitFile(repo, "package.json", tsRealCompilerPackageJSON(version))
			commitFile(repo, "tsconfig.json", tsProjectTSConfigJSON)
			commitFile(repo, "pkg/handlers/tsconfig.json", tsProjectTSConfigJSON)
			commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
			commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersWithoutImport)
			// baseSHA has d.ts and h.ts (no extra.ts): "." count=2, "pkg/handlers" count=1.
			baseSHA := commitFile(repo, "project.json", tsNestedRootsScopeConfigJSON)
			// HEAD adds extra.ts: "." count=3, "pkg/handlers" count=2.
			commitFile(repo, "pkg/handlers/extra.ts", tsHandlersExtraFile)
			installRealTypescriptCompiler(repo, true)

			var exitCode int
			jsonOut, _, exitCode = runCoachCodesignalRaw(repo, baseSHA,
				"--project-config", "project.json", "--project-language", "typescript", "--format=json")
			Expect(exitCode).To(Equal(0))
			textOut, _, exitCode = runCoachCodesignalRaw(repo, baseSHA,
				"--project-config", "project.json", "--project-language", "typescript")
			Expect(exitCode).To(Equal(0))
		})

		It("JSON project_scope.base is present and head vs base roots have exact expected counts", func() {
			report := decodeCoachReport(jsonOut)
			Expect(report.ProjectScope).NotTo(BeNil())
			Expect(report.ProjectScope.Base).NotTo(BeNil(), "project_scope.base must be present in diff mode")

			headDot := findScopeRoot(report.ProjectScope.Head.Roots, ".")
			baseDot := findScopeRoot(report.ProjectScope.Base.Roots, ".")
			Expect(headDot).NotTo(BeNil(), "expected '.' entry in head roots")
			Expect(baseDot).NotTo(BeNil(), "expected '.' entry in base roots")
			Expect(*headDot).To(Equal(projectmodel.ProjectScopeRoot{Root: ".", CandidateFiles: 3, AnalyzedFiles: 3}),
				"HEAD adds extra.ts so '.' has 3/3")
			Expect(*baseDot).To(Equal(projectmodel.ProjectScopeRoot{Root: ".", CandidateFiles: 2, AnalyzedFiles: 2}),
				"base commit has only d.ts and h.ts so '.' has 2/2")

			headHandlers := findScopeRoot(report.ProjectScope.Head.Roots, "pkg/handlers")
			baseHandlers := findScopeRoot(report.ProjectScope.Base.Roots, "pkg/handlers")
			Expect(headHandlers).NotTo(BeNil(), "expected 'pkg/handlers' entry in head roots")
			Expect(baseHandlers).NotTo(BeNil(), "expected 'pkg/handlers' entry in base roots")
			Expect(*headHandlers).To(Equal(projectmodel.ProjectScopeRoot{Root: "pkg/handlers", CandidateFiles: 2, AnalyzedFiles: 2}),
				"HEAD has h.ts and extra.ts under pkg/handlers")
			Expect(*baseHandlers).To(Equal(projectmodel.ProjectScopeRoot{Root: "pkg/handlers", CandidateFiles: 1, AnalyzedFiles: 1}),
				"base commit has only h.ts under pkg/handlers")
		})

		It("JSON project_next_actions is present, contains review_policy_coverage, and does not contain record_baseline (diff mode)", func() {
			var doc map[string]json.RawMessage
			Expect(json.Unmarshal(jsonOut, &doc)).To(Succeed())
			Expect(doc).To(HaveKey("project_next_actions"), "project_next_actions must be present in diff mode")
			var actions []struct {
				Kind string `json:"kind"`
			}
			Expect(json.Unmarshal(doc["project_next_actions"], &actions)).To(Succeed())
			kinds := make([]string, len(actions))
			for i, a := range actions {
				kinds[i] = a.Kind
			}
			Expect(kinds).To(Equal([]string{"review_policy_coverage"}),
				"diff mode must emit exactly [review_policy_coverage], not record_baseline")
		})

		It("text does not contain 'record_baseline'", func() {
			Expect(string(textOut)).NotTo(ContainSubstring("record_baseline"))
		})
	})

	// AC-5 / AC-6 / AC-7: Baseline no-finding, provenance, and path-leak guard —
	// when a clean fixture produces complete coverage and no active findings, the
	// report uses the "complete_no_match" verdict, project_next_actions is ordered
	// record_baseline then review_policy_coverage, runtime provenance matches the
	// host node, and no absolute paths appear in stdout.
	When("a baseline analysis of a clean TypeScript project (no forbidden edge) runs to completion", Label("ts-project-backend"), func() {
		var repo string
		var jsonOut []byte
		var textOut []byte

		BeforeEach(func() {
			repo = newTempGitRepo()
			version := realTypescriptVersion()
			commitFile(repo, ".gitignore", "node_modules/\n")
			commitFile(repo, "package.json", tsRealCompilerPackageJSON(version))
			commitFile(repo, "tsconfig.json", tsProjectTSConfigJSON)
			commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
			commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersWithoutImport)
			commitFile(repo, "project.json", goLayerPolicyConfigJSON)
			installRealTypescriptCompiler(repo, true)

			var exitCode int
			jsonOut, _, exitCode = runCoachCodesignalBaselineRaw(repo,
				"--project-config", "project.json", "--project-language", "typescript", "--format=json")
			Expect(exitCode).To(Equal(0))
			textOut, _, exitCode = runCoachCodesignalBaselineRaw(repo,
				"--project-config", "project.json", "--project-language", "typescript")
			Expect(exitCode).To(Equal(0))
		})

		// AC-5: complete_no_match verdict and next_actions order.

		It("text verdict is 'Complete scan found no configured covered match.' (not the no-findings or incomplete sentences)", func() {
			text := string(textOut)
			Expect(text).To(ContainSubstring("Complete scan found no configured covered match."))
			Expect(text).NotTo(ContainSubstring("No active CodeSignal findings."))
		})

		It("text 'Project scope:' section appears before the complete_no_match verdict", func() {
			text := string(textOut)
			scopeIdx := strings.Index(text, "Project scope:")
			verdictIdx := strings.Index(text, "Complete scan found no configured covered match.")
			Expect(scopeIdx).To(BeNumerically(">=", 0), "expected 'Project scope:' in text")
			Expect(verdictIdx).To(BeNumerically(">=", 0), "expected complete_no_match verdict in text")
			Expect(scopeIdx).To(BeNumerically("<", verdictIdx),
				"'Project scope:' must render before the verdict; scope at %d, verdict at %d", scopeIdx, verdictIdx)
		})

		It("JSON project_next_actions has exactly [record_baseline, review_policy_coverage] in that order", func() {
			var doc map[string]json.RawMessage
			Expect(json.Unmarshal(jsonOut, &doc)).To(Succeed())
			Expect(doc).To(HaveKey("project_next_actions"))
			var actions []struct {
				Kind string `json:"kind"`
			}
			Expect(json.Unmarshal(doc["project_next_actions"], &actions)).To(Succeed())
			Expect(actions).To(HaveLen(2))
			Expect(actions[0].Kind).To(Equal("record_baseline"))
			Expect(actions[1].Kind).To(Equal("review_policy_coverage"))
		})

		It("text Next actions section contains both record_baseline and review_policy_coverage, record_baseline first", func() {
			text := string(textOut)
			rbIdx := strings.Index(text, "record_baseline")
			rpcIdx := strings.Index(text, "review_policy_coverage")
			Expect(rbIdx).To(BeNumerically(">=", 0), "expected 'record_baseline' in text")
			Expect(rpcIdx).To(BeNumerically(">=", 0), "expected 'review_policy_coverage' in text")
			Expect(rbIdx).To(BeNumerically("<", rpcIdx), "'record_baseline' must appear before 'review_policy_coverage'")
		})

		It("JSON summary.unknown_signals key is present", func() {
			var doc map[string]json.RawMessage
			Expect(json.Unmarshal(jsonOut, &doc)).To(Succeed())
			var summary map[string]json.RawMessage
			Expect(json.Unmarshal(doc["summary"], &summary)).To(Succeed())
			Expect(summary).To(HaveKey("unknown_signals"))
		})

		// AC-6: Provenance runtime matches the host node.

		It("JSON project_provenance.runtime.kind is 'node' and runtime.version matches host 'node --version' output", func() {
			hostVersion := hostNodeVersion()
			report := decodeCoachReport(jsonOut)
			Expect(report.ProjectProvenance).NotTo(BeNil())
			Expect(report.ProjectProvenance.Runtime.Kind).To(Equal("node"))
			Expect(report.ProjectProvenance.Runtime.Version).To(Equal(hostVersion))
		})

		It("text report contains the host node version string and 'runtime: node' label", func() {
			hostVersion := hostNodeVersion()
			text := string(textOut)
			Expect(text).To(ContainSubstring(hostVersion))
			Expect(text).To(ContainSubstring("runtime: node"))
		})

		// AC-7: SA-280-039 path-leak guard.

		It("JSON stdout contains no absolute repo path, no 'node:internal', and no 'file://'", func() {
			jsonStr := string(jsonOut)
			Expect(jsonStr).NotTo(ContainSubstring(repo), "JSON must not leak the repo absolute path")
			Expect(jsonStr).NotTo(ContainSubstring("node:internal"), "JSON must not leak node:internal module URLs")
			Expect(jsonStr).NotTo(ContainSubstring("file://"), "JSON must not leak file:// scheme URLs")
		})

		It("text stdout contains no absolute repo path, no 'node:internal', and no 'file://'", func() {
			textStr := string(textOut)
			Expect(textStr).NotTo(ContainSubstring(repo), "text must not leak the repo absolute path")
			Expect(textStr).NotTo(ContainSubstring("node:internal"), "text must not leak node:internal module URLs")
			Expect(textStr).NotTo(ContainSubstring("file://"), "text must not leak file:// scheme URLs")
		})
	})
})
