package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The clean filter of a repository's own configuration runs when git hashes a file from the working tree. Two reads of
// a scan do: `git status`, and the comparison of a committed policy that cannot be used with its copy in the working
// tree, which is how Coach knows that the copy has work in it. The documents say "the one channel left" and name
// both, so a reader who audits what a scan runs finds the second as well as the first.
var _ = Describe("docs: the clean filter is named as the channel it is, with both reads that reach it", func() {
	It("names the comparison of the policy in the CLI reference", func() {
		reference := documentText("docs/cli-codesignal.md")

		Expect(reference).To(ContainSubstring("git diff --name-only <revision> -- <policy>"), "the comparison of a policy with its copy in the working tree hashes that file through the clean filter")
		Expect(reference).To(ContainSubstring("The one channel left is a clean filter, reached by two reads"))
	})

	It("names the comparison of the policy in the evil-input checklist", func() {
		checklist := documentText("docs/development/evil-input-checklist-595.md")

		Expect(checklist).To(ContainSubstring("git diff --name-only <revision> -- <policy>"))
	})
})
