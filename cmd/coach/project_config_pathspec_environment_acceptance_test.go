package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The pathspec variables change how git reads every pathspec, including the literal ones Coach builds to ask
// about one policy, so a caller that exports them (some tools do, for their children) must not change what
// Coach concludes. Each of them is set here against the same state: a committed policy that is gone from the
// working tree, with an edit to it staged. A restore would discard that edit, so none is offered, and the
// edit is named. Read through a mode that makes the literal pathspec match nothing, the same state reads as
// "never committed", the edit goes unmentioned, and the user is offered a new policy to write over it.
//
// GIT_GLOB_PATHSPECS and GIT_NOGLOB_PATHSPECS did not change this verdict before Coach removed them, as git
// reads a literal pathspec the same under either, so those rows guard the list and were green first;
// GIT_LITERAL_PATHSPECS and GIT_ICASE_PATHSPECS were red.
var _ = Describe("coach codesignal: the caller's pathspec settings do not change what Coach says about a policy", func() {
	DescribeTable("a committed policy deleted from the working tree with a staged edit keeps its disclosure",
		func(variable string) {
			const nested = "cfg/p.json"
			repo := commitScannableGoRepo()
			commitFile(repo, nested, singleRootPolicyJSON)
			Expect(os.WriteFile(filepath.Join(repo, nested), []byte(`{"schema_version":"1","roots":["staged"]}`), 0o644)).To(Succeed())
			gitIn(repo, "add", nested)
			Expect(os.Remove(filepath.Join(repo, nested))).To(Succeed())

			stdout, stderr, exitCode := runCoachBinary(commandPath, repo, append(os.Environ(), variable+"=1"), "codesignal", "--baseline", "--project-config", nested, "--format=json", "--no-interactive")

			Expect(exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderr)
			Expect(string(stderr)).To(ContainSubstring("the index holds a staged edit to it that a restore would discard"), "stderr:\n%s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring(suggestCommand), "that offers a new policy over the staged edit:\n%s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("could not check"), "the question must be answered, not reported unanswered:\n%s", stderr)
			Expect(gitIn(repo, "show", ":"+nested)).To(ContainSubstring("staged"), "the staged edit is untouched")
		},
		Entry("GIT_LITERAL_PATHSPECS", "GIT_LITERAL_PATHSPECS"),
		Entry("GIT_GLOB_PATHSPECS", "GIT_GLOB_PATHSPECS"),
		Entry("GIT_NOGLOB_PATHSPECS", "GIT_NOGLOB_PATHSPECS"),
		Entry("GIT_ICASE_PATHSPECS", "GIT_ICASE_PATHSPECS"),
	)
})
