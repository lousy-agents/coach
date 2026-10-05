package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

var _ = Describe("coach codesignal", func() {
	When("--base resolves and there are commits since it", func() {
		It("exits 0", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "a.go", "package a\n")
			commitFile(repo, "b.go", "package a\n\nfunc B() {}\n")

			command := exec.Command(commandPath, "codesignal", "--base", initialSHA)
			command.Dir = repo
			var stdout, stderr bytes.Buffer
			command.Stdout = &stdout
			command.Stderr = &stderr

			err := command.Run()

			Expect(err).NotTo(HaveOccurred(), "stderr: %s", stderr.String())
		})
	})

	When("the comparison contains no supported changed files", func() {
		It("exits 0 and qualifies the no-findings verdict as incomplete because of the unsupported_language diagnostic", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "notes.txt", "before\n")
			commitFile(repo, "notes.txt", "after\n")

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, initialSHA)

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(string(stdout)).To(ContainSubstring("No active CodeSignal findings, but the analysis is incomplete"))
			Expect(string(stdout)).To(ContainSubstring("unsupported_language"))
		})
	})

	When("the working tree has uncommitted source changes", func() {
		It("analyzes only merge-base through HEAD and excludes the uncommitted changes", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "a.go", "package a\n\nfunc A() {}\n")
			commitFile(repo, "a.go", "package a\n\nfunc A() {}\n\n// committed context\n")
			Expect(os.WriteFile(filepath.Join(repo, "a.go"), []byte("package a\n\nfunc Update(input *int) {\n\t*input = 1\n}\n"), 0o644)).To(Succeed())

			report, _ := runCoachCodesignal(repo, initialSHA)

			Expect(report.Signals).To(BeEmpty(), "uncommitted changes must not be included in the revision comparison")
		})
	})

	When("one selected file fails analysis alongside a healthy file that introduces a signal", func() {
		It("exits 0, continues analyzing the healthy file, and reports a diagnostic for the failed one", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "healthy.go", "package a\n\nfunc Get(input *int) int { return *input }\n")
			commitFile(repo, "healthy.go", "package a\n\nfunc Update(input *int) {\n\t*input = 1\n}\n")
			commitFile(repo, "empty.go", "")

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, initialSHA, "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var report codesignal.Report
			Expect(json.Unmarshal(stdout, &report)).To(Succeed())

			Expect(hasDiagnostic(&report, "empty_content", "empty.go")).To(BeTrue())
			signals := signalsForPath(&report, "healthy.go")
			Expect(signals).To(HaveLen(1))
			Expect(signals[0].Lifecycle).To(Equal(codesignal.Lifecycle("introduced")))
		})
	})

	When("a selected Go file contains binary bytes", func() {
		It("records a binary_content diagnostic, prints a report, and exits 0", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "a.go", "package a\n")
			commitFile(repo, "binary.go", "package binary\x00")

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, initialSHA, "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			var report codesignal.Report
			Expect(json.Unmarshal(stdout, &report)).To(Succeed())
			Expect(hasDiagnostic(&report, "binary_content", "binary.go")).To(BeTrue())
		})
	})

	When("a supported path contains spaces, quotes, a newline, and non-ASCII bytes", func() {
		It("preserves the exact Git path in the emitted report", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "seed.go", "package seed\n")
			path := "space quote\" newline\n日本語.go"
			commitFile(repo, path, "package weird\n\nfunc Update(input *int) {\n\t*input = 1\n}\n")

			report, _ := runCoachCodesignal(repo, initialSHA)

			signals := signalsForPath(report, path)
			Expect(signals).To(HaveLen(1))
			Expect(signals[0].Path).To(Equal(path))
		})
	})

	When("the same two commits are analyzed in two independent worktrees", func() {
		It("produces byte-identical JSON output", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "a.go", "package a\n\nfunc Get(input *int) int {\n\treturn *input\n}\n")
			commitFile(repo, "a.go", "package a\n\nfunc Get(input *int) int {\n\treturn *input\n}\n\nfunc Update(input *int) {\n\t*input = 1\n}\n")

			firstRun, stderr1, exitCode1 := runCoachCodesignalRaw(repo, initialSHA, "--format=json")
			Expect(exitCode1).To(Equal(0), "stderr: %s", stderr1)

			worktreeParent, err := os.MkdirTemp("", "coach-acceptance-worktree-*")
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(os.RemoveAll, worktreeParent)
			worktree := filepath.Join(worktreeParent, "second-worktree")
			addWorktree := exec.Command("git", "worktree", "add", "--detach", worktree, "HEAD")
			addWorktree.Dir = repo
			output, err := addWorktree.CombinedOutput()
			Expect(err).NotTo(HaveOccurred(), "git worktree add: %s", output)

			secondRun, stderr2, exitCode2 := runCoachCodesignalRaw(worktree, initialSHA, "--format=json")
			Expect(exitCode2).To(Equal(0), "stderr: %s", stderr2)

			Expect(firstRun).To(Equal(secondRun))
		})
	})
})
