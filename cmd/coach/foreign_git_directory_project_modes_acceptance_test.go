package main

import (
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const foreignGitDirectoryWords = "is not the one that holds this working tree"

func foreignGitDirectoryViaGitDir(_, other string) []string {
	return []string{"GIT_DIR=" + filepath.Join(other, ".git")}
}

func foreignGitDirectoryViaGitDirAndWorkTree(repo, other string) []string {
	return []string{"GIT_DIR=" + filepath.Join(other, ".git"), "GIT_WORK_TREE=" + repo}
}

// expectRefusedAsForeign runs coach in repo with git pointed at other and asserts the refusal, after asserting that
// git itself reports other's git directory for repo's working tree.
func expectRefusedAsForeign(repo string, relocate func(repo, other string) []string, args ...string) {
	GinkgoHelper()
	other := commitScannableGoRepo()
	extra := relocate(repo, other)
	layout := layoutOf(repo, extra)
	Expect(layout.gitDir).To(Equal(filepath.Join(other, ".git")), "the fixture must point git at the other repository")
	Expect(layout.topLevel).To(Equal(repo), "the fixture must run from the first repository's working tree")
	Expect(layout.coreWorktree).To(BeEmpty())

	stdout, stderr, exitCode := runCoachBinary(commandPath, repo, environmentNamingNoRepository(extra...), append([]string{"codesignal"}, args...)...)

	Expect(exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderr)
	Expect(string(stderr)).To(ContainSubstring(foreignGitDirectoryWords))
	Expect(string(stderr)).To(ContainSubstring(layout.gitDir))
	Expect(stdout).To(BeEmpty())
}

var _ = Describe("coach codesignal: the project modes fail closed when git's repository is not the one that holds the working tree (#168 A2b-2)", func() {
	policyEditedOnDisk := func() string {
		repo := typescriptRepoForPolicyFromDisk()
		commitFile(repo, coachConfigPath, singleRootPolicyJSON)
		writeWorktreeFile(repo, coachConfigPath, validPolicyWithLayers)
		return repo
	}

	DescribeTable("--check-project exits 2 with an empty stdout, though a valid policy is on disk",
		func(relocate func(repo, other string) []string, format string) {
			expectRefusedAsForeign(policyEditedOnDisk(), relocate, "--baseline", "--check-project", "--project-language", "typescript", "--format="+format)
		},
		Entry("JSON, GIT_DIR names another repository", foreignGitDirectoryViaGitDir, "json"),
		Entry("text, GIT_DIR names another repository", foreignGitDirectoryViaGitDir, "text"),
		Entry("JSON, GIT_WORK_TREE names the scanned repository", foreignGitDirectoryViaGitDirAndWorkTree, "json"),
		Entry("text, GIT_WORK_TREE names the scanned repository", foreignGitDirectoryViaGitDirAndWorkTree, "text"),
	)

	DescribeTable("--prepare-compiler exits 2 before any prompt",
		func(relocate func(repo, other string) []string) {
			expectRefusedAsForeign(typescriptRepoWithoutPolicy(), relocate, "--baseline", "--prepare-compiler", "--project-language", "typescript")
		},
		Entry("GIT_DIR names another repository", foreignGitDirectoryViaGitDir),
		Entry("GIT_WORK_TREE names the scanned repository", foreignGitDirectoryViaGitDirAndWorkTree),
	)

	DescribeTable("--suggest-project-config exits 2 with an empty stdout",
		func(relocate func(repo, other string) []string, languageArgs ...string) {
			expectRefusedAsForeign(typescriptRepoWithoutPolicy(), relocate, append([]string{"--baseline", "--suggest-project-config"}, languageArgs...)...)
		},
		Entry("Go, GIT_DIR names another repository", foreignGitDirectoryViaGitDir),
		Entry("TypeScript, GIT_DIR names another repository", foreignGitDirectoryViaGitDir, "--project-language", "typescript"),
		Entry("Go, GIT_WORK_TREE names the scanned repository", foreignGitDirectoryViaGitDirAndWorkTree),
		Entry("TypeScript, GIT_WORK_TREE names the scanned repository", foreignGitDirectoryViaGitDirAndWorkTree, "--project-language", "typescript"),
	)
})

var _ = Describe("coach codesignal: the project modes still run in the repository that holds the working tree (#168 A2b-2)", func() {
	It("--check-project reports the policy on disk when GIT_DIR names the repository's own git directory", func() {
		repo := typescriptRepoForPolicyFromDisk()
		commitFile(repo, coachConfigPath, singleRootPolicyJSON)
		writeWorktreeFile(repo, coachConfigPath, validPolicyWithLayers)
		extra := []string{"GIT_DIR=" + filepath.Join(repo, ".git"), "GIT_WORK_TREE=" + repo}

		stdout, stderr, exitCode := runCoachBinary(commandPath, repo, environmentNamingNoRepository(extra...), "codesignal", "--baseline", "--check-project", "--project-language", "typescript", "--format=json")

		Expect(exitCode).To(Equal(0), "stdout=%s stderr=%s", stdout, stderr)
		Expect(string(stdout)).To(ContainSubstring("project_policy_differs_from_head"))
		Expect(string(stderr)).NotTo(ContainSubstring(foreignGitDirectoryWords))
	})

	DescribeTable("a mode that needs a terminal reaches its own terminal check, not the guard",
		func(args ...string) {
			repo := typescriptRepoWithoutPolicy()

			_, stderr, exitCode := runCoachBinary(commandPath, repo, environmentNamingNoRepository(), append([]string{"codesignal", "--baseline"}, args...)...)

			Expect(exitCode).To(Equal(2))
			Expect(string(stderr)).To(ContainSubstring("no controlling terminal is available"))
			Expect(string(stderr)).NotTo(ContainSubstring(foreignGitDirectoryWords))
		},
		Entry("--prepare-compiler", "--prepare-compiler", "--project-language", "typescript"),
		Entry("--suggest-project-config for TypeScript", "--suggest-project-config", "--project-language", "typescript"),
	)

	It("--suggest-project-config for Go suggests roots when git's repository is the working tree's own", func() {
		repo := newTempGitRepo()
		commitFile(repo, "go.mod", "module example.com/single\n\ngo 1.25\n")
		commitFile(repo, "main.go", "package main\n\nfunc main() {}\n")

		stdout, stderr, exitCode := runCoachBinary(commandPath, repo, environmentNamingNoRepository(), "codesignal", "--baseline", "--suggest-project-config")

		Expect(exitCode).To(Equal(0), "stdout=%s stderr=%s", stdout, stderr)
		Expect(string(stdout)).To(ContainSubstring(`"roots"`))
		Expect(string(stderr)).NotTo(ContainSubstring(foreignGitDirectoryWords))
	})
})
