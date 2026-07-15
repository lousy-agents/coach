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

// runCoachCodesignalBaselineRaw runs `coach codesignal --baseline
// [extraArgs...]` in repo, returning raw stdout/stderr without assuming
// success or a particular --format.
func runCoachCodesignalBaselineRaw(repo string, extraArgs ...string) (stdout, stderr []byte, exitCode int) {
	args := append([]string{"codesignal", "--baseline"}, extraArgs...)
	command := exec.Command(commandPath, args...)
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

// runCoachCodesignalBaseline builds and runs `coach codesignal --baseline
// --format=json` in repo, decoding stdout as one codesignal.Report.
func runCoachCodesignalBaseline(repo string) (*codesignal.Report, string) {
	command := exec.Command(commandPath, "codesignal", "--baseline", "--format=json")
	command.Dir = repo
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	err := command.Run()
	Expect(err).NotTo(HaveOccurred(), "stderr: %s", stderr.String())

	var report codesignal.Report
	Expect(json.Unmarshal(stdout.Bytes(), &report)).To(Succeed(), "stdout should be one JSON report: %s", stdout.String())

	return &report, stderr.String()
}

var _ = Describe("coach codesignal --baseline", func() {
	When("--baseline is combined with --base", func() {
		It("exits 2, writes nothing to stdout, and explains they are mutually exclusive", func() {
			repo := newTempGitRepo()
			commitFile(repo, "a.go", "package a\n")

			command := exec.Command(commandPath, "codesignal", "--baseline", "--base", "HEAD")
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
			Expect(stderr.String()).To(ContainSubstring("mutually exclusive"))
		})
	})

	When("run in a directory that is not a Git worktree", func() {
		It("exits 1, writes an actionable message to stderr, and writes nothing to stdout", func() {
			directory, err := os.MkdirTemp("", "coach-acceptance-notgit-baseline-*")
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(os.RemoveAll, directory)

			command := exec.Command(commandPath, "codesignal", "--baseline")
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

	When("HEAD cannot be read because a worktree has no commits", func() {
		It("exits 1, writes one actionable operational error to stderr, and writes nothing to stdout", func() {
			repo := newTempGitRepo()

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo)

			Expect(exitCode).To(Equal(1))
			Expect(stdout).To(BeEmpty())
			Expect(string(stderr)).To(ContainSubstring("HEAD is not readable"))
		})
	})

	When("a file committed once at the very first commit is never touched again", func() {
		It("still includes it in a Repository Baseline scan with baseline lifecycle", func() {
			repo := newTempGitRepo()
			commitFile(repo, "a.go", "package a\n\nfunc Update(input *int) {\n\t*input = 1\n}\n")
			commitFile(repo, "b.go", "package a\n\nfunc B() {}\n")

			report, stderr := runCoachCodesignalBaseline(repo)
			Expect(stderr).To(BeEmpty())

			signals := signalsForPath(report, "a.go")
			Expect(signals).To(HaveLen(1), "a file untouched since the first commit must still be found in a baseline scan")
			Expect(signals[0].Lifecycle).To(Equal(codesignal.Lifecycle("baseline")))
			Expect(signals[0].Lifecycle).NotTo(Equal(codesignal.Lifecycle("introduced")))
			Expect(signals[0].Lifecycle).NotTo(Equal(codesignal.Lifecycle("existing")))
			Expect(signals[0].Lifecycle).NotTo(Equal(codesignal.Lifecycle("resolved")))
			Expect(signals[0].Lifecycle).NotTo(Equal(codesignal.Lifecycle("unknown")))
		})
	})

	When("a Repository Baseline scan completes", func() {
		It("reports scope.baseline true and the resolved HEAD revision", func() {
			repo := newTempGitRepo()
			commitFile(repo, "a.go", "package a\n\nfunc Update(input *int) {\n\t*input = 1\n}\n")
			headSHA := commitFile(repo, "b.go", "package a\n\nfunc B() {}\n")

			report, stderr := runCoachCodesignalBaseline(repo)
			Expect(stderr).To(BeEmpty())

			Expect(report.Scope.Baseline).To(BeTrue())
			Expect(report.Scope.Revision).To(Equal(headSHA))
		})
	})

	When("the tracked tree contains a supported and an unsupported file", func() {
		It("summarizes the unsupported file via Coverage instead of a per-file diagnostic", func() {
			repo := newTempGitRepo()
			commitFile(repo, "a.go", "package a\n\nfunc A() {}\n")
			commitFile(repo, "notes.txt", "hello\n")

			report, stderr := runCoachCodesignalBaseline(repo)
			Expect(stderr).To(BeEmpty())

			Expect(report.Coverage).NotTo(BeNil())
			Expect(report.Coverage.TrackedFilesDiscovered).To(Equal(2))
			Expect(report.Coverage.FilesAnalyzed).To(Equal(1))

			foundUnsupported := false
			for _, g := range report.Coverage.Unsupported {
				if g.Reason == "unsupported_language" && g.Count >= 1 {
					foundUnsupported = true
				}
			}
			Expect(foundUnsupported).To(BeTrue(), "notes.txt should be accounted for in Coverage.Unsupported")

			Expect(hasDiagnostic(report, "unsupported_language", "notes.txt")).To(BeFalse(), "a baseline scan must not flood the report with a per-file unsupported_language diagnostic")
		})
	})

	When("--format is omitted (text) for a Repository Baseline scan", func() {
		It("identifies the report as a repository baseline and never implies a comparison lifecycle", func() {
			repo := newTempGitRepo()
			commitFile(repo, "a.go", "package a\n\nfunc Update(input *int) {\n\t*input = 1\n}\n")
			headSHA := commitFile(repo, "b.go", "package a\n\nfunc B() {}\n")

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo)
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			text := string(stdout)
			Expect(text).To(ContainSubstring("Repository Baseline"))
			Expect(text).To(ContainSubstring(headSHA))
			Expect(text).NotTo(ContainSubstring("introduced"))
			Expect(text).NotTo(ContainSubstring("resolved"))
		})
	})

	When("--scope=production is the default for a Repository Baseline scan", func() {
		It("excludes a test-only Go file from signals and accounts for it in Coverage.Excluded", func() {
			repo := newTempGitRepo()
			commitFile(repo, "shipping/shipping.go", "package shipping\n\nfunc Update(input *int) {\n\t*input = 1\n}\n")
			commitFile(repo, "shipping/shipping_test.go", "package shipping\n\nfunc TestUpdate(input *int) {\n\t*input = 1\n}\n")

			report, stderr := runCoachCodesignalBaseline(repo)
			Expect(stderr).To(BeEmpty())

			Expect(signalsForPath(report, "shipping/shipping.go")).To(HaveLen(1))
			Expect(signalsForPath(report, "shipping/shipping_test.go")).To(BeEmpty(), "default production scope must exclude a test-only Go file from signals")

			Expect(report.Coverage).NotTo(BeNil())
			foundExcluded := false
			for _, g := range report.Coverage.Excluded {
				if g.Reason == "test_only" && g.Language == "go" {
					foundExcluded = true
				}
			}
			Expect(foundExcluded).To(BeTrue(), "the excluded test-only Go file must be accounted for in Coverage.Excluded")
		})
	})
})
