package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --worktree says what it read and on which revision (PRD: counts and revision)", func() {
	changedRepo := func() (repo string) {
		repo = newTempGitRepo()
		commitFile(repo, ".gitignore", "*.log\n")
		commitFile(repo, "go.mod", goModuleFile)
		commitFile(repo, "pkg/mod.go", "package pkg\n")
		commitFile(repo, "pkg/del.go", "package pkg\n")
		commitFile(repo, "pkg/ren_old.go", "package pkg\n")
		write := func(name, contents string) {
			ExpectWithOffset(2, os.WriteFile(filepath.Join(repo, name), []byte(contents), 0o644)).To(Succeed())
		}
		write("pkg/mod.go", "package pkg\n\n// edited\n")
		Expect(os.Remove(filepath.Join(repo, "pkg/del.go"))).To(Succeed())
		runGitInRepo(repo, "mv", "pkg/ren_old.go", "pkg/ren_new.go")
		write("pkg/staged.go", "package pkg\n")
		runGitInRepo(repo, "add", "pkg/staged.go")
		write("pkg/fresh.go", "package pkg\n")
		write("notes.txt", "not a source file\n")
		write("debug.log", "ignored\n")
		return repo
	}
	scopeOf := func(stdout []byte) map[string]any {
		var report struct {
			Scope map[string]any `json:"scope"`
		}
		ExpectWithOffset(1, json.Unmarshal(stdout, &report)).To(Succeed())
		return report.Scope
	}

	It("counts distinct modified, added, deleted and untracked paths, all languages, ignored paths excluded, and names the revision", func() {
		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(changedRepo(), "--worktree", "--format=json")

		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		scope := scopeOf(stdout)
		worktree, _ := scope["worktree"].(map[string]any)
		Expect(worktree).To(HaveKeyWithValue("revision", scope["revision"]))
		Expect(worktree).To(HaveKeyWithValue("modified", float64(1)), "pkg/mod.go")
		Expect(worktree).To(HaveKeyWithValue("added", float64(2)), "pkg/staged.go and the new path of the rename")
		Expect(worktree).To(HaveKeyWithValue("deleted", float64(2)), "pkg/del.go and the old path of the rename")
		Expect(worktree).To(HaveKeyWithValue("untracked", float64(2)), "pkg/fresh.go and notes.txt; debug.log is ignored")
	})

	It("reports zero counts as real zeros, not as absent keys, when nothing changed", func() {
		repo := newTempGitRepo()
		commitFile(repo, "go.mod", goModuleFile)
		commitFile(repo, "pkg/a.go", "package pkg\n")

		stdout, _, exitCode := runCoachCodesignalBaselineRaw(repo, "--worktree", "--format=json")

		Expect(exitCode).To(Equal(0))
		Expect(string(stdout)).To(ContainSubstring(`"modified":0,"added":0,"deleted":0,"untracked":0,"config_files":[]`))
	})

	It("says in a baseline headline that the findings include working-tree changes, and puts the worktree line under it", func() {
		repo := changedRepo()
		sha := strings.TrimSpace(runGitInRepo(repo, "rev-parse", "HEAD"))

		stdout, _, exitCode := runCoachCodesignalBaselineRaw(repo, "--worktree")
		lines := strings.Split(string(stdout), "\n")

		Expect(exitCode).To(Equal(0))
		Expect(lines[0]).To(Equal("Repository Baseline for revision " + sha + " with working-tree changes (not a diff comparison)"))
		Expect(lines[1]).To(Equal("worktree: 1 modified, 2 added, 2 deleted, 2 untracked on revision " + sha[:12] + "."))
	})

	It("names the revision in the baseline headline as the committed headline does, in full", func() {
		committed := newTempGitRepo()
		commitFile(committed, "go.mod", goModuleFile)
		commitFile(committed, "pkg/a.go", "package pkg\n")
		headline := func(stdout []byte) string { return strings.SplitN(string(stdout), "\n", 2)[0] }

		worktreeStdout, _, _ := runCoachCodesignalBaselineRaw(changedRepo(), "--worktree")
		committedStdout, _, _ := runCoachCodesignalBaselineRaw(committed)

		Expect(headline(worktreeStdout)).To(MatchRegexp(`^Repository Baseline for revision [0-9a-f]{40} with working-tree changes \(not a diff comparison\)$`))
		Expect(headline(committedStdout)).To(MatchRegexp(`^Repository Baseline for revision [0-9a-f]{40} \(not a diff comparison\)$`))
	})

	It("says there are no uncommitted changes when there are none", func() {
		repo := newTempGitRepo()
		commitFile(repo, "go.mod", goModuleFile)
		commitFile(repo, "pkg/a.go", "package pkg\n")
		sha := strings.TrimSpace(runGitInRepo(repo, "rev-parse", "HEAD"))[:12]

		stdout, _, _ := runCoachCodesignalBaselineRaw(repo, "--worktree")

		Expect(strings.Split(string(stdout), "\n")[1]).To(Equal("worktree: no uncommitted changes on revision " + sha + "."))
	})

	It("names the merge base in a diff scan and puts the worktree line directly under the summary line", func() {
		repo := newTempGitRepo()
		commitFile(repo, "go.mod", goModuleFile)
		base := commitFile(repo, "pkg/a.go", "package pkg\n")
		commitFile(repo, "pkg/b.go", "package pkg\n")
		Expect(os.WriteFile(filepath.Join(repo, "pkg/a.go"), []byte("package pkg\n\n// edited\n"), 0o644)).To(Succeed())
		head := strings.TrimSpace(runGitInRepo(repo, "rev-parse", "HEAD"))

		stdout, _, exitCode := runCoachCodesignalRaw(repo, base, "--worktree")
		lines := strings.Split(string(stdout), "\n")

		Expect(exitCode).To(Equal(0))
		Expect(lines[1]).To(Equal("worktree: 1 modified, 0 added, 0 deleted, 0 untracked on revision " + head[:12] + ", compared with merge-base " + base[:12] + "."))
	})

	It("lists the configuration files it read from disk after the counts", func() {
		repo := goOverlayRepo()
		overwriteOnDisk(repo, "go.mod", "module example.com/app\n\ngo 1.25\n// edited\n")

		stdout, _, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--worktree")

		Expect(exitCode).To(Equal(0))
		Expect(strings.Split(string(stdout), "\n")[1]).To(MatchRegexp(`^worktree: 1 modified, 0 added, 0 deleted, 0 untracked on revision [0-9a-f]{12}; configuration read from disk: go\.mod\.$`))
	})
	When("the scan counts files it discovered", func() {
		summaryLine := func(stdout []byte, prefix string) string {
			for _, line := range strings.Split(string(stdout), "\n") {
				if strings.Contains(line, prefix) {
					return line
				}
			}
			return ""
		}

		It("keeps the committed-mode label, which names tracked files only (control)", func() {
			stdout, _, exitCode := runCoachCodesignalBaselineRaw(changedRepo(), "--format=json")
			Expect(exitCode).To(Equal(0))
			Expect(decodeCoachReport(stdout).Coverage.TrackedFilesDiscovered).To(BeNumerically(">", 0))

			text, _, textExit := runCoachCodesignalBaselineRaw(changedRepo())

			Expect(textExit).To(Equal(0))
			Expect(summaryLine(text, "discovered:")).To(HavePrefix("tracked files discovered: "))
		})

		It("says that the count includes new files under --worktree, with the number the JSON report carries", func() {
			repo := changedRepo()
			stdout, _, jsonExit := runCoachCodesignalBaselineRaw(repo, "--worktree", "--format=json")
			Expect(jsonExit).To(Equal(0))
			discovered := decodeCoachReport(stdout).Coverage.TrackedFilesDiscovered

			text, _, textExit := runCoachCodesignalBaselineRaw(repo, "--worktree")

			Expect(textExit).To(Equal(0))
			Expect(summaryLine(text, "discovered")).To(HavePrefix(fmt.Sprintf("files discovered (tracked and new): %d, analyzed: ", discovered)))
			Expect(string(text)).NotTo(ContainSubstring("tracked files discovered"))
		})
	})
})
