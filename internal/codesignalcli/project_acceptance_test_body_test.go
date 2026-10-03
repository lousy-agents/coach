package codesignalcli

import (
	"context"
	"encoding/json"
	"io/fs"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func body_projectAcceptanceTest_436() {
	if reason := ensureRealTypeScriptCompilerAvailable(); reason != "" {
		Skip(reason)
	}
}

func body_projectAcceptanceTest_481() {
	if reason := ensureRealTypeScriptCompilerAvailable(); reason != "" {
		Skip(reason)
	}
}

func body_projectAcceptanceTest_519() {
	if reason := ensureRealTypeScriptCompilerAvailable(); reason != "" {
		Skip(reason)
	}
}

func body_projectAcceptanceTest_559() {
	if reason := ensureRealTypeScriptCompilerAvailable(); reason != "" {
		Skip(reason)
	}
}

func body_projectAcceptanceTest_rejectsDocumentsThatExceedTheJSONNestingBudget_759() {
	var b strings.Builder
	for i := 0; i < maxProjectConfigJSONDepth+2; i++ {
		b.WriteString(`{"a":`)
	}
	b.WriteString(`1`)
	for i := 0; i < maxProjectConfigJSONDepth+2; i++ {
		b.WriteByte('}')
	}
	err := validateProjectConfigJSON([]byte(b.String()))
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("nesting budget"))
}

func body_projectAcceptanceTest_neverChangesWhichFindingsTheRealGoBackendProduce_858() {
	dir := acceptanceTempGitRepo()
	acceptanceCommitFile(dir, "go.mod", "module example.com/app\n\ngo 1.25\n")
	acceptanceCommitFile(dir, "pkg/db/db.go", "package db\n\nvar Name = \"db\"\n")
	sha := acceptanceCommitFile(dir, "pkg/handlers/handlers.go", "package handlers\n\nimport \"example.com/app/pkg/db\"\n\nfunc Use() string {\n\treturn db.Name\n}\n")
	files := []SelectedFile{
		{Path: "pkg/db/db.go", Language: "go", Status: "added"},
		{Path: "pkg/handlers/handlers.go", Language: "go", Status: "added"},
	}

	withoutField := json.RawMessage(`{"schema_version":"1","roots":["."],"layers":[{"name":"handlers","prefixes":["pkg/handlers"]},{"name":"db","prefixes":["pkg/db"]}],"forbidden_imports":[{"from":"handlers","to":"db"}]}`)
	withPack := json.RawMessage(`{"schema_version":"1","roots":["."],"layers":[{"name":"handlers","prefixes":["pkg/handlers"]},{"name":"db","prefixes":["pkg/db"]}],"forbidden_imports":[{"from":"handlers","to":"db"}],"source_sink_pack":"builtin-v1"}`)
	Expect(validateProjectConfigJSON(withoutField)).To(Succeed())
	Expect(validateProjectConfigJSON(withPack)).To(Succeed())

	buildReport := func(cfg json.RawMessage) *codesignal.Report {
		project := &ProjectAnalysis{
			ConfigPath:   "project.json",
			Language:     "go",
			Config:       cfg,
			ConfigDigest: ConfigDigest(cfg),
			Backend:      NewGoProjectBackend(),
		}
		report, err := AnalyzeBaseline(context.Background(), dir, sha, files, nil, "", codesignal.Coverage{TrackedFilesDiscovered: 2}, project)
		Expect(err).NotTo(HaveOccurred())
		return report
	}

	withoutReport := buildReport(withoutField)
	withReport := buildReport(withPack)

	Expect(withoutReport.ProjectChanges).NotTo(BeEmpty(), "fixture must produce at least one real project finding, or this comparison cannot catch source_sink_pack being wired to change evaluation")
	Expect(withReport.ProjectChanges).To(HaveLen(len(withoutReport.ProjectChanges)))

	for i := range withoutReport.ProjectChanges {
		without := withoutReport.ProjectChanges[i]
		with := withReport.ProjectChanges[i]

		Expect(with.SemanticKey).To(Equal(without.SemanticKey))
		Expect(with.RuleID).To(Equal(without.RuleID))
		Expect(with.Kind).To(Equal(without.Kind))
		Expect(with.Severity).To(Equal(without.Severity))
		Expect(with.Confidence).To(Equal(without.Confidence))
		Expect(with.Lifecycle).To(Equal(without.Lifecycle))
		Expect(with.Evidence).To(Equal(without.Evidence))
		Expect(with.PrimaryAnchor).To(Equal(without.PrimaryAnchor))
		Expect(with.RelatedLocations).To(Equal(without.RelatedLocations))
		Expect(with.PathSteps).To(Equal(without.PathSteps))
		Expect(with.MachineEvidence).To(Equal(without.MachineEvidence))

		Expect(with.ConfigDigest).NotTo(Equal(without.ConfigDigest))
		Expect(with.ID).NotTo(Equal(without.ID))
		Expect(with.Fingerprint).NotTo(Equal(without.Fingerprint))
	}

	Expect(withReport.ProjectSummary).To(Equal(withoutReport.ProjectSummary))
	Expect(withReport.ProjectCoverage).To(Equal(withoutReport.ProjectCoverage))

	withoutJSON, err := RenderJSON(withoutReport)
	Expect(err).NotTo(HaveOccurred())
	withJSON, err := RenderJSON(withReport)
	Expect(err).NotTo(HaveOccurred())
	Expect(withJSON).NotTo(Equal(withoutJSON), "config_digest/id/fingerprint differ, so the full rendered JSON documents must differ too")
}

func body_projectAcceptanceTest_degradesOnlyTheBaseSideCoverageIncompleteDiagnos_1021() {
	callCount := 0
	buildGoLayerBypass = func(ctx context.Context, snapshot fs.FS, opts projectmodel.LayerBypassOptions) (projectmodel.LayerBypassResult, error) {
		callCount++
		if callCount == 1 {

			return projectmodel.LayerBypassResult{
				Witnesses: []projectmodel.LayerBypassWitness{goLayerBypassFakeWitness},
				Coverage:  projectmodel.Coverage{Phase: "layer_bypass_search", Complete: true},
			}, nil
		}
		return projectmodel.LayerBypassResult{Coverage: projectmodel.Coverage{Phase: "layer_bypass_search", Complete: false}}, nil
	}

	dir := acceptanceTempGitRepo()
	baseSHA := acceptanceCommitFile(dir, "go.mod", "module example.com/app\n\ngo 1.25\n")
	headSHA := acceptanceCommitFile(dir, "pkg/handlers/handlers.go", "package handlers\n\nfunc Handler() {}\n")
	files := []SelectedFile{{Path: "pkg/handlers/handlers.go", Language: "go", Status: "added"}}

	project := &ProjectAnalysis{
		ConfigPath:   "project.json",
		Language:     "go",
		Config:       goLayerBypassSearchConfigJSON,
		ConfigDigest: ConfigDigest(goLayerBypassSearchConfigJSON),
		Backend:      NewGoProjectBackend(),
	}
	report, err := AnalyzeChanges(context.Background(), dir, headSHA, baseSHA, files, nil, "all", nil, project)
	Expect(err).NotTo(HaveOccurred())

	Expect(report.ProjectChanges).To(HaveLen(1))
	Expect(string(report.ProjectChanges[0].Lifecycle)).To(Equal("unknown"), "the base revision's own incomplete search must degrade the whole report's lifecycle claims, including the head-side witness")

	Expect(report.ProjectCoverage).NotTo(BeNil())
	Expect(report.ProjectCoverage.Complete).To(BeTrue(), "sanity: the head-side search alone must have completed cleanly, or this spec would not be isolating the base-side failure it claims to")
	Expect(countDiagnosticsOfKind(report.Diagnostics, "project_layer_bypass_coverage_incomplete")).To(Equal(0), "the head-side search completed, so it must not also report incomplete coverage")
	Expect(countDiagnosticsOfKind(report.Diagnostics, "base_project_layer_bypass_coverage_incomplete")).To(Equal(1))
	Expect(countDiagnosticsOfKind(report.Diagnostics, "project_lifecycle_indeterminate")).To(Equal(1))
}
