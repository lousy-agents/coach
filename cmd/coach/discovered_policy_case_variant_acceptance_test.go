package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func skipUnlessTheFileSystemFoldsCase(directory string) {
	GinkgoHelper()
	probe := filepath.Join(directory, "case-probe")
	Expect(os.WriteFile(probe, nil, 0o644)).To(Succeed())
	DeferCleanup(os.Remove, probe)
	if _, err := os.Lstat(filepath.Join(directory, "CASE-PROBE")); err != nil {
		Skip("needs a file system that ignores case, as macOS and Windows do; internal/codesignalcli/projectconfig pins the same decisions over a disk that folds case")
	}
}

func writeInRepo(repo, name, content string) {
	GinkgoHelper()
	Expect(os.MkdirAll(filepath.Dir(filepath.Join(repo, name)), 0o755)).To(Succeed())
	Expect(os.WriteFile(filepath.Join(repo, name), []byte(content), 0o644)).To(Succeed())
}

// scannableRepoWithTwoCommits returns a repository that commits no policy, and the first of its two commits.
func scannableRepoWithTwoCommits() (repo, first string) {
	repo = newTempGitRepo()
	first = commitFile(repo, "a.go", minimalGoSource)
	commitFile(repo, "b.go", minimalGoSource)
	return repo, first
}

// A policy is discovered only at .lousy-agents/coach.json. When the revision commits nothing there in any
// spelling, a directory or a file that the disk keeps in another case is not a policy for the spelling to be
// wrong about, and a file system that ignores case must not turn it into a refusal of every run: v0.11.0 scanned
// without a policy, and so does this. The refusal that names a spelling is for a revision that commits one.
var _ = Describe("coach codesignal: a default policy path the disk spells in another case, with nothing committed in any spelling", func() {
	variants := []TableEntry{
		Entry("an untracked directory in another case", func(repo string) {
			writeInRepo(repo, ".Lousy-Agents/other", "x\n")
		}, ""),
		Entry("another tool's file, committed in a directory in another case", func(repo string) {
			writeInRepo(repo, ".Lousy-Agents/doctor.json", "{}\n")
			gitOutputIn(repo, "add", "-A")
			gitOutputIn(repo, "commit", "--quiet", "-m", "another tool's file")
		}, ""),
		Entry("an untracked policy file in another case", func(repo string) {
			writeInRepo(repo, ".lousy-agents/Coach.json", singleRootPolicyJSON)
		}, ".lousy-agents/Coach.json"),
	}

	DescribeTable("a baseline scan goes on without a policy",
		func(arrange func(repo string), named string) {
			repo, _ := scannableRepoWithTwoCommits()
			skipUnlessTheFileSystemFoldsCase(repo)
			arrange(repo)

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json", "--no-interactive")

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("--project-config"), "the user passed none")
			report := decodeCoachReport(stdout)
			Expect(report.Diagnostics).NotTo(ContainElement(HaveField("Message", ContainSubstring("stored as"))))
			if named == "" {
				Expect(report.Diagnostics).NotTo(ContainElement(HaveField("Message", ContainSubstring("name_case_differs"))))
			} else {
				Expect(report.Diagnostics).To(ContainElement(HaveField("Message", And(ContainSubstring("cause=name_case_differs"), ContainSubstring(named)))))
			}
		},
		variants,
	)

	DescribeTable("a diff scan goes on without a policy",
		func(arrange func(repo string), _ string) {
			repo, first := scannableRepoWithTwoCommits()
			skipUnlessTheFileSystemFoldsCase(repo)
			arrange(repo)

			_, stderr, exitCode := runCoachBinary(commandPath, repo, nil, "codesignal", "--base", first, "--format=json", "--no-interactive")

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("--project-config"), "the user passed none")
		},
		variants,
	)

	DescribeTable("the project check reports the policy as missing, as it does with no variant on disk",
		func(arrange func(repo string), _ string) {
			repo, _ := scannableRepoWithTwoCommits()
			skipUnlessTheFileSystemFoldsCase(repo)
			arrange(repo)

			doc, _ := checkProjectBothFormats(repo, pathWithoutNode())

			Expect(doc.Checks.Policy.Code).To(Equal("policy_missing"))
			Expect(doc.Gaps).NotTo(ContainElement(HaveField("Cause", "name_case_differs")))
		},
		variants,
	)
})
