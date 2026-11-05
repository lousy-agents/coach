package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

var _ = Describe("coach codesignal --base --worktree treats a file deleted on disk as a deleted change", func() {
	densityLifecycles := func(report *codesignal.Report, path string) []codesignal.Lifecycle {
		lifecycles := []codesignal.Lifecycle{}
		for _, signal := range report.Signals {
			if signal.RuleID == "structure.constructor_density" && signal.Path == path {
				lifecycles = append(lifecycles, signal.Lifecycle)
			}
		}
		return lifecycles
	}
	scan := func(repo, base string, extraArgs ...string) *codesignal.Report {
		stdout, stderr, exitCode := runCoachCodesignalRaw(repo, base, append([]string{"--format=json"}, extraArgs...)...)
		ExpectWithOffset(1, exitCode).To(Equal(0), "stderr: %s", stderr)
		return decodeCoachReport(stdout)
	}
	remove := func(repo, name string) {
		ExpectWithOffset(1, os.Remove(filepath.Join(repo, name))).To(Succeed())
	}

	When("a file edited in the committed diff is then deleted on disk", func() {
		diffRepo := func() (repo, base string) {
			repo = newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			base = commitFile(repo, "pkg/model/m.go", modelFileWithTwoConstructors)
			commitFile(repo, "pkg/model/m.go", modelFileWithTwoConstructors+"\n// committed edit\n")
			return repo, base
		}

		It("keeps the signals as existing in the committed diff scan (control)", func() {
			repo, base := diffRepo()

			Expect(densityLifecycles(scan(repo, base), "pkg/model/m.go")).NotTo(BeEmpty())
			Expect(densityLifecycles(scan(repo, base), "pkg/model/m.go")).To(HaveEach(codesignal.Lifecycle("existing")))
		})

		It("reports the base-side signals as resolved, because nothing is left in the working tree", func() {
			repo, base := diffRepo()
			remove(repo, "pkg/model/m.go")

			lifecycles := densityLifecycles(scan(repo, base, "--worktree"), "pkg/model/m.go")

			Expect(lifecycles).NotTo(BeEmpty())
			Expect(lifecycles).To(HaveEach(codesignal.Lifecycle("resolved")))
		})
	})

	When("a file the committed diff does not touch is deleted on disk", func() {
		diffRepo := func() (repo, base string) {
			repo = newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/model/m.go", modelFileWithTwoConstructors)
			base = commitFile(repo, "pkg/model/seed.go", "package model\n")
			commitFile(repo, "pkg/model/seed.go", "package model\n\n// committed edit\n")
			return repo, base
		}

		It("reports no signal for it without --worktree (control)", func() {
			repo, base := diffRepo()
			remove(repo, "pkg/model/m.go")

			Expect(densityLifecycles(scan(repo, base), "pkg/model/m.go")).To(BeEmpty())
		})

		It("reports its signals as resolved with --worktree", func() {
			repo, base := diffRepo()
			remove(repo, "pkg/model/m.go")

			lifecycles := densityLifecycles(scan(repo, base, "--worktree"), "pkg/model/m.go")

			Expect(lifecycles).NotTo(BeEmpty())
			Expect(lifecycles).To(HaveEach(codesignal.Lifecycle("resolved")))
		})
	})

	When("a file added in the committed diff is deleted on disk", func() {
		It("reports nothing for it, because it was never in the base and is not in the working tree", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			base := commitFile(repo, "pkg/model/seed.go", "package model\n")
			commitFile(repo, "pkg/model/m.go", modelFileWithTwoConstructors)
			remove(repo, "pkg/model/m.go")

			report := scan(repo, base, "--worktree")

			Expect(densityLifecycles(report, "pkg/model/m.go")).To(BeEmpty())
			for _, diagnostic := range report.Diagnostics {
				Expect(diagnostic.Path).NotTo(Equal("pkg/model/m.go"))
			}
		})
	})
})
