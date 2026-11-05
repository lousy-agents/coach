package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

// HEAD holds the source of a staged rename, so a new file written at that path
// again is a modification of the file HEAD holds, as it is when the same tree is
// committed: git then records the rename and an edit of the old path.
var _ = Describe("coach codesignal --base --worktree for a rename source written again as a new file", func() {
	const source = "pkg/model/m.go"

	splitFileRepo := func(rewritten string) (repo, base string) {
		repo = newTempGitRepo()
		commitFile(repo, "go.mod", goModuleFile)
		commitFile(repo, source, modelFileWithTwoConstructors)
		base = commitFile(repo, "pkg/model/seed.go", "package model\n")
		runGitInRepo(repo, "mv", source, "pkg/model/b.go")
		writeUntrackedFile(repo, source, rewritten)
		expectDirtyPorcelain(repo, "?? "+source)
		return repo, base
	}
	lifecyclesAtSource := func(report *codesignal.Report) []codesignal.Lifecycle {
		var found []codesignal.Lifecycle
		for _, signal := range report.Signals {
			if signal.Path == source {
				found = append(found, signal.Lifecycle)
			}
		}
		return found
	}

	It("compares the rewritten file with the one HEAD holds, so its old findings are not introduced", func() {
		repo, base := splitFileRepo(modelFileWithTwoConstructors)

		stdout, stderr, exitCode := runCoachCodesignalRaw(repo, base, "--worktree", "--format=json")

		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		Expect(lifecyclesAtSource(decodeCoachReport(stdout))).NotTo(ContainElement(codesignal.Lifecycle("introduced")),
			"every finding in this file is already in the file HEAD holds at this path")
	})

	It("reports the same lifecycles the committed tree reports (control)", func() {
		repo, base := splitFileRepo(modelFileWithTwoConstructors)
		runGitInRepo(repo, "add", "-A")
		runGitInRepo(repo, "commit", "-q", "-m", "split the file")

		stdout, stderr, exitCode := runCoachCodesignalRaw(repo, base, "--format=json")

		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		Expect(lifecyclesAtSource(decodeCoachReport(stdout))).NotTo(ContainElement(codesignal.Lifecycle("introduced")))
	})
})
