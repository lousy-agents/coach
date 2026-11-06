package gitrepo

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

// partialClone is a clone whose blobs are not local, so that reading one makes git fetch it from the
// promisor remote. It returns the clone, the blob that is missing, and the remote's path. The remote
// is set to remoteURL afterwards, so a test can point it at a program or at a repository that would
// answer.
func partialClone(t *testing.T, remoteURL func(bare string) string) (clone, blob string) {
	t.Helper()
	source := gitfixture.Init(t)
	gitfixture.CommitFile(t, source, "cfg/p.json", "{}")
	blob = strings.TrimSpace(gitOutput(t, source, "rev-parse", "HEAD:cfg/p.json"))
	bare := filepath.Join(t.TempDir(), "bare.git")
	gitOutput(t, source, "clone", "--bare", "--quiet", source, bare)
	gitOutput(t, bare, "config", "uploadpack.allowFilter", "true")
	clone = filepath.Join(t.TempDir(), "part")
	gitOutput(t, t.TempDir(), "clone", "--quiet", "--no-checkout", "--filter=blob:none", "file://"+bare, clone)
	gitOutput(t, clone, "remote", "set-url", "origin", remoteURL(bare))
	return clone, blob
}

// skipIfGitCannotRefuseLazyFetch skips a test that rests on GIT_NO_LAZY_FETCH, which git has from 2.44.
func skipIfGitCannotRefuseLazyFetch(t *testing.T) {
	t.Helper()
	if supported, version := noLazyFetchSupported(RunBytes, "."); !supported {
		t.Skipf("this git (%s) predates GIT_NO_LAZY_FETCH, so it cannot be told not to fetch", version)
	}
}

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, output)
	}
	return string(output)
}

// sshMarker makes a fetch over ssh run a program that leaves a marker, and returns the marker's path.
func sshMarker(t *testing.T) string {
	t.Helper()
	scripts := t.TempDir()
	marker := filepath.Join(scripts, "ssh-ran")
	program := filepath.Join(scripts, "fakessh")
	if err := os.WriteFile(program, []byte("#!/bin/sh\ntouch '"+marker+"'\nexit 1\n"), 0o755); err != nil {
		t.Fatalf("write ssh program: %v", err)
	}
	t.Setenv("GIT_SSH_COMMAND", program)
	return marker
}

func packFiles(t *testing.T, clone string) int {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(clone, ".git", "objects", "pack"))
	if err != nil {
		t.Fatalf("read pack directory: %v", err)
	}
	return len(entries)
}

// missingBlobReads are the ways this package starts git, each asked for the blob a partial clone lacks.
func missingBlobReads(clone, blob string) []gitRead {
	return []gitRead{
		{"the unbounded git read", func(dir string) error { _, err := RunBytes(dir, "cat-file", "-e", blob); return err }},
		{"the bounded runner other reads use", func(dir string) error {
			_, err := BoundedRunner(1<<20, 1<<16, 30*time.Second)(dir, "cat-file", "-e", blob)
			return err
		}},
		{"the batch object reader", func(dir string) error {
			reader, err := NewRevisionFileReader(dir, "HEAD")
			if err != nil {
				return err
			}
			defer func() { _ = reader.Close() }()
			_, err = reader.Next("cfg/p.json")
			return err
		}},
		{"the snapshot read", func(dir string) error { _, err := RunBytes(dir, "archive", "--format=tar", "HEAD"); return err }},
	}
}

func TestGitReadsRunNoSSHProgramToFetchAMissingObject(t *testing.T) {
	skipIfGitCannotRefuseLazyFetch(t)
	marker := sshMarker(t)
	clone, blob := partialClone(t, func(string) string { return "ssh://example.invalid/repo.git" })
	for _, read := range missingBlobReads(clone, blob) {
		t.Run(read.name, func(t *testing.T) {
			assertNoSSHProgramRan(t, read, clone, marker)
		})
	}
}

func assertNoSSHProgramRan(t *testing.T, read gitRead, clone, marker string) {
	t.Helper()
	_ = read.read(clone)

	if _, err := os.Stat(marker); err == nil {
		t.Errorf("git ran the ssh program to fetch an object: a read-only scan must not reach for the network")
	}
}

func TestGitReadsWriteNoObjectsToFetchAMissingObjectFromARemoteThatWouldAnswer(t *testing.T) {
	skipIfGitCannotRefuseLazyFetch(t)
	clone, blob := partialClone(t, func(bare string) string { return "file://" + bare })
	before := packFiles(t, clone)
	for _, read := range missingBlobReads(clone, blob) {
		t.Run(read.name, func(t *testing.T) {
			assertNoObjectsFetched(t, read, clone, before)
		})
	}
}

func assertNoObjectsFetched(t *testing.T, read gitRead, clone string, before int) {
	t.Helper()
	_ = read.read(clone)

	if after := packFiles(t, clone); after != before {
		t.Errorf("the object store went from %d to %d pack files: a read-only scan fetched an object", before, after)
	}
}
