package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --worktree for an untracked directory git cannot open", func() {
	unreadableDirectoryRepo := func() string {
		if os.Geteuid() == 0 {
			Skip("root opens a directory whatever its mode")
		}
		repo := goOverlayRepo()
		overwriteOnDiskCreatingParents(repo, "pkg/y/y.go", handlersImportingDB)
		directory := filepath.Join(repo, "pkg/y")
		ExpectWithOffset(1, os.Chmod(directory, 0o000)).To(Succeed())
		DeferCleanup(os.Chmod, directory, os.FileMode(0o755))
		return repo
	}

	It("qualifies the verdict and exits 3, naming the directory, instead of reporting a clean tree", func() {
		repo := unreadableDirectoryRepo()

		text, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--worktree", "--fail-on-incomplete-coverage")
		jsonOut, _, _ := runCoachCodesignalBaselineRaw(repo, "--worktree", "--fail-on-incomplete-coverage", "--format=json")

		Expect(exitCode).To(Equal(3), "stderr: %s stdout: %s", stderr, text)
		Expect(string(text)).NotTo(MatchRegexp(`worktree: no uncommitted changes`),
			"git could not read part of the tree, so it cannot say there are no changes")
		incomplete := diagnosticsOfKind(decodeCoachReport(jsonOut).Diagnostics, "worktree_scan_incomplete")
		Expect(incomplete).To(HaveLen(1))
		Expect(incomplete[0].Message).To(ContainSubstring("pkg/y/"))
		Expect(incomplete[0].Message).NotTo(ContainSubstring("warning:"), "the message states the cause; it does not relay git's stderr")
	})

	DescribeTable("never drops a directory whose name git quotes oddly, and never prints it raw",
		func(name, shown string) {
			if os.Geteuid() == 0 {
				Skip("root opens a directory whatever its mode")
			}
			repo := goOverlayRepo()
			overwriteOnDiskCreatingParents(repo, filepath.Join("pkg", name, "y.go"), handlersImportingDB)
			directory := filepath.Join(repo, "pkg", name)
			Expect(os.Chmod(directory, 0o000)).To(Succeed())
			DeferCleanup(os.Chmod, directory, os.FileMode(0o755))

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--worktree", "--fail-on-incomplete-coverage", "--format=json")

			Expect(exitCode).To(Equal(3), "stderr: %s stdout: %s", stderr, stdout)
			incomplete := diagnosticsOfKind(decodeCoachReport(stdout).Diagnostics, "worktree_scan_incomplete")
			Expect(incomplete).To(HaveLen(1))
			Expect(incomplete[0].Message).To(ContainSubstring(shown))
			Expect(incomplete[0].Message).NotTo(ContainSubstring("\t"), "a control character in a name is escaped, never relayed")
		},
		Entry("an apostrophe", "it's", "pkg/it's/"),
		Entry("a tab", "tab\there", `"pkg/tab\there/"`),
	)

	It("leaves a readable untracked directory alone (control)", func() {
		repo := goOverlayRepo()
		overwriteOnDiskCreatingParents(repo, "pkg/y/y.go", handlersImportingDB)

		text, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--worktree", "--fail-on-incomplete-coverage", "--format=json")

		Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, text)
		Expect(diagnosticsOfKind(decodeCoachReport(text).Diagnostics, "worktree_scan_incomplete")).To(BeEmpty())
	})
})
