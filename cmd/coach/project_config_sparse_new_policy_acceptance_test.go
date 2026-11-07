package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// A new policy for a directory the sparse checkout leaves out is still the way forward, and git add
// refuses the file it writes unless it is told the path is outside the checkout. The scan says so with
// the offer, and --check-project still names the action.
var _ = Describe("coach codesignal: a new policy in a directory the sparse checkout leaves out", func() {
	It("offers a new policy and says how to stage it", func() {
		repo := commitScannableGoRepo()
		commitFile(repo, "cfg/p.json", singleRootPolicyJSON)
		gitIn(repo, "sparse-checkout", "set", "--no-cone", "/*", "!/cfg/")

		stderr := scanExplicitPolicy(repo, "cfg/zz.json")
		doc := expectCheckProject(repo, "--project-config", "cfg/zz.json")

		Expect(string(stderr)).To(ContainSubstring("--suggest-project-config"), "the offer stays:\n%s", stderr)
		Expect(string(stderr)).To(ContainSubstring("git add --sparse"), "stderr:\n%s", stderr)
		Expect(nextActionKinds(doc)).To(ContainElement("author_policy"), "next actions: %+v", doc.NextActions)
	})
})
