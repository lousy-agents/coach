package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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

// sourceScopeForPath reads the customer-facing source_scope emitted with a
// signal. It intentionally decodes the public JSON document rather than a Go
// report type so this acceptance suite requires the label to be serialized.
func sourceScopeForPath(stdout []byte, path string) string {
	var document struct {
		Signals []struct {
			Path        string `json:"path"`
			SourceScope string `json:"source_scope"`
		} `json:"signals"`
	}
	Expect(json.Unmarshal(stdout, &document)).To(Succeed(), "stdout should be a JSON CodeSignal report: %s", stdout)

	for _, signal := range document.Signals {
		if signal.Path == path {
			return signal.SourceScope
		}
	}
	return ""
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

	When("HEAD cannot be read because a worktree has no commits", func() {
		It("exits 1, writes one actionable operational error to stderr, and writes nothing to stdout", func() {
			repo := newTempGitRepo()

			command := exec.Command(commandPath, "codesignal", "--base", "HEAD")
			command.Dir = repo
			var stdout, stderr bytes.Buffer
			command.Stdout = &stdout
			command.Stderr = &stderr

			err := command.Run()
			var exitErr *exec.ExitError
			Expect(errors.As(err, &exitErr)).To(BeTrue())
			Expect(exitErr.ExitCode()).To(Equal(1))
			Expect(stdout.Bytes()).To(BeEmpty())
			Expect(stderr.String()).To(ContainSubstring("HEAD is not readable"))
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

	When("the comparison contains no supported changed files", func() {
		It("prints a completed no-findings summary and exits 0", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "notes.txt", "before\n")
			commitFile(repo, "notes.txt", "after\n")

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, initialSHA)

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(string(stdout)).To(ContainSubstring("No active CodeSignal findings."))
			Expect(string(stdout)).To(ContainSubstring("unsupported_language"))
		})
	})

	When("the working tree has uncommitted source changes", func() {
		It("analyzes only merge-base through HEAD and excludes the uncommitted changes", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "a.go", "package a\n\nfunc A() {}\n")
			commitFile(repo, "a.go", "package a\n\nfunc A() {}\n\n// committed context\n")
			Expect(os.WriteFile(filepath.Join(repo, "a.go"), []byte("package a\n\nfunc Update(input *int) {\n\t*input = 1\n}\n"), 0o644)).To(Succeed())

			report, _ := runCoachCodesignal(repo, initialSHA)

			Expect(report.Signals).To(BeEmpty(), "uncommitted changes must not be included in the revision comparison")
		})
	})

	When("a Go build target has changed production, test-only, unreachable, and build-tag-excluded files", func() {
		It("defaults to production scope and reports only findings that ship in the selected build target", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "go.mod", "module example.com/scopefixture\n\ngo 1.25.0\n")
			commitFile(repo, "cmd/app/main.go", "package main\n\nimport _ \"example.com/scopefixture/shipping\"\n\nfunc main() {}\n")
			commitFile(repo, "shipping/shipping.go", "package shipping\n\nfunc Update(input *int) {}\n")
			commitFile(repo, "shipping/shipping_test.go", "package shipping\n\nfunc TestUpdate(input *int) {}\n")
			commitFile(repo, "internal/tool/tool.go", "package tool\n\nfunc Update(input *int) {}\n")
			commitFile(repo, "shipping/disabled.go", "//go:build never\n\npackage shipping\n\nfunc Update(input *int) {}\n")

			commitFile(repo, "shipping/shipping.go", "package shipping\n\nfunc Update(input *int) {\n\t*input = 1\n}\n")
			commitFile(repo, "shipping/shipping_test.go", "package shipping\n\nfunc TestUpdate(input *int) {\n\t*input = 1\n}\n")
			commitFile(repo, "internal/tool/tool.go", "package tool\n\nfunc Update(input *int) {\n\t*input = 1\n}\n")
			commitFile(repo, "shipping/disabled.go", "//go:build never\n\npackage shipping\n\nfunc Update(input *int) {\n\t*input = 1\n}\n")

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, initialSHA, "--build-target", "./cmd/app", "--format=json")
			Expect(exitCode).To(Equal(0), "production scope should analyze the selected build target; stderr: %s", stderr)

			var report codesignal.Report
			Expect(json.Unmarshal(stdout, &report)).To(Succeed())
			Expect(signalsForPath(&report, "shipping/shipping.go")).To(HaveLen(1), "the production dependency must remain visible")
			Expect(signalsForPath(&report, "shipping/shipping_test.go")).To(BeEmpty(), "test-only findings must not appear in the default production scope")
			Expect(signalsForPath(&report, "internal/tool/tool.go")).To(BeEmpty(), "a package outside the selected build target must not be claimed to ship")
			Expect(signalsForPath(&report, "shipping/disabled.go")).To(BeEmpty(), "a file excluded by its Go build constraint must not be claimed to ship")
			Expect(sourceScopeForPath(stdout, "shipping/shipping.go")).To(Equal("production"), "JSON must identify why a visible finding is in the production report")
		})

		It("includes the complete changed source set when the user explicitly requests all scope", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "go.mod", "module example.com/scopeallfixture\n\ngo 1.25.0\n")
			commitFile(repo, "cmd/app/main.go", "package main\n\nimport _ \"example.com/scopeallfixture/shipping\"\n\nfunc main() {}\n")
			commitFile(repo, "shipping/shipping.go", "package shipping\n\nfunc Update(input *int) {}\n")
			commitFile(repo, "shipping/shipping_test.go", "package shipping\n\nfunc TestUpdate(input *int) {}\n")

			commitFile(repo, "shipping/shipping.go", "package shipping\n\nfunc Update(input *int) {\n\t*input = 1\n}\n")
			commitFile(repo, "shipping/shipping_test.go", "package shipping\n\nfunc TestUpdate(input *int) {\n\t*input = 1\n}\n")

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, initialSHA, "--scope=all", "--format=json")
			Expect(exitCode).To(Equal(0), "all scope should retain exploratory findings; stderr: %s", stderr)

			var report codesignal.Report
			Expect(json.Unmarshal(stdout, &report)).To(Succeed())
			Expect(signalsForPath(&report, "shipping/shipping.go")).To(HaveLen(1))
			Expect(signalsForPath(&report, "shipping/shipping_test.go")).To(HaveLen(1), "all scope must include test-only findings for exploratory use")
			Expect(sourceScopeForPath(stdout, "shipping/shipping_test.go")).To(Equal("test_only"), "all scope must identify that the extra finding is test-only")
		})
	})

	When("a TypeScript project defines its production compilation in tsconfig.json", func() {
		It("uses that production configuration by default and excludes changed test-only findings", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "tsconfig.json", "{\"include\":[\"src/**/*.ts\"],\"exclude\":[\"test/**/*.ts\"]}\n")
			commitFile(repo, "src/app.ts", "interface Config { name: string }\n\nexport function updateName(cfg: Config, name: string): void {}\n")
			commitFile(repo, "test/app.test.ts", "interface Config { name: string }\n\nexport function updateName(cfg: Config, name: string): void {}\n")

			commitFile(repo, "src/app.ts", "interface Config { name: string }\n\nexport function updateName(cfg: Config, name: string): void {\n  cfg.name = name;\n}\n")
			commitFile(repo, "test/app.test.ts", "interface Config { name: string }\n\nexport function updateName(cfg: Config, name: string): void {\n  cfg.name = name;\n}\n")

			report, _ := runCoachCodesignal(repo, initialSHA)

			Expect(signalsForPath(report, "src/app.ts")).To(HaveLen(1), "a file included by tsconfig.json must remain visible in production scope")
			Expect(signalsForPath(report, "test/app.test.ts")).To(BeEmpty(), "a file excluded by tsconfig.json must not appear in default production scope")
		})

		It("uses tsconfig.json membership for TSX files as well as TypeScript files", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "tsconfig.json", "{\"include\":[\"src/**/*.tsx\"],\"exclude\":[\"test/**/*.tsx\"]}\n")
			commitFile(repo, "src/panel.tsx", "interface Config { name: string }\n\nexport function updateName(cfg: Config, name: string): void {}\n")
			commitFile(repo, "test/panel.test.tsx", "interface Config { name: string }\n\nexport function updateName(cfg: Config, name: string): void {}\n")

			commitFile(repo, "src/panel.tsx", "interface Config { name: string }\n\nexport function updateName(cfg: Config, name: string): void {\n  cfg.name = name;\n}\n")
			commitFile(repo, "test/panel.test.tsx", "interface Config { name: string }\n\nexport function updateName(cfg: Config, name: string): void {\n  cfg.name = name;\n}\n")

			report, _ := runCoachCodesignal(repo, initialSHA)

			Expect(signalsForPath(report, "src/panel.tsx")).To(HaveLen(1), "a TSX file included by tsconfig.json must remain visible in production scope")
			Expect(signalsForPath(report, "test/panel.test.tsx")).To(BeEmpty(), "a TSX test file excluded by tsconfig.json must not appear in default production scope")
		})
	})

	When("production membership cannot be established for a changed supported file", func() {
		It("fails open by retaining the finding and labelling it unknown in JSON", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "orphan.ts", "interface Config { name: string }\n\nexport function updateName(cfg: Config, name: string): void {}\n")
			commitFile(repo, "orphan.ts", "interface Config { name: string }\n\nexport function updateName(cfg: Config, name: string): void {\n  cfg.name = name;\n}\n")

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, initialSHA, "--format=json")
			Expect(exitCode).To(Equal(0), "a file with unknown scope must remain an advisory result; stderr: %s", stderr)
			var report codesignal.Report
			Expect(json.Unmarshal(stdout, &report)).To(Succeed())
			Expect(signalsForPath(&report, "orphan.ts")).To(HaveLen(1), "unknown scope must not silently hide a supported finding")
			Expect(sourceScopeForPath(stdout, "orphan.ts")).To(Equal("unknown"), "JSON must make the uncertain production membership explicit to automation")
		})
	})

	When("--scope is neither production nor all", func() {
		It("prints scope-specific usage guidance to stderr and exits 2 without a report", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "a.go", "package a\n")

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, initialSHA, "--scope=review")

			Expect(exitCode).To(Equal(2))
			Expect(stdout).To(BeEmpty())
			Expect(string(stderr)).To(ContainSubstring("--scope"))
			Expect(string(stderr)).To(ContainSubstring("production"))
			Expect(string(stderr)).To(ContainSubstring("all"))
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
		It("reports a syntax_errors diagnostic, no signals for that file, and exits 0", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "a.go", "package a\n\nfunc A() {}\n")
			commitFile(repo, "a.go", "package a\n\nfunc B(\n")

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, initialSHA, "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var report codesignal.Report
			Expect(json.Unmarshal(stdout, &report)).To(Succeed())

			Expect(hasDiagnostic(&report, "syntax_errors", "a.go")).To(BeTrue())
			Expect(signalsForPath(&report, "a.go")).To(BeEmpty())
			Expect(report.Signals).To(BeEmpty(), "a report with only diagnostics and zero signals is a normal, exit-0 outcome")
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

	When("one selected file fails analysis alongside a healthy file that introduces a signal", func() {
		It("exits 0, continues analyzing the healthy file, and reports a diagnostic for the failed one", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "healthy.go", "package a\n\nfunc Get(input *int) int { return *input }\n")
			commitFile(repo, "healthy.go", "package a\n\nfunc Update(input *int) {\n\t*input = 1\n}\n")
			commitFile(repo, "empty.go", "")

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, initialSHA, "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var report codesignal.Report
			Expect(json.Unmarshal(stdout, &report)).To(Succeed())

			Expect(hasDiagnostic(&report, "empty_content", "empty.go")).To(BeTrue())
			signals := signalsForPath(&report, "healthy.go")
			Expect(signals).To(HaveLen(1))
			Expect(signals[0].Lifecycle).To(Equal(codesignal.Lifecycle("introduced")))
		})
	})

	When("a selected Go file contains binary bytes", func() {
		It("records a binary_content diagnostic, prints a report, and exits 0", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "a.go", "package a\n")
			commitFile(repo, "binary.go", "package binary\x00")

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, initialSHA, "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			var report codesignal.Report
			Expect(json.Unmarshal(stdout, &report)).To(Succeed())
			Expect(hasDiagnostic(&report, "binary_content", "binary.go")).To(BeTrue())
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

	When("a signal is rendered in both text and JSON formats", func() {
		It("uses a 1-based text line while retaining the 0-based JSON location and renders diagnostics after signals", func() {
			repo := newTempGitRepo()
			base := "package a\n\nfunc Get(input *int) int {\n\treturn *input\n}\n"
			head := base + "\nfunc Update(input *int) {\n\t*input = 1\n}\n"
			initialSHA := commitFile(repo, "a.go", base)
			commitFile(repo, "a.go", head)
			commitFile(repo, "empty.go", "")

			text, textStderr, textExitCode := runCoachCodesignalRaw(repo, initialSHA)
			Expect(textExitCode).To(Equal(0), "stderr: %s", textStderr)
			Expect(string(text)).To(ContainSubstring("line: 8"))
			Expect(strings.Index(string(text), "path: a.go")).To(BeNumerically("<", strings.Index(string(text), "Diagnostics:")))

			jsonOutput, jsonStderr, jsonExitCode := runCoachCodesignalRaw(repo, initialSHA, "--format=json")
			Expect(jsonExitCode).To(Equal(0), "stderr: %s", jsonStderr)
			var report codesignal.Report
			Expect(json.Unmarshal(jsonOutput, &report)).To(Succeed())
			signals := signalsForPath(&report, "a.go")
			Expect(signals).To(HaveLen(1))
			Expect(signals[0].Location.StartRow).To(Equal(uint(7)))
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
		It("writes exactly one unwrapped report document followed by exactly one newline", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "a.go", "package a\n\nfunc A() {}\n")
			commitFile(repo, "b.go", "package a\n\nfunc Update(input *int) {\n\t*input = 1\n}\n")
			headSHA := commitFile(repo, "binary.go", "package binary\x00")

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, initialSHA, "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			Expect(bytes.Count(stdout, []byte("\n"))).To(Equal(1))

			var report codesignal.Report
			decoder := json.NewDecoder(bytes.NewReader(stdout))
			Expect(decoder.Decode(&report)).To(Succeed())
			Expect(decoder.More()).To(BeFalse())
			Expect(report.SchemaVersion).NotTo(BeEmpty())
			Expect(report.Scope.Repository).To(BeEmpty())
			Expect(report.Scope.Revision).To(Equal(headSHA))
			Expect(report.Scope.Base).To(Equal(initialSHA))

			var document map[string]json.RawMessage
			Expect(json.Unmarshal(stdout, &document)).To(Succeed())
			Expect(document).To(HaveKey("schema_version"))
			Expect(document).To(HaveKey("scope"))
			Expect(document).To(HaveKey("summary"))
			Expect(document).To(HaveKey("signals"))
			Expect(document).To(HaveKey("diagnostics"))
			Expect(document).To(HaveLen(5), "JSON mode must not add CLI-only fields around codesignal.Report")
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

	When("run with no GitHub token, no model/LLM API key, and no other external service config", func() {
		It("still exits 0 with a valid report, proving zero external service configuration is required", func() {
			repo := newTempGitRepo()
			base := "package a\n\nfunc Get(input *int) int {\n\treturn *input\n}\n"
			head := base + "\nfunc Update(input *int) {\n\t*input = 1\n}\n"
			initialSHA := commitFile(repo, "a.go", base)
			commitFile(repo, "a.go", head)

			command := exec.Command(commandPath, "codesignal", "--base", initialSHA, "--format=json")
			command.Dir = repo
			command.Env = []string{
				"PATH=" + os.Getenv("PATH"),
				"HOME=" + os.Getenv("HOME"),
				"GIT_AUTHOR_NAME=coach-acceptance",
				"GIT_AUTHOR_EMAIL=coach-acceptance@example.com",
				"GIT_COMMITTER_NAME=coach-acceptance",
				"GIT_COMMITTER_EMAIL=coach-acceptance@example.com",
			}
			var stdout, stderr bytes.Buffer
			command.Stdout = &stdout
			command.Stderr = &stderr

			err := command.Run()
			Expect(err).NotTo(HaveOccurred(), "stderr: %s", stderr.String())

			var report codesignal.Report
			Expect(json.Unmarshal(stdout.Bytes(), &report)).To(Succeed(), "stdout should be one JSON report: %s", stdout.String())

			signals := signalsForPath(&report, "a.go")
			Expect(signals).To(HaveLen(1))
			Expect(signals[0].Lifecycle).To(Equal(codesignal.Lifecycle("introduced")))
		})
	})

	When("the git executable is unavailable", func() {
		It("exits 1 with one operational error on stderr and no report on stdout", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "a.go", "package a\n")

			command := exec.Command(commandPath, "codesignal", "--base", initialSHA)
			command.Dir = repo
			command.Env = []string{"PATH=", "HOME=" + os.Getenv("HOME")}
			var stdout, stderr bytes.Buffer
			command.Stdout = &stdout
			command.Stderr = &stderr

			err := command.Run()
			var exitErr *exec.ExitError
			Expect(errors.As(err, &exitErr)).To(BeTrue())
			Expect(exitErr.ExitCode()).To(Equal(1))
			Expect(stdout.Bytes()).To(BeEmpty())
			Expect(stderr.String()).To(ContainSubstring("git executable not found"))
		})
	})

	When("a supported path contains spaces, quotes, a newline, and non-ASCII bytes", func() {
		It("preserves the exact Git path in the emitted report", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "seed.go", "package seed\n")
			path := "space quote\" newline\n日本語.go"
			commitFile(repo, path, "package weird\n\nfunc Update(input *int) {\n\t*input = 1\n}\n")

			report, _ := runCoachCodesignal(repo, initialSHA)

			signals := signalsForPath(report, path)
			Expect(signals).To(HaveLen(1))
			Expect(signals[0].Path).To(Equal(path))
		})
	})

	When("a file is renamed with no content change", func() {
		It("emits an unsupported_change_type diagnostic and never attaches the old path's history to the new path", func() {
			repo := newTempGitRepo()
			oldContent := "package a\n\nfunc Update(input *int) {\n\t*input = 1\n}\n"
			initialSHA := commitFile(repo, "old.go", oldContent)
			renameFile(repo, "old.go", "new.go")

			report, _ := runCoachCodesignal(repo, initialSHA)

			Expect(hasDiagnostic(report, "unsupported_change_type", "new.go")).To(BeTrue())
			Expect(signalsForPath(report, "new.go")).To(BeEmpty(), "a renamed file is not analyzed, so it must not inherit any lifecycle from old.go")
			Expect(signalsForPath(report, "old.go")).To(BeEmpty(), "the old path no longer exists at HEAD and must not appear in the report")
		})
	})

	When("a file is copied with no content change and Git reports a copy", func() {
		It("emits one unsupported_change_type diagnostic for the new path and does not analyze it", func() {
			repo := newTempGitRepo()
			content := "package a\n\nfunc Update(input *int) {\n\t*input = 1\n}\n"
			initialSHA := commitFile(repo, "source.go", content)
			config := exec.Command("git", "config", "diff.renames", "copies")
			config.Dir = repo
			output, err := config.CombinedOutput()
			Expect(err).NotTo(HaveOccurred(), "git config diff.renames copies: %s", output)
			commitFile(repo, "copy.go", content)

			report, _ := runCoachCodesignal(repo, initialSHA)

			Expect(hasDiagnostic(report, "unsupported_change_type", "copy.go")).To(BeTrue())
			Expect(signalsForPath(report, "copy.go")).To(BeEmpty())
		})
	})

	When("the same two commits are analyzed in two independent worktrees", func() {
		It("produces byte-identical JSON output", func() {
			repo := newTempGitRepo()
			initialSHA := commitFile(repo, "a.go", "package a\n\nfunc Get(input *int) int {\n\treturn *input\n}\n")
			commitFile(repo, "a.go", "package a\n\nfunc Get(input *int) int {\n\treturn *input\n}\n\nfunc Update(input *int) {\n\t*input = 1\n}\n")

			firstRun, stderr1, exitCode1 := runCoachCodesignalRaw(repo, initialSHA, "--format=json")
			Expect(exitCode1).To(Equal(0), "stderr: %s", stderr1)

			worktreeParent, err := os.MkdirTemp("", "coach-acceptance-worktree-*")
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(os.RemoveAll, worktreeParent)
			worktree := filepath.Join(worktreeParent, "second-worktree")
			addWorktree := exec.Command("git", "worktree", "add", "--detach", worktree, "HEAD")
			addWorktree.Dir = repo
			output, err := addWorktree.CombinedOutput()
			Expect(err).NotTo(HaveOccurred(), "git worktree add: %s", output)

			secondRun, stderr2, exitCode2 := runCoachCodesignalRaw(worktree, initialSHA, "--format=json")
			Expect(exitCode2).To(Equal(0), "stderr: %s", stderr2)

			Expect(firstRun).To(Equal(secondRun))
		})
	})
})
