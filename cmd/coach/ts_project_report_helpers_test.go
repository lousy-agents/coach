package main

import (
	"context"
	"encoding/json"
	"strings"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func runCoachCodesignalBaselineEnv(repo, path string, extraArgs ...string) (stdout, stderr []byte, exitCode int) {
	return runCoachBinary(commandPath, repo, stubToolchainEnv(path), append([]string{"codesignal", "--baseline"}, extraArgs...)...)
}

// codesignalArgsFromRemediationLine reads only stderr's first line: since
// AC-SET-9 (#330), a no-controlling-terminal scan appends a second
// interactive-setup line after the fit-check invocation this parses.
func codesignalArgsFromRemediationLine(stderr []byte) []string {
	firstLine, _, _ := strings.Cut(string(stderr), "\n")
	line := strings.TrimSpace(firstLine)
	_, invocation, found := strings.Cut(line, ": run ")
	Expect(found).To(BeTrue(), "stderr must print a runnable fit-check invocation, got %q", line)
	fields := strings.Fields(invocation)
	Expect(len(fields)).To(BeNumerically(">=", 3), "printed invocation must be coach codesignal <flags>, got %q", invocation)
	Expect(fields[0]).To(Equal("coach"), "printed invocation must start with coach so a customer can run it unchanged, got %q", invocation)
	Expect(fields[1]).To(Equal("codesignal"), "printed invocation must invoke codesignal, got %q", invocation)
	return fields[2:]
}

// expectTypescriptCompilerMissingScan's fixtures declare no package manager
// and no mise scope at all, so readiness's own AvailableSetupChoices menu
// offers nothing installable: the appended --prepare-compiler command is
// correctly withheld (O2) rather than naming a command that would only open
// to report it had nothing to do.
func expectTypescriptCompilerMissingScan(stdout, stderr []byte, exitCode int) {
	Expect(exitCode).To(Equal(2), "stdout: %s stderr: %s", stdout, stderr)
	Expect(stdout).To(BeEmpty(), "never producing a report means nothing is written to stdout")
	Expect(strings.TrimSpace(string(stderr))).To(Equal("typescript_compiler_missing: run coach codesignal --baseline --check-project --project-language typescript --project-config project.json"))
	Expect(string(stderr)).NotTo(ContainSubstring("coach:"))
	Expect(string(stderr)).NotTo(ContainSubstring("@typescript/"))
}

// splitTextFindingsAndFacts splits RenderText's output at its "\nFacts:\n"
// section marker (render/project_changes.go's renderProjectFacts), so a spec can assert
// separately about the findings section (Signals + "Project findings:"
// ProjectChanges) and everything from "Facts:" onward: RenderText writes
// renderProjectFacts, renderDiagnosticsSection, renderCoverageSection, and
// renderProjectCoverageSection in that order with no further section
// markers this helper splits on, so factsSection is "Facts: through end of
// output", not ProjectFacts alone.
//
// Callers pair this with a JSON-decoded assertion on the same fixture first:
// the JSON checks are the structural source of truth, and the text-format
// checks this helper supports only confirm the text renderer doesn't
// diverge from what JSON already proved, not an independent proof.
func splitTextFindingsAndFacts(text string) (findingsSection, factsSection string) {
	idx := strings.Index(text, "\nFacts:\n")
	ExpectWithOffset(1, idx).To(BeNumerically(">", 0), "expected a \"Facts:\" section in text output, got %q", text)
	return text[:idx], text[idx:]
}

// analyzeTSProjectBackend calls the exported tsProjectBackend contract
// (NewTSProjectBackend/ProjectBackend.Analyze) directly, in-process, rather
// than through the compiled coach binary: project_scope is not yet rendered
// through codesignal.Input/Report (issue #332 Task 10's job, not Task 9
// T1's), so ProjectBackendResult -- the public contract at this boundary --
// is the most meaningful place to observe HeadProjectScope/BaseProjectScope.
// Calling Analyze in-process still spawns the real analyzer subprocess
// (BuildTypeScriptModelViaSidecar), and the analyzer child is still a
// descendant of this test binary, so startAnalyzerEnvironSampler's
// descendant-restricted PID scan observes it exactly as it would through the
// compiled binary.
func analyzeTSProjectBackend(dir, headRevision, baseRevision string, baseline bool, configJSON string) (*codesignalcli.ProjectBackendResult, error) {
	config := json.RawMessage(configJSON)
	backend := codesignalcli.NewTSProjectBackend()
	return backend.Analyze(context.Background(), codesignalcli.ProjectBackendRequest{
		Dir:          dir,
		HeadRevision: headRevision,
		BaseRevision: baseRevision,
		Baseline:     baseline,
		ConfigPath:   "project.json",
		Config:       config,
		ConfigDigest: projectconfig.Digest(config),
		Language:     "typescript",
	})
}

func containsProjectModelDiagnosticCode(diagnostics []projectmodel.Diagnostic, code string) bool {
	for _, d := range diagnostics {
		if d.Code == code {
			return true
		}
	}
	return false
}

// countProjectModelDiagnosticCode asserts a model diagnostic is folded into
// the reported ProjectCoverage exactly once (see tsBypassCoverageForFold in
// internal/codesignalcli/ts_project_revision.go), not once per fold.
func countProjectModelDiagnosticCode(diagnostics []projectmodel.Diagnostic, code string) int {
	count := 0
	for _, d := range diagnostics {
		if d.Code == code {
			count++
		}
	}
	return count
}

// diagnosticMessageForKind returns the Message of the first report
// diagnostic matching kind, or "" if none matches.
func diagnosticMessageForKind(diagnostics []codesignal.Diagnostic, kind string) string {
	for _, d := range diagnostics {
		if d.Kind == kind {
			return d.Message
		}
	}
	return ""
}

// AC-12.
func assertReachabilityNeverSignalOrChange(report *codesignal.Report) {
	for _, signal := range report.Signals {
		Expect(signal.RuleID).NotTo(Equal("possible_call_reachability"), "reachability must never surface as a Signal, got %+v", signal)
		Expect(signal.Kind).NotTo(Equal("possible_call_reachability"), "reachability must never surface as a Signal, got %+v", signal)
	}
	for _, change := range report.ProjectChanges {
		Expect(change.RuleID).NotTo(Equal("possible_call_reachability"), "reachability must never surface as a ProjectChange, got %+v", change)
		Expect(change.Kind).NotTo(Equal("possible_call_reachability"), "reachability must never surface as a ProjectChange, got %+v", change)
	}
}

func projectChangeRuleIDs(changes []codesignal.ProjectChange) map[string]bool {
	seen := map[string]bool{}
	for _, change := range changes {
		seen[change.RuleID] = true
	}
	return seen
}

// backendUnavailableDiagnosticMessage asserts report's project coverage
// records a backend_unavailable diagnostic and returns the last one's
// message.
func backendUnavailableDiagnosticMessage(report *codesignal.Report) string {
	var found bool
	var message string
	for _, diag := range report.ProjectCoverage.Diagnostics {
		if diag.Code == projectmodel.DiagBackendUnavailable {
			found = true
			message = diag.Message
		}
	}
	ExpectWithOffset(1, found).To(BeTrue(), "expected a %s diagnostic in ProjectCoverage.Diagnostics, got %+v", projectmodel.DiagBackendUnavailable, report.ProjectCoverage.Diagnostics)
	return message
}
