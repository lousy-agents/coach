package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const analyzerTestHookRootScopeMissing = "root-scope-missing:pkg/handlers"

// tsMultiRootPolicyConfigJSON mirrors goLayerPolicyConfigJSON but declares a
// second root ("pkg/handlers") the real sidecar's own computeRootScopes then
// has to emit a root_scopes entry for, so the root-scope-missing test hook
// dropping that one entry reproduces a genuine wire-round-tripped mismatch
// rather than a hand-built response.
const tsMultiRootPolicyConfigJSON = `{"schema_version":"1","roots":[".","pkg/handlers"],"layers":[{"name":"handlers","prefixes":["pkg/handlers"]},{"name":"db","prefixes":["pkg/db"]}],"forbidden_imports":[{"from":"handlers","to":"db"}]}`

var _ = Describe("coach codesignal --project-language typescript: root_scopes mismatch after a real, otherwise-successful analysis (AC-RUN-9 project_scope)", Label("ts-project-backend"), func() {
	BeforeEach(func() {
		if reason := ensureRealTypeScriptCompilerAvailable(); reason != "" {
			Skip(reason)
		}
	})

	When("the analyzer completes the real compiler-backed scan but its response omits root_scopes for one of the policy's declared roots", func() {
		It("fails the whole CLI run at exit 1 with the deriving-project-scope error, rather than silently narrowing project scope", func() {
			repo := newTempGitRepo()
			commitRealTSLayerFixture(repo, realTypescriptVersion())
			commitFile(repo, "project.json", tsMultiRootPolicyConfigJSON)
			installRealTypescriptCompiler(repo, true)

			coachPath := buildTestHookCoach(analyzerTestHookRootScopeMissing, false)
			stdout, stderr, exitCode := runCoachBaselineWith(coachPath, repo, "--project-config", "project.json", "--project-language", "typescript", "--format=json")

			Expect(exitCode).To(Equal(1), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stdout).To(BeEmpty(), "a failed analysis call must never emit a report")
			Expect(string(stderr)).To(ContainSubstring("coach codesignal: analysis failed:"), "expected runBaselineAnalysis's swallow-path prefix, got: %s", stderr)
			Expect(string(stderr)).To(ContainSubstring("deriving TypeScript project scope"), "expected the error to name the project_scope derivation step, got: %s", stderr)
			Expect(string(stderr)).To(ContainSubstring("pkg/handlers"), "expected the error to name the unmatched policy root, got: %s", stderr)
		})
	})
})
