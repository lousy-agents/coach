package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --worktree leaves the repository's index and hooks alone", func() {
	// staleIndexRepo commits a file, then moves its modification time so the
	// index entry no longer matches the disk. A git status that may take
	// optional locks refreshes that entry and writes the index, which runs the
	// post-index-change hook the repository installs here.
	staleIndexRepo := func() (repo, hookRuns string) {
		repo = newTempGitRepo()
		commitFile(repo, "go.mod", goModuleFile)
		commitFile(repo, "pkg/handlers/handlers.go", handlersWithoutImport)
		hookRuns = filepath.Join(GinkgoT().TempDir(), "hook-runs")
		hook := filepath.Join(repo, ".git/hooks/post-index-change")
		Expect(os.WriteFile(hook, []byte(fmt.Sprintf("#!/bin/sh\necho ran >> %q\n", hookRuns)), 0o755)).To(Succeed())
		past := time.Now().Add(-time.Hour)
		Expect(os.Chtimes(filepath.Join(repo, "pkg/handlers/handlers.go"), past, past)).To(Succeed())
		return repo, hookRuns
	}
	indexDigest := func(repo string) string {
		bytes, err := os.ReadFile(filepath.Join(repo, ".git/index"))
		ExpectWithOffset(1, err).NotTo(HaveOccurred())
		return fmt.Sprintf("%x", sha256.Sum256(bytes))
	}
	exists := func(path string) bool {
		_, err := os.Stat(path)
		return err == nil
	}

	It("shows the fixture is live: a plain git status rewrites the index and runs the hook (control)", func() {
		repo, hookRuns := staleIndexRepo()
		before := indexDigest(repo)

		runGitInRepo(repo, "status", "--porcelain")

		Expect(exists(hookRuns)).To(BeTrue(), "the post-index-change hook must run for a plain git status")
		Expect(indexDigest(repo)).NotTo(Equal(before), "a plain git status must refresh the stale entry and write the index")
	})

	It("does not run the repository's post-index-change hook", func() {
		repo, hookRuns := staleIndexRepo()

		_, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--worktree", "--format=json")

		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		Expect(exists(hookRuns)).To(BeFalse(), "a repository hook ran during the scan")
	})

	It("does not rewrite the index", func() {
		repo, _ := staleIndexRepo()
		before := indexDigest(repo)

		_, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--worktree", "--format=json")

		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		Expect(indexDigest(repo)).To(Equal(before), "the scan refreshed the index, so it took a lock that git status is not meant to take")
	})
})
