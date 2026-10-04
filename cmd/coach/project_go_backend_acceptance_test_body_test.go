package main

import (
	"encoding/json"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

func body_projectGoBackendAcceptanceTest_staysOnTheSchema1PathMatchesARepositoryThatNever_93() {
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
}

func body_projectGoBackendAcceptanceTest_presentsTheSameStructuredEvidenceInTextAsJSONAnd_372() {
	repo := newTempGitRepo()
	commitFile(repo, "go.mod", goModuleFile)
	commitFile(repo, "pkg/db/db.go", dbPackageFile)
	commitFile(repo, "pkg/handlers/handlers.go", handlersImportingDB)
	By("committing a second importer of pkg/db so the violation group has two sites and RelatedLocations is non-empty")
	commitFile(repo, "pkg/handlers/other.go", "package handlers\n\nimport \"example.com/app/pkg/db\"\n\nfunc Other() string {\n\treturn db.Name\n}\n")
	commitFile(repo, "project.json", goLayerPolicyConfigJSON)

	jsonStdout, jsonStderr, jsonExit := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--format=json")
	Expect(jsonExit).To(Equal(0), "stderr: %s", jsonStderr)
	report := decodeCoachReport(jsonStdout)
	Expect(report.ProjectChanges).To(HaveLen(1))

	textStdout, textStderr, textExit := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json")
	Expect(textExit).To(Equal(0), "stderr: %s", textStderr)
	text := string(textStdout)

	Expect(text).To(ContainSubstring("Project findings:"))
	Expect(text).To(ContainSubstring("semantic_key: " + report.ProjectChanges[0].SemanticKey))
	Expect(text).To(ContainSubstring("rule_id: architecture.layer_violation"))
	Expect(text).To(ContainSubstring("path: pkg/handlers/handlers.go"))
	Expect(text).To(ContainSubstring("lifecycle: baseline"))
	Expect(text).To(ContainSubstring("machine_evidence.importer: pkg/handlers"))
	Expect(text).To(ContainSubstring("machine_evidence.importee: pkg/db"))
	Expect(text).To(ContainSubstring("Project summary: active=1"))
	Expect(text).To(ContainSubstring("Project coverage: phase=go_model_build, complete=true"))

	By("asserting signals[] carries the same structured machine_evidence text shows, so a consumer reading only signals gets full parity")
	Expect(report.Signals).To(HaveLen(1))
	sig := report.Signals[0]
	Expect(sig.MachineEvidence).To(Equal(map[string]string{
		"importer":   "pkg/handlers",
		"importee":   "pkg/db",
		"layer_from": "handlers",
		"layer_to":   "db",
		"rule":       "handlers->db",
	}))
	for key, value := range sig.MachineEvidence {
		Expect(text).To(ContainSubstring("machine_evidence." + key + ": " + value))
	}
	Expect(sig.RelatedLocations).NotTo(BeEmpty())
	Expect(sig.RelatedLocations).To(Equal(report.ProjectChanges[0].RelatedLocations))

	By("asserting text shows the same related location JSON RelatedLocations carries, not only the primary anchor")
	for _, location := range sig.RelatedLocations {
		Expect(text).To(ContainSubstring(fmt.Sprintf("related: %s:%d", location.Path, location.Location.StartRow+1)))
	}

	legacyStdout, legacyStderr, legacyExit := runCoachCodesignalBaselineRaw(repo)
	Expect(legacyExit).To(Equal(0), "stderr: %s", legacyStderr)
	legacyText := string(legacyStdout)
	Expect(legacyText).NotTo(ContainSubstring("Project findings:"))
	Expect(legacyText).NotTo(ContainSubstring("Project coverage:"))
}

func body_projectGoBackendAcceptanceTest_ordersTheArchitectureFindingAheadOfTheLowSeverit_430() {
	repo := newTempGitRepo()
	commitFile(repo, "go.mod", goModuleFile)
	commitFile(repo, "pkg/db/db.go", dbPackageFile)
	commitFile(repo, "pkg/handlers/handlers.go", handlersImportingDB)
	By("committing the structural finding so it naturally lands first in the pre-sort signals slice (Build appends file-local signals before project signals) -- sortSignals's sort.SliceStable would let that incidental order pass this test for the wrong reason unless the comparator itself is what decides the final order")
	commitFile(repo, "pkg/model/model.go", modelFileWithTwoConstructors)
	commitFile(repo, "project.json", goLayerPolicyConfigJSON)

	stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", "project.json", "--format=json")
	Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)
	Expect(stderr).To(BeEmpty())

	report := decodeCoachReport(stdout)

	architectureIndex := -1
	structuralIndex := -1
	for i, sig := range report.Signals {
		switch sig.RuleID {
		case "architecture.layer_violation", "architecture.layer_bypass":
			if architectureIndex == -1 {
				architectureIndex = i
			}
		case "structure.constructor_density":
			if structuralIndex == -1 {
				structuralIndex = i
			}
		}
	}

	Expect(architectureIndex).To(BeNumerically(">=", 0), "expected an architecture.layer_violation or architecture.layer_bypass signal in signals[]: %s", stdout)
	Expect(structuralIndex).To(BeNumerically(">=", 0), "expected a structure.constructor_density signal in signals[]: %s", stdout)

	Expect(report.Signals[architectureIndex].Severity).To(Equal(codesignal.Severity("advisory")))
	Expect(report.Signals[architectureIndex].Confidence).To(Equal(codesignal.Confidence("high")))
	Expect(report.Signals[structuralIndex].Severity).To(Equal(codesignal.Severity("low")))
	Expect(report.Signals[architectureIndex].Lifecycle).To(Equal(report.Signals[structuralIndex].Lifecycle), "both findings must belong to the same lifecycle group for this to test the severity comparator rather than group ordering")

	Expect(architectureIndex).To(BeNumerically("<", structuralIndex), "an advisory-severity, high-confidence architecture finding must outrank a low-severity structural finding in signals[] order (issue #259)")
}

func body_projectGoBackendAcceptanceTest_producesAReportIdenticalToAnEquivalentRevisionWi_473() {
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
}
