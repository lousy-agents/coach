package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The CLI contract says how a file with work in it is told from the committed copy, because a replacement offered over a
// file that was wrongly taken for the committed copy destroys what is in it, and the contract's reader needs to know
// which files are taken for it.
var _ = Describe("the CLI contract: changed_on_disk is decided by the file's bytes", func() {
	It("says the file is read from the working tree and compared byte for byte, without asking git for a diff", func() {
		text := documentText("docs/cli-codesignal.md")

		Expect(text).To(ContainSubstring("whose copy in the working tree differs from the committed one, byte for byte"))
		Expect(text).To(ContainSubstring("The file is read from the working tree and compared with the committed blob, and git is not asked for a diff, so nothing is written to the index and no hook or filter runs"))
	})

	It("says what is taken as differing: a skip-worktree or assume-unchanged mark hides nothing, and a converted checkout differs", func() {
		text := documentText("docs/cli-codesignal.md")

		Expect(text).To(ContainSubstring("a skip-worktree or assume-unchanged mark does not hide an edit"))
		Expect(text).To(ContainSubstring("a checkout whose line endings or content were converted differs from the blob, and is taken to have work in it, which is the safe side"))
	})
})
