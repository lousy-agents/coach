package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

// runCoachBaselineProjectJSON runs coach codesignal --baseline with the
// given project config flags and returns the decoded Report from stdout.
func runCoachBaselineProjectJSON(repo, projectConfig string, extraArgs ...string) (*codesignal.Report, []byte) {
	args := append([]string{"--project-language", "typescript", "--project-config", projectConfig, "--format", "json"}, extraArgs...)
	stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, args...)
	Expect(exitCode).To(Equal(0), "coach codesignal --baseline: stdout=%s stderr=%s", stdout, stderr)
	var report codesignal.Report
	Expect(json.Unmarshal(stdout, &report)).To(Succeed(), "stdout should be one JSON report: %s", stdout)
	return &report, stdout
}

// commitNpmFixture commits a minimal npm TypeScript project into repo.
// A .gitignore ignoring node_modules/ is committed so that the TypeScript
// compiler installed by installRealTypescriptCompiler does not appear in
// git status as untracked files that would trigger a worktree diagnostic.
func commitNpmFixture(repo, version string) {
	commitFile(repo, ".gitignore", "node_modules/\n")
	commitFile(repo, "package.json", `{"name":"example","devDependencies":{"typescript":"`+version+`"}}`)
	commitFile(repo, "package-lock.json", `{"name":"example","lockfileVersion":3,"requires":true,"packages":{}}`)
	commitFile(repo, "tsconfig.json", `{"compilerOptions":{"module":"commonjs","moduleResolution":"node10"}}`)
	commitFile(repo, "a.ts", "export const x = 1;\n")
	commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`)
}

// commitBunFixture commits a minimal bun TypeScript project into repo.
func commitBunFixture(repo, version string) {
	commitFile(repo, ".gitignore", "node_modules/\n")
	commitFile(repo, "package.json", `{"name":"example","packageManager":"bun@1.0.0","devDependencies":{"typescript":"`+version+`"}}`)
	commitFile(repo, "bun.lockb", "bun lockfile binary placeholder\n")
	commitFile(repo, "tsconfig.json", `{"compilerOptions":{"module":"commonjs","moduleResolution":"node10"}}`)
	commitFile(repo, "a.ts", "export const x = 1;\n")
	commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`)
}

var _ = Describe("coach codesignal --baseline TypeScript project provenance", Label("ts-project-backend"), func() {
	BeforeEach(func() {
		if reason := ensureRealTypeScriptCompilerAvailable(); reason != "" {
			Skip(reason)
		}
	})

	Describe("snapshot package manager and worktree diagnostic (SA-280-033)", func() {
		When("committed npm lockfile with dirty bun worktree", func() {
			var report *codesignal.Report
			var rawJSON []byte

			BeforeEach(func() {
				repo := newTempGitRepo()
				version := realTypescriptVersion()
				commitNpmFixture(repo, version)
				installRealTypescriptCompiler(repo, true)

				Expect(os.WriteFile(filepath.Join(repo, "bun.lock"), []byte("# bun lockfile\n"), 0o644)).To(Succeed())

				report, rawJSON = runCoachBaselineProjectJSON(repo, "project.json")
			})

			It("reports package_manager.kind npm from the committed snapshot, not bun from the dirty worktree", func() {
				Expect(report.ProjectProvenance).NotTo(BeNil(), "project_provenance must be present")
				Expect(report.ProjectProvenance.PackageManager).NotTo(BeNil(), "package_manager must be present")
				Expect(report.ProjectProvenance.PackageManager.Kind).To(Equal("npm"))
				Expect(report.ProjectProvenance.PackageManager.Origin).To(Equal("lockfile"))
			})

			It("emits worktree_report_reflects_committed_head diagnostic", func() {
				kinds := make([]string, len(report.Diagnostics))
				for i, d := range report.Diagnostics {
					kinds[i] = d.Kind
				}
				Expect(kinds).To(ContainElement(codesignal.DiagKindWorktreeReportReflectsCommittedHEAD),
					"diagnostics must include worktree_report_reflects_committed_head; got %v", kinds)
				Expect(kinds).NotTo(ContainElement(codesignal.DiagKindWorktreeChangesNotAnalyzed),
					"unsupported lockfile dirt must not use the file-local skip kind; got %v", kinds)
			})

			It("diagnostic message states the report applies to committed HEAD", func() {
				var msg string
				for _, d := range report.Diagnostics {
					if d.Kind == codesignal.DiagKindWorktreeReportReflectsCommittedHEAD {
						msg = d.Message
					}
				}
				Expect(msg).To(ContainSubstring("HEAD"), "message must reference committed HEAD; got %q", msg)
			})

			It("reports runtime.kind=node, runtime.origin=path, and a non-empty runtime.version", func() {
				prov := report.ProjectProvenance
				Expect(prov.Runtime.Kind).To(Equal("node"))
				Expect(prov.Runtime.Origin).To(Equal("path"))
				Expect(prov.Runtime.Version).NotTo(BeEmpty())
			})

			It("contains no absolute paths in project_provenance JSON", func() {
				var prov map[string]json.RawMessage
				fields := make(map[string]json.RawMessage)
				Expect(json.Unmarshal(rawJSON, &fields)).To(Succeed())
				Expect(json.Unmarshal(fields["project_provenance"], &prov)).To(Succeed())
				provStr := string(fields["project_provenance"])
				Expect(provStr).NotTo(ContainSubstring("/tmp"), "absolute /tmp path in provenance JSON: %s", provStr)
				Expect(provStr).NotTo(ContainSubstring("/home"), "absolute /home path in provenance JSON: %s", provStr)
				Expect(provStr).NotTo(ContainSubstring("/root"), "absolute /root path in provenance JSON: %s", provStr)
			})

			It("populates analyzer.digest with sha256: prefix", func() {
				prov := report.ProjectProvenance
				Expect(prov.Analyzer.Digest).To(HavePrefix("sha256:"), "analyzer.digest must be a sha256: hex digest; got %q", prov.Analyzer.Digest)
				Expect(prov.Analyzer.Digest).NotTo(ContainSubstring("/"), "analyzer.digest must not be a path; got %q", prov.Analyzer.Digest)
			})

			It("populates analyzer.version as a stable non-path identifier", func() {
				prov := report.ProjectProvenance
				Expect(prov.Analyzer.Version).NotTo(BeEmpty())
				Expect(prov.Analyzer.Version).NotTo(ContainSubstring("/"), "analyzer.version must not be an absolute path; got %q", prov.Analyzer.Version)
				Expect(prov.Analyzer.Version).To(ContainSubstring("@"),
					"analyzer.version must include a '@version' suffix to be a stable identity, not a bare filename; got %q", prov.Analyzer.Version)
			})
		})

		When("committed bun lockb with dirty package-lock.json in worktree", func() {
			var report *codesignal.Report

			BeforeEach(func() {
				repo := newTempGitRepo()
				version := realTypescriptVersion()
				commitBunFixture(repo, version)
				installRealTypescriptCompiler(repo, true)

				Expect(os.WriteFile(filepath.Join(repo, "package-lock.json"), []byte(`{"lockfileVersion":3}`), 0o644)).To(Succeed())

				report, _ = runCoachBaselineProjectJSON(repo, "project.json")
			})

			It("reports package_manager.kind bun from the committed snapshot, not npm from the dirty lockfile", func() {
				Expect(report.ProjectProvenance).NotTo(BeNil())
				Expect(report.ProjectProvenance.PackageManager).NotTo(BeNil(), "package_manager must be present")
				Expect(report.ProjectProvenance.PackageManager.Kind).To(Equal("bun"))
			})

			It("emits worktree_report_reflects_committed_head diagnostic", func() {
				kinds := make([]string, len(report.Diagnostics))
				for i, d := range report.Diagnostics {
					kinds[i] = d.Kind
				}
				Expect(kinds).To(ContainElement(codesignal.DiagKindWorktreeReportReflectsCommittedHEAD))
				Expect(kinds).NotTo(ContainElement(codesignal.DiagKindWorktreeChangesNotAnalyzed))
			})
		})
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

// commitYarnFixture commits a minimal TypeScript project with a yarn lockfile and
// packageManager field, representing a repository that uses yarn — which coach drops.
func commitYarnFixture(repo, version string) {
	commitFile(repo, ".gitignore", "node_modules/\n")
	commitFile(repo, "package.json", `{"name":"example","packageManager":"yarn@4.0.0","devDependencies":{"typescript":"`+version+`"}}`)
	commitFile(repo, "yarn.lock", "# yarn lockfile v1\n")
	commitFile(repo, "tsconfig.json", `{"compilerOptions":{"module":"commonjs","moduleResolution":"node10"}}`)
	commitFile(repo, "a.ts", "export const x = 1;\n")
	commitFile(repo, "project.json", `{"schema_version":"1","roots":["."]}`)
}

var _ = Describe("coach codesignal --baseline TypeScript project: yarn package manager (SA-280-033)", Label("ts-project-backend"), func() {
	BeforeEach(func() {
		if reason := ensureRealTypeScriptCompilerAvailable(); reason != "" {
			Skip(reason)
		}
	})

	When("the committed project uses yarn (yarn.lock + packageManager field)", func() {
		var report *codesignal.Report

		BeforeEach(func() {
			repo := newTempGitRepo()
			version := realTypescriptVersion()
			commitYarnFixture(repo, version)
			installRealTypescriptCompiler(repo, true)

			report, _ = runCoachBaselineProjectJSON(repo, "project.json")
		})

		It("omits package_manager from project_provenance (yarn is not in the supported manager set)", func() {
			Expect(report.ProjectProvenance).NotTo(BeNil(), "project_provenance must be present")
			Expect(report.ProjectProvenance.PackageManager).To(BeNil(),
				"package_manager must be nil when the repository uses yarn; got %+v", report.ProjectProvenance.PackageManager)
		})

		It("still reports a complete analysis with runtime and analyzer provenance", func() {
			prov := report.ProjectProvenance
			Expect(prov.Runtime.Kind).To(Equal("node"))
			Expect(prov.Analyzer.Version).NotTo(BeEmpty())
		})
	})
})
