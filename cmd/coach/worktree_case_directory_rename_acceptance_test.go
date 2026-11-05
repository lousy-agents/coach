package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Like the file-level case rename spec beside it, this can only fail on a
// case-insensitive filesystem, where the old directory spelling still resolves
// on disk. The portable pin is
// TestOverlayTreatsTheOldSpellingOfACaseOnlyDirectoryRenameAsRemoved, which uses
// a fake case-insensitive disk.
var _ = Describe("coach codesignal --worktree treats a case-only directory rename as one removed path and one added path", func() {
	It("reports each file-local signal once, at the new directory name", func() {
		repo := newTempGitRepo()
		commitFile(repo, "go.mod", goModuleFile)
		commitFile(repo, "Pkg/a.go", nestedIfs("A", 6))
		runGitInRepo(repo, "mv", "Pkg", "pkg_tmp")
		runGitInRepo(repo, "mv", "pkg_tmp", "pkg")

		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--worktree", "--format=json")

		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		report := decodeCoachReport(stdout)
		Expect(diagnosticsOfKind(report.Diagnostics, "worktree_changes_not_analyzed")).To(BeEmpty(),
			"the old directory spelling is the rename's source, not a dirty file the scan failed to read")
		paths := map[string]int{}
		rules := map[string]int{}
		for _, signal := range report.Signals {
			paths[signal.Path]++
			rules[signal.RuleID]++
		}
		Expect(paths).To(HaveLen(1), "signals must all be anchored at one path")
		Expect(paths).To(HaveKey("pkg/a.go"), "a signal at Pkg/a.go points at a path that no longer exists")
		Expect(rules).NotTo(BeEmpty())
		Expect(rules).To(HaveEach(1), "a rule reported twice means the old spelling was read from the renamed directory: %v", rules)
	})
})
