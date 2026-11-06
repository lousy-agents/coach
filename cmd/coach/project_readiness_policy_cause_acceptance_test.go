package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// A policy gap that no remedy fixes says why, in a stable word, on the gap itself.
// Without it a caller of --check-project sees a bare policy_missing for a file that
// is on disk and only needs committing, and cannot tell that from a file that is
// gone. The text prints the same word from the same field, so the two never differ.
var _ = Describe("coach codesignal --check-project: the cause of a policy gap that offers no way out", func() {
	const staged = `{"schema_version":"1","roots":["staged"]}` + "\n"

	removeDirectory := func(repo, name string) { Expect(os.RemoveAll(filepath.Join(repo, name))).To(Succeed()) }

	DescribeTable("names the cause on the gap, in JSON and in text",
		func(policy, wantGap, wantCause string, arrange func(repo string)) {
			repo := commitScannableGoRepo()
			arrange(repo)

			doc := expectCheckProject(repo, "--project-config", policy)
			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, pathWithoutNode(), "--baseline", "--check-project", "--project-language", "typescript", "--project-config", policy)

			Expect(doc.Gaps).To(ContainElement(HaveField("Code", wantGap)), "gaps: %+v", doc.Gaps)
			for _, gap := range doc.Gaps {
				if gap.Code == wantGap {
					Expect(gap.Cause).To(Equal(wantCause), "the cause of %s", wantGap)
				}
			}
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(string(stdout)).To(ContainSubstring("  "+wantGap+" cause="+wantCause+"\n"), "the text prints the same field:\n%s", stdout)
		},
		Entry("a file on disk that is not committed", "un.json", "policy_missing", "not_committed", func(repo string) {
			writeWorktreeFile(repo, "un.json", singleRootPolicyJSON)
		}),
		Entry("a committed policy with a staged edit, then deleted", "cfg/p.json", "policy_missing", "staged_edit", func(repo string) {
			commitFile(repo, "cfg/p.json", singleRootPolicyJSON)
			writeWorktreeFile(repo, "cfg/p.json", staged)
			gitIn(repo, "add", "cfg/p.json")
			removeDirectory(repo, "cfg")
		}),
		Entry("a file where the policy's directory should be", "cfg/p.json", "policy_missing", "parent_not_directory", func(repo string) {
			commitFile(repo, "cfg/p.json", singleRootPolicyJSON)
			removeDirectory(repo, "cfg")
			writeWorktreeFile(repo, "cfg", "a file in the way")
		}),
		Entry("files committed under the name, which makes it a directory", "cfg", "policy_missing", "directory", func(repo string) {
			commitFile(repo, "cfg/p.json", singleRootPolicyJSON)
			removeDirectory(repo, "cfg")
		}),
		Entry("a policy that is a symbolic link", "link.json", "policy_invalid", "unusable_on_disk", func(repo string) {
			writeWorktreeFile(repo, "real.json", singleRootPolicyJSON)
			Expect(os.Symlink("real.json", filepath.Join(repo, "link.json"))).To(Succeed())
		}),
	)

	It("names no cause when a restore or a new policy fixes the gap", func() {
		repo := commitScannableGoRepo()
		commitFile(repo, "cfg/p.json", singleRootPolicyJSON)
		removeDirectory(repo, "cfg")

		doc := expectCheckProject(repo, "--project-config", "cfg/p.json")

		Expect(doc.Gaps).To(ContainElement(HaveField("Code", "policy_missing")))
		for _, gap := range doc.Gaps {
			Expect(gap.Cause).To(BeEmpty(), "a gap with a remedy needs no cause: %+v", gap)
		}
	})
})
