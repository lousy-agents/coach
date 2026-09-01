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

// authoringOutputFiles returns fresh, empty stdout/stderr *os.File values for
// a direct authorProjectConfigTypeScript call, along with a reader for each
// that seeks back to the start before reading everything written so far.
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
		It("warns on stderr before the guided-authoring prompt ever runs, but still enters the session", func() {
			original := tsAuthoringRootBudgets
			tsAuthoringRootBudgets = projectmodel.GoBudgets{MaxInputFiles: 1}
			DeferCleanup(func() { tsAuthoringRootBudgets = original })

			repo := newTempGitRepo()
			commitFile(repo, "package.json", "{}\n")
			commitFile(repo, "tsconfig.json", "{}\n")

			// /dev/null as stdin makes AuthorProjectConfig's prompts hit EOF
			// immediately, so the session declines without blocking; this
			// test only cares about the warning written before that session
			// ever starts.
			stdin, err := os.Open(os.DevNull)
			Expect(err).NotTo(HaveOccurred())
			defer stdin.Close()
			stdout, stderr, _, readStderr := authoringOutputFiles()
			defer stdout.Close()
			defer stderr.Close()

			flags := codesignalFlags{baseline: true, suggestProjectConfig: true, projectLanguage: "typescript"}
			authorProjectConfigTypeScript(repo, flags, stdin, stdout, stderr)

			Expect(readStderr()).To(ContainSubstring("TypeScript root discovery did not complete within its budget; the list below may be partial"))
		})
	})

	When("--output is an invalid, unconfined path", func() {
		It("rejects it before any discovery or interactive prompting, so nothing the user types is discarded", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", "{}\n")
			commitFile(repo, "tsconfig.json", "{}\n")

			// A full answer sequence that would walk all the way through
			// roots -> layers -> forbidden pairs -> required layer ->
			// approval if the session ever started: blank, blank, blank,
			// blank, then the literal approval token. Under the bug this
			// fixes, --output was validated only after this whole sequence
			// ran, so stdout would already contain every prompt by the time
			// the rejection was reported.
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

			// The same full answer sequence used by the invalid-path spec
			// above: under the bug this fixes, the existing-target check ran
			// only inside AuthorProjectConfig's own write step, after the
			// entire interactive session (roots -> layers -> forbidden pairs
			// -> required layer -> coverage preview -> approve) had already
			// completed and printed its output.
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

			// A full answer sequence -- select the discovered root, leave
			// layers/forbidden pairs/required layer blank, reach the coverage
			// preview -- ending in a non-approval answer ("no") rather than the
			// approval token. A declined-but-completed session
			// (Approved == false, Cancelled == false) must still be treated as
			// a failure to write, not a silent success.
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

			// Roots: select the single discovered root by number. Layers,
			// forbidden pairs, required layer: all left blank. Approve.
			stdin := authoringStdin("1\n\n\n\napprove\n")
			defer stdin.Close()
			stdout, stderr, readStdout, readStderr := authoringOutputFiles()
			defer stdout.Close()
			defer stderr.Close()

			flags := codesignalFlags{baseline: true, suggestProjectConfig: true, projectLanguage: "typescript", output: "project.json", outputSet: true}
			exitCode := authorProjectConfigTypeScript(repo, flags, stdin, stdout, stderr)

			Expect(exitCode).To(Equal(0))
			// The interactive transcript -- every prompt, the coverage
			// preview, the approval question -- goes to stderr so a customer
			// can see and answer it even if stdout is redirected; none of it
			// is an error, but stderr is where the whole session lives.
			Expect(readStderr()).To(ContainSubstring("Select the roots to include"), "stderr: %s", readStderr())

			data, err := os.ReadFile(filepath.Join(repo, "project.json"))
			Expect(err).NotTo(HaveOccurred())
			var candidate suggestionCandidateDoc
			Expect(json.Unmarshal(data, &candidate)).To(Succeed(), "written file: %s", data)
			Expect(candidate.SchemaVersion).To(Equal("1"))
			Expect(candidate.Roots).To(Equal([]string{"."}))

			// Nothing reaches stdout when --output is set: the written file
			// is the only place the candidate document appears, and every
			// prompt went to stderr instead.
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

			// stdout must be exactly the candidate document -- parseable on
			// its own, with no interactive prompt text mixed in, so a
			// customer capturing it (e.g. `> project.json`) gets a usable
			// file even though they never saw the redirected stream.
			var candidate suggestionCandidateDoc
			stdoutBytes := []byte(readStdout())
			Expect(json.Unmarshal(stdoutBytes, &candidate)).To(Succeed(), "stdout: %s", readStdout())
			Expect(candidate.SchemaVersion).To(Equal("1"))
			Expect(candidate.Roots).To(Equal([]string{"."}))

			// The full interactive transcript must still be visible on
			// stderr -- the customer answered these prompts on their
			// terminal even though stdout was redirected to a file.
			Expect(readStderr()).To(ContainSubstring("Select the roots to include"))
			Expect(readStderr()).NotTo(ContainSubstring(`"schema_version"`), "the candidate document must never leak into the transcript stream")

			_, statErr := os.Stat(filepath.Join(repo, "project.json"))
			Expect(os.IsNotExist(statErr)).To(BeTrue(), "no file is written when --output is omitted; the candidate is emitted, not saved")
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
