//go:build unix

package gitrepo

import (
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

// cloneLackingHeadObjects is a --filter=blob:none clone made without a checkout, so HEAD's blobs are not here,
// whose remote could not answer, with its branch held as a loose ref.
func cloneLackingHeadObjects() (clone, branchRef string) {
	t := GinkgoT()
	source := gitfixture.Init(t)
	gitfixture.CommitFile(t, source, "a.go", "package a\n")
	bare := filepath.Join(GinkgoT().TempDir(), "bare.git")
	gitfixture.Git(t, source, "clone", "--bare", "--quiet", source, bare)
	gitfixture.Git(t, bare, "config", "uploadpack.allowFilter", "true")
	clone = filepath.Join(GinkgoT().TempDir(), "part")
	gitfixture.Git(t, GinkgoT().TempDir(), "clone", "--quiet", "--filter=blob:none", "--no-checkout", "file://"+bare, clone)
	gitfixture.Git(t, clone, "remote", "set-url", "origin", "ssh://example.invalid/repo.git")
	branch := strings.TrimSpace(gitfixture.Git(t, clone, "symbolic-ref", "HEAD"))
	gitfixture.Git(t, clone, "update-ref", branch, "HEAD")
	return clone, filepath.Join(clone, ".git", filepath.FromSlash(branch))
}

// transportMarker makes the clone's remote transport a program that leaves a marker, for the spec's duration.
func transportMarker() string {
	scripts := GinkgoT().TempDir()
	marker := filepath.Join(scripts, "ssh-ran")
	program := filepath.Join(scripts, "fakessh")
	Expect(os.WriteFile(program, []byte("#!/bin/sh\ntouch '"+marker+"'\nexit 1\n"), 0o755)).To(Succeed())
	GinkgoT().Setenv("GIT_SSH_COMMAND", program)
	return marker
}

// objectFilesIn counts the loose and packed object files of a repository.
func objectFilesIn(repository string) int {
	count := 0
	_ = filepath.WalkDir(filepath.Join(repository, ".git", "objects"), func(_ string, entry os.DirEntry, err error) error {
		if err == nil && !entry.IsDir() {
			count++
		}
		return nil
	})
	return count
}

// pipeInPlaceOf replaces the loose ref file with a named pipe, so a git that reads the ref blocks opening it.
// It returns what the ref held.
func pipeInPlaceOf(ref string) string {
	held, err := os.ReadFile(ref)
	Expect(err).NotTo(HaveOccurred())
	Expect(os.Remove(ref)).To(Succeed())
	Expect(syscall.Mkfifo(ref, 0o644)).To(Succeed())
	return string(held)
}

// answerSlowly writes content to the pipe each time a reader opens it, after delay, until the spec ends.
func answerSlowly(pipe, content string, delay time.Duration) {
	var done atomic.Bool
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		for !done.Load() {
			writer, err := os.OpenFile(pipe, os.O_WRONLY, 0)
			if err != nil {
				return
			}
			time.Sleep(delay)
			_, _ = writer.WriteString(content)
			_ = writer.Close()
			// git reads the ref to its end, which it sees only while no writer has the pipe open.
			time.Sleep(200 * time.Millisecond)
		}
	}()
	DeferCleanup(func() {
		done.Store(true)
		if unblock, err := os.OpenFile(pipe, os.O_RDONLY|syscall.O_NONBLOCK, 0); err == nil {
			<-finished
			_ = unblock.Close()
		}
	})
}

// guardWithin runs the old-git branch of the guard, and fails the spec if it has not returned by limit.
func guardWithin(clone string, limit time.Duration) (time.Duration, error) {
	type outcome struct {
		err     error
		elapsed time.Duration
	}
	ended := make(chan outcome, 1)
	start := time.Now()
	go func() {
		err := refuseWhenReadRevisionsLackObjects(gitBefore244, clone, "", olderThanTheInstruction)
		ended <- outcome{err, time.Since(start)}
	}()
	select {
	case result := <-ended:
		return result.elapsed, result.err
	case <-time.After(limit):
		Fail("the guard had not returned after " + limit.String())
		return limit, nil
	}
}

var _ = Describe("on a git before 2.44, a branch ref that git has to wait for (#737)", func() {
	BeforeEach(func() {
		if inherited, set := os.LookupEnv("GIT_NO_LAZY_FETCH"); set {
			DeferCleanup(os.Setenv, "GIT_NO_LAZY_FETCH", inherited)
			Expect(os.Unsetenv("GIT_NO_LAZY_FETCH")).To(Succeed())
		}
	})

	It("#737: still refuses a clone that lacks objects of HEAD when the ref answers slowly, within the time HEAD is resolved in", func() {
		clone, ref := cloneLackingHeadObjects()
		answerSlowly(ref, pipeInPlaceOf(ref), 2*time.Second)

		_, err := guardWithin(clone, time.Minute)

		Expect(refusalText(err)).To(ContainSubstring("HEAD has 1 object that is not in it"), "the probe must still run: %v", err)
	})

	It("#737: still probes a ref that answers after 5 s and well inside the 30 s a read of HEAD is given, and refuses the clone without running the transport or writing an object", func() {
		clone, ref := cloneLackingHeadObjects()
		marker := transportMarker()
		before := objectFilesIn(clone)
		answerSlowly(ref, pipeInPlaceOf(ref), 8*time.Second)

		_, err := guardWithin(clone, 2*time.Minute)

		Expect(refusalText(err)).To(ContainSubstring("HEAD has 1 object that is not in it"), "a ref that answers is probed, however slowly: %v", err)
		Expect(marker).NotTo(BeAnExistingFile(), "the remote's transport ran")
		Expect(objectFilesIn(clone)).To(Equal(before), "objects were written")
	})

	It("#737: refuses a ref that never answers, naming the question, once the 30 s a read of HEAD is given are up, instead of waiting out the probe", func() {
		clone, ref := cloneLackingHeadObjects()
		pipeInPlaceOf(ref)

		elapsed, err := guardWithin(clone, time.Minute)

		Expect(refusalText(err)).To(MatchRegexp(`git rev-parse: .*timed out after 30s`), "%v", err)
		Expect(elapsed).To(BeNumerically("<", 45*time.Second), "a read of HEAD is held to 30 s, not the probe's two minutes")
	})
})
