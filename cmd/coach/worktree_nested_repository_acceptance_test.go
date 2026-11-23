package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --worktree discloses another repository's files under --worktree", func() {
	baseRepo := func() string {
		repo := newTempGitRepo()
		commitFile(repo, "go.mod", goModuleFile)
		commitFile(repo, "pkg/handlers/handlers.go", handlersWithoutImport)
		return repo
	}
	expectDisclosedWithoutQualifying := func(repo, reason, root, label string) {
		run := runWorktreeScan(repo, nil)
		entry := run.only(reason)
		ExpectWithOffset(1, entry).To(HaveKeyWithValue("root", root))
		ExpectWithOffset(1, entry).To(HaveKeyWithValue("hides_in_scope", "no"))
		ExpectWithOffset(1, pathOf(entry)).To(BeEmpty())
		ExpectWithOffset(1, run.exitCode).To(Equal(0))

		text, _, textExit := runCoachCodesignalBaselineRaw(repo, "--worktree")
		ExpectWithOffset(1, textExit).To(Equal(0))
		ExpectWithOffset(1, noFindingsHeadline(string(text))).To(Equal(unqualifiedAllClear))
		ExpectWithOffset(1, string(text)).To(MatchRegexp(hiddenLine(label)))
		ExpectWithOffset(1, string(text)).To(ContainSubstring(root + "/"))
	}

	When("an untracked directory is a nested Git repository", func() {
		It("discloses the nested repository without qualifying the verdict or exiting 3 (#168 item 22)", func() {
			repo := baseRepo()
			nested := filepath.Join(repo, "nest")
			Expect(os.MkdirAll(nested, 0o755)).To(Succeed())
			runGitInRepo(nested, "init", "-q")
			Expect(os.WriteFile(filepath.Join(nested, "n.go"), []byte(modelFileWithTwoConstructors), 0o644)).To(Succeed())

			expectDisclosedWithoutQualifying(repo, "nested_repository", "nest", "nested repository")
		})
	})

	When("a submodule has working-tree changes", func() {
		It("discloses the dirty submodule under the reason the installed git gives, without qualifying the verdict (#168 item 22)", func() {
			repo := baseRepo()
			library := newTempGitRepo()
			commitFile(library, "lib.go", "package lib\n")
			runGitInRepo(repo, "-c", "protocol.file.allow=always", "submodule", "add", "-q", library, "third/lib")
			runGitInRepo(repo, "commit", "-q", "-m", "add the submodule")
			Expect(os.WriteFile(filepath.Join(repo, "third/lib/lib.go"), []byte(modelFileWithTwoConstructors), 0o644)).To(Succeed())

			expectDisclosedWithoutQualifying(repo, submoduleReasonOnInstalledGit(), "third/lib", `(?:submodule|git \d+\.\d+)`)
		})

		It("does not qualify a submodule whose directory was deleted (control, #168 item 22)", func() {
			repo := baseRepo()
			library := newTempGitRepo()
			commitFile(library, "lib.go", "package lib\n")
			runGitInRepo(repo, "-c", "protocol.file.allow=always", "submodule", "add", "-q", library, "third/lib")
			runGitInRepo(repo, "commit", "-q", "-m", "add the submodule")
			Expect(os.RemoveAll(filepath.Join(repo, "third/lib"))).To(Succeed())
			expectDirtyPorcelain(repo, " D third/lib")

			run := runWorktreeScan(repo, nil)

			Expect(run.qualifying()).To(BeEmpty(), "a submodule that is gone has no working-tree changes to leave unread")
			Expect(run.exitCode).To(Equal(0))
			if installedGitCanBeToldNotToFetch() {
				Expect(run.entries).To(BeEmpty())
			}
		})

		It("does not qualify a clean submodule (control, #168 item 22)", func() {
			repo := baseRepo()
			library := newTempGitRepo()
			commitFile(library, "lib.go", "package lib\n")
			runGitInRepo(repo, "-c", "protocol.file.allow=always", "submodule", "add", "-q", library, "third/lib")
			runGitInRepo(repo, "commit", "-q", "-m", "add the submodule")

			run := runWorktreeScan(repo, nil)

			Expect(run.qualifying()).To(BeEmpty())
			Expect(run.exitCode).To(Equal(0))
			if installedGitCanBeToldNotToFetch() {
				Expect(run.entries).To(BeEmpty())
			}
		})
	})
})
