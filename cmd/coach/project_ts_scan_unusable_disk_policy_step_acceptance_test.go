package main

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
)

// PRD 8.4: a policy on disk that cannot be used is refused for the disk in words, never with a command, and a
// TypeScript scan says what to do about it: fix the file. The words are the same whatever the revision commits there.
var _ = Describe("coach codesignal (TypeScript): a discovered policy on disk that cannot be used says to fix the file (#168 A2b)", func() {
	const fixTheFile = "the policy on disk cannot be used: fix the file."

	type unusableOnDisk struct {
		description, committed, onDisk string
	}
	states := []unusableOnDisk{
		{"invalid JSON on disk beside a valid committed copy", singleRootPolicyJSON, "{bad"},
		{"a schema-invalid document on disk beside a valid committed copy", singleRootPolicyJSON, invalidPolicy},
		{"a document over the size limit on disk beside a valid committed copy", singleRootPolicyJSON, strings.Repeat(" ", int(projectconfig.MaxBytes)+1)},
		{"an invalid document on disk beside a different invalid committed copy", `{"schema_version":"9"}`, "{bad"},
	}

	for _, state := range states {
		It("#168 A2b: refuses "+state.description+", names the path, says to fix the file, and prints no command", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")
			commitFile(repo, coachConfigPath, state.committed)
			scopePolicyRewrite(repo, coachConfigPath, state.onDisk)

			stdout, stderr, exitCode := runCoachCodesignalBaselineEnv(repo, pathWithStubNode("v24.9.9"), "--project-language", "typescript", "--no-interactive")

			Expect(exitCode).To(Equal(2), "stdout: %s\nstderr: %s", stdout, stderr)
			Expect(string(stdout)).To(BeEmpty())
			Expect(string(stderr)).To(ContainSubstring("project_config_invalid"))
			Expect(string(stderr)).To(ContainSubstring(coachConfigPath))
			Expect(string(stderr)).To(ContainSubstring("coach codesignal: " + fixTheFile))
			expectNoRunnableCommand(string(stderr))
			Expect(string(stderr)).NotTo(ContainSubstring("Discovered TypeScript roots"), "no guided authoring opens")
		})

		It("#168 A2b: does not give a Go scan the TypeScript words for "+state.description, func() {
			repo := commitGoRepoWithPolicy(coachConfigPath, state.committed)
			scopePolicyRewrite(repo, coachConfigPath, state.onDisk)

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json")

			Expect(exitCode).To(Equal(2), "stdout: %s\nstderr: %s", stdout, stderr)
			Expect(string(stderr)).To(ContainSubstring("project_config_invalid"))
			Expect(string(stderr)).NotTo(ContainSubstring(fixTheFile), "a Go scan has its own wording:\n%s", stderr)
		})
	}

	explicitStates := []unusableOnDisk{
		{"invalid JSON on disk", singleRootPolicyJSON, "{bad"},
		{"a document over the size limit on disk", singleRootPolicyJSON, strings.Repeat(" ", int(projectconfig.MaxBytes)+1)},
	}
	for _, state := range explicitStates {
		It("#168 A2b: refuses a policy named with --project-config with "+state.description+", says to fix the file, and prints no command", func() {
			const named = "cfg/p.json"
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")
			commitFile(repo, named, state.committed)
			scopePolicyRewrite(repo, named, state.onDisk)

			stdout, stderr, exitCode := runCoachCodesignalBaselineEnv(repo, pathWithStubNode("v24.9.9"), "--project-config", named, "--project-language", "typescript", "--no-interactive")

			Expect(exitCode).To(Equal(2), "stdout: %s\nstderr: %s", stdout, stderr)
			Expect(string(stdout)).To(BeEmpty())
			Expect(string(stderr)).To(ContainSubstring("project_config_invalid"))
			Expect(string(stderr)).To(ContainSubstring(named))
			Expect(strings.Count(string(stderr), fixTheFile)).To(Equal(1), "the sentence is said once:\n%s", stderr)
			expectNoRunnableCommand(string(stderr))
		})
	}
})
