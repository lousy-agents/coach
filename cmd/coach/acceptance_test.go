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

// runCoachCodesignalRaw runs `coach codesignal --base <base> [extraArgs...]`
// in repo, returning raw stdout/stderr without assuming success or a
// particular --format.
func runCoachCodesignalRaw(repo, base string, extraArgs ...string) (stdout, stderr []byte, exitCode int) {
	args := append([]string{"codesignal", "--base", base}, extraArgs...)
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

// removeFile deletes name from repo and commits the removal, returning the
// resulting commit's full SHA.
func removeFile(repo, name string) string {
	rmCmd := exec.Command("git", "rm", name)
	rmCmd.Dir = repo
	output, err := rmCmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "git rm: %s", output)

	commitCmd := exec.Command("git", "commit", "-m", "remove "+name)
	commitCmd.Dir = repo
	commitCmd.Env = commitEnv
	output, err = commitCmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "git commit: %s", output)

	revCmd := exec.Command("git", "rev-parse", "HEAD")
	revCmd.Dir = repo
	output, err = revCmd.Output()
	Expect(err).NotTo(HaveOccurred())

	return string(bytes.TrimSpace(output))
}

// runCoachCodesignal builds and runs `coach codesignal --base <base>` in
// repo, decoding stdout as one codesignal.Report.
func runCoachCodesignal(repo, base string) (*codesignal.Report, string) {
	command := exec.Command(commandPath, "codesignal", "--base", base, "--format=json")
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

func signalsForPath(report *codesignal.Report, path string) []codesignal.Signal {
	var matches []codesignal.Signal
	for _, sig := range report.Signals {
		if sig.Path == path {
			matches = append(matches, sig)
		}
	}
	return matches
}

func hasDiagnostic(report *codesignal.Report, kind, path string) bool {
	for _, d := range report.Diagnostics {
		if d.Kind == kind && d.Path == path {
			return true
		}
	}
	return false
}

var _ = Describe("coach codesignal", func() {
	When("--base is not provided", func() {
		It("prints usage guidance to stderr and exits 2 without writing to stdout", func() {
			repo := newTempGitRepo()

			command := exec.Command(commandPath, "codesignal")
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
			Expect(stderr.String()).NotTo(BeEmpty())
			Expect(stderr.String()).To(ContainSubstring("--base"))
		})
	})

	When("run in a directory that is not a Git worktree", func() {
		It("exits 1, writes an actionable message to stderr, and writes nothing to stdout", func() {
			directory, err := os.MkdirTemp("", "coach-acceptance-notgit-*")
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(os.RemoveAll, directory)

			command := exec.Command(commandPath, "codesignal", "--base", "HEAD")
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

	When("--base cannot be resolved to a commit", func() {
		It("exits 1, writes an actionable message to stderr, and writes nothing to stdout", func() {
			repo := newTempGitRepo()
			commitFile(repo, "a.go", "package a\n")

			command := exec.Command(commandPath, "codesignal", "--base", "doesnotexist12345")
			command.Dir = repo
			var stdout, stderr bytes.Buffer
			command.Stdout = &stdout
			command.Stderr = &stderr

			err := command.Run()

			var exitErr *exec.ExitError
			Expect(err).To(HaveOccurred())
			Expect(errors.As(err, &exitErr)).To(BeTrue())
			Expect(exitErr.ExitCode()).To(Equal(1))
			Expect(stdout.Bytes()).To(BeEmpty())
			Expect(stderr.String()).NotTo(BeEmpty())
		})
	})

	When("--base resolves and there are commits since it", func() {
		It("exits 0", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "a.go", "package a\n")
			commitFile(repo, "b.go", "package a\n\nfunc B() {}\n")

			command := exec.Command(commandPath, "codesignal", "--base", initialSHA)
			command.Dir = repo
			var stdout, stderr bytes.Buffer
			command.Stdout = &stdout
			command.Stderr = &stderr

			err := command.Run()

			Expect(err).NotTo(HaveOccurred(), "stderr: %s", stderr.String())
		})
	})

	When("head introduces a hidden-input-mutation finding not present at base", func() {
		It("reports exactly one introduced signal", func() {
			repo := newTempGitRepo()
			base := "package a\n\nfunc Get(input *int) int {\n\treturn *input\n}\n"
			head := base + "\nfunc Update(input *int) {\n\t*input = 1\n}\n"
			initialSHA := commitFile(repo, "a.go", base)
			commitFile(repo, "a.go", head)

			report, _ := runCoachCodesignal(repo, initialSHA)

			signals := signalsForPath(report, "a.go")
			Expect(signals).To(HaveLen(1))
			Expect(signals[0].Lifecycle).To(Equal(codesignal.Lifecycle("introduced")))
		})
	})

	When("the same hidden-input-mutation finding is present at base and head", func() {
		It("reports the signal as existing", func() {
			repo := newTempGitRepo()
			base := "package a\n\nfunc Update(input *int) {\n\t*input = 1\n}\n"
			head := "package a\n\n// note\nfunc Update(input *int) {\n\t*input = 1\n}\n"
			initialSHA := commitFile(repo, "a.go", base)
			commitFile(repo, "a.go", head)

			report, _ := runCoachCodesignal(repo, initialSHA)

			signals := signalsForPath(report, "a.go")
			Expect(signals).To(HaveLen(1))
			Expect(signals[0].Lifecycle).To(Equal(codesignal.Lifecycle("existing")))
		})
	})

	When("a file with a hidden-input-mutation finding is removed at head", func() {
		It("reports the signal as resolved", func() {
			repo := newTempGitRepo()
			base := "package a\n\nfunc Update(input *int) {\n\t*input = 1\n}\n"
			initialSHA := commitFile(repo, "a.go", base)
			removeFile(repo, "a.go")

			report, _ := runCoachCodesignal(repo, initialSHA)

			signals := signalsForPath(report, "a.go")
			Expect(signals).To(HaveLen(1))
			Expect(signals[0].Lifecycle).To(Equal(codesignal.Lifecycle("resolved")))
		})
	})

	When("a change inserts a new function alongside an untouched one", func() {
		It("marks the inserted finding changed and the untouched finding unchanged", func() {
			repo := newTempGitRepo()
			base := "package a\n\nfunc A(input *int) {\n\t*input = 1\n}\n\nfunc B(input *int) {\n\t*input = 2\n}\n"
			head := "package a\n\nfunc A(input *int) {\n\t*input = 1\n}\n\nfunc C(input *int) {\n\t*input = 3\n}\n\nfunc B(input *int) {\n\t*input = 2\n}\n"
			initialSHA := commitFile(repo, "a.go", base)
			commitFile(repo, "a.go", head)

			report, _ := runCoachCodesignal(repo, initialSHA)

			signals := signalsForPath(report, "a.go")
			Expect(signals).To(HaveLen(3))

			var aSignal, bSignal, cSignal *codesignal.Signal
			for i := range signals {
				switch signals[i].Subject {
				case "A:input":
					aSignal = &signals[i]
				case "B:input":
					bSignal = &signals[i]
				case "C:input":
					cSignal = &signals[i]
				}
			}
			Expect(aSignal).NotTo(BeNil())
			Expect(bSignal).NotTo(BeNil())
			Expect(cSignal).NotTo(BeNil())

			Expect(aSignal.Changed).To(BeFalse())
			Expect(bSignal.Changed).To(BeFalse())
			Expect(cSignal.Changed).To(BeTrue())
			Expect(cSignal.Lifecycle).To(Equal(codesignal.Lifecycle("introduced")))
		})
	})

	When("head content has a syntax error", func() {
		It("reports a syntax_errors diagnostic and no signals for that file", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "a.go", "package a\n\nfunc A() {}\n")
			commitFile(repo, "a.go", "package a\n\nfunc B(\n")

			report, _ := runCoachCodesignal(repo, initialSHA)

			Expect(hasDiagnostic(report, "syntax_errors", "a.go")).To(BeTrue())
			Expect(signalsForPath(report, "a.go")).To(BeEmpty())
		})
	})

	When("base content has a syntax error but head is clean", func() {
		It("reports the head signals as unknown lifecycle plus a base_syntax_errors diagnostic", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "a.go", "package a\n\nfunc B(\n")
			commitFile(repo, "a.go", "package a\n\nfunc Update(input *int) {\n\t*input = 1\n}\n")

			report, _ := runCoachCodesignal(repo, initialSHA)

			Expect(hasDiagnostic(report, "base_syntax_errors", "a.go")).To(BeTrue())
			signals := signalsForPath(report, "a.go")
			Expect(signals).To(HaveLen(1))
			Expect(signals[0].Lifecycle).To(Equal(codesignal.Lifecycle("unknown")))
		})
	})

	When("one selected file fails analysis alongside a healthy file", func() {
		It("still analyzes the healthy file and reports a diagnostic for the failed one", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "healthy.go", "package a\n\nfunc Get(input *int) int { return *input }\n")
			commitFile(repo, "healthy.go", "package a\n\nfunc Update(input *int) {\n\t*input = 1\n}\n")
			commitFile(repo, "empty.go", "")

			report, _ := runCoachCodesignal(repo, initialSHA)

			Expect(hasDiagnostic(report, "empty_content", "empty.go")).To(BeTrue())
			signals := signalsForPath(report, "healthy.go")
			Expect(signals).To(HaveLen(1))
			Expect(signals[0].Lifecycle).To(Equal(codesignal.Lifecycle("introduced")))
		})
	})

	When("--format is omitted and an introduced signal is present", func() {
		It("renders a text report with all required labels and no ANSI escapes", func() {
			repo := newTempGitRepo()
			base := "package a\n\nfunc Get(input *int) int {\n\treturn *input\n}\n"
			head := base + "\nfunc Update(input *int) {\n\t*input = 1\n}\n"
			initialSHA := commitFile(repo, "a.go", base)
			commitFile(repo, "a.go", head)

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, initialSHA)
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			text := string(stdout)
			Expect(text).To(ContainSubstring("files analyzed"))
			Expect(text).To(ContainSubstring("active signals"))
			Expect(text).To(ContainSubstring("diagnostics"))
			Expect(text).To(ContainSubstring("path: a.go"))
			Expect(text).To(ContainSubstring("line: 8"))
			Expect(text).To(ContainSubstring("lifecycle: introduced"))
			Expect(text).To(ContainSubstring("changed: true"))
			Expect(text).To(ContainSubstring("evidence"))
			Expect(text).To(ContainSubstring("why it matters"))
			Expect(text).To(ContainSubstring("recommendation"))
			Expect(text).NotTo(ContainSubstring("\x1b["))
		})
	})

	When("--format=text and there are no active signals but there is a diagnostic", func() {
		It("renders the diagnostics section and the exact no-findings sentence", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "a.go", "package a\n\nfunc A() {}\n")
			commitFile(repo, "a.go", "package a\n\n// updated\nfunc A() {}\n")
			commitFile(repo, "empty.go", "")

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, initialSHA, "--format=text")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			text := string(stdout)
			Expect(text).To(ContainSubstring("No active CodeSignal findings."))
			Expect(text).To(ContainSubstring("empty.go"))
			Expect(text).To(ContainSubstring("empty_content"))
		})
	})

	When("--format=json", func() {
		It("writes exactly one JSON document followed by exactly one newline", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "a.go", "package a\n\nfunc A() {}\n")
			commitFile(repo, "b.go", "package a\n\nfunc B() {}\n")

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, initialSHA, "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			Expect(bytes.Count(stdout, []byte("\n"))).To(Equal(1))

			var report codesignal.Report
			decoder := json.NewDecoder(bytes.NewReader(stdout))
			Expect(decoder.Decode(&report)).To(Succeed())
			Expect(decoder.More()).To(BeFalse())
		})
	})

	When("--format is an unrecognized value", func() {
		It("exits 2, prints usage guidance to stderr, and writes nothing to stdout", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "a.go", "package a\n\nfunc A() {}\n")

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, initialSHA, "--format=xml")

			Expect(exitCode).To(Equal(2))
			Expect(stdout).To(BeEmpty())
			Expect(stderr).NotTo(BeEmpty())
		})
	})

	When("the same two commits are analyzed in two independent temp worktrees", func() {
		It("produces byte-identical JSON output", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "a.go", "package a\n\nfunc Get(input *int) int {\n\treturn *input\n}\n")
			commitFile(repo, "a.go", "package a\n\nfunc Get(input *int) int {\n\treturn *input\n}\n\nfunc Update(input *int) {\n\t*input = 1\n}\n")

			firstRun, stderr1, exitCode1 := runCoachCodesignalRaw(repo, initialSHA, "--format=json")
			Expect(exitCode1).To(Equal(0), "stderr: %s", stderr1)

			secondRun, stderr2, exitCode2 := runCoachCodesignalRaw(repo, initialSHA, "--format=json")
			Expect(exitCode2).To(Equal(0), "stderr: %s", stderr2)

			Expect(firstRun).To(Equal(secondRun))
		})
	})
})
