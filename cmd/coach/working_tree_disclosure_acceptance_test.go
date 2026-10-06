package main

import (
	"path/filepath"

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
				repo := newTempGitRepo()
				commitBenignHistory(repo)
				names := bulkUntrackedNames(20)
				for _, name := range names {
					writeUntrackedFile(repo, name, hiddenInputMutationGo)
				}
				porcelain := expectDirtyPorcelain(repo, "?? bulk00.go")
				for _, name := range names {
					Expect(porcelain).To(ContainSubstring("?? " + name))
				}

				stdout, stderr, exitCode := runWorkingTreeCodesignal(repo, mode, format)
				Expect(exitCode).To(Equal(0), "stderr: %s\nstdout: %s", stderr, stdout)
				Expect(stderr).To(BeEmpty())
				expectBoundedUntrackedDisclosure(stdout, format, names)
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
				repo := newTempGitRepo()
				commitBenignHistory(repo)
				Expect(strings.TrimSpace(gitPorcelain(repo))).To(BeEmpty())

				binDir, env := installGitStatusFailureWrapper()
				expectStatusWrapperBehavior(filepath.Join(binDir, "git"), repo)

				args := []string{"codesignal", "--format=" + format}
				if mode == codesignalModeBaseline {
					args = append(args, "--baseline")
				} else {
					args = append(args, "--base", "HEAD~1")
				}
				stdout, stderr, exitCode := runCoachBinary(commandPath, repo, env, args...)
				Expect(exitCode).To(Equal(0),
					"a failed working-tree status check must not fail the run; stderr: %s\nstdout: %s", stderr, stdout)
				Expect(stdout).NotTo(BeEmpty())
				expectStatusCheckFailureDiagnostic(stdout, format)
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
				repo := newTempGitRepo()
				commitGoProjectForDisclosure(repo)
				Expect(strings.TrimSpace(gitPorcelain(repo))).To(BeEmpty())

				binDir, env := installGitStatusFailureWrapper()
				expectStatusWrapperBehavior(filepath.Join(binDir, "git"), repo)

				args := []string{"codesignal", "--project-config", "project.json", "--format=" + format}
				if mode == codesignalModeBaseline {
					args = append(args, "--baseline")
				} else {
					args = append(args, "--base", "HEAD~1")
				}
				stdout, stderr, exitCode := runCoachBinary(commandPath, repo, env, args...)
				Expect(exitCode).To(Equal(0),
					"a failed working-tree status check must not fail the run; stderr: %s\nstdout: %s", stderr, stdout)
				Expect(stdout).NotTo(BeEmpty())
				expectStatusCheckFailureDiagnostic(stdout, format)
				Expect(strings.ToLower(string(stdout))).NotTo(ContainSubstring("not analyzed"),
					"a status-check failure must not be described as skipped analysis; stdout:\n%s", stdout)
				Expect(countKind(diagnosticKinds(stdout, format), codesignal.DiagKindWorktreeChangesNotAnalyzed)).To(Equal(0),
					"status failure must not emit the skip kind; kinds=%v stdout:\n%s", diagnosticKinds(stdout, format), stdout)
			},
			Entry("baseline text", codesignalModeBaseline, "text"),
			Entry("baseline json", codesignalModeBaseline, "json"),
			Entry("--base HEAD~1 text", codesignalModeBase, "text"),
			Entry("--base HEAD~1 json", codesignalModeBase, "json"),
		)
	})
})
