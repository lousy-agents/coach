package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

const (
	codesignalModeBaseline = "baseline"
	codesignalModeBase     = "base"

	benignGoA = "package a\n\nfunc Alpha() int { return 1 }\n"
	benignGoB = "package a\n\nfunc Alpha() int { return 2 }\n"
)

var _ = Describe("coach codesignal working tree disclosure", func() {
	When("an intent-to-add Go file contains a hidden input mutation the committed snapshot does not", func() {
		DescribeTable("coach codesignal reports the file as untracked and not analyzed",
			func(mode, format string) {
				assertIntentToAddGoDisclosure(mode, format)
			},
			Entry("baseline text", codesignalModeBaseline, "text"),
			Entry("baseline json", codesignalModeBaseline, "json"),
			Entry("--base HEAD~1 text", codesignalModeBase, "text"),
			Entry("--base HEAD~1 json", codesignalModeBase, "json"),
		)
	})

	When("an untracked Go file contains a hidden input mutation the committed snapshot does not", func() {
		DescribeTable("coach codesignal --baseline reports that the file exists and was not analyzed",
			func(format string) {
				assertUntrackedGoDisclosure(codesignalModeBaseline, format)
			},
			Entry("text", "text"),
			Entry("json", "json"),
		)

		DescribeTable("coach codesignal --base HEAD~1 reports that the file exists and was not analyzed",
			func(format string) {
				assertUntrackedGoDisclosure(codesignalModeBase, format)
			},
			Entry("text", "text"),
			Entry("json", "json"),
		)
	})

	When("the working tree is clean", func() {
		It("prints today's unqualified all-clear for --baseline, with no Diagnostics section and no worktree disclosure", func() {
			repo := newTempGitRepo()
			commitBenignHistory(repo)
			Expect(strings.TrimSpace(gitPorcelain(repo))).To(BeEmpty())
			revision := headRevision(repo)
			wantText := expectedCleanBaselineText(revision, 1, 1)
			wantJSON := expectedCleanBaselineJSON(revision)

			textOut, textErr, textExit := runWorkingTreeCodesignal(repo, codesignalModeBaseline, "text")
			Expect(textExit).To(Equal(0), "stderr: %s\nstdout: %s", textErr, textOut)
			Expect(textErr).To(BeEmpty())
			expectCleanReportOmitsWorktreeDisclosure(textOut, "text", wantText)

			jsonOut, jsonErr, jsonExit := runWorkingTreeCodesignal(repo, codesignalModeBaseline, "json")
			Expect(jsonExit).To(Equal(0), "stderr: %s\nstdout: %s", jsonErr, jsonOut)
			Expect(jsonErr).To(BeEmpty())
			expectCleanReportOmitsWorktreeDisclosure(jsonOut, "json", wantJSON)
		})

		It("prints today's unqualified all-clear for --base HEAD~1, with no Diagnostics section and no worktree disclosure", func() {
			repo := newTempGitRepo()
			commitBenignHistory(repo)
			Expect(strings.TrimSpace(gitPorcelain(repo))).To(BeEmpty())
			revision := headRevision(repo)
			base := headParentRevision(repo)
			wantText := expectedCleanDiffText(1)
			wantJSON := expectedCleanBaseJSON(revision, base)

			textOut, textErr, textExit := runWorkingTreeCodesignal(repo, codesignalModeBase, "text")
			Expect(textExit).To(Equal(0), "stderr: %s\nstdout: %s", textErr, textOut)
			Expect(textErr).To(BeEmpty())
			expectCleanReportOmitsWorktreeDisclosure(textOut, "text", wantText)

			jsonOut, jsonErr, jsonExit := runWorkingTreeCodesignal(repo, codesignalModeBase, "json")
			Expect(jsonExit).To(Equal(0), "stderr: %s\nstdout: %s", jsonErr, jsonOut)
			Expect(jsonErr).To(BeEmpty())
			expectCleanReportOmitsWorktreeDisclosure(jsonOut, "json", wantJSON)
		})
	})

	When("a staged Go file contains a hidden input mutation the committed snapshot does not", func() {
		DescribeTable("coach codesignal reports the staged file as staged and not analyzed",
			func(mode, format string) {
				assertStagedGoDisclosure(mode, format)
			},
			Entry("baseline text", codesignalModeBaseline, "text"),
			Entry("baseline json", codesignalModeBaseline, "json"),
			Entry("--base HEAD~1 text", codesignalModeBase, "text"),
			Entry("--base HEAD~1 json", codesignalModeBase, "json"),
		)
	})

	When("a modified tracked Go file contains a hidden input mutation the committed snapshot does not", func() {
		DescribeTable("coach codesignal reports the modified file as modified and not analyzed",
			func(mode, format string) {
				assertModifiedGoDisclosure(mode, format)
			},
			Entry("baseline text", codesignalModeBaseline, "text"),
			Entry("baseline json", codesignalModeBaseline, "json"),
			Entry("--base HEAD~1 text", codesignalModeBase, "text"),
			Entry("--base HEAD~1 json", codesignalModeBase, "json"),
		)
	})

	When("the same hidden-input-mutation bytes are committed in a clean repository", func() {
		It("reports state.hidden_input_mutation, so an uncommitted copy is a finding the dirty-tree scan would have produced", func() {
			repo := newTempGitRepo()
			commitFile(repo, "pending.go", hiddenInputMutationGo)
			Expect(strings.TrimSpace(gitPorcelain(repo))).To(BeEmpty())

			stdout, stderr, exitCode := runWorkingTreeCodesignal(repo, codesignalModeBaseline, "json")
			Expect(exitCode).To(Equal(0), "stderr: %s\nstdout: %s", stderr, stdout)
			Expect(stderr).To(BeEmpty())

			report := decodeCoachReport(stdout)
			signals := signalsForPath(report, "pending.go")
			Expect(signals).NotTo(BeEmpty(), "committed hiddenInputMutationGo must produce a signal; report: %+v", report.Signals)
			Expect(signals[0].RuleID).To(Equal("state.hidden_input_mutation"))
		})
	})

	When("the worktree has an untracked finding-bearing Go file and a committed finding-bearing Go file", func() {
		DescribeTable("signals match the committed snapshot only",
			func(mode string) {
				repo := newTempGitRepo()
				commitFile(repo, "a.go", benignGoA)
				commitFile(repo, "committed.go", hiddenInputMutationGo)
				writeUntrackedFile(repo, "pending.go", hiddenInputMutationGo)
				expectDirtyPorcelain(repo, "?? pending.go")

				stdout, stderr, exitCode := runWorkingTreeCodesignal(repo, mode, "json")
				Expect(exitCode).To(Equal(0), "stderr: %s\nstdout: %s", stderr, stdout)
				Expect(stderr).To(BeEmpty())

				report := decodeCoachReport(stdout)
				committed := signalsForPath(report, "committed.go")
				Expect(committed).NotTo(BeEmpty(), "the committed finding must still be reported")
				Expect(committed[0].RuleID).To(Equal("state.hidden_input_mutation"))
				Expect(signalsForPath(report, "pending.go")).To(BeEmpty(),
					"untracked bytes must not be analyzed into a signal; signals: %+v", report.Signals)
			},
			Entry("baseline", codesignalModeBaseline),
			Entry("--base HEAD~1", codesignalModeBase),
		)
	})

	When("uncommitted supported-language files exist and the run otherwise succeeds", func() {
		DescribeTable("the exit status stays 0",
			func(mode string) {
				repo := newTempGitRepo()
				commitBenignHistory(repo)
				writeUntrackedFile(repo, "pending.go", hiddenInputMutationGo)
				expectDirtyPorcelain(repo, "?? pending.go")

				stdout, stderr, exitCode := runWorkingTreeCodesignal(repo, mode, "text")
				Expect(exitCode).To(Equal(0), "stderr: %s\nstdout: %s", stderr, stdout)
				Expect(stderr).To(BeEmpty())
				Expect(string(stdout)).To(ContainSubstring("active signals:"),
					"exit 0 must still be a codesignal report; stdout:\n%s", stdout)
			},
			Entry("baseline", codesignalModeBaseline),
			Entry("--base HEAD~1", codesignalModeBase),
		)
	})

	When("uncommitted files exist but none is a supported language", func() {
		DescribeTable("the CLI states that the working tree is not clean and does not say analyzable work was skipped",
			func(mode, format string) {
				repo := newTempGitRepo()
				commitBenignHistory(repo)
				writeUntrackedFile(repo, "notes.md", "# notes\n")
				writeUntrackedFile(repo, "readme.txt", "hello\n")
				porcelain := expectDirtyPorcelain(repo, "?? notes.md")
				Expect(porcelain).To(ContainSubstring("?? readme.txt"))

				stdout, stderr, exitCode := runWorkingTreeCodesignal(repo, mode, format)
				Expect(exitCode).To(Equal(0), "stderr: %s\nstdout: %s", stderr, stdout)
				Expect(stderr).To(BeEmpty())
				expectUnsupportedDirtyNotice(stdout)
			},
			Entry("baseline text", codesignalModeBaseline, "text"),
			Entry("baseline json", codesignalModeBaseline, "json"),
			Entry("--base HEAD~1 text", codesignalModeBase, "text"),
			Entry("--base HEAD~1 json", codesignalModeBase, "json"),
		)
	})

	When("many untracked Go files contain a hidden input mutation", func() {
		DescribeTable("the CLI reports the total count and a bounded sample rather than one diagnostic per file",
			func(mode, format string) {
				body_workingTreeDisclosureAcceptanceTest_theCLIReportsTheTotalCountAndABoundedSampleRathe_207(mode, format)
			},
			Entry("baseline text", codesignalModeBaseline, "text"),
			Entry("baseline json", codesignalModeBaseline, "json"),
			Entry("--base HEAD~1 text", codesignalModeBase, "text"),
			Entry("--base HEAD~1 json", codesignalModeBase, "json"),
		)
	})

	When("the working-tree status check itself fails", func() {
		DescribeTable("the CLI still produces a report and records the failure as a diagnostic",
			func(mode, format string) {
				body_workingTreeDisclosureAcceptanceTest_theCLIStillProducesAReportAndRecordsTheFailureAs_233(mode, format)
			},
			Entry("baseline text", codesignalModeBaseline, "text"),
			Entry("baseline json", codesignalModeBaseline, "json"),
			Entry("--base HEAD~1 text", codesignalModeBase, "text"),
			Entry("--base HEAD~1 json", codesignalModeBase, "json"),
		)
	})

	When("a supported-language file is unmerged (UU)", func() {
		DescribeTable("coach codesignal reports the unmerged file and does not print a silent all-clear",
			func(mode, format string) {
				assertUnmergedGoDisclosure(mode, format)
			},
			Entry("baseline text", codesignalModeBaseline, "text"),
			Entry("baseline json", codesignalModeBaseline, "json"),
			Entry("--base HEAD~1 text", codesignalModeBase, "text"),
			Entry("--base HEAD~1 json", codesignalModeBase, "json"),
		)
	})

	When("a supported-language file is unmerged (AA, both added)", func() {
		DescribeTable("coach codesignal reports the file as unmerged, not staged",
			func(mode, format string) {
				assertUnmergedBothAddedGoDisclosure(mode, format)
			},
			Entry("baseline text", codesignalModeBaseline, "text"),
			Entry("baseline json", codesignalModeBaseline, "json"),
			Entry("--base HEAD~1 text", codesignalModeBase, "text"),
			Entry("--base HEAD~1 json", codesignalModeBase, "json"),
		)
	})

	When("file-local disclosure runs with --project-config on unsupported dirt", func() {
		DescribeTable("notes.md or bun.lock stay a not-clean notice without a skip kind",
			func(mode, format, name, contents string) {
				repo := newTempGitRepo()
				commitGoProjectForDisclosure(repo)
				writeUntrackedFile(repo, name, contents)
				expectDirtyPorcelain(repo, "?? "+name)

				stdout, stderr, exitCode := runWorkingTreeCodesignalWithProject(repo, mode, format)
				Expect(exitCode).To(Equal(0), "stderr: %s\nstdout: %s", stderr, stdout)
				Expect(stderr).To(BeEmpty())
				expectUnsupportedDirtyNotice(stdout)
				Expect(countKind(diagnosticKinds(stdout, format), codesignal.DiagKindWorktreeChangesNotAnalyzed)).To(Equal(0),
					"unsupported dirt must not pick up the project skip kind; kinds=%v stdout:\n%s", diagnosticKinds(stdout, format), stdout)
			},
			Entry("baseline text notes.md", codesignalModeBaseline, "text", "notes.md", "# notes\n"),
			Entry("baseline json notes.md", codesignalModeBaseline, "json", "notes.md", "# notes\n"),
			Entry("--base HEAD~1 text bun.lock", codesignalModeBase, "text", "bun.lock", "# bun lockfile\n"),
			Entry("--base HEAD~1 json bun.lock", codesignalModeBase, "json", "bun.lock", "# bun lockfile\n"),
		)
	})

	When("file-local disclosure runs with --project-config on a supported untracked Go file", func() {
		DescribeTable("the skip kind appears at most once, with an optional provenance kind",
			func(mode, format string) {
				repo := newTempGitRepo()
				commitGoProjectForDisclosure(repo)
				writeUntrackedFile(repo, "pending.go", hiddenInputMutationGo)
				expectDirtyPorcelain(repo, "?? pending.go")

				stdout, stderr, exitCode := runWorkingTreeCodesignalWithProject(repo, mode, format)
				Expect(exitCode).To(Equal(0), "stderr: %s\nstdout: %s", stderr, stdout)
				Expect(stderr).To(BeEmpty())
				expectSupportedWorktreeDisclosure(stdout, format, []string{"pending.go"}, "untracked", "staged", "modified", "unmerged")
				kinds := diagnosticKinds(stdout, format)
				Expect(countKind(kinds, codesignal.DiagKindWorktreeChangesNotAnalyzed)).To(BeNumerically("<=", 1),
					"must not emit two copies of the skip kind; kinds=%v stdout:\n%s", kinds, stdout)
				Expect(countKind(kinds, codesignal.DiagKindWorktreeChangesNotAnalyzed)).To(BeNumerically(">=", 1))
			},
			Entry("baseline text", codesignalModeBaseline, "text"),
			Entry("baseline json", codesignalModeBaseline, "json"),
			Entry("--base HEAD~1 text", codesignalModeBase, "text"),
			Entry("--base HEAD~1 json", codesignalModeBase, "json"),
		)
	})

	When("the working-tree status check fails during a --project-config run", func() {
		DescribeTable("the CLI records one status-failure and does not say work was not analyzed",
			func(mode, format string) {
				body_workingTreeDisclosureAcceptanceTest_theCLIRecordsOneStatusFailureAndDoesNotSayWorkWa_332(mode, format)
			},
			Entry("baseline text", codesignalModeBaseline, "text"),
			Entry("baseline json", codesignalModeBaseline, "json"),
			Entry("--base HEAD~1 text", codesignalModeBase, "text"),
			Entry("--base HEAD~1 json", codesignalModeBase, "json"),
		)
	})
})

// worktreeDisclosureBody is the diagnostic text whose kind is
// worktree_changes_not_analyzed. Callers assert the disclosure contract
// against this body so a file name that appears only as an analyzed signal
// does not satisfy it.
func worktreeDisclosureBody(stdout []byte, format string) string {
	if format == "json" {
		report := decodeCoachReport(stdout)
		var b strings.Builder
		for _, diagnostic := range report.Diagnostics {
			if diagnostic.Kind == codesignal.DiagKindWorktreeChangesNotAnalyzed {
				fmt.Fprintf(&b, "%s %s\n", diagnostic.Path, diagnostic.Message)
			}
		}
		return b.String()
	}
	var b strings.Builder
	for _, line := range strings.Split(string(stdout), "\n") {
		if strings.Contains(line, codesignal.DiagKindWorktreeChangesNotAnalyzed) {
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func expectUnsupportedDirtyNotice(stdout []byte) {
	text := string(stdout)
	ExpectWithOffset(1, strings.ToLower(text)).To(ContainSubstring("working tree is not clean"),
		"uncommitted files of an unsupported language must be reported as a dirty working tree without implying analyzable work was skipped; stdout:\n%s", text)
	for _, forbidden := range []string{
		"not analyzed",
		"not_analyzed",
		"supported language",
		"supported-language",
	} {
		ExpectWithOffset(1, strings.ToLower(text)).NotTo(ContainSubstring(forbidden),
			"unsupported uncommitted files must not be described as skipped analyzable work (%q); stdout:\n%s", forbidden, text)
	}
	for _, forbidden := range []string{"incomplete", "skipped"} {
		ExpectWithOffset(1, hasWord(strings.ToLower(text), forbidden)).To(BeFalse(),
			"unsupported uncommitted files must not be described as skipped analyzable work (%q); stdout:\n%s", forbidden, text)
	}
}

// commitBenignHistory leaves two commits on one path. A second, near-duplicate
// file is reported as a copy (continuity_not_determined) under the rename
// flags coach passes, which would make a clean --base tree look dirty.
func commitBenignHistory(repo string) {
	commitFile(repo, "a.go", benignGoA)
	commitFile(repo, "a.go", benignGoB)
}

func headRevision(repo string) string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repo
	output, err := cmd.Output()
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "git rev-parse HEAD: %s", output)
	return strings.TrimSpace(string(output))
}

func headParentRevision(repo string) string {
	cmd := exec.Command("git", "rev-parse", "HEAD~1")
	cmd.Dir = repo
	output, err := cmd.Output()
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "git rev-parse HEAD~1: %s", output)
	return strings.TrimSpace(string(output))
}

func writeUntrackedFile(repo, name, contents string) {
	path := filepath.Join(repo, name)
	ExpectWithOffset(1, os.MkdirAll(filepath.Dir(path), 0o755)).To(Succeed())
	ExpectWithOffset(1, os.WriteFile(path, []byte(contents), 0o644)).To(Succeed())
}

func stageWorktreeFile(repo, name, contents string) {
	writeUntrackedFile(repo, name, contents)
	addCmd := exec.Command("git", "add", "--", name)
	addCmd.Dir = repo
	output, err := addCmd.CombinedOutput()
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "git add: %s", output)
}

func intentToAddWorktreeFile(repo, name, contents string) {
	writeUntrackedFile(repo, name, contents)
	addCmd := exec.Command("git", "add", "-N", "--", name)
	addCmd.Dir = repo
	output, err := addCmd.CombinedOutput()
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "git add -N: %s", output)
}

func modifyTrackedFile(repo, name, contents string) {
	path := filepath.Join(repo, name)
	ExpectWithOffset(1, os.WriteFile(path, []byte(contents), 0o644)).To(Succeed())
}

func gitPorcelain(repo string) string {
	cmd := exec.Command("git", "status", "--porcelain", "--untracked-files=all")
	cmd.Dir = repo
	output, err := cmd.CombinedOutput()
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "git status --porcelain: %s", output)
	return string(output)
}

func expectDirtyPorcelain(repo, snippet string) string {
	porcelain := gitPorcelain(repo)
	ExpectWithOffset(1, strings.TrimSpace(porcelain)).NotTo(BeEmpty(),
		"fixture must be dirty before coach runs; git status --porcelain was empty")
	ExpectWithOffset(1, porcelain).To(ContainSubstring(snippet),
		"git status --porcelain before coach runs:\n%s", porcelain)
	return porcelain
}

func expectedCleanBaselineText(revision string, tracked, analyzed int) string {
	return fmt.Sprintf("Repository Baseline for revision %s (not a diff comparison)\ntracked files discovered: %d, analyzed: %d, unsupported: 0, excluded: 0, unanalyzable: 0, active signals: 0, diagnostics: 0\nNo active CodeSignal findings.\n", revision, tracked, analyzed)
}

func expectedCleanDiffText(filesAnalyzed int) string {
	return fmt.Sprintf("scope: production, filtered: 0, files analyzed: %d, active signals: 0, diagnostics: 0\nNo active CodeSignal findings.\n", filesAnalyzed)
}

func expectedCleanBaselineJSON(revision string) string {
	return fmt.Sprintf("{\"schema_version\":\"1\",\"scope\":{\"revision\":%q,\"applied_scope\":\"production\",\"baseline\":true},\"summary\":{\"files_analyzed\":1,\"files_with_diagnostics\":0,\"active_signals\":0,\"introduced_signals\":0,\"existing_signals\":0,\"resolved_signals\":0,\"baseline_signals\":0,\"unknown_signals\":0},\"signals\":[],\"diagnostics\":[],\"coverage\":{\"tracked_files_discovered\":1,\"files_analyzed\":1,\"files_unanalyzable\":0,\"unsupported\":[],\"excluded\":[]}}\n", revision)
}

func expectedCleanBaseJSON(revision, base string) string {
	return fmt.Sprintf("{\"schema_version\":\"1\",\"scope\":{\"revision\":%q,\"base\":%q,\"applied_scope\":\"production\"},\"summary\":{\"files_analyzed\":1,\"files_with_diagnostics\":0,\"active_signals\":0,\"introduced_signals\":0,\"existing_signals\":0,\"resolved_signals\":0,\"baseline_signals\":0,\"unknown_signals\":0},\"signals\":[],\"diagnostics\":[],\"coverage\":{\"tracked_files_discovered\":0,\"files_analyzed\":0,\"files_unanalyzable\":0,\"unsupported\":[],\"excluded\":[]}}\n", revision, base)
}

func hasWord(text, word string) bool {
	return regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(word) + `\b`).MatchString(text)
}

// installGitStatusFailureWrapper returns a PATH directory whose git fails
// only when the subcommand is status. Coach invokes git as
// "git -C <dir> <subcommand> ...", so options that precede the subcommand
// (and "--name-status", which is a diff option) must not be treated as status.
func installGitStatusFailureWrapper() (binDir string, env []string) {
	realGit, err := exec.LookPath("git")
	Expect(err).NotTo(HaveOccurred())

	binDir, err = os.MkdirTemp("", "coach-git-status-wrapper-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, binDir)

	script := fmt.Sprintf(`#!/bin/sh
real=%q
prev=
for arg in "$@"; do
  if [ -n "$prev" ]; then
    prev=
    continue
  fi
  case "$arg" in
    -C|-c|--git-dir|--work-tree|--namespace)
      prev=$arg
      continue
      ;;
    -*)
      continue
      ;;
    status)
      echo "git status failed" >&2
      exit 1
      ;;
    *)
      exec "$real" "$@"
      ;;
  esac
done
exec "$real" "$@"
`, realGit)
	Expect(os.WriteFile(filepath.Join(binDir, "git"), []byte(script), 0o755)).To(Succeed())
	env = []string{"PATH=" + binDir, "HOME=" + os.Getenv("HOME")}
	return binDir, env
}

func assertUntrackedGoDisclosure(mode, format string) {
	repo := newTempGitRepo()
	commitBenignHistory(repo)
	writeUntrackedFile(repo, "pending.go", hiddenInputMutationGo)
	expectDirtyPorcelain(repo, "?? pending.go")

	stdout, stderr, exitCode := runWorkingTreeCodesignal(repo, mode, format)
	ExpectWithOffset(1, exitCode).To(Equal(0), "stderr: %s\nstdout: %s", stderr, stdout)
	ExpectWithOffset(1, stderr).To(BeEmpty())
	expectSupportedWorktreeDisclosure(stdout, format, []string{"pending.go"}, "untracked", "staged", "modified")
}

func currentBranch(repo string) string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = repo
	output, err := cmd.Output()
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "git rev-parse --abbrev-ref HEAD: %s", output)
	return strings.TrimSpace(string(output))
}

func commitWorktreePath(repo, name, message string) {
	add := exec.Command("git", "add", "--", name)
	add.Dir = repo
	output, err := add.CombinedOutput()
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "git add %s: %s", name, output)

	commit := exec.Command("git", "commit", "-m", message)
	commit.Dir = repo
	commit.Env = commitEnv
	output, err = commit.CombinedOutput()
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "git commit %s: %s", name, output)
}

// leaveUnmergedGoFile starts a conflicting merge on name. Pre-commit name for
// UU (both modified); leave it absent for AA (both added).
func leaveUnmergedGoFile(repo, name, ours, theirs string) {
	start := currentBranch(repo)

	checkoutTheirs := exec.Command("git", "checkout", "-b", "review-unmerged-theirs")
	checkoutTheirs.Dir = repo
	output, err := checkoutTheirs.CombinedOutput()
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "git checkout -b: %s", output)
	ExpectWithOffset(1, os.WriteFile(filepath.Join(repo, name), []byte(theirs), 0o644)).To(Succeed())
	commitWorktreePath(repo, name, "theirs "+name)

	checkoutOurs := exec.Command("git", "checkout", start)
	checkoutOurs.Dir = repo
	output, err = checkoutOurs.CombinedOutput()
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "git checkout %s: %s", start, output)
	ExpectWithOffset(1, os.WriteFile(filepath.Join(repo, name), []byte(ours), 0o644)).To(Succeed())
	commitWorktreePath(repo, name, "ours "+name)

	merge := exec.Command("git", "merge", "--no-ff", "review-unmerged-theirs")
	merge.Dir = repo
	merge.Env = commitEnv
	_ = merge.Run()
}

func commitGoProjectForDisclosure(repo string) {
	commitFile(repo, "go.mod", goModuleFile)
	commitFile(repo, "pkg/db/db.go", dbPackageFile)
	commitFile(repo, "pkg/handlers/handlers.go", handlersWithoutImport)
	commitFile(repo, "project.json", goLayerPolicyConfigJSON)
	commitFile(repo, "a.go", benignGoA)
	commitFile(repo, "a.go", benignGoB)
}
