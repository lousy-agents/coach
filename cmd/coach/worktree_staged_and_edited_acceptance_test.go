package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

// PRD 8.1: "Staged content that differs from disk shall be replaced by the disk
// bytes." Three versions of one path are told apart by the constructors each
// declares, so a signal names the version the scan read.
var _ = Describe("coach codesignal --worktree reads the disk bytes of a path that is staged and then edited again", func() {
	const (
		path        = "pkg/model/m.go"
		committed   = "package model\n"
		stagedBytes = "package model\n\ntype X struct{}\n\ntype Y struct{}\n\nfunc NewX() *X {\n\treturn &X{}\n}\n\nfunc NewY() *Y {\n\treturn &Y{}\n}\n"
	)
	stagedThenEdited := func() string {
		repo := newTempGitRepo()
		commitFile(repo, "go.mod", goModuleFile)
		commitFile(repo, path, committed)
		stageWorktreeFile(repo, path, stagedBytes)
		modifyTrackedFile(repo, path, modelFileWithTwoConstructors)
		expectDirtyPorcelain(repo, "MM "+path)
		return repo
	}
	subjectsAt := func(report *codesignal.Report) []string {
		var subjects []string
		for _, signal := range report.Signals {
			if signal.RuleID == "structure.constructor_density" && signal.Path == path {
				subjects = append(subjects, signal.Subject)
			}
		}
		return subjects
	}
	expectDiskVersionAnalyzed := func(stdout, stderr []byte, exitCode int) {
		ExpectWithOffset(1, exitCode).To(Equal(0), "stderr: %s", stderr)
		report := decodeCoachReport(stdout)
		ExpectWithOffset(1, subjectsAt(report)).To(ConsistOf("NewA", "NewB"),
			"the finding of the bytes on disk is reported, not that of the staged or committed version")
		ExpectWithOffset(1, diagnosticsOfKind(report.Diagnostics, "worktree_changes_not_analyzed")).To(BeEmpty())
	}

	It("reports the finding of the bytes on disk in a diff scan, and does not say the path was not analyzed", func() {
		stdout, stderr, exitCode := runCoachCodesignalRaw(stagedThenEdited(), "HEAD", "--worktree", "--format=json", "--fail-on-incomplete-coverage")

		expectDiskVersionAnalyzed(stdout, stderr, exitCode)
	})

	It("reports the finding of the bytes on disk in a baseline scan, and does not say the path was not analyzed", func() {
		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(stagedThenEdited(), "--worktree", "--format=json", "--fail-on-incomplete-coverage")

		expectDiskVersionAnalyzed(stdout, stderr, exitCode)
	})
})
