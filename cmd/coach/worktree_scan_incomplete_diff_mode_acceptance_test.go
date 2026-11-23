package main

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// diffScanRun is a `--base <ref> --worktree` scan: the worktreeScanRun fields
// plus the raw coverage.excluded groups.
type diffScanRun struct {
	worktreeScanRun
	excluded []any
}

func runDiffWorktreeScan(repo, base string) diffScanRun {
	stdout, stderr, exitCode := runCoachCodesignalRaw(repo, base,
		"--worktree", "--format=json", "--fail-on-incomplete-coverage", "--project-config", "project.json")
	ExpectWithOffset(1, string(stderr)).To(BeEmpty(), "the scan never relays git's stderr")
	var report struct {
		Diagnostics []scanIncompleteEntry `json:"diagnostics"`
		Summary     map[string]any        `json:"summary"`
		Coverage    struct {
			Excluded []any `json:"excluded"`
		} `json:"coverage"`
	}
	ExpectWithOffset(1, json.Unmarshal(stdout, &report)).To(Succeed(), "stdout should be one JSON report: %s", stdout)
	run := diffScanRun{
		worktreeScanRun: worktreeScanRun{all: report.Diagnostics, summary: report.Summary, exitCode: exitCode},
		excluded:        report.Coverage.Excluded,
	}
	for _, diagnostic := range report.Diagnostics {
		if diagnostic["kind"] == worktreeScanIncompleteKind {
			run.entries = append(run.entries, diagnostic)
		}
	}
	return run
}

func runDiffWorktreeText(repo, base string) string {
	stdout, stderr, exitCode := runCoachCodesignalRaw(repo, base, "--worktree", "--project-config", "project.json")
	ExpectWithOffset(1, string(stderr)).To(BeEmpty())
	ExpectWithOffset(1, exitCode).To(Equal(0))
	return string(stdout)
}

// diffScanRepo commits the scanned project plus extra as the base, then
// commits a changed Go test file, which the default --scope production
// excludes, so the diff has changed paths and a non-empty coverage.excluded.
func diffScanRepo(extra map[string]string) (repo, base string) {
	repo = scanRepoWith(extra)
	base = runGitInRepo(repo, "rev-parse", "HEAD")
	commitFile(repo, "pkg/handlers/handlers_test.go", "package handlers\n")
	return repo, base
}

var _ = Describe("coach codesignal --base --worktree decides hides_in_scope against the scope filter (#168 item 22)", func() {
	DescribeTable("a hidden README is disclosed without qualifying the verdict (#168 item 22)",
		func(m hiding) {
			repo, base := diffScanRepo(nil)
			m.hide(repo, "README.md")

			run := runDiffWorktreeScan(repo, base)
			text := runDiffWorktreeText(repo, base)

			entry := run.only(m.reason)
			Expect(entry).To(HaveKeyWithValue("root", "."))
			Expect(entry).To(HaveKeyWithValue("hides_in_scope", "no"))
			Expect(run.exitCode).To(Equal(0))
			Expect(noFindingsHeadline(text)).To(And(HavePrefix("No active CodeSignal findings"), Not(ContainSubstring("incomplete"))))
			Expect(text).To(MatchRegexp(hiddenLine(m.label)))
		},
		Entry("sparse checkout", sparseHiding),
		Entry("skip-worktree", skipWorktreeHiding),
		Entry("assume-unchanged", assumeUnchangedHiding),
	)

	DescribeTable("a hidden production Go file qualifies the verdict and exits 3 (#168 item 22)",
		func(m hiding) {
			repo, base := diffScanRepo(map[string]string{"lib/in_scope.go": "package lib\n"})
			m.hide(repo, "lib/in_scope.go")

			run := runDiffWorktreeScan(repo, base)

			entry := run.only(m.reason)
			Expect(entry).To(HaveKeyWithValue("root", "lib"))
			Expect(entry).To(HaveKeyWithValue("hides_in_scope", "yes"))
			Expect(run.exitCode).To(Equal(3))
		},
		Entry("sparse checkout", sparseHiding),
		Entry("skip-worktree", skipWorktreeHiding),
		Entry("assume-unchanged", assumeUnchangedHiding),
	)

	DescribeTable("hiding paths leaves coverage.excluded as the same scan without hiding reports it (#168 item 22)",
		func(m hiding) {
			extra := map[string]string{"lib/in_scope.go": "package lib\n"}
			control, controlBase := diffScanRepo(extra)
			hidden, hiddenBase := diffScanRepo(extra)
			m.hide(hidden, "lib/in_scope.go")

			unhidden := runDiffWorktreeScan(control, controlBase)
			run := runDiffWorktreeScan(hidden, hiddenBase)

			Expect(unhidden.ofReason(m.reason)).To(BeEmpty(), "the control hides nothing")
			Expect(unhidden.excluded).NotTo(BeEmpty(), "the changed test file must make the excluded group non-empty")
			Expect(run.only(m.reason)).To(HaveKeyWithValue("hides_in_scope", "yes"))
			Expect(run.excluded).To(Equal(unhidden.excluded))
		},
		Entry("sparse checkout", sparseHiding),
		Entry("skip-worktree", skipWorktreeHiding),
		Entry("assume-unchanged", assumeUnchangedHiding),
	)
})
