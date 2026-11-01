package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

const (
	coachConfigPath   = ".lousy-agents/coach.json"
	legacyConfigPath  = "project.json"
	minimalGoSource   = "package a\n\nfunc A() {}\n"
	invalidPolicyText = "not valid json"
)

var _ = Describe("coach project config discovery", func() {
	When("an explicit --project-config is set", func() {
		It("rejects that path and does not fall back to a valid canonical or legacy policy", func() {
			repo := commitScannableGoRepo()
			commitFile(repo, coachConfigPath, singleRootPolicyJSON)
			commitFile(repo, legacyConfigPath, singleRootPolicyJSON)
			commitFile(repo, "other.json", invalidPolicyText)

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "other.json", "--format=json")

			Expect(exitCode).To(Equal(2), "an explicit invalid --project-config must exit 2; stdout=%s stderr=%s", stdout, stderr)
			Expect(stdout).To(BeEmpty(), "a rejected explicit config must write nothing to stdout")
			Expect(string(stderr)).To(ContainSubstring("other.json"))
		})

		It("does not emit project_config_uncommitted when a valid explicit config is used and the canonical file is only in the worktree", func() {
			repo := commitScannableGoRepo()
			commitFile(repo, "other.json", singleRootPolicyJSON)
			writeWorktreeFile(repo, coachConfigPath, singleRootPolicyJSON)

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "other.json", "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			report := decodeCoachReport(stdout)
			Expect(report.Diagnostics).NotTo(ContainElement(HaveField("Kind", "project_config_uncommitted")))
			Expect(report.Diagnostics).NotTo(ContainElement(HaveField("Kind", "project_config_deprecated")))
		})
	})

	When("no --project-config is set and a valid canonical policy is committed", func() {
		It("reads .lousy-agents/coach.json from a subdirectory scan and does not emit project_config_deprecated", func() {
			repo := commitScannableGoRepo()
			commitFile(repo, coachConfigPath, singleRootPolicyJSON)
			commitFile(repo, legacyConfigPath, singleRootPolicyJSON)
			subdir := filepath.Join(repo, "nested")
			Expect(os.MkdirAll(subdir, 0o755)).To(Succeed())

			stdout, stderr, exitCode := runCoachBinary(commandPath, subdir, nil, "codesignal", "--baseline", "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s\nstdout: %s", stderr, stdout)

			report := decodeCoachReport(stdout)
			Expect(report.SchemaVersion).To(Equal("2"))
			Expect(report.Diagnostics).NotTo(ContainElement(HaveField("Kind", "project_config_deprecated")))
		})
	})

	When("no --project-config is set and both a canonical policy and a root project.json are committed and valid", func() {
		const policyWithoutForbiddenPairs = `{"schema_version":"1","roots":["."],"layers":[{"name":"handlers","prefixes":["pkg/handlers"]},{"name":"db","prefixes":["pkg/db"]}]}` + "\n"

		DescribeTable("uses the canonical policy and ignores the root file, even when the two disagree",
			func(canonical, legacy string, wantViolations int) {
				repo := newTempGitRepo()
				commitFile(repo, "go.mod", goModuleFile)
				commitFile(repo, "pkg/db/db.go", dbPackageFile)
				commitFile(repo, "pkg/handlers/handlers.go", handlersImportingDB)
				commitFile(repo, coachConfigPath, canonical)
				commitFile(repo, legacyConfigPath, legacy)

				stdout, stderr, exitCode := runCoachBinary(commandPath, repo, nil, "codesignal", "--baseline", "--format=json")
				Expect(exitCode).To(Equal(0), "stderr: %s\nstdout: %s", stderr, stdout)

				report := decodeCoachReport(stdout)
				Expect(report.SchemaVersion).To(Equal("2"))
				Expect(report.Diagnostics).NotTo(ContainElement(HaveField("Kind", "project_config_deprecated")))
				Expect(report.ProjectChanges).To(HaveLen(wantViolations), "got %+v", report.ProjectChanges)
			},
			Entry("the canonical policy forbids the import and the root file does not", goLayerPolicyConfigJSON, policyWithoutForbiddenPairs, 1),
			Entry("the root file forbids the import and the canonical policy does not", policyWithoutForbiddenPairs, goLayerPolicyConfigJSON, 0),
		)
	})

	When("no --project-config is set and only a valid root project.json is committed", func() {
		It("reports schema 2 with the exact project_config_deprecated diagnostic, including in text output", func() {
			repo := commitScannableGoRepo()
			commitFile(repo, legacyConfigPath, singleRootPolicyJSON)

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			discovered := decodeCoachReport(stdout)
			Expect(discovered.SchemaVersion).To(Equal("2"))
			diag, ok := diagnosticFor(discovered, "project_config_deprecated", legacyConfigPath)
			Expect(ok).To(BeTrue(), "diagnostics: %+v", discovered.Diagnostics)
			Expect(diag.Message).To(Equal("project.json at the repository root is deprecated. Coach will keep reading it until you move it to .lousy-agents/coach.json."))
			skipped, ok := diagnosticFor(discovered, "project_config_migration_skipped", legacyConfigPath)
			Expect(ok).To(BeTrue(), "diagnostics: %+v", discovered.Diagnostics)
			Expect(skipped.Message).To(Equal("project.json can be moved to .lousy-agents/coach.json. Next: mkdir -p .lousy-agents && git mv project.json .lousy-agents/coach.json"))

			textStdout, textStderr, textExit := runCoachCodesignalBaselineRaw(repo)
			Expect(textExit).To(Equal(0), "stderr: %s", textStderr)
			Expect(string(textStdout)).To(ContainSubstring("project_config_deprecated"))
			Expect(string(textStdout)).To(ContainSubstring("project_config_migration_skipped"))
			Expect(string(textStdout)).To(ContainSubstring("mkdir -p .lousy-agents && git mv project.json .lousy-agents/coach.json"))
		})

		It("matches the explicit --project-config project.json schema_version and project_changes, without the deprecation diagnostic", func() {
			repo := commitScannableGoRepo()
			commitFile(repo, legacyConfigPath, singleRootPolicyJSON)

			discoveredStdout, discoveredStderr, discoveredExit := runCoachCodesignalBaselineRaw(repo, "--format=json")
			Expect(discoveredExit).To(Equal(0), "stderr: %s", discoveredStderr)
			explicitStdout, explicitStderr, explicitExit := runCoachCodesignalBaselineRaw(repo, "--project-config", legacyConfigPath, "--format=json")
			Expect(explicitExit).To(Equal(0), "stderr: %s", explicitStderr)

			discovered := decodeCoachReport(discoveredStdout)
			explicit := decodeCoachReport(explicitStdout)
			Expect(explicit.Diagnostics).NotTo(ContainElement(HaveField("Kind", "project_config_deprecated")))
			Expect(discovered.SchemaVersion).To(Equal(explicit.SchemaVersion))
			Expect(discovered.ProjectChanges).To(Equal(explicit.ProjectChanges))
		})
	})

	When("no --project-config is set and the committed canonical policy is invalid", func() {
		It("exits 2 with an empty stdout and does not fall back to a valid project.json", func() {
			repo := commitScannableGoRepo()
			commitFile(repo, coachConfigPath, invalidPolicyText)
			commitFile(repo, legacyConfigPath, singleRootPolicyJSON)

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json")

			Expect(exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderr)
			Expect(stdout).To(BeEmpty())
			Expect(string(stderr)).To(ContainSubstring("project_config_invalid"))
			Expect(string(stderr)).To(ContainSubstring(coachConfigPath))
		})
	})

	When("a typescript baseline scan finds a committed invalid canonical policy and a valid project.json", func() {
		It("prints the same stderr as an explicit --project-config of that canonical file, apart from the path", func() {
			repo := newTempGitRepo()
			commitFile(repo, coachConfigPath, invalidPolicyText)
			commitFile(repo, legacyConfigPath, singleRootPolicyJSON)
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")
			path := pathWithStubNode("v24.9.9")

			implicitStdout, implicitStderr, implicitExit := runCoachCodesignalBaselineEnv(repo, path, "--project-language", "typescript", "--format=json")
			explicitStdout, explicitStderr, explicitExit := runCoachCodesignalBaselineEnv(repo, path, "--project-config", coachConfigPath, "--project-language", "typescript", "--format=json")

			Expect(implicitExit).To(Equal(2), "an invalid canonical policy is a usage error; stdout=%s stderr=%s", implicitStdout, implicitStderr)
			Expect(explicitExit).To(Equal(2), "stdout=%s stderr=%s", explicitStdout, explicitStderr)
			Expect(implicitStdout).To(BeEmpty())
			Expect(explicitStdout).To(BeEmpty())
			Expect(string(explicitStderr)).To(ContainSubstring("typescript_compiler_missing: also failing"), "the explicit path is the reference that already reports the masked compiler gap; stderr=%s", explicitStderr)

			Expect(stderrAsideFromPolicyPath(implicitStderr, coachConfigPath)).To(Equal(stderrAsideFromPolicyPath(explicitStderr, coachConfigPath)),
				"implicit stderr=%s explicit stderr=%s", implicitStderr, explicitStderr)
		})
	})

	When("no --project-config is set and no canonical policy exists", func() {
		It("stays schema 1 and emits no project_config_deprecated diagnostic when project.json is absent", func() {
			repo := commitScannableGoRepo()

			report := expectSchema1WithoutDeprecation(repo)
			Expect(report.SchemaVersion).To(Equal("1"))
		})

		It("stays schema 1 and emits no project_config_deprecated diagnostic when project.json is invalid", func() {
			repo := commitScannableGoRepo()
			commitFile(repo, legacyConfigPath, invalidPolicyText)

			report := expectSchema1WithoutDeprecation(repo)
			Expect(report.SchemaVersion).To(Equal("1"))
			Expect(report.Diagnostics).NotTo(ContainElement(HaveField("Kind", "project_config_deprecated")))
		})
	})

	When("the canonical policy is only in the worktree and a valid project.json is committed", func() {
		It("emits both project_config_uncommitted and project_config_deprecated and reports schema 2", func() {
			repo := commitScannableGoRepo()
			commitFile(repo, legacyConfigPath, singleRootPolicyJSON)
			writeWorktreeFile(repo, coachConfigPath, singleRootPolicyJSON)
			revision := headRevision(repo)

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			report := decodeCoachReport(stdout)
			Expect(report.SchemaVersion).To(Equal("2"))
			deprecated, ok := diagnosticFor(report, "project_config_deprecated", legacyConfigPath)
			Expect(ok).To(BeTrue(), "diagnostics: %+v", report.Diagnostics)
			Expect(deprecated.Message).To(Equal("project.json at the repository root is deprecated. Coach will keep reading it until you move it to .lousy-agents/coach.json."))
			uncommitted, ok := diagnosticFor(report, "project_config_uncommitted", coachConfigPath)
			Expect(ok).To(BeTrue(), "diagnostics: %+v", report.Diagnostics)
			Expect(uncommitted.Message).To(Equal(uncommittedCanonicalMessage(revision)))
		})
	})

	When("the canonical policy is only in the worktree and no valid legacy policy is committed", func() {
		It("stays schema 1 and emits project_config_uncommitted", func() {
			repo := commitScannableGoRepo()
			writeWorktreeFile(repo, coachConfigPath, singleRootPolicyJSON)
			revision := headRevision(repo)

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			report := decodeCoachReport(stdout)
			Expect(report.SchemaVersion).To(Equal("1"))
			uncommitted, ok := diagnosticFor(report, "project_config_uncommitted", coachConfigPath)
			Expect(ok).To(BeTrue(), "diagnostics: %+v", report.Diagnostics)
			Expect(uncommitted.Message).To(ContainSubstring(revision))
		})
	})

	When("--check-project is run with --project-language typescript and no --project-config", func() {
		It("checks a valid canonical policy instead of reporting policy_missing", func() {
			repo := newTempGitRepo()
			commitFile(repo, coachConfigPath, singleRootPolicyJSON)

			doc := expectCheckProject(repo)
			Expect(doc.Checks.Policy.Code).NotTo(Equal("policy_missing"))
			Expect(gapCodes(doc)).NotTo(ContainElement("policy_missing"))
			Expect(doc.Checks.Policy.State).To(Equal("pass"))
		})

		It("reports policy_invalid for an invalid canonical policy and does not fall back to a valid project.json", func() {
			repo := newTempGitRepo()
			commitFile(repo, coachConfigPath, invalidPolicyText)
			commitFile(repo, legacyConfigPath, singleRootPolicyJSON)

			doc := expectCheckProject(repo)
			Expect(doc.Checks.Policy.Code).To(Equal("policy_invalid"))
			Expect(gapCodes(doc)).To(ContainElement("policy_invalid"))
		})

		It("reports policy_invalid for a committed canonical file over 1 MiB and does not fall back to a valid project.json", func() {
			repo := newTempGitRepo()
			commitFile(repo, coachConfigPath, strings.Repeat("x", (1<<20)+1))
			commitFile(repo, legacyConfigPath, singleRootPolicyJSON)

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, pathWithoutNode(), "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")

			Expect(exitCode).To(Equal(0), "an oversized canonical policy is a readiness gap, not a usage error; stdout=%s stderr=%s", stdout, stderr)
			Expect(stdout).NotTo(BeEmpty(), "readiness JSON must be written; stderr=%s", stderr)
			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(doc.Checks.Policy.State).NotTo(Equal("pass"), "the oversized canonical file must not pass the policy check")
			Expect(doc.Checks.Policy.Code).To(Equal("policy_invalid"))
			Expect(gapCodes(doc)).To(ContainElement("policy_invalid"))
		})

		It("checks a valid project.json and warns with code project_config_deprecated", func() {
			repo := newTempGitRepo()
			commitFile(repo, legacyConfigPath, singleRootPolicyJSON)

			doc := expectCheckProject(repo)
			Expect(doc.Checks.Policy.State).To(Equal("pass"))
			Expect(warningCodes(doc)).To(ContainElement("project_config_deprecated"))
		})

		It("reports policy_missing for an invalid project.json and does not warn project_config_deprecated", func() {
			repo := newTempGitRepo()
			commitFile(repo, legacyConfigPath, invalidPolicyText)

			doc := expectCheckProject(repo)
			Expect(doc.Checks.Policy.Code).To(Equal("policy_missing"))
			Expect(gapCodes(doc)).To(ContainElement("policy_missing"))
			Expect(gapCodes(doc)).NotTo(ContainElement("policy_invalid"))
			Expect(warningCodes(doc)).NotTo(ContainElement("project_config_deprecated"))
		})
	})

	When("--check-project is given an explicit invalid --project-config and a valid canonical policy is committed", func() {
		It("reports policy_invalid for the explicit path", func() {
			repo := newTempGitRepo()
			commitFile(repo, coachConfigPath, singleRootPolicyJSON)
			commitFile(repo, legacyConfigPath, invalidPolicyText)

			doc := expectCheckProject(repo, "--project-config", legacyConfigPath)
			Expect(doc.Checks.Policy.Code).To(Equal("policy_invalid"))
			Expect(gapCodes(doc)).To(ContainElement("policy_invalid"))
			Expect(warningCodes(doc)).NotTo(ContainElement("project_config_deprecated"))
		})
	})
})

func commitScannableGoRepo() string {
	repo := newTempGitRepo()
	commitFile(repo, "a.go", minimalGoSource)
	return repo
}

func uncommittedCanonicalMessage(revision string) string {
	return fmt.Sprintf(".lousy-agents/coach.json is in the worktree but is not committed at %s, so this scan did not read it.", revision)
}

func expectSchema1WithoutDeprecation(repo string) *codesignal.Report {
	stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json")
	Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
	report := decodeCoachReport(stdout)
	Expect(report.SchemaVersion).To(Equal("1"))
	Expect(report.Diagnostics).NotTo(ContainElement(HaveField("Kind", "project_config_deprecated")))
	return report
}

func stderrAsideFromPolicyPath(stderr []byte, policyPath string) string {
	return strings.ReplaceAll(string(stderr), policyPath, "<policy-path>")
}

func expectCheckProject(repo string, extraArgs ...string) readinessResultDoc {
	args := append([]string{"--baseline", "--check-project", "--project-language", "typescript", "--format", "json"}, extraArgs...)
	stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, pathWithoutNode(), args...)
	Expect(exitCode).To(Equal(0), "stderr: %s\nstdout: %s", stderr, stdout)
	var doc readinessResultDoc
	Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
	return doc
}
