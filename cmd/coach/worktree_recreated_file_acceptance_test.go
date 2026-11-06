package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

// A file the committed range removed and the user then wrote again on disk is,
// for the working tree, a file that exists: the scan compares it with the merge
// base like any other edit.
var _ = Describe("coach codesignal --base --worktree analyzes a file the committed range removed and the working tree recreated", func() {
	const recreated = "pkg/model/m.go"
	const withThirdConstructor = modelFileWithTwoConstructors + "\ntype C struct{}\n\nfunc NewC() *C {\n\treturn &C{}\n}\n"

	removedThenRecreated := func(contents string) (repo, base string) {
		repo = newTempGitRepo()
		commitFile(repo, "go.mod", goModuleFile)
		base = commitFile(repo, recreated, modelFileWithTwoConstructors)
		removeFile(repo, recreated)
		writeUntrackedFile(repo, recreated, contents)
		expectDirtyPorcelain(repo, "?? "+recreated)
		return repo, base
	}
	scan := func(repo, base string) (*codesignal.Report, int) {
		stdout, stderr, exitCode := runCoachCodesignalRaw(repo, base, "--worktree", "--format=json", "--fail-on-incomplete-coverage")
		ExpectWithOffset(1, exitCode).To(BeElementOf(0, 3), "stderr: %s", stderr)
		return decodeCoachReport(stdout), exitCode
	}
	lifecycles := func(report *codesignal.Report) map[string]codesignal.Lifecycle {
		found := map[string]codesignal.Lifecycle{}
		for _, signal := range report.Signals {
			if signal.RuleID == "structure.constructor_density" && signal.Path == recreated {
				found[signal.Subject] = signal.Lifecycle
			}
		}
		return found
	}

	It("analyzes the recreated file against the merge base: kept constructors are existing, the new one is introduced", func() {
		repo, base := removedThenRecreated(withThirdConstructor)

		report, _ := scan(repo, base)

		Expect(lifecycles(report)).To(Equal(map[string]codesignal.Lifecycle{
			"NewA": "existing", "NewB": "existing", "NewC": "introduced",
		}))
	})

	It("does not say the recreated file was not analyzed, and exits 0", func() {
		repo, base := removedThenRecreated(withThirdConstructor)

		report, exitCode := scan(repo, base)

		Expect(exitCode).To(Equal(0))
		Expect(diagnosticsOfKind(report.Diagnostics, "worktree_changes_not_analyzed")).To(BeEmpty())
		Expect(report.Summary.FilesUnanalyzed).To(BeZero())
	})

	It("still reports the signals as resolved when the file is not recreated (control)", func() {
		repo := newTempGitRepo()
		commitFile(repo, "go.mod", goModuleFile)
		base := commitFile(repo, recreated, modelFileWithTwoConstructors)
		removeFile(repo, recreated)

		report, exitCode := scan(repo, base)

		Expect(exitCode).To(Equal(0))
		Expect(lifecycles(report)).To(Equal(map[string]codesignal.Lifecycle{"NewA": "resolved", "NewB": "resolved"}))
	})
})
