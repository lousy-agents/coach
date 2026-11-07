package main

import (
	"os"
	"path/filepath"
	"regexp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	renameCommand     = regexp.MustCompile(`rename it with (top=.+?) and commit the rename \(project_config_invalid\)`)
	keepOneCommand    = regexp.MustCompile(`to keep "[^"]+", run (.+?) and commit \(project_config_invalid\)`)
	uncommittedPolicy = ".lousy-agents/coach.json is in the worktree but is not committed"
)

// theDiskFoldsCase is whether the repository is on a disk where .GIT is .git.
func theDiskFoldsCase(repo string) bool {
	_, err := os.Lstat(filepath.Join(repo, ".GIT"))
	return err == nil
}

// Once the user has run the printed git mv and not yet committed, the index records the policy under the spelling
// Coach reads, and the move would fail ("not under version control"): the rename only needs committing. The explicit
// path knows this, and said so; the discovered path printed the same move again, and --check-project went on saying
// that the name differed, while the explicit path for the same state said that the policy is not committed.
var _ = Describe("coach codesignal: a policy Coach discovers, whose rename the user has already staged", func() {
	runTheRename := func(repo string, pattern *regexp.Regexp) {
		GinkgoHelper()
		_, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json", "--no-interactive")
		Expect(exitCode).To(Equal(2), "the policy committed in another case is refused first: %s", stderr)
		command := pattern.FindStringSubmatch(string(stderr))
		Expect(command).NotTo(BeNil(), "the message prints the command:\n%s", stderr)
		output, code := runInShell(repo, command[1])
		Expect(code).To(Equal(0), "the printed command must work: %s", output)
	}

	expectOnlyTheCommitIsLeft := func(repo string) {
		GinkgoHelper()
		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json", "--no-interactive")
		Expect(exitCode).To(Equal(0), "a staged rename is not a refusal: stdout=%s stderr=%s", stdout, stderr)
		Expect(string(stderr)).NotTo(MatchRegexp(`\bmv -- `), "the move would fail, the index no longer holds that spelling:\n%s", stderr)
		doc, _ := checkProjectBothFormats(repo, pathWithoutNode())
		Expect(doc.Checks.Policy.Code).To(Equal("policy_missing"))
		if theDiskFoldsCase(repo) {
			Expect(causeOfGap(doc, "policy_missing")).NotTo(Equal("name_case_differs"), "the rename is staged; only the commit is left: %+v", doc.Gaps)
			return
		}
		Expect(decodeCoachReport(stdout).Diagnostics).To(ContainElement(HaveField("Message", ContainSubstring(uncommittedPolicy))), "the policy is left plainly uncommitted")
		Expect(causeOfGap(doc, "policy_missing")).To(Equal("not_committed"), "the cause the explicit path gives for the same state: %+v", doc.Gaps)
	}

	expectTheCommitIsAccepted := func(repo string) {
		GinkgoHelper()
		gitInWithIdentity(repo, "commit", "-q", "-m", "record the rename of the policy")
		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json", "--no-interactive")
		Expect(exitCode).To(Equal(0), "stdout=%s stderr=%s", stdout, stderr)
		Expect(string(stdout)).To(ContainSubstring(`"schema_version":"2"`), "the policy is read, which turns the report into schema 2")
	}

	It("is left to the commit when the single spelling has been moved", func() {
		repo := commitScannableGoRepo()
		commitFile(repo, ".Lousy-Agents/Coach.json", singleRootPolicyJSON)
		runTheRename(repo, renameCommand)

		expectOnlyTheCommitIsLeft(repo)
		expectTheCommitIsAccepted(repo)
	})

	It("is left to the commit when one of several spellings has been kept", func() {
		repo := commitScannableGoRepo()
		commitSpellingsOfThePolicy(repo)
		skipUnlessFilesystemTellsCasesApart()
		gitInWithIdentity(repo, "reset", "-q", "--hard", "HEAD")
		runTheRename(repo, keepOneCommand)

		expectOnlyTheCommitIsLeft(repo)
		expectTheCommitIsAccepted(repo)
	})
})

var _ = Describe("docs: a discovered policy whose rename is staged", func() {
	It("is described as left to the commit, in the CLI reference", func() {
		Expect(documentText("docs/cli-codesignal.md")).To(ContainSubstring("the refusal gives way, the policy is plainly not committed"))
	})
})
