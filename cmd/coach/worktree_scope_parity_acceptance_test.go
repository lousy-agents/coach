package main

import (
	"slices"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

type parityRepo struct {
	dir  string
	base string
}

type parityWriter func(repo, name, contents string)

type parityScan struct {
	json   []byte
	report *codesignal.Report
	text   string
}

func commitChange(repo, name, contents string) { commitFile(repo, name, contents) }

func twinRepos(seed func(repo string) string, change func(write parityWriter, repo string)) (committed, worktree parityRepo) {
	committed = parityRepo{dir: newTempGitRepo()}
	committed.base = seed(committed.dir)
	change(commitChange, committed.dir)

	worktree = parityRepo{dir: newTempGitRepo()}
	worktree.base = seed(worktree.dir)
	change(overwriteOnDisk, worktree.dir)
	return committed, worktree
}

func withoutWorktreeLine(text string) string {
	kept := []string{}
	for _, line := range strings.Split(text, "\n") {
		if !strings.HasPrefix(line, "worktree:") {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}

func scanDiffParity(repo parityRepo, flags ...string) parityScan {
	jsonOut, stderr, exitCode := runCoachCodesignalRaw(repo.dir, repo.base, slices.Concat(flags, []string{"--format=json"})...)
	ExpectWithOffset(1, exitCode).To(Equal(0), "stderr: %s", stderr)
	textOut, stderr, exitCode := runCoachCodesignalRaw(repo.dir, repo.base, flags...)
	ExpectWithOffset(1, exitCode).To(Equal(0), "stderr: %s", stderr)
	return parityScan{json: jsonOut, report: decodeCoachReport(jsonOut), text: withoutWorktreeLine(string(textOut))}
}

func scanBaselineParity(repo parityRepo, flags ...string) parityScan {
	jsonOut, stderr, exitCode := runCoachCodesignalBaselineRaw(repo.dir, slices.Concat(flags, []string{"--format=json"})...)
	ExpectWithOffset(1, exitCode).To(Equal(0), "stderr: %s", stderr)
	textOut, stderr, exitCode := runCoachCodesignalBaselineRaw(repo.dir, flags...)
	ExpectWithOffset(1, exitCode).To(Equal(0), "stderr: %s", stderr)
	return parityScan{json: jsonOut, report: decodeCoachReport(jsonOut), text: withoutWorktreeLine(string(textOut))}
}

func scanTwinsAsDiff(committed, worktree parityRepo, flags ...string) (parityScan, parityScan) {
	return scanDiffParity(committed, flags...), scanDiffParity(worktree, slices.Concat(flags, []string{"--worktree"})...)
}

func paritySignalPaths(report *codesignal.Report) []string {
	paths := []string{}
	for _, signal := range report.Signals {
		paths = append(paths, signal.Path)
	}
	return paths
}

func parityDiagnosticPaths(report *codesignal.Report, kind string) []string {
	paths := []string{}
	for _, diagnostic := range diagnosticsOfKind(report.Diagnostics, kind) {
		paths = append(paths, diagnostic.Path)
	}
	return paths
}

func expectSameReportOutcome(committed, worktree parityScan) {
	ExpectWithOffset(1, worktree.report.Scope.AppliedScope).To(Equal(committed.report.Scope.AppliedScope))
	ExpectWithOffset(1, worktree.report.Signals).To(Equal(committed.report.Signals))
	ExpectWithOffset(1, worktree.report.Summary).To(Equal(committed.report.Summary))
	ExpectWithOffset(1, worktree.report.Diagnostics).To(Equal(committed.report.Diagnostics))
	ExpectWithOffset(1, worktree.report.Coverage).To(Equal(committed.report.Coverage))
	ExpectWithOffset(1, worktree.text).To(Equal(committed.text))
}

const (
	shippingModule     = "module example.com/scopefixture\n\ngo 1.25.0\n"
	shippingMain       = "package main\n\nimport _ \"example.com/scopefixture/shipping\"\n\nfunc main() {}\n"
	shippingBefore     = "package shipping\n\nfunc Update(input *int) {}\n"
	shippingAfter      = "package shipping\n\nfunc Update(input *int) {\n\t*input = 1\n}\n"
	shippingTestBefore = "package shipping\n\nfunc TestUpdate(input *int) {}\n"
	shippingTestAfter  = "package shipping\n\nfunc TestUpdate(input *int) {\n\t*input = 1\n}\n"
)

func seedShippingRepo(repo string) string {
	commitFile(repo, "go.mod", shippingModule)
	commitFile(repo, "cmd/app/main.go", shippingMain)
	commitFile(repo, "shipping/shipping.go", shippingBefore)
	return commitFile(repo, "shipping/shipping_test.go", shippingTestBefore)
}

func changeShippingFiles(write parityWriter, repo string) {
	write(repo, "shipping/shipping.go", shippingAfter)
	write(repo, "shipping/shipping_test.go", shippingTestAfter)
}

func seedUnsupportedRepo(repo string) string {
	commitFile(repo, "go.mod", goModuleFile)
	return commitFile(repo, "README.md", "first\n")
}

var _ = Describe("coach codesignal --base --worktree behaves as the committed scan does for scope and unsupported files (#168 item 23)", func() {
	When("--scope production is given for changed production and test files", func() {
		It("excludes the test file and keeps the production signal, committed and in the working tree alike", func() {
			committedRepo, worktreeRepo := twinRepos(seedShippingRepo, changeShippingFiles)

			committed, worktree := scanTwinsAsDiff(committedRepo, worktreeRepo, "--scope", "production", "--build-target", "./cmd/app")

			for _, scan := range []parityScan{committed, worktree} {
				Expect(scan.report.Scope.AppliedScope).To(Equal("production"))
				Expect(paritySignalPaths(scan.report)).To(Equal([]string{"shipping/shipping.go"}))
				Expect(sourceScopeForPath(scan.json, "shipping/shipping.go")).To(Equal("production"))
				Expect(scan.report.Summary.ActiveSignals).To(Equal(1))
				Expect(scan.report.Coverage.Excluded).To(Equal([]codesignal.CoverageGroup{{Reason: "test_only", Language: "go", Count: 1}}))
				Expect(scan.text).To(ContainSubstring("scope: production, filtered: 1, files analyzed: 1, active signals: 1"))
				Expect(scan.text).To(ContainSubstring("excluded: 1 test_only go files"))
				Expect(scan.text).NotTo(ContainSubstring("shipping_test.go"))
			}
			expectSameReportOutcome(committed, worktree)
		})
	})

	When("--scope all is given for changed production and test files", func() {
		It("includes the test-only signal alongside the production signal, committed and in the working tree alike", func() {
			committedRepo, worktreeRepo := twinRepos(seedShippingRepo, changeShippingFiles)

			committed, worktree := scanTwinsAsDiff(committedRepo, worktreeRepo, "--scope", "all", "--build-target", "./cmd/app")

			for _, scan := range []parityScan{committed, worktree} {
				Expect(scan.report.Scope.AppliedScope).To(Equal("all"))
				Expect(paritySignalPaths(scan.report)).To(ConsistOf("shipping/shipping.go", "shipping/shipping_test.go"))
				Expect(sourceScopeForPath(scan.json, "shipping/shipping_test.go")).To(Equal("test_only"))
				Expect(scan.report.Summary.ActiveSignals).To(Equal(2))
				Expect(scan.report.Coverage.Excluded).To(BeEmpty())
				Expect(scan.text).To(ContainSubstring("scope: all (no scope filtering applied), files analyzed: 2, active signals: 2"))
				Expect(scan.text).To(ContainSubstring("path: shipping/shipping_test.go"))
			}
			expectSameReportOutcome(committed, worktree)
		})

		It("differs from the production run on the same changes, so neither row can pass by accident", func() {
			committedRepo, worktreeRepo := twinRepos(seedShippingRepo, changeShippingFiles)

			productionCommitted, productionWorktree := scanTwinsAsDiff(committedRepo, worktreeRepo, "--scope", "production", "--build-target", "./cmd/app")
			allCommitted, allWorktree := scanTwinsAsDiff(committedRepo, worktreeRepo, "--scope", "all", "--build-target", "./cmd/app")

			for _, production := range []parityScan{productionCommitted, productionWorktree} {
				Expect(signalsForPath(production.report, "shipping/shipping_test.go")).To(BeEmpty())
			}
			for _, all := range []parityScan{allCommitted, allWorktree} {
				Expect(signalsForPath(all.report, "shipping/shipping_test.go")).To(HaveLen(1))
			}
		})
	})

	DescribeTable("a changed file in a language coach does not analyze is named once and the analysis is reported as incomplete",
		func(path string, change func(write parityWriter, repo string), expectedWorktree codesignal.WorktreeScope) {
			committedRepo, worktreeRepo := twinRepos(seedUnsupportedRepo, change)

			committed, worktree := scanTwinsAsDiff(committedRepo, worktreeRepo)

			for _, scan := range []parityScan{committed, worktree} {
				Expect(parityDiagnosticPaths(scan.report, "unsupported_language")).To(Equal([]string{path}))
				Expect(scan.report.Summary.FilesUnanalyzed).To(Equal(1))
				Expect(scan.report.Summary.ActiveSignals).To(BeZero())
				Expect(scan.text).To(ContainSubstring("No active CodeSignal findings, but the analysis is incomplete: 1 path was not analyzed."))
				Expect(scan.text).To(ContainSubstring("path: " + path + ", kind: unsupported_language"))
			}
			expectSameReportOutcome(committed, worktree)

			Expect(committed.report.Scope.Worktree).To(BeNil(), "the committed run is not a --worktree scan")
			Expect(worktree.report.Scope.Worktree).NotTo(BeNil(), "the second run must be the --worktree scan")
			expectedWorktree.Revision = worktreeRepo.base
			Expect(worktree.report.Scope.Revision).To(Equal(worktreeRepo.base))
			Expect(*worktree.report.Scope.Worktree).To(Equal(expectedWorktree), "the working-tree run must have reached the change through the intended git-status category")
		},
		Entry("a tracked file edited after the base",
			"README.md",
			func(write parityWriter, repo string) { write(repo, "README.md", "second\n") },
			codesignal.WorktreeScope{Modified: 1, ConfigFiles: []string{}}),
		Entry("a new file that is untracked in the working tree",
			"fresh.txt",
			func(write parityWriter, repo string) { write(repo, "fresh.txt", "new\n") },
			codesignal.WorktreeScope{Untracked: 1, ConfigFiles: []string{}}),
	)

	When("a diff scan meets unsupported files", func() {
		It("reports coverage.unsupported as an empty list in both modes and carries the unsupported files in diagnostics", func() {
			committedRepo, worktreeRepo := twinRepos(seedUnsupportedRepo, func(write parityWriter, repo string) {
				write(repo, "README.md", "second\n")
				write(repo, "fresh.txt", "new\n")
			})

			committed, worktree := scanTwinsAsDiff(committedRepo, worktreeRepo)

			for _, scan := range []parityScan{committed, worktree} {
				Expect(scan.json).To(ContainSubstring(`"unsupported":[]`))
				Expect(scan.report.Coverage.Unsupported).To(BeEmpty())
				Expect(parityDiagnosticPaths(scan.report, "unsupported_language")).To(ConsistOf("README.md", "fresh.txt"))
				Expect(scan.report.Summary.FilesUnanalyzed).To(Equal(2))
			}
			expectSameReportOutcome(committed, worktree)
		})
	})

	When("a baseline scan meets unsupported files", func() {
		It("groups coverage.unsupported by reason, language and count the same with and without --worktree", func() {
			seedBaseline := func(repo string) string {
				commitFile(repo, "go.mod", goModuleFile)
				commitFile(repo, "a.go", "package a\n\nfunc A() {}\n")
				return commitFile(repo, "README.md", "first\n")
			}
			changeBaseline := func(write parityWriter, repo string) {
				write(repo, "README.md", "edited\n")
				write(repo, "extra.txt", "extra\n")
			}
			committedRepo, worktreeRepo := twinRepos(seedBaseline, changeBaseline)

			committed := scanBaselineParity(committedRepo)
			worktree := scanBaselineParity(worktreeRepo, "--worktree")

			expectedUnsupported := []codesignal.CoverageGroup{
				{Reason: "unsupported_language", Language: ".md", Count: 1},
				{Reason: "unsupported_language", Language: ".mod", Count: 1},
				{Reason: "unsupported_language", Language: ".txt", Count: 1},
			}
			for _, scan := range []parityScan{committed, worktree} {
				Expect(scan.report.Coverage.Unsupported).To(Equal(expectedUnsupported))
				Expect(scan.text).To(ContainSubstring("  unsupported: 1 .md files\n  unsupported: 1 .mod files\n  unsupported: 1 .txt files"))
			}
			Expect(worktree.report.Coverage.Unsupported).To(Equal(committed.report.Coverage.Unsupported))
			Expect(unsupportedCoverageLines(worktree.text)).To(Equal(unsupportedCoverageLines(committed.text)))
			Expect(worktree.report.Scope.Worktree).NotTo(BeNil(), "the second run must be the --worktree scan")
			Expect(worktree.report.Scope.Worktree.Modified).To(Equal(1))
			Expect(worktree.report.Scope.Worktree.Untracked).To(Equal(1))
		})
	})
})

func unsupportedCoverageLines(text string) []string {
	lines := []string{}
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "  unsupported:") {
			lines = append(lines, line)
		}
	}
	return lines
}
