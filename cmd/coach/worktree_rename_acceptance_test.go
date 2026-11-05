package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

var _ = Describe("coach codesignal --base --worktree treats a staged rename like a committed one", func() {
	renamed := func(report *codesignal.Report) (lifecycles []codesignal.Lifecycle, continuity []codesignal.Diagnostic) {
		for _, signal := range report.Signals {
			if signal.RuleID == "structure.constructor_density" && signal.Path == "pkg/model/n.go" {
				lifecycles = append(lifecycles, signal.Lifecycle)
			}
		}
		return lifecycles, diagnosticsOfKind(report.Diagnostics, "continuity_not_determined")
	}
	scan := func(repo, base string, extraArgs ...string) *codesignal.Report {
		stdout, stderr, exitCode := runCoachCodesignalRaw(repo, base, append([]string{"--format=json"}, extraArgs...)...)
		ExpectWithOffset(1, exitCode).To(Equal(0), "stderr: %s", stderr)
		return decodeCoachReport(stdout)
	}
	repoWithMovableFile := func() string {
		repo := newTempGitRepo()
		commitFile(repo, "go.mod", goModuleFile)
		commitFile(repo, "pkg/model/m.go", modelFileWithTwoConstructors)
		return repo
	}

	It("reports the renamed file's signals with undetermined continuity when the rename is committed (control)", func() {
		repo := repoWithMovableFile()
		runGitInRepo(repo, "mv", "pkg/model/m.go", "pkg/model/n.go")
		runGitInRepo(repo, "commit", "-q", "-m", "rename m.go to n.go")

		lifecycles, continuity := renamed(scan(repo, "HEAD~1"))

		Expect(lifecycles).NotTo(BeEmpty())
		Expect(lifecycles).To(HaveEach(codesignal.Lifecycle("unknown")))
		Expect(continuity).To(ContainElement(HaveField("Path", "pkg/model/n.go")))
	})

	It("gives a staged rename the same undetermined continuity instead of reporting its signals as introduced", func() {
		repo := repoWithMovableFile()
		runGitInRepo(repo, "mv", "pkg/model/m.go", "pkg/model/n.go")

		lifecycles, continuity := renamed(scan(repo, "HEAD", "--worktree"))

		Expect(lifecycles).NotTo(BeEmpty())
		Expect(lifecycles).To(HaveEach(codesignal.Lifecycle("unknown")),
			"a moved file's signals are not new defects; the committed rename already says so")
		Expect(continuity).To(ContainElement(HaveField("Path", "pkg/model/n.go")))
	})

	It("keeps the committed rename's wording, which does hold the rename at the head revision (control)", func() {
		repo := repoWithMovableFile()
		runGitInRepo(repo, "mv", "pkg/model/m.go", "pkg/model/n.go")
		runGitInRepo(repo, "commit", "-q", "-m", "rename m.go to n.go")

		_, continuity := renamed(scan(repo, "HEAD~1"))

		Expect(continuity).To(ConsistOf(HaveField("Message", ContainSubstring("not determined at head revision "+headRevision(repo)))))
	})

	It("says a staged rename is uncommitted, because the head revision does not hold it", func() {
		repo := repoWithMovableFile()
		runGitInRepo(repo, "mv", "pkg/model/m.go", "pkg/model/n.go")

		_, continuity := renamed(scan(repo, "HEAD", "--worktree"))

		Expect(continuity).To(ConsistOf(And(
			HaveField("Path", "pkg/model/n.go"),
			HaveField("Message", And(ContainSubstring("uncommitted"), ContainSubstring("working tree"), Not(ContainSubstring("at head revision")))),
			HaveField("Side", "head"),
			HaveField("Revision", headRevision(repo)),
		)))
	})
})
