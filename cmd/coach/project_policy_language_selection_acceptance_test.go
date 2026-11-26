package main

import (
	"fmt"
	"regexp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

// documentedHookArgs is the command the agent self-review hook runs, unchanged for every language.
var documentedHookArgs = []string{"codesignal", "--worktree", "--no-interactive", "--fail-on-incomplete-coverage", "--format", "json"}

const handlersLayerViolationRule = "architecture.layer_violation"

func layerPolicyNamingLanguage(language string) string {
	languageKey := ""
	if language != "" {
		languageKey = fmt.Sprintf(`"language":%q,`, language)
	}
	return `{"schema_version":"1",` + languageKey + `"roots":["."],"layers":[{"name":"handlers","prefixes":["pkg/handlers"]},{"name":"db","prefixes":["pkg/db"]}],"forbidden_imports":[{"from":"handlers","to":"db"}]}` + "\n"
}

// typeScriptRepoHolding is a TypeScript project with the real compiler installed, whose committed policy forbids
// handlers importing db, and whose committed handler is the file given.
func typeScriptRepoHolding(policy, handler string) string {
	repo := newTempGitRepo()
	commitFile(repo, ".gitignore", "node_modules/\n")
	commitFile(repo, "package.json", tsRealCompilerPackageJSON(realTypescriptVersion()))
	commitFile(repo, "tsconfig.json", tsProjectTSConfigJSON)
	commitFile(repo, "pkg/db/d.ts", tsRealDbFile)
	commitFile(repo, "pkg/handlers/h.ts", handler)
	commitFile(repo, coachConfigPath, policy)
	installRealTypescriptCompiler(repo, true)
	return repo
}

func projectLanguageDisclosure(stdout []byte) map[string]any {
	scope := scopePolicyScopeOf(stdout)
	ExpectWithOffset(1, scope).To(HaveKey("project_language"))
	disclosure, _ := scope["project_language"].(map[string]any)
	return disclosure
}

func layerViolations(report *codesignal.Report) []codesignal.Signal {
	var found []codesignal.Signal
	for _, signal := range report.Signals {
		if signal.RuleID == handlersLayerViolationRule {
			found = append(found, signal)
		}
	}
	return found
}

func runHookCommand(repo string, env []string) (stdout, stderr []byte, exitCode int) {
	return runCoachBinary(commandPath, repo, env, documentedHookArgs...)
}

var _ = Describe("coach codesignal: the project policy selects the project-analysis language (#805)", Label("ts-project-backend"), func() {
	BeforeEach(func() {
		skipWithoutRealTypeScriptCompiler()
	})

	When("the policy says typescript and the documented hook command runs with no --project-language", func() {
		It("reports the forbidden import an uncommitted edit adds, as introduced, and discloses where the language came from", func() {
			repo := typeScriptRepoHolding(layerPolicyNamingLanguage("typescript"), tsRealHandlersWithoutImport)
			writeWorktreeFile(repo, "pkg/handlers/h.ts", tsRealHandlersImportingDB)

			stdout, stderr, exitCode := runHookCommand(repo, nil)

			Expect(exitCode).To(Equal(0), "stdout=%.600s stderr=%s", stdout, stderr)
			violations := layerViolations(decodeCoachReport(stdout))
			Expect(violations).To(HaveLen(1), "stdout=%.1500s", stdout)
			Expect(string(violations[0].Lifecycle)).To(Equal("introduced"))
			Expect(violations[0].Path).To(Equal("pkg/handlers/h.ts"))
			Expect(projectLanguageDisclosure(stdout)).To(Equal(map[string]any{"language": "typescript", "source": "policy"}))
		})

		DescribeTable("the same policy selects TypeScript under each scan mode",
			func(args ...string) {
				repo := typeScriptRepoHolding(layerPolicyNamingLanguage("typescript"), tsRealHandlersWithoutImport)
				commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersImportingDB)

				stdout, stderr, exitCode := runCoachBinary(commandPath, repo, nil, append([]string{"codesignal", "--format", "json"}, args...)...)

				Expect(exitCode).To(Equal(0), "stdout=%.600s stderr=%s", stdout, stderr)
				Expect(layerViolations(decodeCoachReport(stdout))).NotTo(BeEmpty(), "stdout=%.1500s", stdout)
				Expect(projectLanguageDisclosure(stdout)).To(HaveKeyWithValue("source", "policy"))
			},
			Entry("--baseline", "--baseline"),
			Entry("--base", "--base", "HEAD~1"),
		)
	})

	When("--project-language disagrees with the policy", func() {
		It("runs the language the flag names and says so, for a flag that names TypeScript against a go policy", func() {
			repo := typeScriptRepoHolding(layerPolicyNamingLanguage("go"), tsRealHandlersWithoutImport)
			commitFile(repo, "pkg/handlers/h.ts", tsRealHandlersImportingDB)

			stdout, stderr, exitCode := runCoachBinary(commandPath, repo, nil, "codesignal", "--baseline", "--format", "json", "--project-language", "typescript")

			Expect(exitCode).To(Equal(0), "stdout=%.600s stderr=%s", stdout, stderr)
			Expect(layerViolations(decodeCoachReport(stdout))).NotTo(BeEmpty(), "the flag's analysis ran")
			Expect(projectLanguageDisclosure(stdout)).To(Equal(map[string]any{"language": "typescript", "source": "flag"}))
			Expect(decodeCoachReport(stdout).Diagnostics).To(ContainElement(And(
				HaveField("Kind", "project_language_overrides_policy"),
				HaveField("Path", coachConfigPath),
				HaveField("Message", And(ContainSubstring("--project-language typescript"), ContainSubstring("go"))),
			)))
		})
	})

	When("the policy says typescript and no TypeScript toolchain can be resolved", func() {
		var revisionHash = regexp.MustCompile(`[0-9a-f]{40}`)

		refusalOf := func(repo string, args ...string) (int, string) {
			_, stderr, exitCode := runCoachBinary(commandPath, repo, stubToolchainEnv(pathWithoutNode()), append([]string{"codesignal", "--baseline", "--no-interactive"}, args...)...)
			return exitCode, revisionHash.ReplaceAllString(string(stderr), "<revision>")
		}

		It("refuses as --project-language typescript does", func() {
			flagged := typeScriptRepoHolding(layerPolicyNamingLanguage(""), tsRealHandlersImportingDB)
			selected := typeScriptRepoHolding(layerPolicyNamingLanguage("typescript"), tsRealHandlersImportingDB)

			flagExit, flagStderr := refusalOf(flagged, "--project-language", "typescript")
			policyExit, policyStderr := refusalOf(selected)

			Expect(flagExit).To(Equal(2), "the flag's own refusal, to be matched: %s", flagStderr)
			Expect(policyExit).To(Equal(flagExit))
			Expect(policyStderr).To(Equal(flagStderr))
		})
	})
})

var _ = Describe("coach codesignal: which language a scan used, and why (#805)", func() {
	scanWith := func(policy string, args ...string) []byte {
		repo := commitScannableGoRepo()
		if policy != "" {
			commitFile(repo, coachConfigPath, policy)
		}
		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, append([]string{"--format=json"}, args...)...)
		ExpectWithOffset(1, exitCode).To(Equal(0), "stdout=%.600s stderr=%s", stdout, stderr)
		return stdout
	}

	DescribeTable("scope.project_language names the language and its source",
		func(policy string, want map[string]any, args ...string) {
			Expect(projectLanguageDisclosure(scanWith(policy, args...))).To(Equal(want))
		},
		Entry("a policy that names go", singleRootPolicyWithLanguage("go"), map[string]any{"language": "go", "source": "policy"}),
		Entry("a policy that does not say, and no flag", singleRootPolicyJSON, map[string]any{"language": "go", "source": "default"}),
		Entry("a flag that agrees with the policy", singleRootPolicyWithLanguage("go"), map[string]any{"language": "go", "source": "flag"}, "--project-language", "go"),
		Entry("a flag, and a policy that does not say", singleRootPolicyJSON, map[string]any{"language": "go", "source": "flag"}, "--project-language", "go"),
	)

	It("carries scope.project_language as null when no project policy is read", func() {
		Expect(projectLanguageDisclosure(scanWith(""))).To(BeNil())
	})

	It("reports no overriding diagnostic when the flag agrees with the policy or the policy does not say", func() {
		agreeing := decodeCoachReport(scanWith(singleRootPolicyWithLanguage("go"), "--project-language", "go"))
		silent := decodeCoachReport(scanWith(singleRootPolicyJSON, "--project-language", "go"))

		Expect(agreeing.Diagnostics).NotTo(ContainElement(HaveField("Kind", "project_language_overrides_policy")))
		Expect(silent.Diagnostics).NotTo(ContainElement(HaveField("Kind", "project_language_overrides_policy")))
	})

	It("reports a flag that names go against a policy that names typescript, running go", func() {
		stdout := scanWith(singleRootPolicyWithLanguage("typescript"), "--project-language", "go")

		Expect(projectLanguageDisclosure(stdout)).To(Equal(map[string]any{"language": "go", "source": "flag"}))
		Expect(decodeCoachReport(stdout).Diagnostics).To(ContainElement(And(
			HaveField("Kind", "project_language_overrides_policy"),
			HaveField("Path", coachConfigPath),
			HaveField("Message", And(ContainSubstring("--project-language go"), ContainSubstring("typescript"))),
		)))
	})

	DescribeTable("the text report states the language and its source when the policy or the flag chose it",
		func(policy string, want string, args ...string) {
			repo := commitScannableGoRepo()
			commitFile(repo, coachConfigPath, policy)

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, args...)

			Expect(exitCode).To(Equal(0), "stdout=%.600s stderr=%s", stdout, stderr)
			Expect(string(stdout)).To(ContainSubstring(want))
		},
		Entry("from the policy", singleRootPolicyWithLanguage("go"), "language: go (from the policy)"),
		Entry("from the flag", singleRootPolicyJSON, "language: go (from --project-language)", "--project-language", "go"),
	)

	It("leaves the text report as it was when the policy names no language and no flag is given", func() {
		repo := commitScannableGoRepo()
		commitFile(repo, coachConfigPath, singleRootPolicyJSON)

		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo)

		Expect(exitCode).To(Equal(0), "stdout=%.600s stderr=%s", stdout, stderr)
		Expect(string(stdout)).NotTo(ContainSubstring("language:"))
	})

	It("states no language in the text report when no policy was read", func() {
		stdout, _, exitCode := runCoachCodesignalBaselineRaw(commitScannableGoRepo())

		Expect(exitCode).To(Equal(0))
		Expect(string(stdout)).NotTo(ContainSubstring("language:"))
	})
})

func singleRootPolicyWithLanguage(language string) string {
	return fmt.Sprintf(`{"schema_version":"1","language":%q,"roots":["."]}`+"\n", language)
}

var _ = Describe("coach codesignal: a policy named with --project-config can name the language too (#805)", func() {
	DescribeTable("the named policy's language is used, and the flag still wins",
		func(policyLanguage string, want map[string]any, args ...string) {
			repo := commitScannableGoRepo()
			commitFile(repo, "cfg/p.json", singleRootPolicyWithLanguage(policyLanguage))

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, append([]string{"--format=json", "--project-config", "cfg/p.json"}, args...)...)

			Expect(exitCode).To(Equal(0), "stdout=%.600s stderr=%s", stdout, stderr)
			Expect(projectLanguageDisclosure(stdout)).To(Equal(want))
		},
		Entry("a policy that names go", "go", map[string]any{"language": "go", "source": "policy"}),
		Entry("a policy that names typescript, overridden by the flag", "typescript", map[string]any{"language": "go", "source": "flag"}, "--project-language", "go"),
	)
})
