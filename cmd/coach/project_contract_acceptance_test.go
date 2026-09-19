package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

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

	// AC-2 requires an offered remediation command to actually be supported
	// for the language it was offered to. classifyAnalysisError's appended
	// --suggest-project-config remediation used to name the TypeScript-only
	// guided-authoring invocation unconditionally, which a --project-language
	// go scan cannot run without a controlling terminal (it refuses outright).
	It("prints a --project-language go remediation command that actually succeeds, instead of the TypeScript-only guided-authoring command (AC-2)", func() {
		repo := newTempGitRepo()
		commitFile(repo, "go.mod", "module example.com/remedy\n\ngo 1.25\n")
		commitFile(repo, "a.go", "package a\n\nfunc A() {}\n")
		commitFile(repo, "project.json", "not valid json")

		_, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--project-language", "go", "--format=json")

		Expect(exitCode).To(Equal(2), "stderr: %s", stderr)
		lines := stderrLines(stderr)
		Expect(lines).To(HaveLen(2), "stderr: %s", stderr)

		remediation := lines[len(lines)-1]
		Expect(remediation).NotTo(ContainSubstring("--project-language typescript"), "a --project-language go scan must never be offered the TypeScript-only guided-authoring command; stderr: %s", stderr)

		fields := strings.Fields(remediation)
		Expect(fields).NotTo(BeEmpty())
		Expect(fields[0]).To(Equal("coach"))
		remediationStdout, remediationStderr, remediationExit := runCoachBinary(commandPath, repo, nil, fields[1:]...)
		Expect(remediationExit).To(Equal(0), "the printed remediation command must actually succeed for this Go repository; stdout: %s stderr: %s", remediationStdout, remediationStderr)
	})

	// loadProjectConfig runs before resolveProjectBackend and never receives
	// --project-language (main.go:465-469), so the underlying class-2 config
	// message and the appended --suggest-project-config remediation stay
	// language-independent by construction. The one exception is AC-SET-13's
	// additional readiness-gap line: prepareProjectAnalysis wraps a class-2
	// TypeScript failure with a fresh readiness snapshot so a simultaneous
	// compiler gap is reported rather than masked, but "go" never computes
	// readiness at all. This spec pins that boundary (first and last stderr
	// lines match; TypeScript alone may carry one readiness-gap line between
	// them) rather than asserting byte-identical stderr.
	It("produces the same class-2 message and remediation for --project-language go and --project-language typescript given the same invalid config", func() {
		repo := newTempGitRepo()
		commitFile(repo, "a.go", "package a\n\nfunc A() {}\n")
		configSHA := commitFile(repo, "project.json", "not valid json")

		goStdout, goStderr, goExitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--project-language", "go", "--format=json")

		path := pathWithStubNode("v24.9.9")
		tsStdout, tsStderr, tsExitCode := runCoachCodesignalBaselineEnv(repo, path, "--project-config", "project.json", "--project-language", "typescript", "--format=json")

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

		goLines := stderrLines(goStderr)
		tsLines := stderrLines(tsStderr)
		Expect(goLines).To(HaveLen(2), "go's class-2 report is the config message plus AC-SET-9's appended remediation")
		Expect(tsLines[0]).To(Equal(goLines[0]), "the underlying class-2 config message must stay language-independent")
		Expect(goLines[len(goLines)-1]).To(Equal("coach codesignal --baseline --suggest-project-config"), "go's appended remediation must be a command go actually supports, not the TypeScript-only guided-authoring invocation (AC-2)")
		Expect(tsLines[len(tsLines)-1]).To(Equal("on a terminal: coach codesignal --baseline --suggest-project-config --project-language typescript -- guided authoring requires a controlling terminal; without one, draft the schema-1 project-config document yourself, have a human review and commit it, then rerun with --project-config <path>"), "typescript's appended remediation must still name its own guided-authoring invocation")
		Expect(len(tsLines)).To(BeNumerically(">=", len(goLines)), "TypeScript may additionally carry AC-SET-13's readiness-gap line; it must never carry fewer lines than go's report")
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
