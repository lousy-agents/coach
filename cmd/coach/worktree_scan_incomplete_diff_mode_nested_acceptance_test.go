package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --base --worktree runs a nested repository's path through the scope filter (#168 item 22)", func() {
	DescribeTable("a nested repository directory is in scope only when its own name is, and coverage.excluded is untouched (#168 item 22)",
		func(name string, wantHides string, wantExit int) {
			repo, base := diffScanRepo(nil)
			addNestedRepository(repo, "pkg/"+name)
			control, controlBase := diffScanRepo(nil)

			run := runDiffWorktreeScan(repo, base)
			unnested := runDiffWorktreeScan(control, controlBase)

			Expect(run.only("nested_repository")).To(And(HaveKeyWithValue("root", "pkg/"+name), HaveKeyWithValue("hides_in_scope", wantHides)))
			Expect(run.exitCode).To(Equal(wantExit))
			Expect(unnested.excluded).NotTo(BeEmpty(), "the changed test file must make the excluded group non-empty")
			Expect(run.excluded).To(Equal(unnested.excluded))
		},
		Entry("a name like a Go file", "x.go", "yes", 3),
		Entry("a name like a text file", "x.txt", "no", 0),
	)
})
