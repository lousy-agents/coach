package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli"
)

// writeFailingSetupExecutableWithResidue writes an executable named `name`
// that models a real npm/pnpm/bun install that partially writes to disk
// before failing: it creates node_modules/residue-pkg/ in its own working
// directory, then exits 1.
func writeFailingSetupExecutableWithResidue(name string) string {
	dir, err := os.MkdirTemp("", "coach-acceptance-stubsetup-fail-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, dir)

	script := "#!/bin/sh\nmkdir -p node_modules/residue-pkg\nprintf '{}' > node_modules/residue-pkg/package.json\nexit 1\n"
	Expect(os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755)).To(Succeed())
	return dir
}

func gitHeadSHA(dir string) string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	Expect(err).NotTo(HaveOccurred())
	return strings.TrimSpace(string(out))
}

var _ = Describe("codesignalcli.RunConfirmedSetup", func() {
	When("confirmed is false", func() {
		It("returns exit 2 with no report and no mutation, without ever calling ExecuteSetup (AC-SET-7 policy, AC-SET-8 cancellation clause)", func() {
			workDir := newTempGitRepo()
			commitFile(workDir, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			beforeHead := gitHeadSHA(workDir)

			stubDir := writeStubSetupExecutable("npm")
			GinkgoT().Setenv("PATH", stubDir+string(os.PathListSeparator)+setupExecutionOnlyPath())

			preview, err := codesignalcli.BuildSetupPreview(codesignalcli.SetupChoice{Kind: codesignalcli.SetupChoiceProjectPackage}, "npm", workDir)
			Expect(err).NotTo(HaveOccurred())

			outcome, err := codesignalcli.RunConfirmedSetup(context.Background(), preview, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(outcome.Kind).To(Equal(codesignalcli.SetupOutcomeCancelled))
			Expect(outcome.ExitCode).To(Equal(2))
			Expect(outcome.Execution).To(Equal(codesignalcli.SetupExecutionResult{}))
			Expect(outcome.ChangedPaths).To(BeEmpty())

			Expect(stubSetupInvoked(stubDir, "npm")).To(BeFalse(), "cancellation must never start a subprocess")
			Expect(gitHeadSHA(workDir)).To(Equal(beforeHead), "cancellation must not mutate the working directory")

			status, statusErr := exec.Command("git", "-C", workDir, "status", "--porcelain", "--ignored").Output()
			Expect(statusErr).NotTo(HaveOccurred())
			Expect(strings.TrimSpace(string(status))).To(BeEmpty(), "cancellation must leave the working directory exactly as committed")
		})
	})

	When("ExecuteSetup runs and the command exits non-zero after partially writing to disk", func() {
		It("returns exit 2 with no report and identifies files that may have changed, without any destructive rollback (AC-SET-7)", func() {
			workDir := newTempGitRepo()
			commitFile(workDir, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			// node_modules/ must actually be gitignored, or this spec cannot
			// distinguish "--ignored was passed to git status" from "the
			// residue directory merely happened to be untracked" -- both
			// look identical (`?? node_modules/`) without a .gitignore.
			commitFile(workDir, ".gitignore", "node_modules/\n")
			beforeHead := gitHeadSHA(workDir)

			stubDir := writeFailingSetupExecutableWithResidue("npm")
			GinkgoT().Setenv("PATH", stubDir+string(os.PathListSeparator)+setupExecutionOnlyPath())

			preview, err := codesignalcli.BuildSetupPreview(codesignalcli.SetupChoice{Kind: codesignalcli.SetupChoiceProjectPackage}, "npm", workDir)
			Expect(err).NotTo(HaveOccurred())

			outcome, err := codesignalcli.RunConfirmedSetup(context.Background(), preview, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(outcome.Kind).To(Equal(codesignalcli.SetupOutcomeFailed))
			Expect(outcome.ExitCode).To(Equal(2))
			Expect(outcome.Execution.Succeeded).To(BeFalse())
			Expect(outcome.Execution.ExitCode).To(Equal(1))
			Expect(outcome.ChangedPaths).NotTo(BeEmpty(), "a failure that partially wrote to disk must identify what may have changed")
			joined := strings.Join(outcome.ChangedPaths, "\n")
			Expect(joined).To(ContainSubstring("node_modules"), "the residue disclosure must surface the partially populated node_modules directory")

			Expect(filepath.Join(workDir, "node_modules", "residue-pkg", "package.json")).To(BeAnExistingFile(), "the fixture must actually have partially installed, or this proves nothing")

			// No destructive rollback: HEAD, the index, and the originally
			// committed file are exactly as they were before the failed run.
			Expect(gitHeadSHA(workDir)).To(Equal(beforeHead))
			data, readErr := os.ReadFile(filepath.Join(workDir, "package.json"))
			Expect(readErr).NotTo(HaveOccurred())
			Expect(string(data)).To(Equal(`{"name":"example","version":"1.0.0"}` + "\n"))

			diffOut, diffErr := exec.Command("git", "-C", workDir, "diff", "--name-only", "HEAD").Output()
			Expect(diffErr).NotTo(HaveOccurred())
			Expect(strings.TrimSpace(string(diffOut))).To(BeEmpty(), "no tracked file may have been modified by a failed setup")
		})
	})

	When("ExecuteSetup succeeds", func() {
		It("signals proceed, carrying no exit-2 policy", func() {
			workDir := newTempGitRepo()
			commitFile(workDir, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")

			stubDir := writeStubSetupExecutable("npm")
			GinkgoT().Setenv("PATH", stubDir+string(os.PathListSeparator)+setupExecutionOnlyPath())

			preview, err := codesignalcli.BuildSetupPreview(codesignalcli.SetupChoice{Kind: codesignalcli.SetupChoiceProjectPackage}, "npm", workDir)
			Expect(err).NotTo(HaveOccurred())

			outcome, err := codesignalcli.RunConfirmedSetup(context.Background(), preview, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(outcome.Kind).To(Equal(codesignalcli.SetupOutcomeSucceeded))
			Expect(outcome.ExitCode).To(Equal(0))
			Expect(outcome.Execution.Succeeded).To(BeTrue())
		})
	})
})

// destructiveGitVerbPattern matches a quoted Go string literal for a
// mutating git subcommand (reset/clean/checkout) -- the kind of thing a
// rollback attempt would pass as a git exec argument. It only matches
// inside quotes, so this check does not trip on those words appearing in
// prose inside a doc comment.
var destructiveGitVerbPattern = regexp.MustCompile(`"(reset|clean|checkout)"`)

var _ = Describe("project_ts_setup_execute.go's source (no destructive rollback, AC-SET-7/AC-18)", func() {
	It("never spawns a git reset/clean/checkout, or any other mutating git subcommand", func() {
		source, err := os.ReadFile(filepath.Join("..", "..", "internal", "codesignalcli", "project_ts_setup_execute.go"))
		Expect(err).NotTo(HaveOccurred())
		Expect(destructiveGitVerbPattern.FindString(string(source))).To(BeEmpty(), "project_ts_setup_execute.go must not invoke a mutating git subcommand as part of failure handling")
	})
})
