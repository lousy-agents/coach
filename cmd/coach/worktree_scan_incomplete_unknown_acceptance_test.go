//go:build unix

package main

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --worktree cannot tell whether what git could not read hides source (#168 item 22)", func() {
	const unreadableWarning = "warning: could not open directory 'pkg/y/': Permission denied"

	It("discloses an unrecognized git warning as unknown, with no root and none of git's words, and exits 3 (#168 item 22)", func() {
		repo := scanRepoWith(nil)

		run := runWorktreeScan(repo, envWithStatusWarnings("warning: something unexpected"), "--project-config", "project.json")

		entry := run.only("unrecognized_git_warning")
		Expect(entry).To(HaveKeyWithValue("hides_in_scope", "unknown"))
		Expect(entry).NotTo(HaveKey("root"))
		Expect(pathOf(entry)).To(BeEmpty())
		Expect(entry["message"]).NotTo(ContainSubstring("something unexpected"))
		Expect(run.exitCode).To(Equal(3))
	})

	It("discloses a directory git could not open as unreadable_directory rooted there, unknown, and exits 3 (#168 item 22)", func() {
		repo := scanRepoWith(nil)

		run := runWorktreeScan(repo, envWithStatusWarnings(unreadableWarning), "--project-config", "project.json")

		entry := run.only("unreadable_directory")
		Expect(entry).To(HaveKeyWithValue("root", "pkg/y"))
		Expect(entry).To(HaveKeyWithValue("hides_in_scope", "unknown"))
		Expect(entry["message"]).NotTo(ContainSubstring("warning:"))
		Expect(run.exitCode).To(Equal(3))
	})

	It("qualifies the text verdict for an unreadable directory and names it without relaying git's stderr (#168 item 22)", func() {
		repo := scanRepoWith(nil)
		stdout, stderr, exitCode := runCoachBinary(commandPath, repo, envWithStatusWarnings(unreadableWarning),
			"codesignal", "--baseline", "--worktree", "--project-config", "project.json")

		Expect(string(stderr)).To(BeEmpty())
		Expect(exitCode).To(Equal(0))
		Expect(noFindingsHeadline(string(stdout))).NotTo(Equal(unqualifiedAllClear))
		Expect(string(stdout)).To(ContainSubstring("pkg/y/"))
		Expect(string(stdout)).NotTo(ContainSubstring("warning:"))
	})

	It("makes the truncation entry unknown, not no, when 21 unreadable directories are unlisted by one (#168 item 22)", func() {
		repo := scanRepoWith(nil)
		var warnings []string
		for _, root := range rootsNamed("pkg/u", 1, 21) {
			warnings = append(warnings, fmt.Sprintf("warning: could not open directory '%s/': Permission denied", root))
		}

		run := runWorktreeScan(repo, envWithStatusWarnings(warnings...), "--project-config", "project.json")

		Expect(run.roots("unreadable_directory")).To(Equal(rootsNamed("pkg/u", 1, 20)))
		truncation, truncated := run.truncation("unreadable_directory")
		Expect(truncated).To(BeTrue(), "entries: %v", run.entries)
		Expect(truncation).To(And(HaveKeyWithValue("roots_not_listed", BeNumerically("==", 1)), HaveKeyWithValue("hides_in_scope", "unknown"), Not(HaveKey("root"))))
		Expect(run.exitCode).To(Equal(3))
	})

	It("discloses a directory the process cannot open as unreadable_directory, unknown (#168 item 22)", func() {
		if os.Geteuid() == 0 {
			Skip("root opens a directory whatever its mode; `mise run acceptance-ci-env` runs it as non-root")
		}
		repo := scanRepoWith(nil)
		overwriteOnDiskCreatingParents(repo, "pkg/y/y.go", handlersImportingDB)
		directory := filepath.Join(repo, "pkg/y")
		Expect(os.Chmod(directory, 0o000)).To(Succeed())
		DeferCleanup(os.Chmod, directory, os.FileMode(0o755))

		run := runProjectScan(repo)

		Expect(run.only("unreadable_directory")).To(And(HaveKeyWithValue("root", "pkg/y"), HaveKeyWithValue("hides_in_scope", "unknown")))
		Expect(run.exitCode).To(Equal(3))
	})
})
