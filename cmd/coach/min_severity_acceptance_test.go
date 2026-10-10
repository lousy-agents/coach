package main

import (
	"encoding/json"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// jsonDocument decodes a report as raw top-level members so a spec can compare
// summary and coverage as JSON values and see exactly which keys are emitted.
func jsonDocument(stdout []byte) map[string]json.RawMessage {
	var document map[string]json.RawMessage
	ExpectWithOffset(1, json.Unmarshal(stdout, &document)).To(Succeed(), "stdout should be one JSON report: %s", stdout)
	return document
}

func signalPaths(document map[string]json.RawMessage) []string {
	var signals []struct {
		Path     string `json:"path"`
		Severity string `json:"severity"`
	}
	ExpectWithOffset(1, json.Unmarshal(document["signals"], &signals)).To(Succeed())
	paths := make([]string, 0, len(signals))
	for _, signal := range signals {
		paths = append(paths, signal.Path)
	}
	return paths
}

// severityFloorFixture commits two functions whose findings straddle the
// `high` floor: deep.go sits above 2x each complexity threshold (high) and
// tangle.go above the thresholds but below 2x (medium), two signals each.
func severityFloorFixture() string {
	repo := newTempGitRepo()
	commitFile(repo, "tangle.go", nestedIfs("tangle", 6))
	commitFile(repo, "deep.go", nestedIfs("deep", 8))
	return repo
}

var _ = Describe("coach codesignal --min-severity", func() {
	When("a baseline scan is narrowed to findings at or above high", func() {
		It("renders only the high findings and states how many it withheld", func() {
			repo := severityFloorFixture()

			fullStdout, fullStderr, fullExit := runCoachCodesignalBaselineRaw(repo, "--format=json")
			Expect(fullExit).To(Equal(0), "stderr: %s", fullStderr)
			full := decodeCoachReport(fullStdout)
			belowFloor := 0
			for _, signal := range full.Signals {
				if signal.Severity != "high" {
					belowFloor++
				}
			}
			Expect(full.Signals).To(HaveLen(4), "fixture must keep two high and two medium signals")
			Expect(belowFloor).To(Equal(2), "fixture must keep both a retained and a withheld signal")

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--min-severity", "high")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			text := string(stdout)

			Expect(text).To(ContainSubstring("path: deep.go"))
			Expect(text).NotTo(ContainSubstring("path: tangle.go"))
			Expect(text).NotTo(ContainSubstring("severity: medium"))
			Expect(text).To(ContainSubstring("withheld: 2 signals below --min-severity high"))
		})

		It("keeps summary and coverage describing the full analysis in JSON and adds the withheld count", func() {
			repo := severityFloorFixture()

			fullStdout, fullStderr, fullExit := runCoachCodesignalBaselineRaw(repo, "--format=json")
			Expect(fullExit).To(Equal(0), "stderr: %s", fullStderr)
			full := jsonDocument(fullStdout)

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json", "--min-severity", "high")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			narrowed := jsonDocument(stdout)

			Expect(signalPaths(narrowed)).To(ConsistOf("deep.go", "deep.go"))
			Expect(narrowed["summary"]).To(MatchJSON(full["summary"]))
			Expect(narrowed["coverage"]).To(MatchJSON(full["coverage"]))
			Expect(narrowed["diagnostics"]).To(MatchJSON(full["diagnostics"]))

			var summary struct {
				ActiveSignals int `json:"active_signals"`
			}
			Expect(json.Unmarshal(narrowed["summary"], &summary)).To(Succeed())
			Expect(summary.ActiveSignals).To(Equal(4))

			Expect(narrowed["signals_withheld"]).To(MatchJSON(`{"min_severity":"high","below_min_severity":2}`))
		})
	})

	When("the floor is at or below every finding's severity", func() {
		It("renders everything and still reports that nothing was withheld", func() {
			repo := severityFloorFixture()

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--min-severity", "medium")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			text := string(stdout)
			Expect(text).To(ContainSubstring("path: deep.go"))
			Expect(text).To(ContainSubstring("path: tangle.go"))
			Expect(text).To(ContainSubstring("withheld: 0 signals below --min-severity medium"))
		})
	})

	When("the floor withholds every finding", func() {
		It("does not claim there are no active findings", func() {
			repo := newTempGitRepo()
			commitFile(repo, "tangle.go", nestedIfs("tangle", 6))

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--min-severity", "high")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			text := string(stdout)
			Expect(text).NotTo(ContainSubstring("path: tangle.go"))
			Expect(text).NotTo(ContainSubstring("No active CodeSignal findings.\n"))
			Expect(text).To(ContainSubstring("No active CodeSignal findings at or above --min-severity high"))
			Expect(text).To(ContainSubstring("withheld: 2 signals below --min-severity high"))
		})
	})

	When("the flag is omitted", func() {
		It("emits no withheld count in text or JSON", func() {
			repo := severityFloorFixture()

			textStdout, _, textExit := runCoachCodesignalBaselineRaw(repo)
			Expect(textExit).To(Equal(0))
			Expect(string(textStdout)).NotTo(ContainSubstring("withheld"))

			jsonStdout, _, jsonExit := runCoachCodesignalBaselineRaw(repo, "--format=json")
			Expect(jsonExit).To(Equal(0))
			Expect(jsonDocument(jsonStdout)).NotTo(HaveKey("signals_withheld"))
		})
	})

	When("a floor narrows a diff comparison", func() {
		It("renders only high findings and states the withheld count", func() {
			repo := newTempGitRepo()
			base := commitFile(repo, "a.go", "package a\n")
			commitFile(repo, "tangle.go", nestedIfs("tangle", 6))
			commitFile(repo, "deep.go", nestedIfs("deep", 8))

			fullStdout, fullStderr, fullExit := runCoachCodesignalRaw(repo, base, "--format=json")
			Expect(fullExit).To(Equal(0), "stderr: %s", fullStderr)
			full := jsonDocument(fullStdout)
			Expect(signalPaths(full)).To(HaveLen(4))

			stdout, stderr, exitCode := runCoachCodesignalRaw(repo, base, "--min-severity", "high")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			text := string(stdout)
			Expect(text).To(ContainSubstring("path: deep.go"))
			Expect(text).NotTo(ContainSubstring("path: tangle.go"))
			Expect(text).To(ContainSubstring("active signals: 4"))
			Expect(text).To(ContainSubstring("withheld: 2 signals below --min-severity high"))

			jsonOut, _, jsonExit := runCoachCodesignalRaw(repo, base, "--format=json", "--min-severity", "high")
			Expect(jsonExit).To(Equal(0))
			narrowed := jsonDocument(jsonOut)
			Expect(narrowed["summary"]).To(MatchJSON(full["summary"]))
			Expect(narrowed["signals_withheld"]).To(MatchJSON(`{"min_severity":"high","below_min_severity":2}`))
		})
	})

	When("a project finding sits below the floor", func() {
		It("withholds the project change from the rendered findings once and leaves the project summary whole", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/db/db.go", dbPackageFile)
			commitFile(repo, "pkg/handlers/handlers.go", handlersImportingDB)
			commitFile(repo, "project.json", goLayerPolicyConfigJSON)

			fullStdout, fullStderr, fullExit := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--format=json")
			Expect(fullExit).To(Equal(0), "stderr: %s", fullStderr)
			full := jsonDocument(fullStdout)
			fullReport := decodeCoachReport(fullStdout)
			Expect(fullReport.ProjectChanges).To(HaveLen(1))
			Expect(string(fullReport.ProjectChanges[0].Severity)).To(Equal("advisory"))
			Expect(fullReport.Signals).To(HaveLen(1), "the project change is mirrored once in signals")

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--format=json", "--min-severity", "medium")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			narrowed := jsonDocument(stdout)
			Expect(string(narrowed["signals"])).To(Equal("[]"))
			Expect(string(narrowed["project_changes"])).To(Equal("[]"))
			Expect(narrowed["project_summary"]).To(MatchJSON(full["project_summary"]))
			Expect(narrowed["signals_withheld"]).To(MatchJSON(`{"min_severity":"medium","below_min_severity":1}`))

			textStdout, textStderr, textExit := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--min-severity", "medium")
			Expect(textExit).To(Equal(0), "stderr: %s", textStderr)
			Expect(string(textStdout)).NotTo(ContainSubstring("semantic_key:"))
			Expect(string(textStdout)).To(ContainSubstring("withheld: 1 signal below --min-severity medium"))
		})
	})

	When("a floor is supplied", func() {
		It("leaves the exit status independent of what the report contains", func() {
			repo := severityFloorFixture()

			for _, args := range [][]string{
				{},
				{"--min-severity", "high"},
				{"--min-severity", "low"},
			} {
				_, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, args...)
				Expect(exitCode).To(Equal(0), "args %v, stderr: %s", args, stderr)
			}
		})

		It("rejects a value outside high, medium, advisory, and low as a usage error", func() {
			repo := severityFloorFixture()

			for _, value := range []string{"urgent", "HIGH", ""} {
				stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--min-severity", value)
				Expect(exitCode).To(Equal(2), "value %q", value)
				Expect(stdout).To(BeEmpty())
				Expect(string(stderr)).To(ContainSubstring(fmt.Sprintf("invalid --min-severity value %q", value)))
			}
		})

		It("documents the flag in the usage text", func() {
			repo := severityFloorFixture()

			stdout, _, exitCode := runCoachCodesignalBaselineRaw(repo, "--help")
			Expect(exitCode).To(Equal(0))
			Expect(strings.Split(string(stdout), "\n")[0]).To(ContainSubstring("[--min-severity high|medium|advisory|low]"))
		})
	})

	When("a non-scan mode is combined with the floor", func() {
		It("rejects it rather than silently ignoring it", func() {
			repo := severityFloorFixture()

			_, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--check-project", "--project-language", "typescript", "--min-severity", "high")
			Expect(exitCode).To(Equal(2))
			Expect(string(stderr)).To(ContainSubstring("--check-project cannot be combined with --min-severity"))
		})
	})
})
