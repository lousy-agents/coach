//go:build unix

package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const raceCanary = "TOP-SECRET-CANARY-7f3a91"

// On Linux git can lstat the entry as a file, see it swapped for a FIFO, and block opening it. The scan
// then ends at the wall-time bound of its git status read, so the ceiling is that bound plus a margin.
const racedScanCeiling = gitrepo.WorktreeStatusTimeout + 15*time.Second

// placeEntry puts one kind of entry at scratch: a policy file, a FIFO, a symbolic link out of the
// repository, or a symbolic link to a regular file.
func placeEntry(round int, scratch, outside, regular string) {
	switch round % 5 {
	case 0, 1:
		_ = os.WriteFile(scratch, []byte(singleRootPolicyJSON), 0o644)
	case 2:
		_ = syscall.Mkfifo(scratch, 0o644)
	case 3:
		_ = os.Symlink(outside, scratch)
	default:
		_ = os.Symlink(regular, scratch)
	}
}

func raceStopped(stop <-chan struct{}) bool {
	select {
	case <-stop:
		return true
	default:
		return false
	}
}

// racePolicyPath swaps the entry at target between kinds, and now and then removes it, until stop is closed.
func racePolicyPath(target, outside, regular string, stop <-chan struct{}) {
	scratch := target + ".swap"
	for round := 0; !raceStopped(stop); round++ {
		_ = os.Remove(scratch)
		placeEntry(round, scratch, outside, regular)
		_ = os.Rename(scratch, target)
		if round%17 == 0 {
			_ = os.Remove(target)
		}
	}
}

// racePolicyParent swaps the directory above the policy between itself and a FIFO, and now and then
// removes it, until stop is closed. It always puts the directory back before it returns.
func racePolicyParent(parent string, stop <-chan struct{}) {
	away, fifo := parent+".away", parent+".fifo"
	_ = syscall.Mkfifo(fifo, 0o644)
	for !raceStopped(stop) {
		_ = os.Rename(parent, away)
		_ = os.Rename(fifo, parent)
		_ = os.Rename(parent, fifo)
		_ = os.Rename(away, parent)
	}
}

// runRacedScan is runCoachBinary with a limit, so that a scan stuck on the raced entry is killed and
// fails the spec instead of holding up the whole suite.
func runRacedScan(limit time.Duration, repo string) (stdout, stderr []byte, exitCode int) {
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()
	command := exec.CommandContext(ctx, commandPath, "codesignal", "--baseline", "--project-config", explicitPolicyPath, "--format=json")
	command.Dir = repo
	var outBuf, errBuf bytes.Buffer
	command.Stdout, command.Stderr = &outBuf, &errBuf
	err := command.Run()
	var exitErr *exec.ExitError
	if err != nil && !errors.As(err, &exitErr) {
		ExpectWithOffset(2, err).NotTo(HaveOccurred())
	}
	if exitErr != nil {
		return outBuf.Bytes(), errBuf.Bytes(), exitErr.ExitCode()
	}
	return outBuf.Bytes(), errBuf.Bytes(), 0
}

// expectRacedScanInvariants runs one scan against the raced path and requires what must hold whichever
// entry it met: it ends, it does not panic, it prints no report beside a refusal, and it prints nothing
// from the file outside the repository. This is a smoke test for hangs and crashes under a race. That a
// symbolic link out of the repository is refused and never read is pinned by the deterministic specs in
// project_config_hostile_input_acceptance_test.go, because a scan that did read the file would exit 0
// and print nothing from it, so no assertion here could tell.
func expectRacedScanInvariants(repo string) {
	begin := time.Now()
	stdout, stderr, exitCode := runRacedScan(racedScanCeiling+15*time.Second, repo)

	ExpectWithOffset(1, time.Since(begin)).To(BeNumerically("<", racedScanCeiling), "a scan hung on the raced entry")
	ExpectWithOffset(1, exitCode).To(BeElementOf(0, 2), "stderr: %s", stderr)
	ExpectWithOffset(1, string(stderr)).NotTo(ContainSubstring("panic:"))
	ExpectWithOffset(1, string(stdout)+string(stderr)).NotTo(ContainSubstring(raceCanary))
	if exitCode == 2 {
		ExpectWithOffset(1, stdout).To(BeEmpty(), "a report was printed beside a refusal")
	}
}

var _ = Describe("coach codesignal: an explicit --project-config path raced on disk while the scan runs", func() {
	It("never hangs, panics, prints a report beside a refusal, or prints anything from a file outside the repository", func() {
		repo := commitExplicitPolicyRepo()
		side := GinkgoT().TempDir()
		outside := filepath.Join(side, "outside.json")
		Expect(os.WriteFile(outside, []byte(`{"canary":"`+raceCanary+`","schema_version":"1","roots":["."]}`), 0o644)).To(Succeed())
		regular := filepath.Join(side, "regular.json")
		Expect(os.WriteFile(regular, []byte(singleRootPolicyJSON), 0o644)).To(Succeed())
		stop := make(chan struct{})
		var swapper sync.WaitGroup
		swapper.Add(1)
		go func() {
			defer swapper.Done()
			racePolicyPath(filepath.Join(repo, explicitPolicyPath), outside, regular, stop)
		}()
		DeferCleanup(func() { close(stop); swapper.Wait() })

		for i := 0; i < 40; i++ {
			expectRacedScanInvariants(repo)
		}
	})

	It("never waits on a parent directory that is swapped for a FIFO", func() {
		repo := commitExplicitPolicyRepo()
		stop := make(chan struct{})
		var swapper sync.WaitGroup
		swapper.Add(1)
		go func() {
			defer swapper.Done()
			racePolicyParent(filepath.Join(repo, filepath.Dir(explicitPolicyPath)), stop)
		}()
		DeferCleanup(func() { close(stop); swapper.Wait() })

		for i := 0; i < 40; i++ {
			expectRacedScanInvariants(repo)
		}
	})
})
