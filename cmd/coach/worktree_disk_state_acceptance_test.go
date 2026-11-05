//go:build unix

package main

import (
	"os"
	"path/filepath"
	"syscall"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

var _ = Describe("coach codesignal --worktree classifies what it finds on disk", func() {
	diagnosticsAt := func(report *codesignal.Report, path string) []codesignal.Diagnostic {
		matching := []codesignal.Diagnostic{}
		for _, diagnostic := range report.Diagnostics {
			if diagnostic.Path == path {
				matching = append(matching, diagnostic)
			}
		}
		return matching
	}

	When("a directory that held a tracked file is replaced by a regular file", func() {
		It("treats the file under it as removed, as it does for a plain deletion (control)", func() {
			repo := goOverlayRepo()
			Expect(os.Remove(filepath.Join(repo, "pkg/db/db.go"))).To(Succeed())

			report := scanGoBaseline(repo, "--worktree")

			Expect(diagnosticsAt(report, "pkg/db/db.go")).To(BeEmpty())
		})

		It("does not report the file under it as unreadable", func() {
			repo := goOverlayRepo()
			Expect(os.RemoveAll(filepath.Join(repo, "pkg/db"))).To(Succeed())
			Expect(os.WriteFile(filepath.Join(repo, "pkg/db"), []byte("not a directory\n"), 0o644)).To(Succeed())

			report := scanGoBaseline(repo, "--worktree")

			Expect(diagnosticsAt(report, "pkg/db/db.go")).To(BeEmpty(),
				"the directory is gone, so the file under it is removed; it is not a file that failed to read")
		})
	})

	When("a changed file cannot be read because its permissions deny it", func() {
		It("names the cause in the diagnostic", func() {
			if os.Geteuid() == 0 {
				Skip("root reads files regardless of their mode")
			}
			repo := goOverlayRepo()
			overwriteOnDisk(repo, "pkg/handlers/handlers.go", handlersWithoutImport+"\n// edited\n")
			path := filepath.Join(repo, "pkg/handlers/handlers.go")
			Expect(os.Chmod(path, 0o000)).To(Succeed())
			DeferCleanup(os.Chmod, path, os.FileMode(0o644))

			report := scanGoBaseline(repo, "--worktree")

			diagnostics := diagnosticsAt(report, "pkg/handlers/handlers.go")
			Expect(diagnostics).To(HaveLen(1))
			Expect(diagnostics[0].Kind).To(Equal("worktree_changes_not_analyzed"))
			Expect(diagnostics[0].Message).To(ContainSubstring("permission denied"))
		})
	})

	When("a changed file cannot be read because it is not a regular file", func() {
		It("names the cause in the diagnostic without opening the file", func() {
			repo := goOverlayRepo()
			path := filepath.Join(repo, "pkg/handlers/handlers.go")
			Expect(os.Remove(path)).To(Succeed())
			Expect(syscall.Mkfifo(path, 0o644)).To(Succeed())

			report := scanGoBaseline(repo, "--worktree")

			diagnostics := diagnosticsAt(report, "pkg/handlers/handlers.go")
			Expect(diagnostics).To(HaveLen(1))
			Expect(diagnostics[0].Kind).To(Equal("worktree_changes_not_analyzed"))
			Expect(diagnostics[0].Message).To(ContainSubstring("could not be read (not a regular file)"))
		})
	})

	When("a changed file is larger than the overlay will read", func() {
		It("names the cause in the diagnostic", func() {
			repo := goOverlayRepo()
			path := filepath.Join(repo, "pkg/handlers/handlers.go")
			Expect(os.Truncate(path, 32<<20+1)).To(Succeed())

			report := scanGoBaseline(repo, "--worktree")

			diagnostics := diagnosticsAt(report, "pkg/handlers/handlers.go")
			Expect(diagnostics).To(HaveLen(1))
			Expect(diagnostics[0].Kind).To(Equal("worktree_changes_not_analyzed"))
			Expect(diagnostics[0].Message).To(ContainSubstring("could not be read (over the size limit)"))
		})
	})
})
