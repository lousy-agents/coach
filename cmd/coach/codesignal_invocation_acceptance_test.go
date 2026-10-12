package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

var _ = Describe("coach codesignal", func() {
	When("--base is not provided", func() {
		It("prints usage guidance to stderr and exits 2 without writing to stdout", func() {
			repo := newTempGitRepo()

			command := exec.Command(commandPath, "codesignal")
			command.Dir = repo
			var stdout, stderr bytes.Buffer
			command.Stdout = &stdout
			command.Stderr = &stderr

			err := command.Run()

			var exitErr *exec.ExitError
			Expect(err).To(HaveOccurred())
			Expect(errors.As(err, &exitErr)).To(BeTrue())
			Expect(exitErr.ExitCode()).To(Equal(2))
			Expect(stdout.Bytes()).To(BeEmpty())
			Expect(stderr.String()).NotTo(BeEmpty())
			Expect(stderr.String()).To(ContainSubstring("--base"))
		})
	})

	When("run in a directory that is not a Git worktree", func() {
		It("exits 1, writes an actionable message to stderr, and writes nothing to stdout", func() {
			directory, err := os.MkdirTemp("", "coach-acceptance-notgit-*")
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(os.RemoveAll, directory)

			command := exec.Command(commandPath, "codesignal", "--base", "HEAD")
			command.Dir = directory
			var stdout, stderr bytes.Buffer
			command.Stdout = &stdout
			command.Stderr = &stderr

			err = command.Run()

			var exitErr *exec.ExitError
			Expect(err).To(HaveOccurred())
			Expect(errors.As(err, &exitErr)).To(BeTrue())
			Expect(exitErr.ExitCode()).To(Equal(1))
			Expect(stdout.Bytes()).To(BeEmpty())
			Expect(stderr.String()).NotTo(BeEmpty())
		})
	})

	When("--base cannot be resolved to a commit", func() {
		It("exits 1, writes an actionable message to stderr, and writes nothing to stdout", func() {
			repo := newTempGitRepo()
			commitFile(repo, "a.go", "package a\n")

			command := exec.Command(commandPath, "codesignal", "--base", "doesnotexist12345")
			command.Dir = repo
			var stdout, stderr bytes.Buffer
			command.Stdout = &stdout
			command.Stderr = &stderr

			err := command.Run()

			var exitErr *exec.ExitError
			Expect(err).To(HaveOccurred())
			Expect(errors.As(err, &exitErr)).To(BeTrue())
			Expect(exitErr.ExitCode()).To(Equal(1))
			Expect(stdout.Bytes()).To(BeEmpty())
			Expect(stderr.String()).NotTo(BeEmpty())
		})
	})

	When("HEAD cannot be read because a worktree has no commits", func() {
		It("exits 1, writes one actionable operational error to stderr, and writes nothing to stdout", func() {
			repo := newTempGitRepo()

			command := exec.Command(commandPath, "codesignal", "--base", "HEAD")
			command.Dir = repo
			var stdout, stderr bytes.Buffer
			command.Stdout = &stdout
			command.Stderr = &stderr

			err := command.Run()
			var exitErr *exec.ExitError
			Expect(errors.As(err, &exitErr)).To(BeTrue())
			Expect(exitErr.ExitCode()).To(Equal(1))
			Expect(stdout.Bytes()).To(BeEmpty())
			Expect(stderr.String()).To(ContainSubstring("HEAD is not readable"))
		})
	})

	When("--scope is neither production nor all", func() {
		It("prints scope-specific usage guidance to stderr and exits 2 without a report", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "a.go", "package a\n")

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, initialSHA, "--scope=review")

			Expect(exitCode).To(Equal(2))
			Expect(stdout).To(BeEmpty())
			Expect(string(stderr)).To(ContainSubstring("--scope"))
			Expect(string(stderr)).To(ContainSubstring("production"))
			Expect(string(stderr)).To(ContainSubstring("all"))
		})
	})

	When("--format is an unrecognized value", func() {
		It("exits 2, prints usage guidance to stderr, and writes nothing to stdout", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "a.go", "package a\n\nfunc A() {}\n")

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, initialSHA, "--format=xml")

			Expect(exitCode).To(Equal(2))
			Expect(stdout).To(BeEmpty())
			Expect(stderr).NotTo(BeEmpty())
		})
	})

	When("run with no GitHub token, no model/LLM API key, and no other external service config", func() {
		It("still exits 0 with a valid report, proving zero external service configuration is required", func() {
			repo := newTempGitRepo()
			base := "package a\n\nfunc Get(input *int) int {\n\treturn *input\n}\n"
			head := base + "\nfunc Update(input *int) {\n\t*input = 1\n}\n"
			initialSHA := commitFile(repo, "a.go", base)
			commitFile(repo, "a.go", head)

			command := exec.Command(commandPath, "codesignal", "--base", initialSHA, "--format=json")
			command.Dir = repo
			command.Env = []string{
				"PATH=" + os.Getenv("PATH"),
				"HOME=" + os.Getenv("HOME"),
				"GIT_AUTHOR_NAME=coach-acceptance",
				"GIT_AUTHOR_EMAIL=coach-acceptance@example.com",
				"GIT_COMMITTER_NAME=coach-acceptance",
				"GIT_COMMITTER_EMAIL=coach-acceptance@example.com",
			}
			var stdout, stderr bytes.Buffer
			command.Stdout = &stdout
			command.Stderr = &stderr

			err := command.Run()
			Expect(err).NotTo(HaveOccurred(), "stderr: %s", stderr.String())

			var report codesignal.Report
			Expect(json.Unmarshal(stdout.Bytes(), &report)).To(Succeed(), "stdout should be one JSON report: %s", stdout.String())

			signals := signalsForPath(&report, "a.go")
			Expect(signals).To(HaveLen(1))
			Expect(signals[0].Lifecycle).To(Equal(codesignal.Lifecycle("introduced")))
		})
	})

	When("the git executable is unavailable", func() {
		It("exits 1 with one operational error on stderr and no report on stdout", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "a.go", "package a\n")

			command := exec.Command(commandPath, "codesignal", "--base", initialSHA)
			command.Dir = repo
			command.Env = []string{"PATH=", "HOME=" + os.Getenv("HOME")}
			var stdout, stderr bytes.Buffer
			command.Stdout = &stdout
			command.Stderr = &stderr

			err := command.Run()
			var exitErr *exec.ExitError
			Expect(errors.As(err, &exitErr)).To(BeTrue())
			Expect(exitErr.ExitCode()).To(Equal(1))
			Expect(stdout.Bytes()).To(BeEmpty())
			Expect(stderr.String()).To(ContainSubstring("git executable not found"))
		})
	})
})
