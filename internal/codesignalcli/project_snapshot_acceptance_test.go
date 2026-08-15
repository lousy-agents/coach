package codesignalcli

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// This package's Ginkgo specs all run under the single TestProjectTextAcceptance
// entrypoint in project_acceptance_test.go: Ginkgo v2 does not support calling
// RunSpecs more than once per test binary, so this file intentionally defines
// no Test*Acceptance function of its own -- its Describe block below registers
// into that one shared run, per every other package in this module
// (cmd/coach, pkg/projectmodel, pkg/githubingest each define exactly one
// acceptance_suite_test.go entrypoint, not one per feature file).

func hasEnvPrefix(env []string, prefix string) bool {
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			return true
		}
	}
	return false
}

func snapshotGroundTruthLsTree(dir, revision string) []string {
	cmd := exec.Command("git", "-C", dir, "ls-tree", "-r", "--name-only", revision)
	output, err := cmd.Output()
	Expect(err).NotTo(HaveOccurred())
	lines := strings.Split(strings.TrimRight(string(output), "\n"), "\n")
	sort.Strings(lines)
	return lines
}

func snapshotGroundTruthShow(dir, revision, path string) []byte {
	cmd := exec.Command("git", "-C", dir, "show", revision+":"+path)
	output, err := cmd.Output()
	Expect(err).NotTo(HaveOccurred())
	return output
}

var _ = Describe("NewGoSnapshotFS", func() {
	var dir, sha string

	BeforeEach(func() {
		dir = acceptanceTempGitRepo()
		Expect(os.MkdirAll(filepath.Join(dir, "pkg"), 0o755)).To(Succeed())
		acceptanceCommitFile(dir, "main.go", "package main\n\nfunc main() {}\n")
		acceptanceCommitFile(dir, "pkg/lib.go", "package pkg\n\nfunc Lib() {}\n")
		sha = acceptanceCommitFile(dir, "README.md", "hello\n")
	})

	It("lists every file tracked at revision, matching git ls-tree ground truth", func() {
		fsys, err := NewGoSnapshotFS(dir, sha)
		Expect(err).NotTo(HaveOccurred())

		var got []string
		Expect(fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, walkErr error) error {
			Expect(walkErr).NotTo(HaveOccurred())
			if !d.IsDir() {
				got = append(got, p)
			}
			return nil
		})).To(Succeed())
		sort.Strings(got)

		Expect(got).To(Equal(snapshotGroundTruthLsTree(dir, sha)))
	})

	It("reads a file's content via the returned fs.FS matching git show ground truth", func() {
		fsys, err := NewGoSnapshotFS(dir, sha)
		Expect(err).NotTo(HaveOccurred())

		got, err := fs.ReadFile(fsys, "pkg/lib.go")
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(Equal(snapshotGroundTruthShow(dir, sha, "pkg/lib.go")))
	})

	It("visits every tracked file exactly once in lexical order via fs.WalkDir", func() {
		fsys, err := NewGoSnapshotFS(dir, sha)
		Expect(err).NotTo(HaveOccurred())

		var visited []string
		Expect(fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, walkErr error) error {
			Expect(walkErr).NotTo(HaveOccurred())
			if !d.IsDir() {
				visited = append(visited, p)
			}
			return nil
		})).To(Succeed())

		sorted := append([]string(nil), visited...)
		sort.Strings(sorted)
		Expect(visited).To(Equal(sorted), "fs.WalkDir must visit files in lexical order")
		Expect(visited).To(ConsistOf("README.md", "main.go", "pkg/lib.go"))
	})

	It("returns an fs.ErrNotExist-compatible error for a path not tracked at the revision", func() {
		fsys, err := NewGoSnapshotFS(dir, sha)
		Expect(err).NotTo(HaveOccurred())

		_, err = fs.ReadFile(fsys, "does/not/exist.go")
		Expect(err).To(HaveOccurred())
		Expect(errors.Is(err, fs.ErrNotExist)).To(BeTrue(), "error must satisfy errors.Is(err, fs.ErrNotExist): %v", err)
	})

	It("builds every git snapshot child with a sanitized environment, never forwarding Go-tooling or proxy variables", func() {
		Expect(os.Setenv("GOPROXY", "http://example.invalid")).To(Succeed())
		Expect(os.Setenv("HTTP_PROXY", "http://example.invalid")).To(Succeed())
		Expect(os.Setenv("GOFLAGS", "-mod=mod")).To(Succeed())
		DeferCleanup(func() {
			os.Unsetenv("GOPROXY")
			os.Unsetenv("HTTP_PROXY")
			os.Unsetenv("GOFLAGS")
		})

		cmd := snapshotGitCommandContext(context.Background(), dir, "show", sha+":main.go")

		Expect(hasEnvPrefix(cmd.Env, "GOPROXY=")).To(BeFalse())
		Expect(hasEnvPrefix(cmd.Env, "HTTP_PROXY=")).To(BeFalse())
		Expect(hasEnvPrefix(cmd.Env, "HTTPS_PROXY=")).To(BeFalse())
		Expect(hasEnvPrefix(cmd.Env, "GOFLAGS=")).To(BeFalse())
		Expect(hasEnvPrefix(cmd.Env, "GOPATH=")).To(BeFalse())
		Expect(hasEnvPrefix(cmd.Env, "GO111MODULE=")).To(BeFalse())
		Expect(hasEnvPrefix(cmd.Env, "PATH=")).To(BeTrue(), "PATH must still be forwarded so the git executable can be found")
		Expect(cmd.Env).To(ContainElement("GIT_TERMINAL_PROMPT=0"))
		Expect(cmd.Env).To(ContainElement("GIT_CONFIG_NOSYSTEM=1"))
		Expect(cmd.Env).To(ContainElement("GIT_NO_LAZY_FETCH=1"), "a blobless partial-clone promisor fetch must never be allowed to reach the network")
	})

	It("returns an error rather than an empty FS for an unresolvable revision", func() {
		_, err := NewGoSnapshotFS(dir, "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
		Expect(err).To(HaveOccurred())
	})

	It("returns an error rather than an empty FS when dir is not a Git repository", func() {
		nonGitDir := GinkgoT().TempDir()
		_, err := NewGoSnapshotFS(nonGitDir, "HEAD")
		Expect(err).To(HaveOccurred())
	})

	It("bounds a hung git child by its own wall-time budget rather than hanging the test suite", func() {
		originalRunner := runSnapshotGit
		originalCmd := snapshotGitCommandContext
		DeferCleanup(func() {
			runSnapshotGit = originalRunner
			snapshotGitCommandContext = originalCmd
		})

		snapshotGitCommandContext = func(ctx context.Context, d string, args ...string) *exec.Cmd {
			return exec.CommandContext(ctx, "sleep", "60")
		}
		runSnapshotGit = func(d string, maxStdout, maxStderr int64, timeout time.Duration, args ...string) ([]byte, error) {
			return runGitBytesBoundedWith(snapshotGitCommandContext, d, maxStdout, maxStderr, 50*time.Millisecond, args...)
		}

		started := time.Now()
		_, err := NewGoSnapshotFS(dir, sha)
		elapsed := time.Since(started)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("timed out"))
		Expect(elapsed).To(BeNumerically("<", 2*time.Second), "hung git child must be bounded by the timeout, not the test suite; elapsed=%s", elapsed)
	})
})
