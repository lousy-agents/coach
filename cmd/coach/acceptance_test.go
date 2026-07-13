package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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
})
