package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// An intent-to-add entry holds no content, so it is not a staged copy to keep. When it
// carries a skip-worktree mark, git add refuses a new file at the path, so Coach still
// offers no new policy, and the message names the empty entry and the mark and not a
// copy that does not exist.
var _ = Describe("coach codesignal: a policy nothing commits whose index holds only an empty entry marked skip-worktree", func() {
	const policy = "cfg/p.json"

	arrange := func(sparse bool) string {
		repo := commitScannableGoRepo()
		writeWorktreeFile(repo, policy, singleRootPolicyJSON)
		gitIn(repo, "add", "-N", policy)
		if sparse {
			gitIn(repo, "config", "core.sparseCheckout", "true")
		}
		gitIn(repo, "update-index", "--skip-worktree", policy)
		Expect(os.RemoveAll(filepath.Join(repo, "cfg"))).To(Succeed())
		return repo
	}

	DescribeTable("calls it an empty entry, and offers neither a restore nor a new policy",
		func(sparse bool, wantWords ...string) {
			stderr := scanExplicitPolicy(arrange(sparse), policy)

			Expect(printedRestoreCommand(stderr)).To(BeEmpty(), "stderr:\n%s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("--suggest-project-config"), "git add refuses the path while the mark is on:\n%s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("staged copy"), "the entry holds no content:\n%s", stderr)
			for _, words := range wantWords {
				Expect(string(stderr)).To(ContainSubstring(words), "stderr:\n%s", stderr)
			}
		},
		Entry("marked by hand", false, "empty entry", "marked skip-worktree", "update-index --no-skip-worktree"),
		Entry("left out by a sparse checkout", true, "empty entry", "sparse checkout"),
	)
})
