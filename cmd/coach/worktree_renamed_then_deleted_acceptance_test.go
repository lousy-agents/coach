package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

// A file the committed range renamed and the working tree then deleted leaves
// nothing at either name, so its findings are resolved at the name the merge base
// holds, as they are once the deletion is committed.
var _ = Describe("coach codesignal --base --worktree for a committed rename whose new path the working tree deleted", func() {
	const source = "pkg/model/m.go"
	const destination = "pkg/model/r.go"

	renamedThenDeleted := func() (repo, base string) {
		repo = newTempGitRepo()
		commitFile(repo, "go.mod", goModuleFile)
		commitFile(repo, source, modelFileWithTwoConstructors)
		base = commitFile(repo, "pkg/model/seed.go", "package model\n")
		runGitInRepo(repo, "mv", source, destination)
		runGitInRepo(repo, "commit", "-q", "-m", "rename "+source)
		Expect(os.Remove(filepath.Join(repo, destination))).To(Succeed())
		return repo, base
	}
	atSource := func(report *codesignal.Report) (lifecycles []codesignal.Lifecycle) {
		for _, signal := range report.Signals {
			if signal.Path == source {
				lifecycles = append(lifecycles, signal.Lifecycle)
			}
		}
		return lifecycles
	}

	It("resolves the findings at the old path and names no undetermined continuity", func() {
		repo, base := renamedThenDeleted()

		stdout, stderr, exitCode := runCoachCodesignalRaw(repo, base, "--worktree", "--format=json")

		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		report := decodeCoachReport(stdout)
		Expect(atSource(report)).To(HaveEach(codesignal.Lifecycle("resolved")))
		Expect(atSource(report)).NotTo(BeEmpty(), "the merge base's findings at the old path are resolved by the deletion")
		Expect(diagnosticsOfKind(report.Diagnostics, "continuity_not_determined")).To(BeEmpty(),
			"nothing is left at the new path to carry a finding over to")
		Expect(report.Summary.FilesUnanalyzed).To(Equal(0))
	})

	It("reports the same once the deletion is committed (control)", func() {
		repo, base := renamedThenDeleted()
		runGitInRepo(repo, "add", "-A")
		runGitInRepo(repo, "commit", "-q", "-m", "delete the renamed file")

		stdout, stderr, exitCode := runCoachCodesignalRaw(repo, base, "--format=json")

		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		report := decodeCoachReport(stdout)
		Expect(atSource(report)).NotTo(BeEmpty())
		Expect(atSource(report)).To(HaveEach(codesignal.Lifecycle("resolved")))
		Expect(diagnosticsOfKind(report.Diagnostics, "continuity_not_determined")).To(BeEmpty())
	})
})
