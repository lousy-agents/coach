//go:build linux

package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

const (
	projectConfigCopyPrompt                  = "Copy project.json to .lousy-agents/coach.json? Coach will not delete project.json. [y/N] "
	projectConfigCopiedLine                  = "Copied project.json to .lousy-agents/coach.json. Commit that file so the next scan reads it. project.json was left in place.\n"
	wantProjectConfigMigrationSkippedMessage = "project.json can be moved to .lousy-agents/coach.json. Next: top=\"$(git rev-parse --show-toplevel)\" && mkdir -p \"$top\"/.lousy-agents && git -C \"$top\" mv -- project.json .lousy-agents/coach.json"
)

var _ = Describe("coach project config migration", func() {
	When("a baseline scan selects the deprecated root project.json and stdin is a controlling terminal", func() {
		It("treats an empty answer as no, leaves both files unchanged, and keeps only the deprecation diagnostic", func() {
			repo := commitDeprecatedProjectConfigRepo()
			before := readRepoFile(repo, legacyConfigPath)

			stdout, stderr, exitCode := runMigrationBaselineOnTerminal(repo, projectConfigMigrationEnv(), "\n")

			Expect(exitCode).To(Equal(0), "stderr: %s\nstdout: %s", stderr, stdout)
			Expect(strings.Count(string(stderr), projectConfigCopyPrompt)).To(Equal(1), "the prompt must be printed exactly once; stderr: %s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("Copied project.json"))
			expectRepoFileAbsent(repo, coachConfigPath)
			expectRepoFileAbsent(repo, ".lousy-agents")
			Expect(readRepoFile(repo, legacyConfigPath)).To(Equal(before))

			report := decodeCoachReport(stdout)
			Expect(report.SchemaVersion).To(Equal("2"))
			expectDiagnosticKind(report, "project_config_deprecated", legacyConfigPath)
			expectNoDiagnosticKind(report, "project_config_migration_skipped")
			expectNoDiagnosticKind(report, "project_config_uncommitted")
		})

		It("copies project.json when the answer is yes, leaves project.json in place, and matches the no answer's exit code and diagnostics", func() {
			repo := commitDeprecatedProjectConfigRepo()
			before := readRepoFile(repo, legacyConfigPath)
			noStdout, noStderr, noExit := runMigrationBaselineOnTerminal(repo, projectConfigMigrationEnv(), "\n")
			Expect(noExit).To(Equal(0), "stderr: %s\nstdout: %s", noStderr, noStdout)
			Expect(readRepoFile(repo, legacyConfigPath)).To(Equal(before))
			expectRepoFileAbsent(repo, coachConfigPath)
			noReport := decodeCoachReport(noStdout)

			stdout, stderr, exitCode := runMigrationBaselineOnTerminal(repo, projectConfigMigrationEnv(), "y\n")

			Expect(exitCode).To(Equal(0), "stderr: %s\nstdout: %s", stderr, stdout)
			Expect(exitCode).To(Equal(noExit), "a yes answer must not change the scan exit code")
			Expect(strings.Count(string(stderr), projectConfigCopyPrompt)).To(Equal(1), "the prompt must be printed exactly once; stderr: %s", stderr)
			Expect(string(stderr)).To(ContainSubstring(projectConfigCopiedLine))
			Expect(readRepoFile(repo, coachConfigPath)).To(Equal(before), "the copy must be the worktree project.json bytes")
			Expect(readRepoFile(repo, legacyConfigPath)).To(Equal(before), "project.json must stay in place and unchanged")

			report := decodeCoachReport(stdout)
			Expect(report.SchemaVersion).To(Equal("2"))
			Expect(report.Signals).To(Equal(noReport.Signals), "a yes answer still analyzes the committed project.json")
			Expect(report.Diagnostics).To(Equal(noReport.Diagnostics))
			expectDiagnosticKind(report, "project_config_deprecated", legacyConfigPath)
			expectNoDiagnosticKind(report, "project_config_uncommitted")
			expectNoDiagnosticKind(report, "project_config_migration_skipped")
		})
	})

	When("a baseline scan selects the deprecated root project.json on a controlling terminal but the invocation is non-interactive", func() {
		It("does not prompt or write when --no-interactive is set, and emits project_config_migration_skipped", func() {
			repo := commitDeprecatedProjectConfigRepo()

			stdout, stderr, exitCode := runMigrationBaselineOnTerminal(repo, projectConfigMigrationEnv(), "y\n", "--no-interactive")

			Expect(exitCode).To(Equal(0), "stderr: %s\nstdout: %s", stderr, stdout)
			Expect(string(stderr)).NotTo(ContainSubstring(projectConfigCopyPrompt))
			expectRepoFileAbsent(repo, coachConfigPath)
			expectMigrationSkipped(decodeCoachReport(stdout))
		})

		It("does not prompt or write when CI is set, and emits project_config_migration_skipped", func() {
			repo := commitDeprecatedProjectConfigRepo()

			stdout, stderr, exitCode := runMigrationBaselineOnTerminal(repo, projectConfigMigrationEnv("CI=true"), "y\n")

			Expect(exitCode).To(Equal(0), "stderr: %s\nstdout: %s", stderr, stdout)
			Expect(string(stderr)).NotTo(ContainSubstring(projectConfigCopyPrompt))
			expectRepoFileAbsent(repo, coachConfigPath)
			expectMigrationSkipped(decodeCoachReport(stdout))
		})
	})

	When("a baseline scan selects the deprecated root project.json and stdin is not a controlling terminal", func() {
		It("emits project_config_migration_skipped and does not create .lousy-agents/coach.json", func() {
			repo := commitDeprecatedProjectConfigRepo()

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json")

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			expectRepoFileAbsent(repo, coachConfigPath)
			expectMigrationSkipped(decodeCoachReport(stdout))
		})
	})

	When("the customer accepts the copy and the repository cannot be written to", func() {
		It("emits project_config_migration_failed, creates nothing, and leaves project.json unchanged", func() {
			if os.Geteuid() == 0 {
				Skip("root writes to a read-only directory, so this state cannot be built")
			}
			repo := commitDeprecatedProjectConfigRepo()
			Expect(os.Chmod(repo, 0o555)).To(Succeed())
			DeferCleanup(func() { _ = os.Chmod(repo, 0o755) })
			before := treeOf(repo)

			stdout, stderr, exitCode := runMigrationBaselineOnTerminal(repo, projectConfigMigrationEnv(), "y\n")

			Expect(exitCode).To(Equal(0), "stderr: %s\nstdout: %s", stderr, stdout)
			Expect(treeOf(repo)).To(Equal(before), "a copy that failed left the worktree as it was")
			report := decodeCoachReport(stdout)
			Expect(report.SchemaVersion).To(Equal("2"))
			expectDiagnosticKind(report, "project_config_deprecated", legacyConfigPath)
			failed, ok := diagnosticFor(report, "project_config_migration_failed", coachConfigPath)
			Expect(ok).To(BeTrue(), "diagnostics: %+v", report.Diagnostics)
			Expect(failed.Message).To(HavePrefix("Could not copy project.json to .lousy-agents/coach.json ("))
			Expect(failed.Message).To(HaveSuffix("). Left both files unchanged."))
		})
	})

	DescribeTable("offers no copy and changes nothing, whatever the answer, when the path holds", requireNoOfferWhereThePathIsOccupied, occupiedCanonicalEntries())

	When("an uncommitted .lousy-agents/coach.json already exists beside a valid committed project.json", func() {
		It("does not prompt and does not change the existing file", func() {
			repo := commitDeprecatedProjectConfigRepo()
			existing := []byte("{\"schema_version\":\"1\",\"roots\":[\".\"],\"note\":\"keep\"}\n")
			writeWorktreeFile(repo, coachConfigPath, string(existing))

			stdout, stderr, exitCode := runMigrationBaselineOnTerminal(repo, projectConfigMigrationEnv(), "y\n")

			Expect(exitCode).To(Equal(0), "stderr: %s\nstdout: %s", stderr, stdout)
			Expect(string(stderr)).NotTo(ContainSubstring(projectConfigCopyPrompt))
			Expect(readRepoFile(repo, coachConfigPath)).To(Equal(existing))

			report := decodeCoachReport(stdout)
			expectDiagnosticKind(report, "project_config_deprecated", legacyConfigPath)
			expectDiagnosticKind(report, "project_config_uncommitted", coachConfigPath)
			expectNoDiagnosticKind(report, "project_config_migration_skipped")
		})
	})

	When("the command is not a codesignal scan", func() {
		It("does not prompt or create .lousy-agents/coach.json for --check-project", func() {
			repo := commitDeprecatedProjectConfigRepo()

			stdout, stderr, _, exitCode := runCoachBinaryWithControllingTerminal(commandPath, repo, projectConfigMigrationEnv(), "",
				"codesignal", "--baseline", "--check-project", "--project-language", "typescript", "--format=json")

			Expect(exitCode).To(Equal(0), "stderr: %s\nstdout: %s", stderr, stdout)
			Expect(string(stderr)).NotTo(ContainSubstring("Copy project.json"))
			Expect(string(stdout)).NotTo(ContainSubstring("project_config_migration_skipped"))
			expectRepoFileAbsent(repo, coachConfigPath)
		})

		It("does not prompt to copy a committed project.json for --suggest-project-config, and a written .lousy-agents/coach.json is not that copy", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", "module example.com/suggest\n\ngo 1.25\n")
			commitFile(repo, "main.go", "package main\n\nfunc main() {}\n")
			commitFile(repo, legacyConfigPath, singleRootPolicyJSON)
			projectJSON := readRepoFile(repo, legacyConfigPath)

			stdout, stderr, _, exitCode := runCoachBinaryWithControllingTerminal(commandPath, repo, projectConfigMigrationEnv(), "",
				"codesignal", "--baseline", "--suggest-project-config")

			Expect(exitCode).To(Equal(0), "stderr: %s\nstdout: %s", stderr, stdout)
			Expect(string(stderr)).NotTo(ContainSubstring("Copy project.json"))
			Expect(readRepoFile(repo, legacyConfigPath)).To(Equal(projectJSON))
			written, err := os.ReadFile(filepath.Join(repo, coachConfigPath))
			if err != nil {
				Expect(os.IsNotExist(err)).To(BeTrue(), "reading %s: %v", coachConfigPath, err)
				return
			}
			Expect(written).NotTo(Equal(projectJSON), ".lousy-agents/coach.json must be the suggest candidate, not a copy of project.json")
		})

		It("does not prompt, emit project_config_migration_skipped, or copy project.json for --prepare-compiler", func() {
			repo := commitDeprecatedProjectConfigRepo()
			before := readRepoFile(repo, legacyConfigPath)

			stdout, stderr, _, exitCode := runCoachBinaryWithControllingTerminal(commandPath, repo, projectConfigMigrationEnv(), "",
				"codesignal", "--baseline", "--prepare-compiler", "--project-language", "typescript", "--no-interactive")

			Expect(string(stderr)).NotTo(ContainSubstring("Copy project.json"), "exit %d stderr: %s stdout: %s", exitCode, stderr, stdout)
			Expect(string(stdout)).NotTo(ContainSubstring("project_config_migration_skipped"), "stdout: %s", stdout)
			Expect(string(stderr)).NotTo(ContainSubstring("project_config_migration_skipped"), "stderr: %s", stderr)
			Expect(readRepoFile(repo, legacyConfigPath)).To(Equal(before))
			expectRepoFileAbsent(repo, coachConfigPath)
		})
	})
})

func projectConfigMigrationEnv(extra ...string) []string {
	env := []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME")}
	return append(env, extra...)
}

func runMigrationBaselineOnTerminal(repo string, env []string, stdinScript string, extraArgs ...string) (stdout, stderr []byte, exitCode int) {
	args := append([]string{"codesignal", "--baseline", "--format=json"}, extraArgs...)
	stdout, stderr, _, exitCode = runCoachBinaryWithControllingTerminal(commandPath, repo, env, stdinScript, args...)
	return stdout, stderr, exitCode
}

func readRepoFile(repo, name string) []byte {
	GinkgoHelper()
	body, err := os.ReadFile(filepath.Join(repo, name))
	Expect(err).NotTo(HaveOccurred(), "reading %s", name)
	return body
}

func expectRepoFileAbsent(repo, name string) {
	GinkgoHelper()
	_, err := os.Stat(filepath.Join(repo, name))
	Expect(os.IsNotExist(err)).To(BeTrue(), "expected %s to be absent, stat: %v", name, err)
}

func expectMigrationSkipped(report *codesignal.Report) {
	GinkgoHelper()
	diag, ok := diagnosticFor(report, "project_config_migration_skipped", legacyConfigPath)
	Expect(ok).To(BeTrue(), "diagnostics: %+v", report.Diagnostics)
	Expect(diag.Message).To(Equal(wantProjectConfigMigrationSkippedMessage))
}

func requireNoOfferWhereThePathIsOccupied(s canonicalState) {
	repo := commitDeprecatedProjectConfigRepo()
	s.arrange(repo)
	before := treeOf(repo)

	stdout, stderr, exitCode := runMigrationBaselineOnTerminal(repo, projectConfigMigrationEnv(), "y\n")

	Expect(exitCode).To(Equal(0), "stderr: %s\nstdout: %s", stderr, stdout)
	Expect(string(stderr)).NotTo(ContainSubstring("Copy project.json"), "the path cannot be the policy, so there is nowhere to copy to")
	report := decodeCoachReport(stdout)
	expectDiagnosticKind(report, s.wantKind, coachConfigPath)
	expectNoDiagnosticKind(report, "project_config_migration_failed")
	expectNoDiagnosticKind(report, "project_config_migration_skipped")
	Expect(treeOf(repo)).To(Equal(before), "answering yes changed the worktree")
}

// treeOf is every path under repo but .git with what Lstat says of it and where a link points, so that a spec can
// see that a run changed nothing. A directory the process cannot read is listed as unreadable.
func treeOf(repo string) map[string]string {
	tree := map[string]string{}
	_ = filepath.WalkDir(repo, func(full string, entry fs.DirEntry, walkErr error) error {
		relative, _ := filepath.Rel(repo, full)
		if relative == ".git" {
			return filepath.SkipDir
		}
		tree[relative] = describeTreeEntry(full, walkErr)
		return nil
	})
	return tree
}

func describeTreeEntry(full string, walkErr error) string {
	info, err := os.Lstat(full)
	if walkErr != nil || err != nil {
		return "unreadable"
	}
	target, _ := os.Readlink(full)
	return fmt.Sprintf("%s %d %s", info.Mode(), info.Size(), target)
}
