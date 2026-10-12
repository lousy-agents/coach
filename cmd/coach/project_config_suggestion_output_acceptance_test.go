package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --baseline --suggest-project-config", func() {
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
})
