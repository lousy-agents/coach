package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --worktree for an untracked directory git cannot open", func() {
	unreadableDirectoryRepo := func() string {
		if os.Geteuid() == 0 {
			Skip("root opens a directory whatever its mode; `mise run acceptance-ci-env` runs it as non-root")
		}
		repo := goOverlayRepo()
		overwriteOnDiskCreatingParents(repo, "pkg/y/y.go", handlersImportingDB)
		directory := filepath.Join(repo, "pkg/y")
		ExpectWithOffset(1, os.Chmod(directory, 0o000)).To(Succeed())
		DeferCleanup(os.Chmod, directory, os.FileMode(0o755))
		return repo
	}

	It("qualifies the verdict and exits 3, naming the directory in its own entry, instead of reporting a clean tree", func() {
		repo := unreadableDirectoryRepo()

		text, stderr, _ := runCoachCodesignalBaselineRaw(repo, "--worktree")
		run := runWorktreeScan(repo, nil)

		Expect(run.exitCode).To(Equal(3), "stderr: %s stdout: %s", stderr, text)
		Expect(string(text)).NotTo(MatchRegexp(`worktree: no uncommitted changes`),
			"git could not read part of the tree, so it cannot say there are no changes")
		entry := run.only("unreadable_directory")
		Expect(entry).To(HaveKeyWithValue("root", "pkg/y"))
		Expect(entry["message"]).NotTo(ContainSubstring("warning:"), "the message states the cause; it does not relay git's stderr")
		Expect(string(text)).To(ContainSubstring("pkg/y/"))
	})

	DescribeTable("never drops a directory whose name git quotes oddly, and never prints it raw (#168 item 22)",
		func(name, root, shown string) {
			if os.Geteuid() == 0 {
				Skip("root opens a directory whatever its mode; `mise run acceptance-ci-env` runs it as non-root")
			}
			repo := goOverlayRepo()
			overwriteOnDiskCreatingParents(repo, filepath.Join("pkg", name, "y.go"), handlersImportingDB)
			directory := filepath.Join(repo, "pkg", name)
			Expect(os.Chmod(directory, 0o000)).To(Succeed())
			DeferCleanup(os.Chmod, directory, os.FileMode(0o755))

			text, _, _ := runCoachCodesignalBaselineRaw(repo, "--worktree")
			run := runWorktreeScan(repo, nil)

			Expect(run.exitCode).To(Equal(3), "stdout: %s", text)
			Expect(run.only("unreadable_directory")).To(HaveKeyWithValue("root", root), "the JSON root is the real directory name")
			rawJSON, _, _ := runCoachCodesignalBaselineRaw(repo, "--worktree", "--format=json")
			encodedRoot, err := json.Marshal(root)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(rawJSON)).To(ContainSubstring(`"root":`+string(encodedRoot)), "control characters are escaped in the JSON bytes")
			Expect(string(rawJSON)).NotTo(ContainSubstring("\t"), "no raw control character reaches the JSON")
			Expect(string(text)).To(ContainSubstring(shown))
			Expect(string(text)).NotTo(ContainSubstring("\t"), "a control character in a name is escaped, never relayed")
		},
		Entry("an apostrophe", "it's", "pkg/it's", "pkg/it's/"),
		Entry("a tab", "tab\there", "pkg/tab\there", `"pkg/tab\there/"`),
	)

	It("leaves a readable untracked directory alone (control)", func() {
		repo := goOverlayRepo()
		overwriteOnDiskCreatingParents(repo, "pkg/y/y.go", handlersImportingDB)

		text, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--worktree", "--fail-on-incomplete-coverage", "--format=json")

		Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, text)
		Expect(diagnosticsOfKind(decodeCoachReport(text).Diagnostics, "worktree_scan_incomplete")).To(BeEmpty())
	})
})
