package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	spellingCause       = "its name on disk differs from the name Git records"
	composedCafeHandler = "pkg/handlers/café.go"
	decomposedCafeName  = "pkg/handlers/café.go"
)

// Each case needs a filesystem that resolves two spellings to one file: letter
// case for the first, canonically equivalent Unicode for the others (APFS and
// HFS+ do both). Elsewhere the spelling Git records is the spelling on disk and
// there is nothing to disclose, so the specs skip. The portable pins are the
// TestOverlay*SpelledDifferently* unit tests, which use a fake disk.
func skipUnlessFilesystemResolves(written, read string) {
	probe, err := os.MkdirTemp("", "coach-acceptance-names-*")
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, probe)
	ExpectWithOffset(1, os.WriteFile(filepath.Join(probe, written), nil, 0o644)).To(Succeed())
	if _, err := os.Lstat(filepath.Join(probe, read)); err != nil {
		Skip("this filesystem does not resolve " + read + " to " + written + "; the unit pin covers it with a fake disk")
	}
}

var _ = Describe("coach codesignal --worktree for a path spelled on disk differently from the name Git records", func() {
	scanFailingOnIncomplete := func(repo string) (report []byte, stderr []byte, exitCode int) {
		return runCoachCodesignalBaselineRaw(repo,
			"--project-config", "project.json", "--format=json", "--worktree", "--fail-on-incomplete-coverage")
	}

	expectNamedAsUnread := func(repo, path string) {
		stdout, stderr, exitCode := scanFailingOnIncomplete(repo)

		Expect(exitCode).To(Equal(3), "stderr: %s stdout: %s", stderr, stdout)
		named := diagnosticsOfKind(decodeCoachReport(stdout).Diagnostics, "worktree_changes_not_analyzed")
		Expect(named).To(HaveLen(1), "the dirty path is the only one the scan could not read")
		Expect(named[0].Path).To(Equal(path))
		Expect(named[0].Message).To(ContainSubstring(spellingCause))
	}

	When("a tracked file was renamed in letter case only, without staging, and then edited", func() {
		It("names the path and exits 3 instead of printing an all-clear", func() {
			skipUnlessFilesystemResolves("probe.go", "PROBE.GO")
			repo := goOverlayRepo()
			Expect(os.Rename(filepath.Join(repo, "pkg/handlers/handlers.go"), filepath.Join(repo, "pkg/handlers/Handlers.go"))).To(Succeed())
			overwriteOnDisk(repo, "pkg/handlers/Handlers.go", handlersImportingDB+"\nfunc Extra() {}\n")
			expectDirtyPorcelain(repo, " M pkg/handlers/handlers.go")

			expectNamedAsUnread(repo, "pkg/handlers/handlers.go")
		})
	})

	When("a tracked file was renamed in letter case only inside a directory renamed the same way, then edited", func() {
		It("names the path and exits 3 instead of printing an all-clear", func() {
			skipUnlessFilesystemResolves("probe.go", "PROBE.GO")
			repo := goOverlayRepo()
			Expect(os.Rename(filepath.Join(repo, "pkg/handlers"), filepath.Join(repo, "pkg/Handlers"))).To(Succeed())
			overwriteOnDisk(repo, "pkg/Handlers/handlers.go", handlersImportingDB+"\nfunc Extra() {}\n")
			expectDirtyPorcelain(repo, " M pkg/handlers/handlers.go")

			expectNamedAsUnread(repo, "pkg/handlers/handlers.go")
		})
	})

	When("a new file's name is stored decomposed on disk and Git records it composed", func() {
		It("names the untracked path and exits 3 instead of printing an all-clear", func() {
			skipUnlessFilesystemResolves("café.go", "café.go")
			repo := goOverlayRepo()
			overwriteOnDisk(repo, decomposedCafeName, handlersImportingDB)
			expectDirtyPorcelain(repo, "??")

			expectNamedAsUnread(repo, composedCafeHandler)
		})
	})

	When("a tracked file with such a name is edited", func() {
		It("names the path and exits 3 instead of printing an all-clear", func() {
			skipUnlessFilesystemResolves("café.go", "café.go")
			repo := goOverlayRepo()
			commitFile(repo, decomposedCafeName, handlersImportingDB)
			overwriteOnDisk(repo, decomposedCafeName, handlersImportingDB+"\nfunc Extra() {}\n")
			expectDirtyPorcelain(repo, " M ")

			expectNamedAsUnread(repo, composedCafeHandler)
		})
	})
})
