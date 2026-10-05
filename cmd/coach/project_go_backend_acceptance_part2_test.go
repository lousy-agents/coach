package main

import (
	"encoding/json"

	"strings"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

// expectLayerBypassChange asserts the shared architecture.layer_bypass
// ProjectChange shape (structured path steps, provenance, stable semantic
// identity), leaving the caller to assert Lifecycle/Changed.
func expectLayerBypassChange(change codesignal.ProjectChange) {
	ExpectWithOffset(1, change.RuleID).To(Equal("architecture.layer_bypass"))
	ExpectWithOffset(1, change.Kind).To(Equal("architecture.layer_bypass"))
	ExpectWithOffset(1, change.Severity).To(Equal(codesignal.Severity("advisory")))
	ExpectWithOffset(1, change.Confidence).To(Equal(codesignal.Confidence("high")))
	ExpectWithOffset(1, change.SemanticKey).To(Equal("architecture.layer_bypass:service:example.com/app/pkg/handlers.Handler->(*database/sql.DB).Query"))
	ExpectWithOffset(1, change.PrimaryAnchor.Path).To(Equal("pkg/handlers/handlers.go"))
	ExpectWithOffset(1, change.PathSteps).NotTo(BeEmpty())
	for _, step := range change.PathSteps {
		ExpectWithOffset(1, step.NodeID).NotTo(BeEmpty())
		ExpectWithOffset(1, step.Confidence).To(Equal(codesignal.Confidence("high")))
	}
	ExpectWithOffset(1, change.PathSteps[0].NodeID).To(Equal("example.com/app/pkg/handlers.Handler"))
	ExpectWithOffset(1, change.PathSteps[len(change.PathSteps)-1].NodeID).To(Equal("(*database/sql.DB).Query"))
	ExpectWithOffset(1, change.MachineEvidence).To(Equal(map[string]string{
		"source":         "example.com/app/pkg/handlers.Handler",
		"sink":           "(*database/sql.DB).Query",
		"required_layer": "service",
		"path":           strings.Join(layerBypassPathNodeIDs(change), "->"),
	}))
	ExpectWithOffset(1, change.WhyItMatters).NotTo(BeEmpty())
	ExpectWithOffset(1, change.Recommendation).NotTo(BeEmpty())
	ExpectWithOffset(1, change.Provenance).To(Equal(codesignal.Provenance{Producer: "projectmodel", FindingKind: "architecture.layer_bypass"}))
}

func layerBypassPathNodeIDs(change codesignal.ProjectChange) []string {
	nodeIDs := make([]string, len(change.PathSteps))
	for i, step := range change.PathSteps {
		nodeIDs[i] = step.NodeID
	}
	return nodeIDs
}

func normalizeProjectChangesModulePath(changes []codesignal.ProjectChange, modulePath string) []string {
	normalized := make([]string, len(changes))
	for i, change := range changes {
		raw, err := json.Marshal(change)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())
		normalized[i] = strings.ReplaceAll(string(raw), modulePath, "<MODULE_PATH>")
	}
	return normalized
}

func decodeCoachReport(stdout []byte) *codesignal.Report {
	var report codesignal.Report
	ExpectWithOffset(1, json.Unmarshal(stdout, &report)).To(Succeed(), "stdout should be one JSON report: %s", stdout)
	return &report
}

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
