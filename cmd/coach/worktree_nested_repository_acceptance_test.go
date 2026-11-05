package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

var _ = Describe("coach codesignal --worktree does not print an all-clear over a repository it cannot read", func() {
	baseRepo := func() string {
		repo := newTempGitRepo()
		commitFile(repo, "go.mod", goModuleFile)
		commitFile(repo, "pkg/handlers/handlers.go", handlersWithoutImport)
		return repo
	}
	scanIncomplete := func(report *codesignal.Report) []codesignal.Diagnostic {
		return diagnosticsOfKind(report.Diagnostics, "worktree_scan_incomplete")
	}
	expectQualified := func(repo, cause string) {
		jsonOut, stderr, jsonExit := runCoachCodesignalBaselineRaw(repo, "--worktree", "--format=json")
		ExpectWithOffset(1, jsonExit).To(Equal(0), "stderr: %s", stderr)
		diagnostics := scanIncomplete(decodeCoachReport(jsonOut))
		ExpectWithOffset(1, diagnostics).To(HaveLen(1))
		ExpectWithOffset(1, diagnostics[0].Message).To(ContainSubstring(cause))

		text, _, textExit := runCoachCodesignalBaselineRaw(repo, "--worktree")
		ExpectWithOffset(1, textExit).To(Equal(0))
		ExpectWithOffset(1, string(text)).To(ContainSubstring("the analysis is incomplete"))
		ExpectWithOffset(1, noFindingsHeadline(string(text))).NotTo(Equal("No active CodeSignal findings."))

		_, _, failExit := runCoachCodesignalBaselineRaw(repo, "--worktree", "--fail-on-incomplete-coverage")
		ExpectWithOffset(1, failExit).To(Equal(3))
	}

	When("an untracked directory is a nested Git repository", func() {
		It("qualifies the verdict, names the cause and exits 3 under --fail-on-incomplete-coverage", func() {
			repo := baseRepo()
			nested := filepath.Join(repo, "nest")
			Expect(os.MkdirAll(nested, 0o755)).To(Succeed())
			runGitInRepo(nested, "init", "-q")
			Expect(os.WriteFile(filepath.Join(nested, "n.go"), []byte(modelFileWithTwoConstructors), 0o644)).To(Succeed())

			expectQualified(repo, "nested repository")
		})
	})

	When("a submodule has working-tree changes", func() {
		It("qualifies the verdict, names the cause and exits 3 under --fail-on-incomplete-coverage", func() {
			repo := baseRepo()
			library := newTempGitRepo()
			commitFile(library, "lib.go", "package lib\n")
			runGitInRepo(repo, "-c", "protocol.file.allow=always", "submodule", "add", "-q", library, "third/lib")
			runGitInRepo(repo, "commit", "-q", "-m", "add the submodule")
			Expect(os.WriteFile(filepath.Join(repo, "third/lib/lib.go"), []byte(modelFileWithTwoConstructors), 0o644)).To(Succeed())

			expectQualified(repo, "submodule")
		})

		It("does not qualify a submodule whose directory was deleted (control)", func() {
			repo := baseRepo()
			library := newTempGitRepo()
			commitFile(library, "lib.go", "package lib\n")
			runGitInRepo(repo, "-c", "protocol.file.allow=always", "submodule", "add", "-q", library, "third/lib")
			runGitInRepo(repo, "commit", "-q", "-m", "add the submodule")
			Expect(os.RemoveAll(filepath.Join(repo, "third/lib"))).To(Succeed())
			expectDirtyPorcelain(repo, " D third/lib")

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--worktree", "--format=json")
			_, _, failExit := runCoachCodesignalBaselineRaw(repo, "--worktree", "--fail-on-incomplete-coverage")

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(scanIncomplete(decodeCoachReport(stdout))).To(BeEmpty(), "a submodule that is gone has no working-tree changes to leave unread")
			Expect(failExit).To(Equal(0))
		})

		It("does not qualify a clean submodule (control)", func() {
			repo := baseRepo()
			library := newTempGitRepo()
			commitFile(library, "lib.go", "package lib\n")
			runGitInRepo(repo, "-c", "protocol.file.allow=always", "submodule", "add", "-q", library, "third/lib")
			runGitInRepo(repo, "commit", "-q", "-m", "add the submodule")

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--worktree", "--format=json")

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(scanIncomplete(decodeCoachReport(stdout))).To(BeEmpty())
		})
	})
})
