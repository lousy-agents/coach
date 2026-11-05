package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// A working-tree status git cannot produce is an operational error: the scan
// has nothing to describe, so it exits 1 and says what it could not do. It never
// prints a clean verdict.
var _ = Describe("coach codesignal --worktree when git cannot read the working tree status", func() {
	corruptIndexRepo := func() (repo, head string) {
		repo = goOverlayRepo()
		head = commitFile(repo, "pkg/handlers/extra.go", "package handlers\n")
		Expect(os.WriteFile(filepath.Join(repo, ".git", "index"), []byte("not an index"), 0o644)).To(Succeed())
		return repo, head
	}

	DescribeTable("exits 1 with a message that names what coach could not do",
		func(run func(repo, head string) ([]byte, []byte, int)) {
			repo, head := corruptIndexRepo()

			_, stderr, exitCode := run(repo, head)

			Expect(exitCode).To(Equal(1), "stderr: %s", stderr)
			Expect(string(stderr)).To(HavePrefix("coach: listing working tree changes: "),
				"a raw git failure with no coach prefix does not say what the scan was doing")
		},
		Entry("a baseline scan", func(repo, _ string) ([]byte, []byte, int) {
			return runCoachCodesignalBaselineRaw(repo, "--worktree")
		}),
		Entry("a diff scan", func(repo, head string) ([]byte, []byte, int) {
			return runCoachCodesignalRaw(repo, head, "--worktree")
		}),
	)
})
