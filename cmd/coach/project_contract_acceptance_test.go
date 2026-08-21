package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	"github.com/lousy-agents/coach/internal/codesignalcli"
	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

var _ = Describe("coach project-analysis failure reports", func() {
	It("writes nothing to stdout and an actionable message to stderr for an invalid revision config", func() {
		repo := newTempGitRepo()
		initialSHA := commitFile(repo, "a.go", "package a\n\nfunc A() {}\n")
		configSHA := commitFile(repo, "project.json", "not valid json")

		stdout, stderr, exitCode := runCoachCodesignalRaw(repo, initialSHA, "--project-config", "project.json", "--format=json")

		Expect(exitCode).To(Equal(2))
		Expect(stdout).To(BeEmpty(), "a --project-config load/validation failure must write NOTHING to stdout")
		Expect(string(stderr)).To(ContainSubstring("project.json"), "stderr must identify the --project-config path")
		// The analyzed revision is HEAD (configSHA, the project.json commit),
		// not --base (initialSHA), which only bounds the diff.
		Expect(string(stderr)).To(ContainSubstring(configSHA), "stderr must identify the analyzed revision")
	})

	// "go" and "typescript" both have registered backends now, so no real
	// --project-language flag value can reach
	// project_backend_unavailable through the CLI any more. This test
	// instead overrides the loadProjectConfig/resolveProjectBackend seams
	// in-process (see "coach codesignal project-mode exit-code
	// classification" in project_acceptance_test.go for the same pattern)
	// to keep exercising the full report shape a genuinely unavailable
	// backend still produces.
	It("writes a local report and structured diagnostic when the selected backend is unavailable", func() {
		originalLoadProjectConfig := loadProjectConfig
		originalResolveProjectBackend := resolveProjectBackend
		DeferCleanup(func() {
			loadProjectConfig = originalLoadProjectConfig
			resolveProjectBackend = originalResolveProjectBackend
		})
		loadProjectConfig = func(string, string, string) (json.RawMessage, error) {
			return json.RawMessage(`{"schema_version":"1","roots":["."]}`), nil
		}
		resolveProjectBackend = func(string) error {
			return &codesignalcli.ProjectBackendUnavailableError{
				Message: `coach codesignal: no project-analysis backend is available for language "rust" yet (project_backend_unavailable)`,
			}
		}

		stdout, stderr, exitCode := runInProcess("codesignal", "--baseline", "--project-config", "project.json", "--format=json")

		Expect(exitCode).To(Equal(3))
		Expect(stderr).To(BeEmpty())
		var report struct {
			SchemaVersion string `json:"schema_version"`
			Diagnostics   []struct {
				Kind string `json:"kind"`
			} `json:"diagnostics"`
		}
		Expect(json.Unmarshal(stdout, &report)).To(Succeed())
		Expect(report.SchemaVersion).To(Equal("1"))
		Expect(report.Diagnostics).To(ContainElement(MatchFields(IgnoreExtras, Fields{
			"Kind": Equal("project_backend_unavailable"),
		})))
	})

	It("rejects unknown configuration fields before backend selection", func() {
		repo := newTempGitRepo()
		initialSHA := commitFile(repo, "a.go", "package a\n\nfunc A() {}\n")
		commitFile(repo, "project.json", `{"schema_version":"1","roots":["."],"unexpected":true}`)

		stdout, stderr, exitCode := runCoachCodesignalRaw(repo, initialSHA, "--project-config", "project.json", "--format=json")

		Expect(exitCode).To(Equal(2))
		Expect(stdout).To(BeEmpty())
		Expect(string(stderr)).To(ContainSubstring("project.json"))
	})

	It("reads configuration from the selected revision instead of the uncommitted worktree, naming commit as the remedy", func() {
		repo := newTempGitRepo()
		initialSHA := commitFile(repo, "a.go", "package a\n\nfunc A() {}\n")
		Expect(os.WriteFile(filepath.Join(repo, "project.json"), []byte(`{"schema_version":"1","roots":["."]}`), 0o644)).To(Succeed())

		stdout, stderr, exitCode := runCoachCodesignalRaw(repo, initialSHA, "--project-config", "project.json", "--format=json")

		Expect(exitCode).To(Equal(2))
		Expect(stdout).To(BeEmpty())
		Expect(string(stderr)).To(ContainSubstring("project.json"))
		Expect(string(stderr)).To(ContainSubstring(initialSHA))
		Expect(string(stderr)).To(ContainSubstring("commit"), "an uncommitted worktree file must name commit as the remedy (AC-6)")
	})

	It("names committing as the remedy when a --suggest-project-config --output candidate is fed back without being committed", func() {
		repo := newTempGitRepo()
		headSHA := commitFile(repo, "go.mod", "module example.com/remedy\n\ngo 1.25\n")

		_, suggestStderr, suggestExit := runCoachSuggest(repo, "--baseline", "--suggest-project-config", "--output", "project.json")
		Expect(suggestExit).To(Equal(0), "stderr: %s", suggestStderr)

		stdout, stderr, exitCode := runCoachCodesignalRaw(repo, headSHA, "--project-config", "project.json", "--format=json")

		Expect(exitCode).To(Equal(2))
		Expect(stdout).To(BeEmpty(), "a --project-config load/validation failure must write NOTHING to stdout")
		Expect(string(stderr)).To(ContainSubstring("project.json"))
		Expect(string(stderr)).To(ContainSubstring(headSHA))
		Expect(string(stderr)).To(ContainSubstring("commit"), "an uncommitted --suggest-project-config --output candidate must name commit as the remedy (AC-6)")
	})

	// When project-mode appends a diagnostic after Build, re-sorting must keep
	// the same path/kind/location/message key order as pkg/codesignal. A
	// path/kind/message-only sort reorders equal path+kind diagnostics whose
	// messages disagree with location order and breaks deterministic reports.
	It("keeps location-aware diagnostic order when appending a project diagnostic", func() {
		loc := func(row uint) *semantics.Location {
			return &semantics.Location{StartRow: row}
		}
		report := &codesignal.Report{
			Diagnostics: []codesignal.Diagnostic{
				{Path: "a.go", Kind: "syntax_errors", Message: "zzz", Location: loc(1)},
				{Path: "a.go", Kind: "syntax_errors", Message: "aaa", Location: loc(2)},
			},
		}
		report = withProjectDiagnostic(report, &codesignal.Diagnostic{
			Kind:    "project_config_invalid",
			Path:    "project.json",
			Message: "bad config",
		})

		Expect(report.Diagnostics).To(HaveLen(3))
		Expect(report.Diagnostics[0].Message).To(Equal("zzz"), "location row 1 must sort before row 2 even when its message is lexicographically later")
		Expect(report.Diagnostics[0].Location.StartRow).To(Equal(uint(1)))
		Expect(report.Diagnostics[1].Message).To(Equal("aaa"))
		Expect(report.Diagnostics[1].Location.StartRow).To(Equal(uint(2)))
		Expect(report.Diagnostics[2].Kind).To(Equal("project_config_invalid"))
	})
})
