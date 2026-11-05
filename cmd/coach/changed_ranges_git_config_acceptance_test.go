package main

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// A repository's own git configuration must not change which lines of a committed
// diff a scan treats as changed. These run the committed scan, with no --worktree.
var _ = Describe("coach codesignal --base for a repository configured to colour or convert its diffs", func() {
	const oneConstructor = "package model\n\ntype A struct{}\n\nfunc NewA() *A {\n\treturn &A{}\n}\n"

	// twoCommits returns the base and a head that adds a second constructor.
	twoCommits := func() (repo, base string) {
		repo = newTempGitRepo()
		commitFile(repo, "go.mod", goModuleFile)
		commitFile(repo, ".gitattributes", "*.go diff=instrumented\n")
		base = commitFile(repo, "pkg/model/m.go", oneConstructor)
		commitFile(repo, "pkg/model/m.go", modelFileWithTwoConstructors)
		return repo, base
	}
	changedSignalSubjects := func(stdout []byte) []string {
		var subjects []string
		for _, signal := range decodeCoachReport(stdout).Signals {
			if signal.Path == "pkg/model/m.go" && signal.Changed {
				subjects = append(subjects, signal.Subject)
			}
		}
		return subjects
	}

	It("marks the lines a commit added as changed (control)", func() {
		repo, base := twoCommits()

		stdout, stderr, exitCode := runCoachCodesignalRaw(repo, base, "--format=json")

		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		Expect(changedSignalSubjects(stdout)).NotTo(BeEmpty())
	})

	It("still marks them changed when git is configured to colour its diff output", func() {
		repo, base := twoCommits()
		runGitInRepo(repo, "config", "color.diff", "always")
		runGitInRepo(repo, "config", "color.ui", "always")

		stdout, stderr, exitCode := runCoachCodesignalRaw(repo, base, "--format=json")

		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		Expect(changedSignalSubjects(stdout)).NotTo(BeEmpty(),
			"escape codes in front of the hunk headers must not hide every changed line")
	})

	It("does not run a textconv driver the repository configures for the file", func() {
		repo, base := twoCommits()
		scripts := GinkgoT().TempDir()
		marker := filepath.Join(scripts, "textconv-ran")
		textconv := filepath.Join(scripts, "textconv.sh")
		Expect(os.WriteFile(textconv, []byte(fmt.Sprintf("#!/bin/sh\ntouch %q\ncat \"$1\"\n", marker)), 0o755)).To(Succeed())
		runGitInRepo(repo, "config", "diff.instrumented.textconv", textconv)

		stdout, stderr, exitCode := runCoachCodesignalRaw(repo, base, "--format=json")

		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		Expect(changedSignalSubjects(stdout)).NotTo(BeEmpty())
		_, statErr := os.Stat(marker)
		Expect(os.IsNotExist(statErr)).To(BeTrue(), "a configured textconv command ran during the scan")
	})
})
