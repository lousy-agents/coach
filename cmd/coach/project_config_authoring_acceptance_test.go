package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

// authoringStdin writes content to a fresh temp file, rewinds it, and
// returns it as an *os.File -- authorProjectConfigTypeScript takes stdin as
// an *os.File (not an io.Reader) so it stays callable with the same real
// terminal-detection contract runAuthorProjectConfigTypeScript uses for
// os.Stdin. The caller is responsible for closing the returned file.
func authoringStdin(content string) *os.File {
	f, err := os.CreateTemp(GinkgoT().TempDir(), "authoring-stdin")
	Expect(err).NotTo(HaveOccurred())
	_, err = f.WriteString(content)
	Expect(err).NotTo(HaveOccurred())
	_, err = f.Seek(0, 0)
	Expect(err).NotTo(HaveOccurred())
	return f
}

func authoringOutputFiles() (stdout, stderr *os.File, readStdout, readStderr func() string) {
	var err error
	stdout, err = os.CreateTemp(GinkgoT().TempDir(), "authoring-stdout")
	Expect(err).NotTo(HaveOccurred())
	stderr, err = os.CreateTemp(GinkgoT().TempDir(), "authoring-stderr")
	Expect(err).NotTo(HaveOccurred())

	read := func(f *os.File) string {
		_, err := f.Seek(0, 0)
		Expect(err).NotTo(HaveOccurred())
		data, err := io.ReadAll(f)
		Expect(err).NotTo(HaveOccurred())
		return string(data)
	}
	return stdout, stderr, func() string { return read(stdout) }, func() string { return read(stderr) }
}

var _ = Describe("coach codesignal --baseline --suggest-project-config --project-language typescript", func() {
	When("no controlling terminal is available on stdin", func() {
		It("refuses to enter guided authoring, writes no policy file, and exits 2 with a clear stderr message", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", "{}\n")
			commitFile(repo, "tsconfig.json", "{}\n")

			stdinFile, err := os.Open(filepath.Join(repo, "package.json"))
			Expect(err).NotTo(HaveOccurred())
			defer stdinFile.Close()

			command := exec.Command(commandPath, "codesignal", "--baseline", "--suggest-project-config", "--project-language", "typescript", "--output", "project.json")
			command.Dir = repo
			command.Stdin = stdinFile
			var stdout, stderr bytes.Buffer
			command.Stdout = &stdout
			command.Stderr = &stderr

			runErr := command.Run()
			var exitErr *exec.ExitError
			Expect(errors.As(runErr, &exitErr)).To(BeTrue(), "expected an ExitError, got: %s (stderr: %s)", runErr, stderr.String())
			// The controlling-terminal gate is documented to share
			// --suggest-project-config's usage/discovery-rejection exit code
			// (2), never the snapshot/revision-failure exit code (3): pin the
			// exact value so a regression back to a different exit-code
			// family (e.g. classifyAnalysisError's 1/2/3 table) is caught,
			// not just "any non-zero code".
			Expect(exitErr.ExitCode()).To(Equal(2))
			Expect(stdout.String()).To(BeEmpty(), "no candidate/prompt output must reach stdout when there is no controlling terminal")
			Expect(stderr.String()).To(ContainSubstring("controlling terminal"), "stderr: %s", stderr.String())
			// A coding agent without a terminal must be steered toward
			// drafting the documented schema-1 policy itself and handing it
			// to a human to review, commit, and rerun -- not toward faking a
			// PTY and inventing architecture on the human's behalf (AC-POL-3).
			Expect(stderr.String()).To(ContainSubstring("--project-config"), "stderr must name the non-interactive alternative: %s", stderr.String())
			Expect(stderr.String()).To(ContainSubstring("review"), "stderr must instruct a human to review the drafted policy: %s", stderr.String())

			// --output is supplied above specifically so this assertion can fail
			// for the intended reason: without --output, AuthorProjectConfig never
			// writes a file on any path (it emits the candidate to stdout instead),
			// so this check would pass even if the controlling-terminal gate did
			// not run at all.
			_, statErr := os.Stat(filepath.Join(repo, "project.json"))
			Expect(os.IsNotExist(statErr)).To(BeTrue(), "no policy config file must be written when there is no controlling terminal")
		})
	})

	When("the repository has no commits yet, so the baseline revision cannot be resolved", func() {
		It("exits 3 without prompting, mirroring the Go family's own snapshot-unavailable exit code", func() {
			repo := newTempGitRepo()

			stdin := authoringStdin("")
			defer stdin.Close()
			stdout, stderr, readStdout, readStderr := authoringOutputFiles()
			defer stdout.Close()
			defer stderr.Close()

			flags := codesignalFlags{baseline: true, suggestProjectConfig: true, projectLanguage: "typescript"}
			exitCode := authorProjectConfigTypeScript(repo, flags, stdin, stdout, stderr)

			Expect(exitCode).To(Equal(3))
			Expect(readStdout()).To(BeEmpty(), "no prompt output must reach stdout when the baseline revision cannot even be resolved")
			Expect(readStderr()).To(ContainSubstring("could not resolve the baseline revision"))
		})
	})

	When("TypeScript root discovery reports the snapshot itself is unreadable (not merely budget-truncated)", func() {
		It("exits 3 with a snapshot-unavailable message before ever prompting, distinct from the budget-truncation warning", func() {
			original := discoverTSRoots
			discoverTSRoots = func(snapshot fs.FS, budgets projectmodel.GoBudgets) (projectmodel.TSRootDiscoveryResult, error) {
				return projectmodel.TSRootDiscoveryResult{
					Complete: false,
					Coverage: projectmodel.Coverage{
						Diagnostics: []projectmodel.Diagnostic{{Code: projectmodel.DiagTSRootUnavailable, Path: "."}},
					},
				}, nil
			}
			DeferCleanup(func() { discoverTSRoots = original })

			repo := newTempGitRepo()
			commitFile(repo, "package.json", "{}\n")

			stdin := authoringStdin("")
			defer stdin.Close()
			stdout, stderr, readStdout, readStderr := authoringOutputFiles()
			defer stdout.Close()
			defer stderr.Close()

			flags := codesignalFlags{baseline: true, suggestProjectConfig: true, projectLanguage: "typescript"}
			exitCode := authorProjectConfigTypeScript(repo, flags, stdin, stdout, stderr)

			Expect(exitCode).To(Equal(3))
			Expect(readStdout()).To(BeEmpty(), "no prompt output must reach stdout when the snapshot itself could not be read")
			stderrText := readStderr()
			Expect(stderrText).To(ContainSubstring("could not read the TypeScript root-discovery snapshot"))
			Expect(stderrText).NotTo(ContainSubstring("did not complete within its budget"), "an unreadable snapshot must not be misreported as mere budget truncation")
		})
	})

	When("TypeScript root discovery is truncated by its budget before it finishes walking", func() {
		It("exits 2 with a refusal message before any prompt, never entering the session against a partial root list", func() {
			original := tsAuthoringRootBudgets
			tsAuthoringRootBudgets = projectmodel.GoBudgets{MaxInputFiles: 1}
			DeferCleanup(func() { tsAuthoringRootBudgets = original })

			repo := newTempGitRepo()
			commitFile(repo, "package.json", "{}\n")
			commitFile(repo, "tsconfig.json", "{}\n")

			// stdin is never read: this spec's whole point is that the
			// session must refuse before it ever tries.
			stdin, err := os.Open(os.DevNull)
			Expect(err).NotTo(HaveOccurred())
			defer stdin.Close()
			stdout, stderr, readStdout, readStderr := authoringOutputFiles()
			defer stdout.Close()
			defer stderr.Close()

			flags := codesignalFlags{baseline: true, suggestProjectConfig: true, projectLanguage: "typescript"}
			exitCode := authorProjectConfigTypeScript(repo, flags, stdin, stdout, stderr)

			Expect(exitCode).To(Equal(2))
			Expect(readStdout()).To(BeEmpty(), "no prompt output must reach stdout when discovery is refused for being budget-truncated")
			stderrText := readStderr()
			Expect(stderrText).To(ContainSubstring("TypeScript root discovery did not complete within its budget"))
			Expect(stderrText).NotTo(ContainSubstring("Select the roots to include"), "a budget-truncated discovery must never enter the guided-authoring prompt sequence")
		})
	})

	When("--output is an invalid, unconfined path", func() {
		It("rejects it before any discovery or interactive prompting, so nothing the user types is discarded", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", "{}\n")
			commitFile(repo, "tsconfig.json", "{}\n")

			stdin := authoringStdin("\n\n\n\napprove\n")
			defer stdin.Close()
			stdout, stderr, readStdout, readStderr := authoringOutputFiles()
			defer stdout.Close()
			defer stderr.Close()

			flags := codesignalFlags{baseline: true, suggestProjectConfig: true, projectLanguage: "typescript", output: "../escape.json", outputSet: true}
			exitCode := authorProjectConfigTypeScript(repo, flags, stdin, stdout, stderr)

			Expect(exitCode).To(Equal(2))
			Expect(readStdout()).To(BeEmpty(), "--output must be rejected before any prompt is printed, so nothing the user typed is wasted")
			Expect(readStderr()).To(ContainSubstring("repository"), "stderr: %s", readStderr())

			_, statErr := os.Stat(filepath.Join(repo, "escape.json"))
			Expect(os.IsNotExist(statErr)).To(BeTrue(), "no policy config file must be written when --output is rejected")
		})
	})

	When("--output already names an existing file", func() {
		It("refuses before any discovery or interactive prompting, so a completed session is never thrown away", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", "{}\n")
			commitFile(repo, "tsconfig.json", "{}\n")

			preexisting := "this is not a project config and must not be overwritten"
			Expect(os.WriteFile(filepath.Join(repo, "project.json"), []byte(preexisting), 0o644)).To(Succeed())

			stdin := authoringStdin("\n\n\n\napprove\n")
			defer stdin.Close()
			stdout, stderr, readStdout, readStderr := authoringOutputFiles()
			defer stdout.Close()
			defer stderr.Close()

			flags := codesignalFlags{baseline: true, suggestProjectConfig: true, projectLanguage: "typescript", output: "project.json", outputSet: true}
			exitCode := authorProjectConfigTypeScript(repo, flags, stdin, stdout, stderr)

			Expect(exitCode).To(Equal(2))
			Expect(readStdout()).To(BeEmpty(), "--output's existing-target rejection must happen before any prompt is printed, so nothing the user typed is wasted")
			Expect(readStderr()).To(ContainSubstring("already exists"))

			data, err := os.ReadFile(filepath.Join(repo, "project.json"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(Equal(preexisting), "the pre-existing target's content must be left completely untouched")
		})
	})

	When("TypeScript root discovery reports a single unreadable file, not the whole snapshot", func() {
		It("still exits 3 before prompting, even though some roots were already collected before the failure", func() {
			original := discoverTSRoots
			discoverTSRoots = func(snapshot fs.FS, budgets projectmodel.GoBudgets) (projectmodel.TSRootDiscoveryResult, error) {
				return projectmodel.TSRootDiscoveryResult{
					Roots:    []string{"apps/api"},
					Complete: false,
					Coverage: projectmodel.Coverage{
						Diagnostics: []projectmodel.Diagnostic{{Code: projectmodel.DiagTSRootUnavailable, Path: "apps/web/tsconfig.json"}},
					},
				}, nil
			}
			DeferCleanup(func() { discoverTSRoots = original })

			repo := newTempGitRepo()
			commitFile(repo, "package.json", "{}\n")

			stdin := authoringStdin("")
			defer stdin.Close()
			stdout, stderr, readStdout, readStderr := authoringOutputFiles()
			defer stdout.Close()
			defer stderr.Close()

			flags := codesignalFlags{baseline: true, suggestProjectConfig: true, projectLanguage: "typescript"}
			exitCode := authorProjectConfigTypeScript(repo, flags, stdin, stdout, stderr)

			Expect(exitCode).To(Equal(3))
			Expect(readStdout()).To(BeEmpty(), "no prompt output must reach stdout when any part of the snapshot could not be read, even if some roots were already collected")
			stderrText := readStderr()
			Expect(stderrText).To(ContainSubstring("could not read the TypeScript root-discovery snapshot (apps/web/tsconfig.json)"))
			Expect(stderrText).NotTo(ContainSubstring("did not complete within its budget"))
		})
	})

	When("guided authoring completes every stage but the user declines the final approval", func() {
		It("exits 2, reports that authoring was cancelled or not approved, and writes no policy file", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", "{}\n")
			commitFile(repo, "tsconfig.json", "{}\n")

			stdin := authoringStdin("1\n\n\n\nno\n")
			defer stdin.Close()
			stdout, stderr, readStdout, readStderr := authoringOutputFiles()
			defer stdout.Close()
			defer stderr.Close()

			flags := codesignalFlags{baseline: true, suggestProjectConfig: true, projectLanguage: "typescript", output: "project.json", outputSet: true}
			exitCode := authorProjectConfigTypeScript(repo, flags, stdin, stdout, stderr)

			Expect(exitCode).To(Equal(2))
			Expect(readStderr()).To(ContainSubstring("cancelled or not approved"), "stderr: %s", readStderr())
			Expect(readStdout()).To(BeEmpty(), "the interactive transcript goes to stderr, so nothing -- prompts or a candidate document -- must ever reach stdout for a declined session")

			_, statErr := os.Stat(filepath.Join(repo, "project.json"))
			Expect(os.IsNotExist(statErr)).To(BeTrue(), "no policy config file must be written when the session completes without approval")
		})
	})

	When("guided authoring is approved with --output set", func() {
		It("exits 0 and writes the validated candidate to the repository-relative output path", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", "{}\n")
			commitFile(repo, "tsconfig.json", "{}\n")

			stdin := authoringStdin("1\n\n\n\napprove\n")
			defer stdin.Close()
			stdout, stderr, readStdout, readStderr := authoringOutputFiles()
			defer stdout.Close()
			defer stderr.Close()

			flags := codesignalFlags{baseline: true, suggestProjectConfig: true, projectLanguage: "typescript", output: "project.json", outputSet: true}
			exitCode := authorProjectConfigTypeScript(repo, flags, stdin, stdout, stderr)

			Expect(exitCode).To(Equal(0))
			Expect(readStderr()).To(ContainSubstring("Select the roots to include"), "stderr: %s", readStderr())

			data, err := os.ReadFile(filepath.Join(repo, "project.json"))
			Expect(err).NotTo(HaveOccurred())
			var candidate suggestionCandidateDoc
			Expect(json.Unmarshal(data, &candidate)).To(Succeed(), "written file: %s", data)
			Expect(candidate.SchemaVersion).To(Equal("1"))
			Expect(candidate.Roots).To(Equal([]string{"."}))
			Expect(readStdout()).To(BeEmpty())
		})
	})

	When("guided authoring is approved with --output omitted", func() {
		It("exits 0, emits only the validated candidate on stdout, and writes the full transcript to stderr", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", "{}\n")
			commitFile(repo, "tsconfig.json", "{}\n")

			stdin := authoringStdin("1\n\n\n\napprove\n")
			defer stdin.Close()
			stdout, stderr, readStdout, readStderr := authoringOutputFiles()
			defer stdout.Close()
			defer stderr.Close()

			flags := codesignalFlags{baseline: true, suggestProjectConfig: true, projectLanguage: "typescript"}
			exitCode := authorProjectConfigTypeScript(repo, flags, stdin, stdout, stderr)

			Expect(exitCode).To(Equal(0))

			var candidate suggestionCandidateDoc
			stdoutBytes := []byte(readStdout())
			Expect(json.Unmarshal(stdoutBytes, &candidate)).To(Succeed(), "stdout: %s", readStdout())
			Expect(candidate.SchemaVersion).To(Equal("1"))
			Expect(candidate.Roots).To(Equal([]string{"."}))
			Expect(readStderr()).To(ContainSubstring("Select the roots to include"))
			Expect(readStderr()).NotTo(ContainSubstring(`"schema_version"`), "the candidate document must never leak into the transcript stream")

			_, statErr := os.Stat(filepath.Join(repo, "project.json"))
			Expect(os.IsNotExist(statErr)).To(BeTrue(), "no file is written when --output is omitted; the candidate is emitted, not saved")
		})
	})

	When("the worktree has modified tracked, untracked, and ignored TypeScript files", func() {
		It("still discovers only the committed HEAD snapshot, matching a clean-worktree run", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", "{}\n")
			commitFile(repo, "tsconfig.json", "{}\n")

			runAuthoring := func() (stdout, stderr string, exitCode int) {
				stdin := authoringStdin("1\n\n\n\napprove\n")
				defer stdin.Close()
				stdoutFile, stderrFile, readStdout, readStderr := authoringOutputFiles()
				defer stdoutFile.Close()
				defer stderrFile.Close()

				flags := codesignalFlags{baseline: true, suggestProjectConfig: true, projectLanguage: "typescript"}
				exitCode = authorProjectConfigTypeScript(repo, flags, stdin, stdoutFile, stderrFile)
				return readStdout(), readStderr(), exitCode
			}

			cleanStdout, cleanStderr, cleanExit := runAuthoring()
			Expect(cleanExit).To(Equal(0), "stderr: %s", cleanStderr)

			// Modify the tracked tsconfig.json's content (discovery only cares
			// about a manifest's existence, never its content), add an
			// untracked tsconfig.json in a new directory (a second root that
			// must never appear), and add a gitignored directory with its own
			// tsconfig.json (equally uncommitted, equally invisible).
			Expect(os.WriteFile(filepath.Join(repo, "tsconfig.json"), []byte(`{"compilerOptions":{}}`), 0o644)).To(Succeed())
			Expect(os.MkdirAll(filepath.Join(repo, "untracked-app"), 0o755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(repo, "untracked-app", "tsconfig.json"), []byte("{}\n"), 0o644)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(repo, ".gitignore"), []byte("ignored/\n"), 0o644)).To(Succeed())
			Expect(os.MkdirAll(filepath.Join(repo, "ignored"), 0o755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(repo, "ignored", "tsconfig.json"), []byte("{}\n"), 0o644)).To(Succeed())

			noisyStdout, noisyStderr, noisyExit := runAuthoring()
			Expect(noisyExit).To(Equal(0), "stderr: %s", noisyStderr)

			Expect(noisyStdout).To(Equal(cleanStdout), "worktree noise must not affect the emitted candidate document")
			Expect(noisyStderr).To(Equal(cleanStderr), "worktree noise must not affect the discovered-roots transcript")
		})
	})
})

var _ = Describe("coach codesignal --baseline --suggest-project-config (Go path unaffected by TypeScript guided authoring)", func() {
	When("--suggest-project-config is used alone (no --project-language) against a single-module Go repository", func() {
		It("still emits a candidate with roots: [\".\"] on stdout and a provenance envelope on stderr, exit 0", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", "module example.com/tswiringregression\n\ngo 1.25\n")
			commitFile(repo, "main.go", "package main\n\nfunc main() {}\n")

			stdout, stderr, exitCode := runCoachSuggest(repo, "--baseline", "--suggest-project-config")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			var candidate suggestionCandidateDoc
			Expect(json.Unmarshal(stdout, &candidate)).To(Succeed(), "stdout: %s", stdout)
			Expect(candidate.SchemaVersion).To(Equal("1"))
			Expect(candidate.Roots).To(Equal([]string{"."}))

			var envelope suggestionEnvelopeDoc
			Expect(json.Unmarshal(stderr, &envelope)).To(Succeed(), "stderr: %s", stderr)
			Expect(envelope.Kind).To(Equal("project_config_suggestion"))
			Expect(envelope.HeuristicVersion).To(Equal("go-project-config-roots@1"))
			Expect(envelope.Diagnostics).To(HaveLen(1))
			Expect(envelope.Diagnostics[0].Code).To(Equal("project_config_suggestion_ready"))
		})
	})
})
