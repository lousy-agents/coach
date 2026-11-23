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
func guardWithin(clone string, limit time.Duration) (error, time.Duration) {
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
		return result.err, result.elapsed
	case <-time.After(limit):
		Fail("the guard had not returned after " + limit.String())
		return nil, limit
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

		err, _ := guardWithin(clone, time.Minute)

		Expect(refusalText(err)).To(ContainSubstring("HEAD has 1 object that is not in it"), "the probe must still run: %v", err)
	})

	It("#737: leaves a ref that never answers to the scan's own read, which ends the run, instead of waiting out the probe", func() {
		clone, ref := cloneLackingHeadObjects()
		pipeInPlaceOf(ref)

		err, elapsed := guardWithin(clone, 30*time.Second)

		Expect(err).NotTo(HaveOccurred())
		Expect(elapsed).To(BeNumerically("<", 15*time.Second), "resolving HEAD is held to a few seconds, not the probe's two minutes")
	})
})
