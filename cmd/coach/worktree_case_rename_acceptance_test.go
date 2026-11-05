package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// This spec can only fail on a case-insensitive filesystem, where the old
// spelling still resolves on disk. On a case-sensitive one, as on CI, it passes
// with or without the exact-spelling check, so the portable pin is the unit test
// TestOverlayTreatsTheOldSpellingOfACaseOnlyRenameAsRemoved, which uses a fake
// case-insensitive disk.
var _ = Describe("coach codesignal --worktree treats a case-only rename as one removed path and one added path", func() {
	It("reports the file once, at its new name, instead of at both names", func() {
		repo := goOverlayRepo()
		runGitInRepo(repo, "mv", "pkg/handlers/handlers.go", "pkg/handlers/handlers_tmp.go")
		runGitInRepo(repo, "mv", "pkg/handlers/handlers_tmp.go", "pkg/handlers/Handlers.go")

		report := scanGoBaseline(repo, "--worktree")

		Expect(report.ProjectCoverage).NotTo(BeNil())
		Expect(report.ProjectCoverage.Counts["files_seen"]).To(Equal(2),
			"db.go and Handlers.go; a second entry means the old name was read from the renamed file")
		Expect(report.ProjectChanges).To(HaveLen(1))
		Expect(report.ProjectChanges[0].PrimaryAnchor.Path).To(Equal("pkg/handlers/Handlers.go"),
			"a finding anchored at the old name points at a path that no longer exists")
		Expect(diagnosticsOfKind(report.Diagnostics, "worktree_changes_not_analyzed")).To(BeEmpty(),
			"the old spelling is the rename's source, not a dirty file the scan failed to read")
	})
})
