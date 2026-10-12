package main

import (
	"os"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

var _ = Describe("coach codesignal --baseline TypeScript project provenance", Label("ts-project-backend"), func() {
	BeforeEach(func() {
		skipWithoutRealTypeScriptCompiler()
	})

	Describe("runtime provenance fields", func() {
		When("a clean committed npm TypeScript project is scanned", func() {
			var report *codesignal.Report

			BeforeEach(func() {
				repo := newTempGitRepo()
				version := realTypescriptVersion()
				commitNpmFixture(repo, version)
				installRealTypescriptCompiler(repo, true)
				report, _ = runCoachBaselineProjectJSON(repo, "project.json")
			})

			It("reports runtime.kind=node, runtime.origin=path, and a semver runtime.version", func() {
				prov := report.ProjectProvenance
				Expect(prov.Runtime.Kind).To(Equal("node"))
				Expect(prov.Runtime.Origin).To(Equal("path"))
				Expect(prov.Runtime.Version).To(MatchRegexp(`^v?\d+\.\d+\.\d+`))
			})

			It("reports package_manager.kind=npm from lockfile origin", func() {
				Expect(report.ProjectProvenance).NotTo(BeNil())
				Expect(report.ProjectProvenance.PackageManager).NotTo(BeNil())
				Expect(report.ProjectProvenance.PackageManager.Kind).To(Equal("npm"))
				Expect(report.ProjectProvenance.PackageManager.Origin).To(Equal("lockfile"))
			})

			It("reports package_manager.version as a semver string when npm is on PATH", func() {
				if _, err := exec.LookPath("npm"); err != nil {
					Skip("npm not on PATH: cannot assert version")
				}
				Expect(report.ProjectProvenance.PackageManager).NotTo(BeNil())
				Expect(report.ProjectProvenance.PackageManager.Version).To(MatchRegexp(`^\d+\.\d+\.\d+`),
					"package_manager.version must be a semver string when npm is on PATH; got %q", report.ProjectProvenance.PackageManager.Version)
			})

			It("does not emit a worktree skip or provenance diagnostic for a clean worktree", func() {
				for _, d := range report.Diagnostics {
					Expect(d.Kind).NotTo(Equal(codesignal.DiagKindWorktreeChangesNotAnalyzed),
						"clean worktree must not trigger worktree_changes_not_analyzed")
					Expect(d.Kind).NotTo(Equal(codesignal.DiagKindWorktreeReportReflectsCommittedHEAD),
						"clean worktree must not trigger worktree_report_reflects_committed_head")
				}
			})

			It("reports analyzer.digest with sha256: prefix and analyzer.version as stable identifier", func() {
				prov := report.ProjectProvenance
				Expect(prov.Analyzer.Digest).To(HavePrefix("sha256:"))
				Expect(prov.Analyzer.Version).NotTo(BeEmpty())
				Expect(prov.Analyzer.Version).NotTo(ContainSubstring("/"))
				Expect(strings.Contains(prov.Analyzer.Version, string(os.PathSeparator))).To(BeFalse(),
					"analyzer.version must not be a filesystem path; got %q", prov.Analyzer.Version)
			})
		})
	})
})
