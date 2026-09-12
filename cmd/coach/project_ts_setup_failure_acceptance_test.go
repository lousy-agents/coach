package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
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

	When("WorkingDirectory is a subdirectory of a larger repository that also has unrelated dirt elsewhere", func() {
		It("scopes the residue disclosure to WorkingDirectory, naming it root-relative rather than collapsing or leaking unrelated paths (AC-SET-7)", func() {
			repoRoot := newTempGitRepo()
			commitFile(repoRoot, ".gitignore", "node_modules/\n")
			commitFile(repoRoot, "packages/app/package.json", `{"name":"app","version":"1.0.0"}`+"\n")
			commitFile(repoRoot, "unrelated/tracked.txt", "original\n")

			// Dirt that has nothing to do with this setup run: a modified
			// tracked file and an untracked file, both outside
			// packages/app. If the residue read is not scoped to
			// WorkingDirectory, these leak into ChangedPaths.
			Expect(os.WriteFile(filepath.Join(repoRoot, "unrelated", "tracked.txt"), []byte("modified\n"), 0o644)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(repoRoot, "unrelated", "untracked.txt"), []byte("new\n"), 0o644)).To(Succeed())

			appDir := filepath.Join(repoRoot, "packages", "app")
			stubDir := writeFailingSetupExecutableWithResidue("npm")
			GinkgoT().Setenv("PATH", stubDir+string(os.PathListSeparator)+setupExecutionOnlyPath())

			preview, err := codesignalcli.BuildSetupPreview(codesignalcli.SetupChoice{Kind: codesignalcli.SetupChoiceProjectPackage}, "npm", appDir)
			Expect(err).NotTo(HaveOccurred())

			outcome, err := codesignalcli.RunConfirmedSetup(context.Background(), preview, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(outcome.Kind).To(Equal(codesignalcli.SetupOutcomeFailed))
			Expect(outcome.ChangedPaths).NotTo(BeEmpty())

			joined := strings.Join(outcome.ChangedPaths, "\n")
			Expect(joined).To(ContainSubstring("packages/app/node_modules"), "the residue path must be reported root-relative, naming the subdirectory setup actually ran in")
			Expect(joined).NotTo(ContainSubstring("unrelated"), "dirt outside WorkingDirectory must never be reported as this run's residue")
			for _, path := range outcome.ChangedPaths {
				Expect(path).NotTo(Equal("packages/"), "the residue disclosure must not collapse to an ancestor directory that doesn't even name node_modules")
			}
		})
	})

	When("WorkingDirectory is not inside any Git worktree", func() {
		It("reports the failure with ResidueUnknown rather than a value indistinguishable from a real status read (AC-SET-7)", func() {
			workDir := newSetupExecutionWorkDir()

			stubDir := writeFailingSetupExecutableWithResidue("npm")
			GinkgoT().Setenv("PATH", stubDir+string(os.PathListSeparator)+setupExecutionOnlyPath())

			preview, err := codesignalcli.BuildSetupPreview(codesignalcli.SetupChoice{Kind: codesignalcli.SetupChoiceProjectPackage}, "npm", workDir)
			Expect(err).NotTo(HaveOccurred())

			outcome, err := codesignalcli.RunConfirmedSetup(context.Background(), preview, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(outcome.Kind).To(Equal(codesignalcli.SetupOutcomeFailed))
			Expect(outcome.ExitCode).To(Equal(2))
			Expect(outcome.ResidueUnknown).To(BeTrue(), "a git status failure must be signaled distinctly, not silently reported as if it were a real result")
		})
	})

	When("ExecuteSetup itself refuses because the preview fails frozen-matrix verification", func() {
		It("reports the failure without scanning for residue, since no subprocess ever started (AC-SET-7, AC-18)", func() {
			workDir := newSetupExecutionWorkDir()

			tampered := codesignalcli.SetupPreview{
				Executable:       "npm",
				Args:             []string{"ci"}, // --ignore-scripts dropped: fails ExecuteSetup's verification
				WorkingDirectory: workDir,
				Timeout:          codesignalcli.SetupPreviewTimeout,
			}

			outcome, err := codesignalcli.RunConfirmedSetup(context.Background(), tampered, true)
			Expect(errors.Is(err, codesignalcli.ErrSetupExecutionUnverifiedCommand)).To(BeTrue())
			Expect(outcome.Kind).To(Equal(codesignalcli.SetupOutcomeFailed))
			Expect(outcome.ExitCode).To(Equal(2))
			Expect(outcome.Execution).To(Equal(codesignalcli.SetupExecutionResult{}))
			Expect(outcome.ChangedPaths).To(BeEmpty(), "a run that never started must never report residue -- workDir is not even a Git worktree, so a residue scan here would silently fall back to a value identical to a real failure's disclosure")
			Expect(outcome.ResidueUnknown).To(BeFalse(), "nothing was scanned, so this is not the 'scan failed' case either")
		})
	})
})
