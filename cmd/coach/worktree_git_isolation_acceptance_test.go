package main

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --worktree does not run commands a repository configures for git", func() {
	const oneConstructor = "package model\n\ntype A struct{}\n\nfunc NewA() *A {\n\treturn &A{}\n}\n"

	// instrumentedRepo configures an fsmonitor hook and a diff textconv driver
	// that each leave a marker file when git runs them.
	instrumentedRepo := func() (repo, base, fsmonitorMarker, textconvMarker string) {
		scripts := GinkgoT().TempDir()
		fsmonitorMarker = filepath.Join(scripts, "fsmonitor-ran")
		textconvMarker = filepath.Join(scripts, "textconv-ran")
		fsmonitor := filepath.Join(scripts, "fsmonitor.sh")
		textconv := filepath.Join(scripts, "textconv.sh")
		Expect(os.WriteFile(fsmonitor, []byte(fmt.Sprintf("#!/bin/sh\ntouch %q\nexit 1\n", fsmonitorMarker)), 0o755)).To(Succeed())
		Expect(os.WriteFile(textconv, []byte(fmt.Sprintf("#!/bin/sh\ntouch %q\ncat \"$1\"\n", textconvMarker)), 0o755)).To(Succeed())

		repo = newTempGitRepo()
		commitFile(repo, "go.mod", goModuleFile)
		commitFile(repo, ".gitattributes", "*.go diff=instrumented\n")
		base = commitFile(repo, "pkg/other/other.go", oneConstructor)
		commitFile(repo, "pkg/other/seed.go", "package model\n")
		runGitInRepo(repo, "config", "core.fsmonitor", fsmonitor)
		runGitInRepo(repo, "config", "diff.instrumented.textconv", textconv)
		overwriteOnDisk(repo, "pkg/other/other.go", modelFileWithTwoConstructors)
		return repo, base, fsmonitorMarker, textconvMarker
	}
	exists := func(path string) bool {
		_, err := os.Stat(path)
		return err == nil
	}

	It("shows the fixture is live: plain git status and git diff run both commands (control)", func() {
		repo, base, fsmonitorMarker, textconvMarker := instrumentedRepo()

		runGitInRepo(repo, "status", "--porcelain")
		runGitInRepo(repo, "diff", "--unified=0", base, "--", "pkg/other/other.go")

		Expect(exists(fsmonitorMarker)).To(BeTrue(), "the fsmonitor hook must run for a plain git status")
		Expect(exists(textconvMarker)).To(BeTrue(), "the textconv driver must run for a plain git diff of a changed file")
	})

	scanInstrumented := func() (fsmonitorMarker, textconvMarker string) {
		repo, base, fsmonitorMarker, textconvMarker := instrumentedRepo()

		stdout, stderr, exitCode := runCoachCodesignalRaw(repo, base, "--worktree", "--format=json")

		ExpectWithOffset(1, exitCode).To(Equal(0), "stderr: %s", stderr)
		ExpectWithOffset(1, decodeCoachReport(stdout).Scope.Worktree).NotTo(BeNil())
		return fsmonitorMarker, textconvMarker
	}

	It("does not run the configured fsmonitor hook", func() {
		fsmonitorMarker, _ := scanInstrumented()

		Expect(exists(fsmonitorMarker)).To(BeFalse(), "a configured fsmonitor command ran during the scan")
	})

	It("does not run the configured textconv driver", func() {
		_, textconvMarker := scanInstrumented()

		Expect(exists(textconvMarker)).To(BeFalse(), "a configured textconv command ran during the scan")
	})
	When("a clean filter is configured for a file edited on disk", func() {
		const threeConstructors = modelFileWithTwoConstructors + "\ntype C struct{}\n\nfunc NewC() *C {\n\treturn &C{}\n}\n"

		// filteredRepo commits m.go with two constructors, configures a clean
		// filter for it, and leaves a third constructor on disk.
		filteredRepo := func(cleanCommand func(scripts string) string) (repo, base, scripts string) {
			scripts = GinkgoT().TempDir()
			repo = newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			base = commitFile(repo, "pkg/model/m.go", modelFileWithTwoConstructors)
			commitFile(repo, "pkg/model/seed.go", "package model\n")
			Expect(os.WriteFile(filepath.Join(repo, ".git/info/attributes"), []byte("pkg/model/m.go filter=probe\n"), 0o644)).To(Succeed())
			runGitInRepo(repo, "config", "filter.probe.clean", cleanCommand(scripts))
			overwriteOnDisk(repo, "pkg/model/m.go", threeConstructors)
			return repo, base, scripts
		}
		changedBySubject := func(stdout []byte) map[string]bool {
			flags := map[string]bool{}
			for _, signal := range decodeCoachReport(stdout).Signals {
				if signal.RuleID == "structure.constructor_density" {
					flags[signal.Subject] = signal.Changed
				}
			}
			return flags
		}
		recordingFilter := func(scripts string) string {
			return fmt.Sprintf("sh -c 'echo ran >> %s; cat'", filepath.Join(scripts, "filter-ran"))
		}
		hidingFilter := func(scripts string) string {
			original := filepath.Join(scripts, "committed.go")
			Expect(os.WriteFile(original, []byte(modelFileWithTwoConstructors), 0o644)).To(Succeed())
			return "cat " + original
		}

		It("shows the fixture is live: a plain git diff of the edited file runs the filter (control)", func() {
			repo, base, scripts := filteredRepo(recordingFilter)

			runGitInRepo(repo, "diff", "--unified=0", base, "--", "pkg/model/m.go")

			Expect(exists(filepath.Join(scripts, "filter-ran"))).To(BeTrue())
		})

		It("does not run the filter, because the changed ranges come from bytes already read", func() {
			repo, base, scripts := filteredRepo(recordingFilter)

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, base, "--worktree", "--format=json")

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(exists(filepath.Join(scripts, "filter-ran"))).To(BeFalse(), "a repository clean filter ran during the scan")
			Expect(changedBySubject(stdout)).To(Equal(map[string]bool{"NewA": false, "NewB": false, "NewC": true}))
		})

		It("marks the lines edited on disk as changed even when a filter would hide the edit from git", func() {
			repo, base, _ := filteredRepo(hidingFilter)

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, base, "--worktree", "--format=json")

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(changedBySubject(stdout)).To(Equal(map[string]bool{"NewA": false, "NewB": false, "NewC": true}),
				"git diff against the working tree sees the filtered bytes, not the bytes the scan analyzed")
		})
	})
	When("git is configured to color its diff output", func() {
		It("still marks the lines a committed diff scan added as changed", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			base := commitFile(repo, "pkg/model/m.go", modelFileWithTwoConstructors)
			commitFile(repo, "pkg/model/m.go", modelFileWithTwoConstructors+"\ntype C struct{}\n\nfunc NewC() *C {\n\treturn &C{}\n}\n")
			runGitInRepo(repo, "config", "color.diff", "always")

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, base, "--format=json")

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			changed := map[string]bool{}
			for _, signal := range decodeCoachReport(stdout).Signals {
				if signal.RuleID == "structure.constructor_density" {
					changed[signal.Subject] = signal.Changed
				}
			}
			Expect(changed).To(Equal(map[string]bool{"NewA": false, "NewB": false, "NewC": true}),
				"color escape codes in front of the hunk headers hide every changed range")
		})
	})
})
