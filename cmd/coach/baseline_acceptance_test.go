package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"

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

	When("--output is given without --suggest-project-config", func() {
		It("exits 2, writes nothing to stdout, writes no file, and rejects --output instead of silently ignoring it", func() {
			repo := newTempGitRepo()
			commitFile(repo, "a.go", "package a\n")

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--output", "whatever.json")

			Expect(exitCode).To(Equal(2))
			Expect(stdout).To(BeEmpty())
			Expect(string(stderr)).To(ContainSubstring("--output"))

			_, statErr := os.Stat(filepath.Join(repo, "whatever.json"))
			Expect(os.IsNotExist(statErr)).To(BeTrue(), "--output must not write a file when --suggest-project-config was not requested")
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
			body_baselineAcceptanceTest_summarizesTheUnsupportedFileViaCoverageInsteadOf_164()
		})
	})

	// Story 3 (cognitive-complexity.spec.md): coach codesignal surfaces
	// complexity.cognitive_complexity without new flags when a function
	// scores at or above the codesignal threshold (15).
	When("a tracked Go file contains a function with Cognitive Complexity >= 15", func() {
		It("shall include complexity.cognitive_complexity in baseline JSON with locked evidence shape", func() {
			body_baselineAcceptanceTest_shallIncludeComplexityCognitiveComplexityInBasel_192()
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
			body_baselineAcceptanceTest_excludesATestOnlyGoFileFromSignalsAndAccountsFor_252()
		})
	})

	When("--scope all is given for a Repository Baseline scan", func() {
		It("records the resolved --scope value in scope.applied_scope", func() {
			repo := newTempGitRepo()
			commitFile(repo, "a.go", "package a\n\nfunc A() {}\n")

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--scope", "all", "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var report codesignal.Report
			Expect(json.Unmarshal(stdout, &report)).To(Succeed(), "stdout should be one JSON report: %s", stdout)

			Expect(report.Scope.AppliedScope).To(Equal("all"))
		})
	})

	When("--scope is omitted for a Repository Baseline scan", func() {
		It("records the CLI's default applied scope value in scope.applied_scope", func() {
			repo := newTempGitRepo()
			commitFile(repo, "a.go", "package a\n\nfunc A() {}\n")

			report, stderr := runCoachCodesignalBaseline(repo)
			Expect(stderr).To(BeEmpty())

			Expect(report.Scope.AppliedScope).To(Equal("production"), "the default --scope value used by diff mode must also be recorded for baseline mode")
		})
	})
})
