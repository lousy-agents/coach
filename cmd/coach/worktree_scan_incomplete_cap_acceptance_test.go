package main

import (
	"maps"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --worktree lists at most 20 roots per reason and discloses the rest in one entry (#168 item 22)", func() {
	const hiddenName = "notes.md"

	DescribeTable("lists 20 sorted roots and one truncation entry carrying the count and no root (#168 item 22)",
		func(m hiding) {
			repo := scanRepoWith(rootedFiles("d", 25, hiddenName, "# notes\n"))
			hidePaths(repo, m, rootsNamed("d", 1, 25), hiddenName)

			run := runProjectScan(repo)

			Expect(run.roots(m.reason)).To(Equal(rootsNamed("d", 1, 20)))
			Expect(run.ofReason(m.reason)).To(HaveLen(21))
			truncation, truncated := run.truncation(m.reason)
			Expect(truncated).To(BeTrue(), "entries: %v", run.entries)
			Expect(truncation).To(And(HaveKeyWithValue("roots_not_listed", BeNumerically("==", 5)), HaveKeyWithValue("hides_in_scope", "no"), Not(HaveKey("root"))))
			Expect(run.exitCode).To(Equal(0))
		},
		Entry("sparse checkout", sparseHiding),
		Entry("skip-worktree", skipWorktreeHiding),
		Entry("assume-unchanged", assumeUnchangedHiding),
	)

	It("caps the roots of a reason B2 adds as it caps B1's (#168 item 22)", func() {
		repo := scanRepoWith(nil)
		for _, root := range rootsNamed("nest", 1, 21) {
			addNestedRepository(repo, root)
		}

		run := runProjectScan(repo)

		Expect(run.roots("nested_repository")).To(Equal(rootsNamed("nest", 1, 20)))
		truncation, truncated := run.truncation("nested_repository")
		Expect(truncated).To(BeTrue(), "entries: %v", run.entries)
		Expect(truncation).To(And(HaveKeyWithValue("roots_not_listed", BeNumerically("==", 1)), HaveKeyWithValue("hides_in_scope", "no"), Not(HaveKey("root"))))
		Expect(run.exitCode).To(Equal(0))
	})

	It("makes the truncation entry yes when the one unlisted root hides in-scope source, and exits 3 (#168 item 22)", func() {
		files := rootedFiles("d", 20, hiddenName, "# notes\n")
		files["z-dir/hidden.go"] = "package z\n"
		repo := scanRepoWith(files)
		hidePaths(repo, skipWorktreeHiding, rootsNamed("d", 1, 20), hiddenName)
		skipWorktreeHiding.hide(repo, "z-dir/hidden.go")

		run := runProjectScan(repo)

		Expect(run.roots("skip_worktree")).To(Equal(rootsNamed("d", 1, 20)))
		Expect(run.listed("skip_worktree")).To(HaveEach(HaveKeyWithValue("hides_in_scope", "no")))
		truncation, truncated := run.truncation("skip_worktree")
		Expect(truncated).To(BeTrue(), "entries: %v", run.entries)
		Expect(truncation).To(And(HaveKeyWithValue("roots_not_listed", BeNumerically("==", 1)), HaveKeyWithValue("hides_in_scope", "yes")))
		Expect(run.exitCode).To(Equal(3))
	})

	It("makes the truncation entry no when only a listed root hides in-scope source, and still exits 3 from that root (#168 item 22)", func() {
		files := rootedFiles("d", 21, hiddenName, "# notes\n")
		files["d01/notes.go"] = "package d\n"
		repo := scanRepoWith(files)
		hidePaths(repo, skipWorktreeHiding, rootsNamed("d", 2, 21), hiddenName)
		skipWorktreeHiding.hide(repo, "d01/notes.go")

		run := runProjectScan(repo)

		Expect(run.roots("skip_worktree")).To(Equal(rootsNamed("d", 1, 20)))
		Expect(run.hidesInScopeByRoot("skip_worktree")).To(HaveKeyWithValue("d01", "yes"))
		truncation, truncated := run.truncation("skip_worktree")
		Expect(truncated).To(BeTrue(), "entries: %v", run.entries)
		Expect(truncation).To(And(HaveKeyWithValue("roots_not_listed", BeNumerically("==", 1)), HaveKeyWithValue("hides_in_scope", "no")),
			"the truncation entry is decided by the unlisted roots alone")
		Expect(run.exitCode).To(Equal(3), "the listed d01 entry qualifies the verdict")
	})

	It("makes the truncation entry yes when a yes root sits between no roots among the unlisted ones (#168 item 22)", func() {
		files := rootedFiles("d", 21, hiddenName, "# notes\n")
		files["d22/hidden.go"] = "package d\n"
		files["d23/notes.md"] = "# notes\n"
		repo := scanRepoWith(files)
		hidePaths(repo, skipWorktreeHiding, rootsNamed("d", 1, 21), hiddenName)
		skipWorktreeHiding.hide(repo, "d22/hidden.go", "d23/notes.md")

		run := runProjectScan(repo)

		Expect(run.roots("skip_worktree")).To(Equal(rootsNamed("d", 1, 20)))
		truncation, truncated := run.truncation("skip_worktree")
		Expect(truncated).To(BeTrue(), "entries: %v", run.entries)
		Expect(truncation).To(And(HaveKeyWithValue("roots_not_listed", BeNumerically("==", 3)), HaveKeyWithValue("hides_in_scope", "yes")),
			"the first unlisted root (d21) and the last (d23) both read no, so only an OR over every unlisted root says yes")
		Expect(run.exitCode).To(Equal(3))
	})

	It("adds no truncation entry for a reason with exactly 20 roots (#168 item 22)", func() {
		repo := scanRepoWith(rootedFiles("d", 20, hiddenName, "# notes\n"))
		hidePaths(repo, skipWorktreeHiding, rootsNamed("d", 1, 20), hiddenName)

		run := runProjectScan(repo)

		Expect(run.roots("skip_worktree")).To(Equal(rootsNamed("d", 1, 20)))
		Expect(run.ofReason("skip_worktree")).To(HaveLen(20))
		_, truncated := run.truncation("skip_worktree")
		Expect(truncated).To(BeFalse())
	})

	It("counts each reason's roots on its own: 25 skip-worktree roots truncate, 3 assume-unchanged roots do not (#168 item 22)", func() {
		files := rootedFiles("d", 25, hiddenName, "# notes\n")
		maps.Copy(files, rootedFiles("a", 3, hiddenName, "# notes\n"))
		repo := scanRepoWith(files)
		hidePaths(repo, skipWorktreeHiding, rootsNamed("d", 1, 25), hiddenName)
		hidePaths(repo, assumeUnchangedHiding, rootsNamed("a", 1, 3), hiddenName)

		run := runProjectScan(repo)

		Expect(run.roots("assume_unchanged")).To(Equal(rootsNamed("a", 1, 3)))
		Expect(run.ofReason("assume_unchanged")).To(HaveLen(3))
		_, assumeTruncated := run.truncation("assume_unchanged")
		Expect(assumeTruncated).To(BeFalse())
		_, skipTruncated := run.truncation("skip_worktree")
		Expect(skipTruncated).To(BeTrue(), "entries: %v", run.entries)
	})
})
