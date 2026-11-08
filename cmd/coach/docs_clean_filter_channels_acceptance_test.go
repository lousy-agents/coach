package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The clean filter of a repository's own configuration runs when git hashes a file from the working tree. One read of a
// scan does: `git status`. The comparison of a committed policy that cannot be used with its copy in the working tree,
// which is how Coach knows that the copy has work in it, used to be a second, and is not: it compares the bytes, and asks
// git for the file's `filter` attribute before it asks git to hash it. The documents say "the one channel left" and say so,
// so a reader who audits what a scan runs finds the one that is left and is told that the other is closed.
var _ = Describe("docs: the clean filter is named as the one channel it is, with the one read that reaches it", func() {
	It("names the status read as the one channel and the comparison of the policy as closed, in the CLI reference", func() {
		reference := documentText("docs/cli-codesignal.md")

		Expect(reference).To(ContainSubstring("The one channel left is a clean filter, reached by one read"))
		Expect(reference).To(ContainSubstring("is not a second channel for a clean filter"))
		Expect(reference).NotTo(ContainSubstring("git diff --name-only"), "the comparison no longer asks git for a diff")
	})

	It("records in the evil-input checklist that the comparison of the policy is no longer a channel", func() {
		checklist := documentText("docs/development/evil-input-checklist-595.md")

		Expect(checklist).To(ContainSubstring("used to be a second channel (`git diff --name-only <revision> -- <policy>`, more than once in a scan); it is closed"))
	})
})
