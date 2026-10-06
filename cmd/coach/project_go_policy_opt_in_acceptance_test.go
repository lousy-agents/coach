package main

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

type disabledProjectAnalysisReportPair struct {
	repoWithConfigFile      string
	stdoutWithConfigFile    []byte
	reportWithConfigFile    *codesignal.Report
	reportWithoutConfigFile *codesignal.Report
}

var _ = Describe("coach codesignal --project-config with the real Go project-language backend", func() {
	When("--baseline is run against a repository with no --project-config supplied", func() {
		It("stays schema-1 even though the committed config would otherwise report a violation", func() {
			repo := newTempGitRepo()
			commitFile(repo, "go.mod", goModuleFile)
			commitFile(repo, "pkg/db/db.go", dbPackageFile)
			commitFile(repo, "pkg/handlers/handlers.go", handlersImportingDB)
			commitFile(repo, "project.json", goLayerPolicyConfigJSON)

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(stderr).To(BeEmpty())

			var document map[string]json.RawMessage
			Expect(json.Unmarshal(stdout, &document)).To(Succeed())
			var schemaVersion string
			Expect(json.Unmarshal(document["schema_version"], &schemaVersion)).To(Succeed())
			Expect(schemaVersion).To(Equal("1"))
			Expect(document).NotTo(HaveKey("project_changes"))
			Expect(document).NotTo(HaveKey("project_summary"))
			Expect(document).NotTo(HaveKey("project_coverage"))
		})
	})

	When("the CLI is invoked without --project-config against a repository that could otherwise report an architecture.layer_violation and a structural finding", func() {
		It("stays on the schema-1 path, matches a repository that never had a project-analysis config at all, and leaks no project_* keys, schema_version 2, or project-analysis-only text", func() {
			pair := buildDisabledProjectAnalysisReportPair()

			By("(a) staying on the schema-1 path and (c) leaking no project_* key in JSON")
			var document map[string]json.RawMessage
			Expect(json.Unmarshal(pair.stdoutWithConfigFile, &document)).To(Succeed())
			var schemaVersion string
			Expect(json.Unmarshal(document["schema_version"], &schemaVersion)).To(Succeed())
			Expect(schemaVersion).To(Equal("1"), "disabled project analysis must stay on the schema-1 path")
			for _, key := range []string{"project_changes", "project_facts", "project_summary", "project_coverage"} {
				Expect(document).NotTo(HaveKey(key), "disabled project analysis must never leak %q", key)
			}
			Expect(string(pair.stdoutWithConfigFile)).NotTo(ContainSubstring(`"schema_version":"2"`))

			By("(b) matching a repository that never had a project-analysis config present at all")

			By("(c) leaking no project-analysis-only marker in text output")
			textStdout, textStderr, textExit := runCoachCodesignalBaselineRaw(pair.repoWithConfigFile)
			Expect(textExit).To(Equal(0), "stderr: %s", textStderr)
			text := string(textStdout)
			for _, marker := range []string{"Project findings:", "Project summary:", "Project coverage:", "Facts:", "coverage_ref:"} {
				Expect(text).NotTo(ContainSubstring(marker), "disabled project analysis text output must never show %q", marker)
			}
		})
	})

	When("--baseline is run without --project-config against a revision that also carries an unreferenced project.json, a layer-violation import, and a low-severity structural finding", func() {
		It("produces a report identical to an equivalent revision with no project.json at all, since no advisory signal can ever be produced without --project-config", func() {
			pair := buildDisabledProjectAnalysisReportPair()
			reportWithoutConfigFile := pair.reportWithoutConfigFile

			By("also pinning today's exact signals[] sequence, so a future severityRank/comparator change that perturbs non-advisory ordering fails here even though it would perturb both reports above identically")
			for _, sig := range reportWithoutConfigFile.Signals {
				Expect(sig.Severity).NotTo(Equal(codesignal.Severity("advisory")), "no advisory signal can ever be produced without --project-config: %+v", sig)
			}

			type ruleIDPathSeverity struct {
				RuleID   string
				Path     string
				Severity codesignal.Severity
			}
			gotSequence := make([]ruleIDPathSeverity, 0, len(reportWithoutConfigFile.Signals))
			for _, sig := range reportWithoutConfigFile.Signals {
				gotSequence = append(gotSequence, ruleIDPathSeverity{RuleID: sig.RuleID, Path: sig.Path, Severity: sig.Severity})
			}
			Expect(gotSequence).To(Equal([]ruleIDPathSeverity{
				{RuleID: "structure.constructor_density", Path: "pkg/model/model.go", Severity: codesignal.Severity("low")},
				{RuleID: "structure.pointer_return_density", Path: "pkg/model/model.go", Severity: codesignal.Severity("low")},
				{RuleID: "structure.constructor_density", Path: "pkg/model/model.go", Severity: codesignal.Severity("low")},
				{RuleID: "structure.pointer_return_density", Path: "pkg/model/model.go", Severity: codesignal.Severity("low")},
			}), "the no-config-file signals[] sequence must stay this literal shape: one constructor_density/pointer_return_density pair per constructor (NewA, NewB) in modelFileWithTwoConstructors")
		})
	})
})

func buildDisabledProjectAnalysisReportPair() disabledProjectAnalysisReportPair {
	repoWithConfigFile := newTempGitRepo()
	commitFile(repoWithConfigFile, "go.mod", goModuleFile)
	commitFile(repoWithConfigFile, "pkg/db/db.go", dbPackageFile)
	commitFile(repoWithConfigFile, "pkg/handlers/handlers.go", handlersImportingDB)
	commitFile(repoWithConfigFile, "pkg/model/model.go", modelFileWithTwoConstructors)
	commitFile(repoWithConfigFile, "project.json", goLayerPolicyConfigJSON)

	repoWithoutConfigFile := newTempGitRepo()
	commitFile(repoWithoutConfigFile, "go.mod", goModuleFile)
	commitFile(repoWithoutConfigFile, "pkg/db/db.go", dbPackageFile)
	commitFile(repoWithoutConfigFile, "pkg/handlers/handlers.go", handlersImportingDB)
	commitFile(repoWithoutConfigFile, "pkg/model/model.go", modelFileWithTwoConstructors)

	stdoutWithConfigFile, stderrWithConfigFile, exitWithConfigFile := runCoachCodesignalBaselineRaw(repoWithConfigFile, "--format=json")
	ExpectWithOffset(1, exitWithConfigFile).To(Equal(0), "stderr: %s", stderrWithConfigFile)
	ExpectWithOffset(1, stderrWithConfigFile).To(BeEmpty())
	reportWithConfigFile := decodeCoachReport(stdoutWithConfigFile)

	stdoutWithoutConfigFile, stderrWithoutConfigFile, exitWithoutConfigFile := runCoachCodesignalBaselineRaw(repoWithoutConfigFile, "--format=json")
	ExpectWithOffset(1, exitWithoutConfigFile).To(Equal(0), "stderr: %s", stderrWithoutConfigFile)
	ExpectWithOffset(1, stderrWithoutConfigFile).To(BeEmpty())
	reportWithoutConfigFile := decodeCoachReport(stdoutWithoutConfigFile)

	ExpectWithOffset(1, reportWithConfigFile.Coverage.TrackedFilesDiscovered).To(
		Equal(reportWithoutConfigFile.Coverage.TrackedFilesDiscovered+1),
		"an unreferenced project.json must add exactly one to TrackedFilesDiscovered")
	expectedUnsupported := append(append([]codesignal.CoverageGroup{}, reportWithoutConfigFile.Coverage.Unsupported...),
		codesignal.CoverageGroup{Reason: "unsupported_language", Language: ".json", Count: 1})
	ExpectWithOffset(1, reportWithConfigFile.Coverage.Unsupported).To(ConsistOf(expectedUnsupported),
		"an unreferenced project.json must add exactly one unsupported_language(.json) group and change nothing else in Unsupported")

	reportWithConfigFile.Scope.Revision = reportWithoutConfigFile.Scope.Revision
	reportWithConfigFile.Coverage.TrackedFilesDiscovered = reportWithoutConfigFile.Coverage.TrackedFilesDiscovered
	reportWithConfigFile.Coverage.Unsupported = reportWithoutConfigFile.Coverage.Unsupported

	ExpectWithOffset(1, reportWithConfigFile).To(Equal(reportWithoutConfigFile),
		"an unreferenced project.json in the tree must not change the report when --project-config is not supplied")

	return disabledProjectAnalysisReportPair{
		repoWithConfigFile:      repoWithConfigFile,
		stdoutWithConfigFile:    stdoutWithConfigFile,
		reportWithConfigFile:    reportWithConfigFile,
		reportWithoutConfigFile: reportWithoutConfigFile,
	}
}
