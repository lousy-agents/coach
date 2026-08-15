package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// runCoachSuggest runs `coach codesignal [args...]` in repo, returning raw
// stdout/stderr without assuming success.
func runCoachSuggest(repo string, args ...string) (stdout, stderr []byte, exitCode int) {
	command := exec.Command(commandPath, append([]string{"codesignal"}, args...)...)
	command.Dir = repo
	var outBuf, errBuf bytes.Buffer
	command.Stdout = &outBuf
	command.Stderr = &errBuf

	err := command.Run()
	if err == nil {
		return outBuf.Bytes(), errBuf.Bytes(), 0
	}

	var exitErr *exec.ExitError
	Expect(errors.As(err, &exitErr)).To(BeTrue(), "expected an ExitError, got: %s (stderr: %s)", err, errBuf.String())
	return outBuf.Bytes(), errBuf.Bytes(), exitErr.ExitCode()
}

// cloneTempGitRepo clones source into a fresh temp directory via `git
// clone`, preserving commit SHAs while changing the absolute path -- used
// to prove determinism isn't accidentally passing because two runs share
// the same repository path.
func cloneTempGitRepo(source string) string {
	directory, err := os.MkdirTemp("", "coach-acceptance-clone-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, directory)

	cloneCmd := exec.Command("git", "clone", source, directory)
	output, err := cloneCmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "git clone: %s", output)

	return directory
}

// cloneTempGitRepoWithQuoteInPath is cloneTempGitRepo, except the clone's
// own directory name contains a double quote -- reproducing the class of
// repository root that NewGoSnapshotFS's error text renders with %q
// (strconv.Quote escapes '"', so the raw and %q-escaped forms of the path
// differ), to prove leak-guarding code strips both forms, not just the raw
// one.
func cloneTempGitRepoWithQuoteInPath(source string) string {
	parent, err := os.MkdirTemp("", "coach-acceptance-clone-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, parent)

	directory := filepath.Join(parent, `re"po`)
	cloneCmd := exec.Command("git", "clone", source, directory)
	output, err := cloneCmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "git clone: %s", output)

	return directory
}

type suggestionCandidateDoc struct {
	SchemaVersion string   `json:"schema_version"`
	Roots         []string `json:"roots"`
}

type suggestionEnvelopeDoc struct {
	DiagnosticVersion string   `json:"diagnostic_version"`
	Kind              string   `json:"kind"`
	Revision          string   `json:"revision"`
	HeuristicVersion  string   `json:"heuristic_version"`
	RootsConsidered   []string `json:"roots_considered"`
	Coverage          struct {
		Phase    string         `json:"phase"`
		Complete bool           `json:"complete"`
		Counts   map[string]int `json:"counts"`
		Budgets  map[string]int `json:"budgets"`
	} `json:"coverage"`
	Diagnostics []struct {
		Code    string `json:"code"`
		Path    string `json:"path"`
		Message string `json:"message"`
	} `json:"diagnostics"`
}

var _ = Describe("coach codesignal --baseline --suggest-project-config", func() {
	When("a single-module Go repository is scanned", func() {
		It("emits a candidate with roots: [\".\"] on stdout and a provenance envelope on stderr, exit 0", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", "module example.com/single\n\ngo 1.25\n")
			headSHA := commitFile(repo, "main.go", "package main\n\nfunc main() {}\n")

			stdout, stderr, exitCode := runCoachSuggest(repo, "--baseline", "--suggest-project-config")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var candidate suggestionCandidateDoc
			Expect(json.Unmarshal(stdout, &candidate)).To(Succeed(), "stdout: %s", stdout)
			Expect(candidate.SchemaVersion).To(Equal("1"))
			Expect(candidate.Roots).To(Equal([]string{"."}))

			var envelope suggestionEnvelopeDoc
			Expect(json.Unmarshal(stderr, &envelope)).To(Succeed(), "stderr: %s", stderr)
			Expect(envelope.DiagnosticVersion).To(Equal("1"))
			Expect(envelope.Kind).To(Equal("project_config_suggestion"))
			Expect(envelope.Revision).To(Equal(headSHA))
			Expect(envelope.HeuristicVersion).To(Equal("go-project-config-roots@1"))
			Expect(envelope.RootsConsidered).To(Equal([]string{"."}))
			Expect(envelope.Diagnostics).To(HaveLen(1))
			Expect(envelope.Diagnostics[0].Code).To(Equal("project_config_suggestion_ready"))
		})
	})

	When("a go.mod exists under a nested testdata/ fixture directory", func() {
		It("excludes the testdata fixture from the discovered roots, matching the go tool's own convention", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", "module example.com/testdatafixture\n\ngo 1.25\n")
			commitFile(repo, "pkg/thing/testdata/fixture/go.mod", "module example.com/testdatafixture/pkg/thing/testdata/fixture\n\ngo 1.25\n")

			stdout, stderr, exitCode := runCoachSuggest(repo, "--baseline", "--suggest-project-config")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var candidate suggestionCandidateDoc
			Expect(json.Unmarshal(stdout, &candidate)).To(Succeed(), "stdout: %s", stdout)
			Expect(candidate.Roots).To(Equal([]string{"."}),
				"pkg/thing/testdata/fixture must not be reported as a root")
		})
	})

	When("a go.work multi-module workspace is scanned", func() {
		It("emits every distinct module/workspace root, sorted", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.work", "go 1.25\n\nuse (\n\t./modulea\n\t./moduleb\n)\n")
			commitFile(repo, "modulea/go.mod", "module example.com/modulea\n\ngo 1.25\n")
			commitFile(repo, "moduleb/go.mod", "module example.com/moduleb\n\ngo 1.25\n")

			stdout, stderr, exitCode := runCoachSuggest(repo, "--baseline", "--suggest-project-config")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var candidate suggestionCandidateDoc
			Expect(json.Unmarshal(stdout, &candidate)).To(Succeed(), "stdout: %s", stdout)
			Expect(candidate.Roots).To(Equal([]string{".", "modulea", "moduleb"}))
		})
	})

	When("the same repository, checked out at two different paths, is scanned", func() {
		It("produces byte-identical stdout and byte-identical stderr, proving no absolute path leaks in", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", "module example.com/deterministic\n\ngo 1.25\n")

			cloneA := cloneTempGitRepo(repo)
			cloneB := cloneTempGitRepo(repo)
			Expect(cloneA).NotTo(Equal(cloneB))

			stdout1, stderr1, exitCode1 := runCoachSuggest(cloneA, "--baseline", "--suggest-project-config")
			Expect(exitCode1).To(Equal(0), "stderr: %s", stderr1)

			stdout2, stderr2, exitCode2 := runCoachSuggest(cloneB, "--baseline", "--suggest-project-config")
			Expect(exitCode2).To(Equal(0), "stderr: %s", stderr2)

			Expect(stdout1).To(Equal(stdout2))
			Expect(stderr1).To(Equal(stderr2))
		})
	})

	When("the worktree has modified tracked, untracked, and ignored files", func() {
		It("still reflects only the HEAD snapshot, matching a clean-worktree run", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", "module example.com/noisy\n\ngo 1.25\n")
			commitFile(repo, "main.go", "package main\n\nfunc main() {}\n")

			cleanStdout, cleanStderr, cleanExit := runCoachSuggest(repo, "--baseline", "--suggest-project-config")
			Expect(cleanExit).To(Equal(0), "stderr: %s", cleanStderr)

			Expect(os.WriteFile(filepath.Join(repo, "main.go"), []byte("package main\n\nfunc main() { println(1) }\n"), 0o644)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(repo, "untracked.go"), []byte("package main\n"), 0o644)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(repo, ".gitignore"), []byte("ignored/\n"), 0o644)).To(Succeed())
			Expect(os.MkdirAll(filepath.Join(repo, "ignored"), 0o755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(repo, "ignored", "extra.go"), []byte("package ignored\n"), 0o644)).To(Succeed())

			noisyStdout, noisyStderr, noisyExit := runCoachSuggest(repo, "--baseline", "--suggest-project-config")
			Expect(noisyExit).To(Equal(0), "stderr: %s", noisyStderr)

			Expect(noisyStdout).To(Equal(cleanStdout), "worktree noise must not affect the HEAD-snapshot-based candidate")
			Expect(noisyStderr).To(Equal(cleanStderr), "worktree noise must not affect the HEAD-snapshot-based provenance envelope")
		})
	})

	When("no go.mod or go.work exists anywhere in the repository", func() {
		It("exits 2 with project_config_suggestion_no_go_modules, empty stdout", func() {
			repo := newTempGitRepo()
			commitFile(repo, "README.md", "no go here\n")

			stdout, stderr, exitCode := runCoachSuggest(repo, "--baseline", "--suggest-project-config")
			Expect(exitCode).To(Equal(2), "stderr: %s", stderr)
			Expect(stdout).To(BeEmpty())
			Expect(string(stderr)).To(ContainSubstring("project_config_suggestion_no_go_modules"))
		})
	})

	When("the repository has no commits yet, so HEAD cannot be resolved", func() {
		It("exits 3 with project_config_suggestion_snapshot_unavailable and an empty revision, not the plain-text operational-error exit-1 path", func() {
			repo := newTempGitRepo()

			stdout, stderr, exitCode := runCoachSuggest(repo, "--baseline", "--suggest-project-config")
			Expect(exitCode).To(Equal(3), "stderr: %s", stderr)
			Expect(stdout).To(BeEmpty())

			var envelope suggestionEnvelopeDoc
			Expect(json.Unmarshal(stderr, &envelope)).To(Succeed(), "stderr: %s", stderr)
			Expect(envelope.Revision).To(Equal(""))
			Expect(envelope.Diagnostics).To(HaveLen(1))
			Expect(envelope.Diagnostics[0].Code).To(Equal("project_config_suggestion_snapshot_unavailable"))
		})
	})

	When("HEAD resolves to a commit whose tree object is missing from the object store", func() {
		It("exits 3 with project_config_suggestion_snapshot_unavailable, empty stdout", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", "module example.com/corrupttree\n\ngo 1.25\n")

			treeCmd := exec.Command("git", "rev-parse", "HEAD^{tree}")
			treeCmd.Dir = repo
			treeOutput, err := treeCmd.Output()
			Expect(err).NotTo(HaveOccurred())
			tree := strings.TrimSpace(string(treeOutput))
			Expect(os.Remove(filepath.Join(repo, ".git", "objects", tree[:2], tree[2:]))).To(Succeed())

			stdout, stderr, exitCode := runCoachSuggest(repo, "--baseline", "--suggest-project-config")
			Expect(exitCode).To(Equal(3), "stderr: %s", stderr)
			Expect(stdout).To(BeEmpty())
			Expect(string(stderr)).To(ContainSubstring("project_config_suggestion_snapshot_unavailable"))
			Expect(string(stderr)).NotTo(ContainSubstring(repo), "the envelope must not leak the absolute repository path")
		})
	})

	When("HEAD resolves to a commit whose tree object is missing, and the repository path contains a character %q must escape", func() {
		It("exits 3 with project_config_suggestion_snapshot_unavailable, leaking neither the raw nor the %q-escaped absolute repository root", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", "module example.com/corrupttreequote\n\ngo 1.25\n")

			treeCmd := exec.Command("git", "rev-parse", "HEAD^{tree}")
			treeCmd.Dir = repo
			treeOutput, err := treeCmd.Output()
			Expect(err).NotTo(HaveOccurred())
			tree := strings.TrimSpace(string(treeOutput))

			quotedRepo := cloneTempGitRepoWithQuoteInPath(repo)
			Expect(os.Remove(filepath.Join(quotedRepo, ".git", "objects", tree[:2], tree[2:]))).To(Succeed())

			stdout, stderr, exitCode := runCoachSuggest(quotedRepo, "--baseline", "--suggest-project-config")
			Expect(exitCode).To(Equal(3), "stderr: %s", stderr)
			Expect(stdout).To(BeEmpty())

			var envelope suggestionEnvelopeDoc
			Expect(json.Unmarshal(stderr, &envelope)).To(Succeed(), "stderr: %s", stderr)
			Expect(envelope.Diagnostics).To(HaveLen(1))
			Expect(envelope.Diagnostics[0].Code).To(Equal("project_config_suggestion_snapshot_unavailable"))

			// Assert against the JSON-decoded message, not the raw NDJSON
			// bytes: JSON re-escapes '"' and '\' in the message field
			// regardless of whether the leak was fixed, so a raw-bytes
			// substring check on stderr would pass vacuously either way.
			message := envelope.Diagnostics[0].Message
			Expect(message).NotTo(ContainSubstring(quotedRepo), "the envelope must not leak the raw absolute repository path")
			quoted := strconv.Quote(quotedRepo)
			Expect(message).NotTo(ContainSubstring(quoted[1:len(quoted)-1]), "the envelope must not leak the %q-escaped absolute repository path")
		})
	})

	When("the invocation directory is not inside a Git worktree at all", func() {
		It("exits 3 with project_config_suggestion_snapshot_unavailable without leaking the absolute invocation directory", func() {
			nonRepo, err := os.MkdirTemp("", "coach-acceptance-nonrepo-*")
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(os.RemoveAll, nonRepo)

			stdout, stderr, exitCode := runCoachSuggest(nonRepo, "--baseline", "--suggest-project-config")
			Expect(exitCode).To(Equal(3), "stderr: %s", stderr)
			Expect(stdout).To(BeEmpty())

			var envelope suggestionEnvelopeDoc
			Expect(json.Unmarshal(stderr, &envelope)).To(Succeed(), "stderr: %s", stderr)
			Expect(envelope.Revision).To(Equal(""))
			Expect(envelope.Diagnostics).To(HaveLen(1))
			Expect(envelope.Diagnostics[0].Code).To(Equal("project_config_suggestion_snapshot_unavailable"))
			Expect(string(stderr)).NotTo(ContainSubstring(nonRepo), "the envelope must not leak the absolute invocation directory")
		})
	})

	When("--suggest-project-config=false is combined with an unrelated malformed flag", func() {
		It("takes the plain usage-text error path (exit 2, no JSON envelope), since suggest mode was explicitly disabled", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", "module example.com/falsesuggest\n\ngo 1.25\n")

			stdout, stderr, exitCode := runCoachSuggest(repo, "--baseline", "--suggest-project-config=false", "--not-a-real-flag")
			Expect(exitCode).To(Equal(2), "stderr: %s", stderr)
			Expect(stdout).To(BeEmpty())
			Expect(string(stderr)).NotTo(ContainSubstring("project_config_suggestion_invalid_arguments"),
				"an explicitly disabled --suggest-project-config must not trigger the suggestion envelope for an unrelated flag error")
			Expect(string(stderr)).To(ContainSubstring("not-a-real-flag"), "expected the standard flag-package usage text: %s", stderr)
		})
	})

	DescribeTable("rejected flag combinations",
		func(extraArgs ...string) {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", "module example.com/rejects\n\ngo 1.25\n")

			stdout, stderr, exitCode := runCoachSuggest(repo, extraArgs...)
			Expect(exitCode).To(Equal(2), "stderr: %s", stderr)
			Expect(stdout).To(BeEmpty())
			Expect(string(stderr)).To(ContainSubstring("project_config_suggestion_invalid_arguments"))
		},
		Entry("missing --baseline", "--suggest-project-config"),
		Entry("combined with --base", "--suggest-project-config", "--base", "HEAD"),
		Entry("combined with --project-config", "--baseline", "--suggest-project-config", "--project-config", "project.json"),
		Entry("combined with explicit --project-language go", "--baseline", "--suggest-project-config", "--project-language", "go"),
		Entry("combined with --format", "--baseline", "--suggest-project-config", "--format", "json"),
		Entry("combined with --scope", "--baseline", "--suggest-project-config", "--scope", "all"),
		Entry("combined with --build-target", "--baseline", "--suggest-project-config", "--build-target", "./..."),
		Entry("combined with an unknown flag", "--baseline", "--suggest-project-config", "--not-a-real-flag"),
		Entry("combined with a positional argument", "--baseline", "--suggest-project-config", "extra-positional"),
		Entry("duplicate --suggest-project-config", "--baseline", "--suggest-project-config", "--suggest-project-config"),
		Entry("duplicate --output", "--baseline", "--suggest-project-config", "--output", "a.json", "--output", "b.json"),
	)

	When("--output is given and discovery succeeds", func() {
		It("writes the candidate bytes to the target, leaves stdout empty, and still writes the envelope to stderr", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", "module example.com/output\n\ngo 1.25\n")

			stdout, stderr, exitCode := runCoachSuggest(repo, "--baseline", "--suggest-project-config", "--output", "project.json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(stdout).To(BeEmpty())

			written, err := os.ReadFile(filepath.Join(repo, "project.json"))
			Expect(err).NotTo(HaveOccurred())

			var candidate suggestionCandidateDoc
			Expect(json.Unmarshal(written, &candidate)).To(Succeed())
			Expect(candidate.Roots).To(Equal([]string{"."}))
			Expect(strings.HasSuffix(string(written), "\n")).To(BeTrue())

			var envelope suggestionEnvelopeDoc
			Expect(json.Unmarshal(stderr, &envelope)).To(Succeed(), "stderr: %s", stderr)
			Expect(envelope.Diagnostics[0].Code).To(Equal("project_config_suggestion_ready"))
		})
	})

	When("the --output write fails mid-write", func() {
		It("removes the partially-written target and reports project_config_suggestion_output_invalid without leaking an absolute path", func() {
			if runtime.GOOS == "windows" {
				Skip("ulimit -f is not available on windows")
			}
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", "module example.com/midwrite\n\ngo 1.25\n")

			// ulimit -f 0 lets the create-only O_EXCL open succeed but makes
			// the very next Write fail with EFBIG, exercising the same
			// write-failure branch as ENOSPC/EIO without needing root or a
			// size-limited filesystem. Stderr must be a bytes.Buffer, not an
			// *os.File: an *os.File stderr is also subject to the file-size
			// limit and the run degrades to exit 1 with an empty envelope,
			// which would be a false green for this branch.
			command := exec.Command("sh", "-c", "ulimit -f 0; exec "+commandPath+" codesignal --baseline --suggest-project-config --output out.json")
			command.Dir = repo
			var stderr bytes.Buffer
			command.Stderr = &stderr

			err := command.Run()
			var exitErr *exec.ExitError
			Expect(errors.As(err, &exitErr)).To(BeTrue(), "expected an ExitError, got: %s (stderr: %s)", err, stderr.String())
			Expect(exitErr.ExitCode()).To(Equal(2))
			Expect(stderr.String()).To(ContainSubstring("project_config_suggestion_output_invalid"))
			Expect(stderr.String()).NotTo(ContainSubstring(repo), "the envelope must not leak the absolute repository path")

			_, statErr := os.Stat(filepath.Join(repo, "out.json"))
			Expect(os.IsNotExist(statErr)).To(BeTrue(), "the partially-written target must be removed on write failure")
		})
	})

	When("--baseline --suggest-project-config is run from a nested module subdirectory", func() {
		It("discovers roots relative to the repository root, not the invocation directory", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", "module example.com/subdir\n\ngo 1.25\n")
			commitFile(repo, "services/payments/go.mod", "module example.com/subdir/services/payments\n\ngo 1.25\n")

			subdir := filepath.Join(repo, "services", "payments")
			stdout, stderr, exitCode := runCoachSuggest(subdir, "--baseline", "--suggest-project-config")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var candidate suggestionCandidateDoc
			Expect(json.Unmarshal(stdout, &candidate)).To(Succeed(), "stdout: %s", stdout)
			Expect(candidate.Roots).To(Equal([]string{".", "services/payments"}), "roots must be repository-root-relative regardless of the invocation directory")
		})
	})

	When("--output is given while running from a subdirectory", func() {
		It("resolves the output path against the repository root, not the current working directory", func() {
			repo := newTempGitRepo()
			// A module at the repo root and a module inside the
			// subdirectory both exist; the roots assertion below pins that
			// discovery is repository-root-relative even when invoked from
			// the subdirectory, so this fixture can't silently regress to
			// only discovering the invocation-directory-relative root.
			commitFile(repo, "go.mod", "module example.com/subdir\n\ngo 1.25\n")
			commitFile(repo, "services/payments/go.mod", "module example.com/subdir/services/payments\n\ngo 1.25\n")

			subdir := filepath.Join(repo, "services", "payments")
			stdout, stderr, exitCode := runCoachSuggest(subdir, "--baseline", "--suggest-project-config", "--output", "out.json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(stdout).To(BeEmpty())

			written, err := os.ReadFile(filepath.Join(repo, "out.json"))
			Expect(err).NotTo(HaveOccurred(), "expected out.json at the repository root")

			var candidate suggestionCandidateDoc
			Expect(json.Unmarshal(written, &candidate)).To(Succeed(), "out.json: %s", written)
			Expect(candidate.Roots).To(Equal([]string{".", "services/payments"}))

			_, err = os.Stat(filepath.Join(subdir, "out.json"))
			Expect(os.IsNotExist(err)).To(BeTrue(), "out.json must not be written under the invocation subdirectory")
		})
	})

	When("--output points into a directory without write permission", func() {
		It("exits 2 with project_config_suggestion_output_invalid instead of the defensive failed code", func() {
			if os.Geteuid() == 0 {
				Skip("cannot exercise a permission-denied write while running as root")
			}
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", "module example.com/readonlydir\n\ngo 1.25\n")
			readonlyDir := filepath.Join(repo, "readonly")
			Expect(os.MkdirAll(readonlyDir, 0o755)).To(Succeed())
			Expect(os.Chmod(readonlyDir, 0o555)).To(Succeed())
			DeferCleanup(func() { os.Chmod(readonlyDir, 0o755) })

			stdout, stderr, exitCode := runCoachSuggest(repo, "--baseline", "--suggest-project-config", "--output", "readonly/out.json")
			Expect(exitCode).To(Equal(2), "stderr: %s", stderr)
			Expect(stdout).To(BeEmpty())
			Expect(string(stderr)).To(ContainSubstring("project_config_suggestion_output_invalid"))
			Expect(string(stderr)).NotTo(ContainSubstring(repo), "the envelope must not leak the absolute repository path")
		})
	})

	When("--output targets a path that already exists", func() {
		It("exits 2 with project_config_suggestion_output_exists and leaves the target untouched", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", "module example.com/exists\n\ngo 1.25\n")
			Expect(os.WriteFile(filepath.Join(repo, "already-there.json"), []byte("do-not-touch"), 0o644)).To(Succeed())

			stdout, stderr, exitCode := runCoachSuggest(repo, "--baseline", "--suggest-project-config", "--output", "already-there.json")
			Expect(exitCode).To(Equal(2), "stderr: %s", stderr)
			Expect(stdout).To(BeEmpty())
			Expect(string(stderr)).To(ContainSubstring("project_config_suggestion_output_exists"))

			untouched, err := os.ReadFile(filepath.Join(repo, "already-there.json"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(untouched)).To(Equal("do-not-touch"))
		})

		It("rejects an existing directory target the same way", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", "module example.com/existsdir\n\ngo 1.25\n")
			Expect(os.MkdirAll(filepath.Join(repo, "already-a-dir"), 0o755)).To(Succeed())

			stdout, stderr, exitCode := runCoachSuggest(repo, "--baseline", "--suggest-project-config", "--output", "already-a-dir")
			Expect(exitCode).To(Equal(2), "stderr: %s", stderr)
			Expect(stdout).To(BeEmpty())
			Expect(string(stderr)).To(ContainSubstring("project_config_suggestion_output_exists"))
		})

		It("rejects an existing symlink target without following it", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", "module example.com/existssymlink\n\ngo 1.25\n")
			elsewhere, err := os.MkdirTemp("", "coach-acceptance-symlink-target-*")
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(os.RemoveAll, elsewhere)
			Expect(os.Symlink(filepath.Join(elsewhere, "nonexistent"), filepath.Join(repo, "already-a-symlink"))).To(Succeed())

			stdout, stderr, exitCode := runCoachSuggest(repo, "--baseline", "--suggest-project-config", "--output", "already-a-symlink")
			Expect(exitCode).To(Equal(2), "stderr: %s", stderr)
			Expect(stdout).To(BeEmpty())
			Expect(string(stderr)).To(ContainSubstring("project_config_suggestion_output_exists"))

			info, statErr := os.Lstat(filepath.Join(repo, "already-a-symlink"))
			Expect(statErr).NotTo(HaveOccurred())
			Expect(info.Mode()&os.ModeSymlink).NotTo(Equal(os.FileMode(0)), "the existing symlink target must be left untouched, not replaced")
		})

		It("reports no_go_modules, not output_exists, when the repository has no Go modules at all", func() {
			// Issue #220's failure precedence puts "an existing --output
			// target" at the LAST stage, after root discovery -- so a
			// no-Go-module repository must fail on that first, even though
			// an unrelated --output target already exists.
			repo := newTempGitRepo()
			commitFile(repo, "README.md", "no go here\n")
			Expect(os.WriteFile(filepath.Join(repo, "taken.json"), []byte("do-not-touch"), 0o644)).To(Succeed())

			stdout, stderr, exitCode := runCoachSuggest(repo, "--baseline", "--suggest-project-config", "--output", "taken.json")
			Expect(exitCode).To(Equal(2), "stderr: %s", stderr)
			Expect(stdout).To(BeEmpty())
			Expect(string(stderr)).To(ContainSubstring("project_config_suggestion_no_go_modules"))
			Expect(string(stderr)).NotTo(ContainSubstring("project_config_suggestion_output_exists"))

			untouched, err := os.ReadFile(filepath.Join(repo, "taken.json"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(untouched)).To(Equal("do-not-touch"))
		})
	})

	When("--output is an empty value or the literal \"-\"", func() {
		It("rejects an explicit empty --output value", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", "module example.com/emptyoutput\n\ngo 1.25\n")

			stdout, stderr, exitCode := runCoachSuggest(repo, "--baseline", "--suggest-project-config", "--output=")
			Expect(exitCode).To(Equal(2), "stderr: %s", stderr)
			Expect(stdout).To(BeEmpty())
			Expect(string(stderr)).To(ContainSubstring("project_config_suggestion_output_invalid"))
		})

		It("rejects the literal \"-\"", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", "module example.com/dashoutput\n\ngo 1.25\n")

			stdout, stderr, exitCode := runCoachSuggest(repo, "--baseline", "--suggest-project-config", "--output", "-")
			Expect(exitCode).To(Equal(2), "stderr: %s", stderr)
			Expect(stdout).To(BeEmpty())
			Expect(string(stderr)).To(ContainSubstring("project_config_suggestion_output_invalid"))
		})
	})

	When("HEAD contains an unparseable go.mod", func() {
		It("exits 2 with project_config_suggestion_ambiguous_roots instead of a candidate", func() {
			repo := newTempGitRepo()
			commitFile(repo, "bad/go.mod", "this is not a valid go.mod {{{\n")

			stdout, stderr, exitCode := runCoachSuggest(repo, "--baseline", "--suggest-project-config")
			Expect(exitCode).To(Equal(2), "stderr: %s", stderr)
			Expect(stdout).To(BeEmpty())
			Expect(string(stderr)).To(ContainSubstring("project_config_suggestion_ambiguous_roots"))
		})
	})

	When("--output escapes the repository, is absolute, or contains a .git component", func() {
		It("rejects a ..-escaping path", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", "module example.com/escape\n\ngo 1.25\n")

			stdout, stderr, exitCode := runCoachSuggest(repo, "--baseline", "--suggest-project-config", "--output", "../escape.json")
			Expect(exitCode).To(Equal(2), "stderr: %s", stderr)
			Expect(stdout).To(BeEmpty())
			Expect(string(stderr)).To(ContainSubstring("project_config_suggestion_output_invalid"))
		})

		It("rejects an absolute path without leaking it into the envelope's path field", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", "module example.com/abs\n\ngo 1.25\n")

			stdout, stderr, exitCode := runCoachSuggest(repo, "--baseline", "--suggest-project-config", "--output", "/tmp/coach-suggest-abs.json")
			Expect(exitCode).To(Equal(2), "stderr: %s", stderr)
			Expect(stdout).To(BeEmpty())
			Expect(string(stderr)).To(ContainSubstring("project_config_suggestion_output_invalid"))

			var envelope suggestionEnvelopeDoc
			Expect(json.Unmarshal(stderr, &envelope)).To(Succeed(), "stderr: %s", stderr)
			Expect(envelope.Diagnostics).To(HaveLen(1))
			Expect(envelope.Diagnostics[0].Path).To(BeEmpty(), "no valid repository-relative form exists for a rejected absolute --output value, so path must be omitted rather than leak the raw absolute value")
		})

		It("rejects a path with a .git component", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", "module example.com/gitcomponent\n\ngo 1.25\n")

			stdout, stderr, exitCode := runCoachSuggest(repo, "--baseline", "--suggest-project-config", "--output", ".git/project.json")
			Expect(exitCode).To(Equal(2), "stderr: %s", stderr)
			Expect(stdout).To(BeEmpty())
			Expect(string(stderr)).To(ContainSubstring("project_config_suggestion_output_invalid"))
		})

		It("rejects a .git path component regardless of case, without relying on filesystem case-folding", func() {
			// A single-segment target (not ".GIT/project.json") deliberately
			// avoids checkOutputParents' parent-existence Lstat: on this
			// case-sensitive Linux filesystem, a nested ".GIT/..." target
			// would already fail there (only ".git" exists on disk) even
			// without the case-insensitive shape check this test exists to
			// prove, which would be a false green. A bare "--output .GIT"
			// has zero parent segments to Lstat, so the only thing that can
			// reject it is validateSuggestOutputPathShape's segment check.
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", "module example.com/gitcomponentcase\n\ngo 1.25\n")

			stdout, stderr, exitCode := runCoachSuggest(repo, "--baseline", "--suggest-project-config", "--output", ".GIT")
			Expect(exitCode).To(Equal(2), "stderr: %s", stderr)
			Expect(stdout).To(BeEmpty())
			Expect(string(stderr)).To(ContainSubstring("project_config_suggestion_output_invalid"))

			// On case-insensitive filesystems (APFS/HFS+), Lstat(".GIT") resolves
			// to the existing .git directory — prove no regular-file candidate
			// was written, not that the path is absent.
			info, statErr := os.Lstat(filepath.Join(repo, ".GIT"))
			if statErr == nil {
				Expect(info.Mode().IsRegular()).To(BeFalse(), "must not write a regular-file candidate at the rejected .GIT path")
			} else {
				Expect(os.IsNotExist(statErr)).To(BeTrue(), "unexpected Lstat error for rejected .GIT path: %v", statErr)
			}
		})

		It("rejects a path through a symlinked parent directory", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", "module example.com/symlinkparent\n\ngo 1.25\n")

			elsewhere, err := os.MkdirTemp("", "coach-acceptance-elsewhere-*")
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(os.RemoveAll, elsewhere)
			Expect(os.Symlink(elsewhere, filepath.Join(repo, "linked"))).To(Succeed())

			stdout, stderr, exitCode := runCoachSuggest(repo, "--baseline", "--suggest-project-config", "--output", "linked/out.json")
			Expect(exitCode).To(Equal(2), "stderr: %s", stderr)
			Expect(stdout).To(BeEmpty())
			Expect(string(stderr)).To(ContainSubstring("project_config_suggestion_output_invalid"))
		})
	})

	When("the candidate and envelope JSON shapes are inspected directly", func() {
		It("uses 2-space indentation for the stdout candidate, and compact single-line NDJSON with the fixed key order for the stderr envelope", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", "module example.com/shape\n\ngo 1.25\n")

			stdout, stderr, exitCode := runCoachSuggest(repo, "--baseline", "--suggest-project-config")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			Expect(string(stdout)).To(Equal("{\n  \"schema_version\": \"1\",\n  \"roots\": [\n    \".\"\n  ]\n}\n"))

			text := string(stderr)
			Expect(strings.Count(text, "\n")).To(Equal(1), "expected exactly one newline, at the very end, for NDJSON: got %q", text)
			Expect(strings.HasSuffix(text, "\n")).To(BeTrue())
			Expect(text).NotTo(ContainSubstring("\n  \""), "expected compact single-line JSON, not pretty-printed multi-line JSON")
			order := []string{`"diagnostic_version"`, `"kind"`, `"revision"`, `"heuristic_version"`, `"roots_considered"`, `"coverage"`, `"diagnostics"`}
			lastIndex := -1
			for _, key := range order {
				idx := strings.Index(text, key)
				Expect(idx).To(BeNumerically(">", lastIndex), "expected %s to appear after the previous key in %s", key, text)
				lastIndex = idx
			}
		})
	})

	When("--help is requested even outside a Git worktree", func() {
		It("exits 0 and never attempts to scan", func() {
			directory, err := os.MkdirTemp("", "coach-acceptance-suggest-help-*")
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(os.RemoveAll, directory)

			command := exec.Command(commandPath, "codesignal", "--help")
			command.Dir = directory
			var stdout, stderr bytes.Buffer
			command.Stdout = &stdout
			command.Stderr = &stderr

			err = command.Run()
			Expect(err).NotTo(HaveOccurred(), "stderr: %s", stderr.String())
			Expect(stdout.String()).NotTo(BeEmpty())
			Expect(stderr.String()).To(BeEmpty())
		})
	})

	When("the coverage.phase field is inspected on both a successful and a failing envelope", func() {
		It("always reports the mandated project_config_suggestion phase, not go_root_discovery's own phase name", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", "module example.com/coveragephase\n\ngo 1.25\n")

			_, successStderr, successExit := runCoachSuggest(repo, "--baseline", "--suggest-project-config")
			Expect(successExit).To(Equal(0), "stderr: %s", successStderr)
			var successEnvelope suggestionEnvelopeDoc
			Expect(json.Unmarshal(successStderr, &successEnvelope)).To(Succeed(), "stderr: %s", successStderr)
			Expect(successEnvelope.Coverage.Phase).To(Equal("project_config_suggestion"), "a successful envelope's coverage.phase must not leak pkg/projectmodel's internal go_root_discovery phase name")

			emptyRepo := newTempGitRepo()
			commitFile(emptyRepo, "README.md", "no go here\n")
			_, failureStderr, failureExit := runCoachSuggest(emptyRepo, "--baseline", "--suggest-project-config")
			Expect(failureExit).To(Equal(2), "stderr: %s", failureStderr)
			var failureEnvelope suggestionEnvelopeDoc
			Expect(json.Unmarshal(failureStderr, &failureEnvelope)).To(Succeed(), "stderr: %s", failureStderr)
			Expect(failureEnvelope.Coverage.Phase).To(Equal("project_config_suggestion"))

			Expect(successEnvelope.Coverage.Phase).To(Equal(failureEnvelope.Coverage.Phase), "coverage.phase must not differ between a success and a failure envelope")
		})
	})

	When("a successful envelope's coverage has no supporting diagnostics", func() {
		It("emits a literal empty array, not an omitted key", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", "module example.com/coveragediagnostics\n\ngo 1.25\n")

			_, stderr, exitCode := runCoachSuggest(repo, "--baseline", "--suggest-project-config")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(string(stderr)).To(ContainSubstring(`"diagnostics":[]`), "coverage.diagnostics must be emitted as [] when there are no supporting details, not omitted; got %s", stderr)
		})
	})

	When("HEAD contains two Go modules and one module's go.mod blob is missing from the object store", func() {
		It("fails closed instead of silently emitting a smaller, wrongly 'ready' candidate", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", "module example.com/blobmissing\n\ngo 1.25\n")
			commitFile(repo, "services/payments/go.mod", "module example.com/blobmissing/services/payments\n\ngo 1.25\n")

			healthyStdout, healthyStderr, healthyExit := runCoachSuggest(repo, "--baseline", "--suggest-project-config")
			Expect(healthyExit).To(Equal(0), "stderr: %s", healthyStderr)
			var healthyCandidate suggestionCandidateDoc
			Expect(json.Unmarshal(healthyStdout, &healthyCandidate)).To(Succeed(), "stdout: %s", healthyStdout)
			Expect(healthyCandidate.Roots).To(Equal([]string{".", "services/payments"}), "sanity check: both roots must be discovered before the blob is deleted")

			blobCmd := exec.Command("git", "rev-parse", "HEAD:services/payments/go.mod")
			blobCmd.Dir = repo
			blobOutput, err := blobCmd.Output()
			Expect(err).NotTo(HaveOccurred())
			blob := strings.TrimSpace(string(blobOutput))
			Expect(os.Remove(filepath.Join(repo, ".git", "objects", blob[:2], blob[2:]))).To(Succeed())

			stdout, stderr, exitCode := runCoachSuggest(repo, "--baseline", "--suggest-project-config")
			Expect(exitCode).To(Equal(3), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stdout).To(BeEmpty(), "must not emit a candidate when a module's go.mod could not be read")

			var envelope suggestionEnvelopeDoc
			Expect(json.Unmarshal(stderr, &envelope)).To(Succeed(), "stderr: %s", stderr)
			Expect(envelope.Diagnostics).To(HaveLen(1))
			Expect(envelope.Diagnostics[0].Code).To(Equal("project_config_suggestion_snapshot_unavailable"), "must fail closed instead of silently reporting the ready diagnostic with a wrong, truncated root set")
			Expect(envelope.Diagnostics[0].Path).To(Equal("services/payments/go.mod"), "the diagnostic must identify the specific unreadable path")
		})
	})

	When("a candidate generated by --suggest-project-config --output is fed back into --project-config", func() {
		It("is accepted as schema-valid and produces a real project-analysis report", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", "module example.com/roundtrip\n\ngo 1.25\n")
			commitFile(repo, "services/payments/go.mod", "module example.com/roundtrip/services/payments\n\ngo 1.25\n")

			_, suggestStderr, suggestExit := runCoachSuggest(repo, "--baseline", "--suggest-project-config", "--output", "project.json")
			Expect(suggestExit).To(Equal(0), "stderr: %s", suggestStderr)

			// --project-config reads from the resolved Git revision, not the
			// worktree (the same immutable-snapshot contract --suggest-
			// project-config itself relies on), so the generated candidate
			// must be committed before it can be fed back in.
			addCmd := exec.Command("git", "add", "project.json")
			addCmd.Dir = repo
			addOutput, addErr := addCmd.CombinedOutput()
			Expect(addErr).NotTo(HaveOccurred(), "git add: %s", addOutput)
			commitCmd := exec.Command("git", "commit", "-m", "commit generated project.json")
			commitCmd.Dir = repo
			commitCmd.Env = commitEnv
			commitOutput, commitErr := commitCmd.CombinedOutput()
			Expect(commitErr).NotTo(HaveOccurred(), "git commit: %s", commitOutput)

			stdout, stderr, exitCode := runCoachSuggest(repo, "--baseline", "--project-config", "project.json", "--format=json")
			// The Go project-analysis backend is registered (unlike when this
			// test was first written), so a roots-only candidate (no layers/
			// forbidden_imports) now reaches real dispatch and produces a
			// genuine schema-2 report rather than merely proving dispatch was
			// attempted via a predictable exit-3 failure.
			Expect(exitCode).To(Equal(0), "stdout: %s stderr: %s", stdout, stderr)
			Expect(stderr).To(BeEmpty())
			Expect(string(stdout)).NotTo(ContainSubstring(`"kind":"project_config_invalid"`), "the generated candidate must not be rejected as schema-invalid by its own consumer")
			Expect(string(stdout)).NotTo(ContainSubstring(`"kind":"project_backend_unavailable"`))

			var report struct {
				SchemaVersion  string `json:"schema_version"`
				ProjectSummary *struct {
					ActiveChanges int `json:"active_changes"`
				} `json:"project_summary"`
				ProjectCoverage *struct {
					Phase    string `json:"phase"`
					Complete bool   `json:"complete"`
				} `json:"project_coverage"`
			}
			Expect(json.Unmarshal(stdout, &report)).To(Succeed(), "stdout: %s", stdout)
			Expect(report.SchemaVersion).To(Equal("2"), "project analysis must be enabled for the fed-back candidate")
			Expect(report.ProjectSummary).NotTo(BeNil())
			Expect(report.ProjectSummary.ActiveChanges).To(Equal(0), "a roots-only candidate declares no layers/forbidden_imports, so there is no policy to violate")
			Expect(report.ProjectCoverage).NotTo(BeNil())
			Expect(report.ProjectCoverage.Complete).To(BeTrue())
		})
	})
})
