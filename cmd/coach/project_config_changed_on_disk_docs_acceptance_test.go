package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The CLI contract says how a file with work in it is told from the committed copy, because a replacement offered over a
// file that was wrongly taken for the committed copy destroys what is in it, and the contract's reader needs to know
// which files are taken for it.
var _ = Describe("the CLI contract: changed_on_disk is decided by the file's bytes, then by git's own conversion", func() {
	It("says the file is read from the working tree and compared with the committed blob, without asking git for a diff", func() {
		text := documentText("docs/cli-codesignal.md")

		Expect(text).To(ContainSubstring("whose copy in the working tree differs from the committed one, byte for byte"))
		Expect(text).To(ContainSubstring("The file is read from the working tree and compared with the committed blob, and git is not asked for a diff, so nothing is written to the index and no hook runs"))
	})

	It("says bytes that differ are still the committed copy when git's own conversion gives the blob, as in a checkout made on Windows", func() {
		text := documentText("docs/cli-codesignal.md")

		Expect(text).To(ContainSubstring("Bytes that differ are the committed copy all the same when git's own conversion of the file for its path gives the blob (the line endings that `core.autocrlf` and the `text` and `eol` attributes change, `$Id$` expansion and working-tree encoding), so a checkout made on Windows is not a file with work in it."))
		Expect(text).To(ContainSubstring("Git is asked for that conversion only when no `filter` attribute is set for the path, and it refreshes no index, takes no lock and runs no hook or program."))
	})

	It("says what is taken as having work in it: a filter attribute, what git cannot answer, and a mark does not hide an edit", func() {
		text := documentText("docs/cli-codesignal.md")

		Expect(text).To(ContainSubstring("A file that a `filter` attribute selects a program for, or that git cannot be asked about, is taken to have work in it, because no program runs on a scan."))
		Expect(text).To(ContainSubstring("a skip-worktree or assume-unchanged mark does not hide an edit"))
	})

	It("describes one channel for a clean filter, the status read, and says the comparison of the policy is not a second one", func() {
		text := documentText("docs/cli-codesignal.md")

		Expect(text).To(ContainSubstring("The one channel left is a clean filter, reached by one read: when `git status` has to hash a tracked file"))
		Expect(text).To(ContainSubstring("is not a second channel for a clean filter"))
		for _, stale := range []string{"reached by two reads", "git diff --name-only", "a scan asks it more than once", "twice in the scan measured", "Comparing the bytes without the filter would call a file modified"} {
			Expect(text).NotTo(ContainSubstring(stale), "the CLI contract still carries %q from the git diff read that is gone", stale)
		}
	})
})
