package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectcheck"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
	"github.com/lousy-agents/coach/internal/codesignalcli/tssetup"
	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
)

var _ = Describe("coach's interim standalone prepare_compiler mise setup dispatch (coach#328 Task 5, AC-SET-1..AC-SET-8/19/22/23/24)", func() {
	When("mise is trusted but neither scope declares an exact supported TypeScript version", func() {
		It("offers no installation choice, exits 0, and never invokes mise install (AC-SET-1, AC-23)", func() {
			repo := noSupportedCompilerRepo()
			path, miseDir := pathWithStatefulStubNodeAndMise("v24.9.9", "7.0.2")
			GinkgoT().Setenv("PATH", path)
			GinkgoT().Setenv("HOME", os.Getenv("HOME"))

			GinkgoT().Setenv("CI", "")

			stdin := authoringStdin("mise_project\ninstall\n")
			defer stdin.Close()
			stdoutFile, stderrFile, readStdout, readStderr := authoringOutputFiles()
			defer stdoutFile.Close()
			defer stderrFile.Close()

			exitCode := prepareCompilerMiseTypeScript(repo, stdin, stdoutFile, stderrFile, "")

			transcript := readStderr()
			Expect(exitCode).To(Equal(0), "stderr: %s", transcript)
			Expect(readStdout()).To(BeEmpty(), "no report must ever reach stdout from this flow")
			Expect(transcript).To(ContainSubstring("no executable mise compiler-setup choice is currently offered"), "transcript: %s", transcript)
			Expect(transcript).NotTo(ContainSubstring("  - mise_project"), "a scope with no in-set pin cannot make post-install readiness pass, transcript: %s", transcript)
			Expect(transcript).NotTo(ContainSubstring("  - mise_global"), "transcript: %s", transcript)
			Expect(miseInvocationsIncludeInstall(miseDir)).To(BeFalse(), "a non-executable choice must never invoke `mise install`")
		})
	})

	When("both mise_project and mise_global are executable and verified, and the user selects one and then declines the install confirmation", func() {
		It("shows the full AC-SET-2 preview naming both offered choices, then exits 2 with empty stdout and no mise mutation on decline (AC-SET-2, AC-SET-5, AC-SET-8)", func() {
			repo := noSupportedCompilerRepo()
			writeWorktreeFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \"7.0.2\"\n")
			path, miseDir := pathWithStatefulStubNodeAndMiseGlobalAware("v24.9.9", "7.0.2")
			GinkgoT().Setenv("PATH", path)
			GinkgoT().Setenv("HOME", os.Getenv("HOME"))

			stdin := authoringStdin("mise_project\ndecline\n")
			defer stdin.Close()
			stdoutFile, stderrFile, readStdout, readStderr := authoringOutputFiles()
			defer stdoutFile.Close()
			defer stderrFile.Close()

			exitCode := prepareCompilerMiseTypeScript(repo, stdin, stdoutFile, stderrFile, "")

			Expect(exitCode).To(Equal(2))
			Expect(readStdout()).To(BeEmpty(), "no report must ever reach stdout from this flow")

			transcript := readStderr()
			Expect(transcript).To(ContainSubstring("mise_project"), "transcript: %s", transcript)
			Expect(transcript).To(ContainSubstring("mise_global"), "both verified choices must be listed so selection has no default, transcript: %s", transcript)

			Expect(transcript).To(ContainSubstring("Executable: mise"), "transcript: %s", transcript)
			Expect(transcript).To(ContainSubstring("Arguments: install npm:typescript@7.0.2"), "transcript: %s", transcript)
			Expect(transcript).To(ContainSubstring("Working directory:"), "transcript: %s", transcript)
			Expect(transcript).To(ContainSubstring("Expected mise changes:"), "transcript: %s", transcript)
			Expect(transcript).To(ContainSubstring("Network use:"), "transcript: %s", transcript)
			Expect(transcript).To(ContainSubstring("Lifecycle-script policy:"), "transcript: %s", transcript)
			Expect(transcript).To(ContainSubstring("Timeout: 5m0s"), "transcript: %s", transcript)

			Expect(transcript).To(ContainSubstring("cancelled"), "transcript: %s", transcript)
			Expect(miseInvocationsIncludeInstall(miseDir)).To(BeFalse(), "a declined confirmation must never invoke `mise install`")
		})
	})

	When("the choice-selection answer names neither offered mise scope", func() {
		It("cancels without ever showing the install preview, proving there is no default choice (AC-SET-5, AC-SET-8)", func() {
			repo := noSupportedCompilerRepo()
			writeWorktreeFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \"7.0.2\"\n")
			path, miseDir := pathWithStatefulStubNodeAndMiseGlobalAware("v24.9.9", "7.0.2")
			GinkgoT().Setenv("PATH", path)
			GinkgoT().Setenv("HOME", os.Getenv("HOME"))

			stdin := authoringStdin("not-a-real-choice\n")
			defer stdin.Close()
			stdoutFile, stderrFile, readStdout, readStderr := authoringOutputFiles()
			defer stdoutFile.Close()
			defer stderrFile.Close()

			exitCode := prepareCompilerMiseTypeScript(repo, stdin, stdoutFile, stderrFile, "")

			Expect(exitCode).To(Equal(2))
			Expect(readStdout()).To(BeEmpty())

			transcript := readStderr()
			Expect(transcript).NotTo(ContainSubstring("Executable: mise"), "an unrecognized selection must never reach the install preview, transcript: %s", transcript)
			Expect(miseInvocationsIncludeInstall(miseDir)).To(BeFalse(), "an unrecognized selection must never invoke `mise install`")
		})
	})

	When("there is no controlling terminal on stdin", func() {
		It("never prompts, never mutates mise state, and exits 2 (AC-SET-24)", func() {
			repo := noSupportedCompilerRepo()
			path, miseDir := pathWithStatefulStubNodeAndMise("v24.9.9", "7.0.2")

			GinkgoT().Setenv("PATH", path)
			GinkgoT().Setenv("HOME", os.Getenv("HOME"))

			GinkgoT().Setenv("CI", "")

			stdin := authoringStdin("mise_project\ninstall\n")
			defer stdin.Close()
			stdoutFile, stderrFile, readStdout, readStderr := authoringOutputFiles()
			defer stdoutFile.Close()
			defer stderrFile.Close()

			exitCode := runPrepareCompilerMiseTypeScript(repo, codesignalFlags{}, stdin, stdoutFile, stderrFile)

			Expect(exitCode).To(Equal(2))
			Expect(readStdout()).To(BeEmpty())
			Expect(readStderr()).To(ContainSubstring("controlling terminal"))
			_, statErr := os.Stat(filepath.Join(miseDir, stubMiseInvocationLog))
			Expect(os.IsNotExist(statErr)).To(BeTrue(), "no controlling terminal must mean mise is never invoked at all, not even for a read-only probe")
		})
	})

	When("both a policy gap and a compiler gap exist (AC-SET-13)", func() {
		It("lists no mise installation choice, exits 2, and stderr names author_policy as the required first action", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")

			path, miseDir := pathWithStatefulStubNodeAndMise("v24.9.9", "7.0.2")
			GinkgoT().Setenv("PATH", path)
			GinkgoT().Setenv("HOME", os.Getenv("HOME"))

			stdin := authoringStdin("")
			defer stdin.Close()
			stdoutFile, stderrFile, readStdout, readStderr := authoringOutputFiles()
			defer stdoutFile.Close()
			defer stderrFile.Close()

			exitCode := prepareCompilerMiseTypeScript(repo, stdin, stdoutFile, stderrFile, "")

			Expect(exitCode).To(Equal(2))
			Expect(readStdout()).To(BeEmpty(), "no report must ever reach stdout from this flow")

			transcript := readStderr()
			Expect(transcript).NotTo(ContainSubstring("mise_project"), "transcript: %s", transcript)
			Expect(transcript).NotTo(ContainSubstring("mise_global"), "transcript: %s", transcript)
			lines := strings.Split(strings.TrimRight(transcript, "\n"), "\n")
			Expect(lines).To(HaveLen(1), "stderr must be exactly one line, transcript: %s", transcript)
			Expect(transcript).To(ContainSubstring("author_policy"), "transcript: %s", transcript)
			Expect(miseInvocationsIncludeInstall(miseDir)).To(BeFalse(), "compiler setup must never be attempted while a policy gap remains")

			stdout, stderr, exitCodeCheck := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--format", "json")
			Expect(exitCodeCheck).To(Equal(0), "stderr: %s", stderr)
			var doc readinessResultDoc
			Expect(json.Unmarshal(stdout, &doc)).To(Succeed(), "stdout: %s", stdout)
			Expect(gapCodes(doc)).To(ContainElement("policy_missing"))
			Expect(gapCodes(doc)).To(ContainElement("typescript_compiler_missing"))
			prepareAction, foundPrepare := nextActionOfKind(doc, "prepare_compiler")
			policyAction, foundPolicy := nextActionOfKind(doc, "author_policy")
			Expect(foundPolicy).To(BeTrue(), "next_actions: %+v", doc.NextActions)
			Expect(foundPrepare).To(BeTrue(), "next_actions: %+v", doc.NextActions)
			Expect(policyAction.Executable).To(BeFalse())
			Expect(prepareAction.Executable).To(BeTrue(), "--check-project's own next_actions must be unaffected by --prepare-compiler's own withholding")
		})
	})

	When("the project mise.toml already declares the frozen TypeScript version but it is not yet installed, and the user selects mise_project and confirms", func() {
		It("installs it, reruns readiness, and the fresh result reports the compiler check passing at that version (AC-SET-3, AC-SET-6, AC-SET-19, AC-SET-23)", func() {
			repo := noSupportedCompilerRepo()
			writeWorktreeFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \"7.0.2\"\n")
			path, miseDir := pathWithStatefulStubNodeAndMise("v24.9.9", "7.0.2")
			GinkgoT().Setenv("PATH", path)
			GinkgoT().Setenv("HOME", os.Getenv("HOME"))

			revision, err := gitrepo.ResolveBaselineRevision(repo)
			Expect(err).NotTo(HaveOccurred())
			before, err := projectcheck.Run(repo, revision, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(before.Checks.Compiler.State).To(Equal(projectreadiness.Fail), "sanity: the fixture must start without a usable compiler")
			Expect(before.Checks.Compiler.Code).To(Equal(projectreadiness.GapTypescriptCompilerMissing))

			stdin := authoringStdin("mise_project\ninstall\n")
			defer stdin.Close()
			stdoutFile, stderrFile, readStdout, readStderr := authoringOutputFiles()
			defer stdoutFile.Close()
			defer stderrFile.Close()

			exitCode := prepareCompilerMiseTypeScript(repo, stdin, stdoutFile, stderrFile, "")

			transcript := readStderr()
			Expect(exitCode).To(Equal(0), "stderr: %s", transcript)
			Expect(readStdout()).To(BeEmpty(), "no report must ever reach stdout from this flow")
			Expect(miseInvocationsIncludeInstall(miseDir)).To(BeTrue(), "a confirmed selection must invoke `mise install`")

			Expect(transcript).To(ContainSubstring("installed TypeScript 7.0.2 via mise_project; rerun readiness reports compiler check pass (version=7.0.2)"), "transcript: %s", transcript)
		})
	})

	When("the project mise.toml already declares the frozen TypeScript version but `mise install` itself fails, and the user selects mise_project and confirms", func() {
		It("exits 2 with empty stdout, a transcript naming the version and scope, and no repository or mise-store mutation from Coach itself (AC-SET-7)", func() {
			repo := noSupportedCompilerRepo()
			writeWorktreeFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \"7.0.2\"\n")
			path, miseDir := pathWithFailingInstallStubNodeAndMise("v24.9.9")
			GinkgoT().Setenv("PATH", path)
			GinkgoT().Setenv("HOME", os.Getenv("HOME"))

			statusBefore := gitStatusPorcelain(repo)

			stdin := authoringStdin("mise_project\ninstall\n")
			defer stdin.Close()
			stdoutFile, stderrFile, readStdout, readStderr := authoringOutputFiles()
			defer stdoutFile.Close()
			defer stderrFile.Close()

			exitCode := prepareCompilerMiseTypeScript(repo, stdin, stdoutFile, stderrFile, "")

			transcript := readStderr()
			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", readStdout(), transcript)
			Expect(readStdout()).To(BeEmpty(), "no report must ever reach stdout from this flow")
			Expect(miseInvocationsIncludeInstall(miseDir)).To(BeTrue(), "a confirmed selection must still invoke `mise install`, even though it fails")

			Expect(transcript).To(MatchRegexp(`mise install failed; mise's install store may now contain a partial or failed install of TypeScript 7\.0\.2 under the mise_project scope`), "the failure message must name the actual version and scope that may have been partially installed, not an empty string, transcript: %s", transcript)
			Expect(transcript).To(ContainSubstring("Coach does not attempt to clean this up"))

			Expect(gitStatusPorcelain(repo)).To(Equal(statusBefore), "a failed install must never leave Coach itself having mutated the repository (no rollback is attempted, but none should be needed)")
		})
	})

	When("the project mise.toml already declares the frozen TypeScript version and `mise install` itself exits 0, but the freshly-installed compiler never becomes locatable/eligible, and the user selects mise_project and confirms", func() {
		It("exits 2 with empty stdout and a transcript that names the real verification failure, never the wrong 'mise install failed' wording (coach#328 Task 5 integration repair, Finding 2)", func() {
			repo := noSupportedCompilerRepo()
			writeWorktreeFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \"7.0.2\"\n")
			path, miseDir := pathWithIneligibleInstallStubNodeAndMise("v24.9.9")
			GinkgoT().Setenv("PATH", path)
			GinkgoT().Setenv("HOME", os.Getenv("HOME"))

			stdin := authoringStdin("mise_project\ninstall\n")
			defer stdin.Close()
			stdoutFile, stderrFile, readStdout, readStderr := authoringOutputFiles()
			defer stdoutFile.Close()
			defer stderrFile.Close()

			exitCode := prepareCompilerMiseTypeScript(repo, stdin, stdoutFile, stderrFile, "")

			transcript := readStderr()
			Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", readStdout(), transcript)
			Expect(readStdout()).To(BeEmpty(), "no report must ever reach stdout from this flow")
			Expect(miseInvocationsIncludeInstall(miseDir)).To(BeTrue(), "a confirmed selection must still invoke `mise install`, even though verification later fails")

			Expect(transcript).To(ContainSubstring("mise install exited 0 but the installed TypeScript 7.0.2 is not eligible (absent); expected the native platform package "+tstoolchain.NativeTypescriptPackageName()+" alongside it -- Coach does not attempt to repair or clean this up."), "transcript: %s", transcript)
			Expect(transcript).NotTo(ContainSubstring("mise install failed"), "a subprocess that exited 0 must never be described as having failed, transcript: %s", transcript)
		})
	})

	When("mise install could never even be started because its own private working directory could not be created", func() {
		It("reports the install could not even be started, naming the insulation-failure gap code, distinctly from a subprocess that actually ran and failed (coach#328 Task 5 integration repair, Finding 5)", func() {
			stdoutFile, stderrFile, readStdout, readStderr := authoringOutputFiles()
			defer stdoutFile.Close()
			defer stderrFile.Close()

			result := tssetup.PrepareCompilerMiseResult{
				Trusted: true,
				Code:    projectreadiness.GapPackageManagerConfigUnverifiable,
			}

			exitCode := reportPrepareCompilerMiseResult(result, stderrFile)

			transcript := readStderr()
			Expect(exitCode).To(Equal(2), "stderr: %s", transcript)
			Expect(readStdout()).To(BeEmpty(), "no report must ever reach stdout from this flow")
			Expect(transcript).To(ContainSubstring("mise install could not even be started (package_manager_config_unverifiable)."), "transcript: %s", transcript)
			Expect(transcript).NotTo(ContainSubstring("mise install failed"), "an install that never started must never be described as having failed, transcript: %s", transcript)
		})
	})

	When("the project mise scope is untrusted (a hazardous mise.toml) so only mise_global is offered, and the user selects it and confirms", func() {
		It("installs via the global scope, reruns readiness, and the fresh result reports the compiler check passing at that version (AC-SET-3, AC-SET-6, AC-SET-19, AC-SET-23)", func() {
			repo := noSupportedCompilerRepo()
			writeWorktreeFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \"7.0.2\"\n\n[hooks]\npostinstall = \"echo pwned\"\n")
			miseDir := writeStatefulStubMiseScriptGlobalAware("7.0.2")
			path := writeStubNodeScript("v24.9.9") + string(os.PathListSeparator) + miseDir + string(os.PathListSeparator) + pathExcludingToolchain()
			GinkgoT().Setenv("PATH", path)
			GinkgoT().Setenv("HOME", os.Getenv("HOME"))

			revision, err := gitrepo.ResolveBaselineRevision(repo)
			Expect(err).NotTo(HaveOccurred())
			before, err := projectcheck.Run(repo, revision, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(before.Checks.Compiler.State).To(Equal(projectreadiness.Fail), "sanity: the fixture must start without a usable compiler")

			stdin := authoringStdin("mise_global\ninstall\n")
			defer stdin.Close()
			stdoutFile, stderrFile, readStdout, readStderr := authoringOutputFiles()
			defer stdoutFile.Close()
			defer stderrFile.Close()

			exitCode := prepareCompilerMiseTypeScript(repo, stdin, stdoutFile, stderrFile, "")

			transcript := readStderr()

			Expect(transcript).To(ContainSubstring("  - mise_global"), "transcript: %s", transcript)
			Expect(transcript).NotTo(ContainSubstring("  - mise_project"), "the hazardous project mise.toml must withhold mise_project from the offered choices, transcript: %s", transcript)

			Expect(exitCode).To(Equal(0), "stderr: %s", transcript)
			Expect(readStdout()).To(BeEmpty(), "no report must ever reach stdout from this flow")
			Expect(miseInvocationsIncludeInstall(miseDir)).To(BeTrue(), "a confirmed selection must invoke `mise install`")

			Expect(transcript).To(ContainSubstring("installed TypeScript 7.0.2 via mise_global; rerun readiness reports compiler check pass (version=7.0.2)"), "transcript: %s", transcript)
		})
	})

	When("the caller supplies a context deadline shorter than mise's own five-minute install timeout, and `mise install` runs long enough to exceed it", func() {
		It("cuts the install off at that shorter deadline: attempted but never observed, well before the stub's own sleep would otherwise finish", func() {
			repo := noSupportedCompilerRepo()
			writeWorktreeFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \"7.0.2\"\n")
			miseDir, err := os.MkdirTemp("", "coach-acceptance-slowinstallmise-*")
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(os.RemoveAll, miseDir)
			script := fmt.Sprintf("#!/bin/sh\necho \"$@\" >> %q\n"+
				"if [ \"$1\" = \"--version\" ]; then echo \"2026.9.5 linux-x64 (2026-09-10)\"; exit 0; fi\n"+
				"if [ \"$1\" = \"config\" ] && [ \"$2\" = \"ls\" ]; then echo \"[]\"; exit 0; fi\n"+
				"if [ \"$1\" = \"config\" ] && [ \"$2\" = \"get\" ]; then exit 1; fi\n"+
				"if [ \"$1\" = \"install\" ]; then exec sleep 30; fi\n"+
				"exit 1\n", filepath.Join(miseDir, stubMiseInvocationLog))
			Expect(os.WriteFile(filepath.Join(miseDir, "mise"), []byte(script), 0o755)).To(Succeed())
			path := writeStubNodeScript("v24.9.9") + string(os.PathListSeparator) + miseDir + string(os.PathListSeparator) + pathExcludingToolchain()
			GinkgoT().Setenv("PATH", path)
			GinkgoT().Setenv("HOME", os.Getenv("HOME"))

			revision, err := gitrepo.ResolveBaselineRevision(repo)
			Expect(err).NotTo(HaveOccurred())
			readiness, err := projectcheck.Run(repo, revision, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(readiness.Checks.Compiler.Code).To(Equal(projectreadiness.GapTypescriptCompilerMissing), "sanity: the fixture must start without a usable compiler")

			start := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			stdin := authoringStdin("mise_project\ninstall\n")
			defer stdin.Close()
			var transcript bytes.Buffer

			result := tssetup.RunPrepareCompilerMiseSetup(ctx, repo, revision, "", readiness, stdin, &transcript)
			elapsed := time.Since(start)

			Expect(elapsed).To(BeNumerically(">=", 2*time.Second), "the install must run at least as long as the supplied context deadline, not fail some unrelated, faster way, got elapsed=%s transcript=%s", elapsed, transcript.String())
			Expect(elapsed).To(BeNumerically("<", 15*time.Second), "a 2s context deadline must cut the install off well before the stub's 30s sleep would otherwise finish, got elapsed=%s transcript=%s", elapsed, transcript.String())
			Expect(result.Trusted).To(BeTrue(), "%+v", result)
			Expect(result.Attempted).To(BeTrue(), "the install subprocess must have actually started: %+v", result)
			Expect(result.Succeeded).To(BeFalse(), "a deadline-cut install must never be reported as succeeded: %+v", result)
			Expect(miseInvocationsIncludeInstall(miseDir)).To(BeTrue())
		})
	})
})
