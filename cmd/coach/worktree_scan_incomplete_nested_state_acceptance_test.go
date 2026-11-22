package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --worktree runs a nested repository's or submodule's own path through the scope filter (#168 item 22)", func() {
	DescribeTable("a nested repository directory is in scope only when its own name is (#168 item 22)",
		func(name string, wantHides string, wantExit int, extraArgs ...string) {
			repo := scanRepoWith(nil)
			addNestedRepository(repo, "pkg/"+name)

			run := runProjectScan(repo, extraArgs...)

			Expect(run.only("nested_repository")).To(And(HaveKeyWithValue("root", "pkg/"+name), HaveKeyWithValue("hides_in_scope", wantHides)))
			Expect(run.exitCode).To(Equal(wantExit))
		},
		Entry("a name like a Go file", "x.go", "yes", 3),
		Entry("a name like a TypeScript file", "x.ts", "yes", 3),
		Entry("a name like a Go test file under the default scope", "x_test.go", "no", 0),
		Entry("the same Go test file name under --scope all, where only the scope differs", "x_test.go", "yes", 3, "--scope", "all"),
		Entry("a name like a text file", "x.txt", "no", 0),
	)

	It("runs the path of a submodule named like a Go file through the scope filter (#168 item 22)", func() {
		repo := scanRepoWith(nil)
		addLibrarySubmodule(repo, "third/x.go")
		dirtySubmodule(repo, "third/x.go")

		run := runProjectScan(repo)

		Expect(run.only(submoduleReasonOnInstalledGit())).To(And(HaveKeyWithValue("root", "third/x.go"), HaveKeyWithValue("hides_in_scope", "yes")))
		Expect(run.exitCode).To(Equal(3))
	})
})
