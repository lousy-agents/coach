package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --worktree for a path whose kind changed since the commit", func() {
	const extraFile = "pkg/handlers/extra/extra.go"

	When("a tracked file became a directory that holds a new Go file importing a forbidden layer", func() {
		kindChangedRepo := func() string {
			repo := goOverlayRepo()
			commitFile(repo, "pkg/handlers/extra", "a plain file, not Go source\n")
			Expect(os.Remove(filepath.Join(repo, "pkg/handlers/extra"))).To(Succeed())
			overwriteOnDiskCreatingParents(repo, extraFile, handlersImportingDB)
			return repo
		}

		It("sees the new file in the model, so both layer violations are reported and the exit is clean", func() {
			repo := kindChangedRepo()
			expectDirtyPorcelain(repo, "?? pkg/handlers/extra/extra.go")

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo,
				"--project-config", "project.json", "--format=json", "--worktree", "--fail-on-incomplete-coverage")

			report := decodeCoachReport(stdout)
			Expect(layerViolationCount(report)).To(Equal(2),
				"handlers.go and extra/extra.go both import db; one means the new file under the replaced path was never read")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(diagnosticsOfKind(report.Diagnostics, "worktree_changes_not_analyzed")).To(BeEmpty())
		})

		It("reports what the committed tree reports once the same change is committed (control)", func() {
			repo := kindChangedRepo()
			runGitInRepo(repo, "add", "-A")
			runGitInRepo(repo, "commit", "-q", "-m", "replace the file with a directory")

			Expect(layerViolationCount(scanGoBaseline(repo))).To(Equal(2))
		})
	})

	When("a tracked directory became a file", func() {
		It("drops the files that were under it instead of reading them from the commit", func() {
			repo := goOverlayRepo()
			commitFile(repo, extraFile, handlersImportingDB)
			Expect(os.RemoveAll(filepath.Join(repo, "pkg/handlers/extra"))).To(Succeed())
			overwriteOnDisk(repo, "pkg/handlers/extra", "a plain file, not Go source\n")

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo,
				"--project-config", "project.json", "--format=json", "--worktree", "--fail-on-incomplete-coverage")

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(layerViolationCount(decodeCoachReport(stdout))).To(Equal(1),
				"only handlers.go remains; two means the deleted directory's committed file was still served")
		})
	})
})

func overwriteOnDiskCreatingParents(repo, name, contents string) {
	ExpectWithOffset(1, os.MkdirAll(filepath.Dir(filepath.Join(repo, name)), 0o755)).To(Succeed())
	overwriteOnDisk(repo, name, contents)
}
