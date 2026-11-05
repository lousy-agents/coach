package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

// These specs run without a project policy, so only the file-local path and the
// no-policy early return of applyProjectBackend are exercised.
var _ = Describe("coach codesignal --worktree without a project policy", func() {
	signalsFor := func(report *codesignal.Report, rule string) []codesignal.Signal {
		var matched []codesignal.Signal
		for _, signal := range report.Signals {
			if signal.RuleID == rule {
				matched = append(matched, signal)
			}
		}
		return matched
	}

	When("a tracked file is replaced by a symbolic link", func() {
		It("qualifies the verdict and names the link, instead of printing a clean result", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "m.go", "package m\n")
			target := filepath.Join(GinkgoT().TempDir(), "target.go")
			Expect(os.WriteFile(target, []byte("package m\n"), 0o644)).To(Succeed())
			Expect(os.Remove(filepath.Join(repo, "m.go"))).To(Succeed())
			Expect(os.Symlink(target, filepath.Join(repo, "m.go"))).To(Succeed())

			text, _, textExit := runCoachCodesignalBaselineRaw(repo, "--worktree")
			jsonOut, _, _ := runCoachCodesignalBaselineRaw(repo, "--worktree", "--format=json")

			Expect(textExit).To(Equal(0))
			Expect(string(text)).To(ContainSubstring("the analysis is incomplete: 1 path was not analyzed"))
			Expect(string(text)).NotTo(MatchRegexp(`(?m)^No active CodeSignal findings\.$`))
			skipped := diagnosticsOfKind(decodeCoachReport(jsonOut).Diagnostics, "worktree_symlink_skipped")
			Expect(skipped).To(HaveLen(1))
			Expect(skipped[0].Path).To(Equal("m.go"))
		})
	})

	When("a tracked file with findings is deleted on disk", func() {
		It("does not analyze the deleted file, and neither reports a read failure nor claims it was not analyzed", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/model/m.go", modelFileWithTwoConstructors)
			Expect(os.Remove(filepath.Join(repo, "pkg/model/m.go"))).To(Succeed())

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--worktree", "--format=json")
			report := decodeCoachReport(stdout)

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(report.Summary.FilesAnalyzed).To(Equal(0), "the deleted file is absent, so nothing is analyzed")
			Expect(signalsFor(report, "structure.constructor_density")).To(BeEmpty())
			for _, diagnostic := range report.Diagnostics {
				Expect(diagnostic.Path).NotTo(Equal("pkg/model/m.go"), "kind %s names the deleted file", diagnostic.Kind)
			}
		})
	})

	When("a baseline scan reads a tracked file edited on disk", func() {
		const oneConstructor = "package model\n\ntype A struct{}\n\nfunc NewA() *A {\n\treturn &A{}\n}\n"

		It("sees the edit with --worktree, and does not without it", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/model/model.go", oneConstructor)
			overwriteOnDisk(repo, "pkg/model/model.go", modelFileWithTwoConstructors)

			without, _, withoutExit := runCoachCodesignalBaselineRaw(repo, "--format=json")
			with, withStderr, withExit := runCoachCodesignalBaselineRaw(repo, "--format=json", "--worktree")

			Expect(withoutExit).To(Equal(0))
			Expect(signalsFor(decodeCoachReport(without), "structure.constructor_density")).To(BeEmpty(), "the committed file has one constructor")
			Expect(withExit).To(Equal(0), "stderr: %s", withStderr)
			withSignals := signalsFor(decodeCoachReport(with), "structure.constructor_density")
			Expect(withSignals).NotTo(BeEmpty(), "a signal that exists only in the edited bytes proves the head was read from disk")
			for _, signal := range withSignals {
				Expect(signal.Path).To(Equal("pkg/model/model.go"))
			}
		})
	})

	When("a diff scan reads a file edited on disk after the last commit", func() {
		const oneConstructor = "package model\n\ntype A struct{}\n\nfunc NewA() *A {\n\treturn &A{}\n}\n"

		It("takes each signal's changed flag from the working tree: the constructor added on disk is changed, the committed one is not", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			base := commitFile(repo, "pkg/model/model.go", oneConstructor)
			commitFile(repo, "pkg/model/model.go", oneConstructor+"\n// committed edit\n")
			twoConstructors := "package model\n\ntype A struct{}\n\ntype B struct{}\n\nfunc NewA() *A {\n\treturn &A{}\n}\n\nfunc NewB() *B {\n\treturn &B{}\n}\n\n// committed edit\n"
			Expect(os.WriteFile(filepath.Join(repo, "pkg/model/model.go"), []byte(twoConstructors), 0o644)).To(Succeed())

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, base, "--worktree", "--format=json")

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			changedBySubject := map[string]bool{}
			for _, signal := range signalsFor(decodeCoachReport(stdout), "structure.constructor_density") {
				changedBySubject[signal.Subject] = signal.Changed
			}
			Expect(changedBySubject).To(Equal(map[string]bool{"NewA": false, "NewB": true}))
		})
	})
})
