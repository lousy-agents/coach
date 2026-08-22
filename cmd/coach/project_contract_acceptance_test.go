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
	It("writes nothing to stdout and an actionable message to stderr for an invalid --project-config", func() {
		repo := newTempGitRepo()
		baseSHA := commitFile(repo, "a.go", "package a\n\nfunc A() {}\n")
		headSHA := commitFile(repo, "project.json", "not valid json")

		stdout, stderr, exitCode := runCoachCodesignalRaw(repo, baseSHA, "--project-config", "project.json", "--format=json")

		Expect(exitCode).To(Equal(2))
		Expect(stdout).To(BeEmpty(), "a --project-config load/validation failure must write NOTHING to stdout")
		Expect(string(stderr)).To(ContainSubstring("project.json"), "stderr must identify the --project-config path")
		Expect(string(stderr)).To(ContainSubstring(headSHA), "stderr must identify the analyzed revision")
	})

	It("writes nothing to stdout and an actionable message to stderr for an invalid --project-config when --project-language is typescript", func() {
		repo := newTempGitRepo()
		baseSHA := commitFile(repo, "a.ts", "export const A = 1;\n")
		headSHA := commitFile(repo, "project.json", "not valid json")

		stdout, stderr, exitCode := runCoachCodesignalRaw(repo, baseSHA, "--project-config", "project.json", "--project-language", "typescript", "--format=json")

		Expect(exitCode).To(Equal(2))
		Expect(stdout).To(BeEmpty(), "a --project-config load/validation failure must write NOTHING to stdout")
		Expect(string(stderr)).To(ContainSubstring("project.json"), "stderr must identify the --project-config path")
		Expect(string(stderr)).To(ContainSubstring(headSHA), "stderr must identify the analyzed revision")
	})

	// loadProjectConfig runs before resolveProjectBackend and never receives
	// --project-language (main.go:465-469), so class-2 config failures are
	// language-independent by construction. This spec guards against a
	// reordering that would break that invariant.
	It("produces the same class-2 shape for --project-language go and --project-language typescript given the same invalid config", func() {
		repo := newTempGitRepo()
		commitFile(repo, "a.go", "package a\n\nfunc A() {}\n")
		configSHA := commitFile(repo, "project.json", "not valid json")

		goStdout, goStderr, goExitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--project-language", "go", "--format=json")
		tsStdout, tsStderr, tsExitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--project-language", "typescript", "--format=json")

		Expect(goExitCode).To(Equal(2))
		Expect(tsExitCode).To(Equal(2))
		Expect(tsExitCode).To(Equal(goExitCode), "class-2 config failures must exit the same code regardless of --project-language")

		Expect(goStdout).To(BeEmpty(), "go: a --project-config load/validation failure must write NOTHING to stdout")
		Expect(tsStdout).To(BeEmpty(), "typescript: a --project-config load/validation failure must write NOTHING to stdout")

		Expect(string(goStderr)).NotTo(BeEmpty())
		Expect(string(tsStderr)).NotTo(BeEmpty())
		Expect(string(goStderr)).To(ContainSubstring("project.json"), "go: stderr must identify the --project-config path")
		Expect(string(tsStderr)).To(ContainSubstring("project.json"), "typescript: stderr must identify the --project-config path")
		Expect(string(goStderr)).To(ContainSubstring(configSHA), "go: stderr must identify the analyzed revision")
		Expect(string(tsStderr)).To(ContainSubstring(configSHA), "typescript: stderr must identify the analyzed revision")
		Expect(string(tsStderr)).To(Equal(string(goStderr)), "class-2 config failures must produce the same message regardless of --project-language")
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

	// Re-sorting after Build must match pkg/codesignal's path/kind/location/message
	// key order; a message-only tiebreaker would reorder rows whose messages
	// disagree with their location order, breaking determinism.
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
