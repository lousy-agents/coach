package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

var _ = Describe("coach codesignal --worktree qualifies the verdict when any reason may hide in-scope source (#168 item 22)", func() {
	It("qualifies and exits 3 when one reason hides no source and another hides in-scope source (#168 item 22)", func() {
		repo := scanRepoWith(map[string]string{"docs/guide.md": "# guide\n", "lib/in_scope.go": "package lib\n"})
		skipWorktreeHiding.hide(repo, "docs/guide.md")
		assumeUnchangedHiding.hide(repo, "lib/in_scope.go")

		run := runProjectScan(repo)
		text := runProjectText(repo)

		Expect(run.only("skip_worktree")).To(HaveKeyWithValue("hides_in_scope", "no"))
		Expect(run.only("assume_unchanged")).To(HaveKeyWithValue("hides_in_scope", "yes"))
		Expect(run.exitCode).To(Equal(3))
		Expect(noFindingsHeadline(text)).NotTo(Equal(unqualifiedAllClear))
		Expect(text).To(MatchRegexp(hiddenLine("skip-worktree")))
		Expect(text).To(ContainSubstring(assumeUnchangedHiding.hint))
	})

	It("stays unqualified and exits 0 when every reason hides no source, disclosing both (#168 item 22)", func() {
		repo := scanRepoWith(map[string]string{"docs/guide.md": "# guide\n", "notes/todo.md": "# todo\n"})
		skipWorktreeHiding.hide(repo, "docs/guide.md")
		assumeUnchangedHiding.hide(repo, "notes/todo.md")

		run := runProjectScan(repo)
		text := runProjectText(repo)

		Expect(run.only("skip_worktree")).To(HaveKeyWithValue("hides_in_scope", "no"))
		Expect(run.only("assume_unchanged")).To(HaveKeyWithValue("hides_in_scope", "no"))
		Expect(run.exitCode).To(Equal(0))
		Expect(noFindingsHeadline(text)).To(Equal(unqualifiedAllClear))
		Expect(text).To(MatchRegexp(hiddenLine("skip-worktree")))
		Expect(text).To(MatchRegexp(hiddenLine("assume-unchanged")))
	})

	It("still qualifies for a skipped symbolic link beside a reason that hides no source (#168 item 22)", func() {
		repo := scanRepoWith(map[string]string{"docs/guide.md": "# guide\n"})
		skipWorktreeHiding.hide(repo, "docs/guide.md")
		target := filepath.Join(GinkgoT().TempDir(), "target.go")
		Expect(os.WriteFile(target, []byte(handlersWithoutImport), 0o644)).To(Succeed())
		Expect(os.Remove(filepath.Join(repo, "pkg/handlers/handlers.go"))).To(Succeed())
		Expect(os.Symlink(target, filepath.Join(repo, "pkg/handlers/handlers.go"))).To(Succeed())

		run := runProjectScan(repo)

		Expect(run.only("skip_worktree")).To(HaveKeyWithValue("hides_in_scope", "no"))
		Expect(run.ofKind("worktree_symlink_skipped")).NotTo(BeEmpty(), "the symlink diagnostic is what qualifies the verdict")
		Expect(run.exitCode).To(Equal(3))
	})

	It("does not count the new entries as unanalyzed files (#168 item 22)", func() {
		repo := scanRepoWith(map[string]string{"docs/guide.md": "# guide\n", "lib/in_scope.go": "package lib\n", "notes/todo.md": "# todo\n"})
		skipWorktreeHiding.hide(repo, "docs/guide.md", "lib/in_scope.go")
		assumeUnchangedHiding.hide(repo, "notes/todo.md")

		run := runProjectScan(repo)

		Expect(run.ofReason("skip_worktree")).To(HaveLen(2))
		Expect(run.only("assume_unchanged")).To(HaveKey("hides_in_scope"))
		Expect(run.summary).To(HaveKeyWithValue("files_unanalyzed", BeNumerically("==", 0)))
	})

	DescribeTable("reads hides_in_scope from a report's JSON: only \"no\" lets the report stay complete (#168 item 22)",
		func(diagnostic string, wantIncomplete bool) {
			var report codesignal.Report
			document := `{"scope":{"worktree":{}},"diagnostics":[` + diagnostic + `]}`
			Expect(json.Unmarshal([]byte(document), &report)).To(Succeed())

			Expect(codesignal.RequiredCoverageIncomplete(&report)).To(Equal(wantIncomplete))
		},
		Entry("no", `{"kind":"worktree_scan_incomplete","reason":"skip_worktree","hides_in_scope":"no","message":"m"}`, false),
		Entry("yes", `{"kind":"worktree_scan_incomplete","reason":"skip_worktree","hides_in_scope":"yes","message":"m"}`, true),
		Entry("unknown", `{"kind":"worktree_scan_incomplete","reason":"unreadable_directory","hides_in_scope":"unknown","message":"m"}`, true),
		Entry("a missing key reads as unknown", `{"kind":"worktree_scan_incomplete","reason":"skip_worktree","message":"m"}`, true),
	)
})
