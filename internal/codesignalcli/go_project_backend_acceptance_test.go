package codesignalcli

import (
	"context"
	"encoding/json"
	"io/fs"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/render"
	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/projectmodel"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("source_sink_pack config field disposition", func() {
	It("never changes which findings the real Go backend produces or their content, but does change config_digest/id/fingerprint", func() {
		body_projectAcceptanceTest_neverChangesWhichFindingsTheRealGoBackendProduce_858()
	})
})

var _ = Describe("Go layer-bypass search coverage folding into project lifecycle", func() {
	var originalBuildGoLayerBypass func(ctx context.Context, snapshot fs.FS, opts projectmodel.LayerBypassOptions) (projectmodel.LayerBypassResult, error)

	BeforeEach(func() {
		originalBuildGoLayerBypass = buildGoLayerBypass
		DeferCleanup(func() {
			buildGoLayerBypass = originalBuildGoLayerBypass
		})
	})

	It("degrades a found witness to lifecycle unknown and surfaces project_layer_bypass_coverage_incomplete when the search itself did not complete", func() {
		buildGoLayerBypass = func(ctx context.Context, snapshot fs.FS, opts projectmodel.LayerBypassOptions) (projectmodel.LayerBypassResult, error) {
			return projectmodel.LayerBypassResult{
				Witnesses: []projectmodel.LayerBypassWitness{goLayerBypassFakeWitness},
				Coverage:  projectmodel.Coverage{Phase: "layer_bypass_search", Complete: false},
			}, nil
		}

		dir := gitfixture.Init(GinkgoT())
		gitfixture.CommitFile(GinkgoT(), dir, "go.mod", "module example.com/app\n\ngo 1.25\n")
		sha := gitfixture.CommitFile(GinkgoT(), dir, "pkg/handlers/handlers.go", "package handlers\n\nfunc Handler() {}\n")
		files := []gitrepo.SelectedFile{{Path: "pkg/handlers/handlers.go", Language: "go", Status: "added"}}

		project := &ProjectAnalysis{
			ConfigPath:   "project.json",
			Language:     "go",
			Config:       goLayerBypassSearchConfigJSON,
			ConfigDigest: projectconfig.Digest(goLayerBypassSearchConfigJSON),
			Backend:      NewGoProjectBackend(),
		}
		report, err := AnalyzeBaseline(context.Background(), dir, sha, files, nil, "", codesignal.Coverage{TrackedFilesDiscovered: 1}, project)
		Expect(err).NotTo(HaveOccurred())

		Expect(report.ProjectChanges).To(HaveLen(1), "the high-confidence witness must still surface as a ProjectChange even though the search was incomplete")
		change := report.ProjectChanges[0]
		Expect(change.RuleID).To(Equal("architecture.layer_bypass"))
		Expect(string(change.Lifecycle)).To(Equal("unknown"), "an incomplete layer-bypass search must never let a witness claim a determinate lifecycle")
		Expect(report.ProjectCoverage).NotTo(BeNil())
		Expect(report.ProjectCoverage.Complete).To(BeFalse(), "combineProjectCoverage must fold the bypass search's own incomplete Coverage into the reported project coverage")

		Expect(countDiagnosticsOfKind(report.Diagnostics, "project_layer_bypass_coverage_incomplete")).To(Equal(1))
		Expect(countDiagnosticsOfKind(report.Diagnostics, "project_lifecycle_indeterminate")).To(Equal(1))
	})

	It("passes MaxSearchNodes as 0 (unbounded) deliberately, not goProjectBudgets.MaxGraphNodes", func() {
		calls := 0
		var capturedMaxSearchNodes int
		buildGoLayerBypass = func(ctx context.Context, snapshot fs.FS, opts projectmodel.LayerBypassOptions) (projectmodel.LayerBypassResult, error) {
			calls++
			capturedMaxSearchNodes = opts.MaxSearchNodes
			return projectmodel.LayerBypassResult{Coverage: projectmodel.Coverage{Phase: "layer_bypass_search", Complete: true}}, nil
		}

		dir := gitfixture.Init(GinkgoT())
		gitfixture.CommitFile(GinkgoT(), dir, "go.mod", "module example.com/app\n\ngo 1.25\n")
		sha := gitfixture.CommitFile(GinkgoT(), dir, "pkg/handlers/handlers.go", "package handlers\n\nfunc Handler() {}\n")
		files := []gitrepo.SelectedFile{{Path: "pkg/handlers/handlers.go", Language: "go", Status: "added"}}

		project := &ProjectAnalysis{
			ConfigPath:   "project.json",
			Language:     "go",
			Config:       goLayerBypassSearchConfigJSON,
			ConfigDigest: projectconfig.Digest(goLayerBypassSearchConfigJSON),
			Backend:      NewGoProjectBackend(),
		}
		report, err := AnalyzeBaseline(context.Background(), dir, sha, files, nil, "", codesignal.Coverage{TrackedFilesDiscovered: 1}, project)
		Expect(err).NotTo(HaveOccurred())

		Expect(calls).To(Equal(1), "the bypass seam must actually have been invoked, or the MaxSearchNodes assertion below is vacuous")

		Expect(capturedMaxSearchNodes).To(Equal(0))
		Expect(countDiagnosticsOfKind(report.Diagnostics, "project_layer_bypass_coverage_incomplete")).To(Equal(0))
	})

	It("degrades only the base-side coverage-incomplete diagnostic to base_-prefixed and still marks the report indeterminate when only the base revision's search is incomplete", func() {
		body_projectAcceptanceTest_degradesOnlyTheBaseSideCoverageIncompleteDiagnos_1021()
	})

	It("keeps head- and base-side coverage-incomplete diagnostics distinct when both revisions' searches are incomplete", func() {
		buildGoLayerBypass = func(ctx context.Context, snapshot fs.FS, opts projectmodel.LayerBypassOptions) (projectmodel.LayerBypassResult, error) {
			return projectmodel.LayerBypassResult{Coverage: projectmodel.Coverage{Phase: "layer_bypass_search", Complete: false}}, nil
		}

		dir := gitfixture.Init(GinkgoT())
		baseSHA := gitfixture.CommitFile(GinkgoT(), dir, "go.mod", "module example.com/app\n\ngo 1.25\n")
		headSHA := gitfixture.CommitFile(GinkgoT(), dir, "pkg/handlers/handlers.go", "package handlers\n\nfunc Handler() {}\n")
		files := []gitrepo.SelectedFile{{Path: "pkg/handlers/handlers.go", Language: "go", Status: "added"}}

		project := &ProjectAnalysis{
			ConfigPath:   "project.json",
			Language:     "go",
			Config:       goLayerBypassSearchConfigJSON,
			ConfigDigest: projectconfig.Digest(goLayerBypassSearchConfigJSON),
			Backend:      NewGoProjectBackend(),
		}
		report, err := AnalyzeChanges(context.Background(), dir, headSHA, baseSHA, files, nil, "all", nil, project)
		Expect(err).NotTo(HaveOccurred())

		Expect(countDiagnosticsOfKind(report.Diagnostics, "project_layer_bypass_coverage_incomplete")).To(Equal(1), "the head-side incompleteness must not be collapsed into or duplicated by the base-side one")
		Expect(countDiagnosticsOfKind(report.Diagnostics, "base_project_layer_bypass_coverage_incomplete")).To(Equal(1))
	})
})

func body_projectAcceptanceTest_neverChangesWhichFindingsTheRealGoBackendProduce_858() {
	dir := gitfixture.Init(GinkgoT())
	gitfixture.CommitFile(GinkgoT(), dir, "go.mod", "module example.com/app\n\ngo 1.25\n")
	gitfixture.CommitFile(GinkgoT(), dir, "pkg/db/db.go", "package db\n\nvar Name = \"db\"\n")
	sha := gitfixture.CommitFile(GinkgoT(), dir, "pkg/handlers/handlers.go", "package handlers\n\nimport \"example.com/app/pkg/db\"\n\nfunc Use() string {\n\treturn db.Name\n}\n")
	files := []gitrepo.SelectedFile{
		{Path: "pkg/db/db.go", Language: "go", Status: "added"},
		{Path: "pkg/handlers/handlers.go", Language: "go", Status: "added"},
	}

	withoutField := json.RawMessage(`{"schema_version":"1","roots":["."],"layers":[{"name":"handlers","prefixes":["pkg/handlers"]},{"name":"db","prefixes":["pkg/db"]}],"forbidden_imports":[{"from":"handlers","to":"db"}]}`)
	withPack := json.RawMessage(`{"schema_version":"1","roots":["."],"layers":[{"name":"handlers","prefixes":["pkg/handlers"]},{"name":"db","prefixes":["pkg/db"]}],"forbidden_imports":[{"from":"handlers","to":"db"}],"source_sink_pack":"builtin-v1"}`)
	_, err := projectconfig.Parse(withoutField)
	Expect(err).NotTo(HaveOccurred())
	_, err = projectconfig.Parse(withPack)
	Expect(err).NotTo(HaveOccurred())

	buildReport := func(cfg json.RawMessage) *codesignal.Report {
		project := &ProjectAnalysis{
			ConfigPath:   "project.json",
			Language:     "go",
			Config:       cfg,
			ConfigDigest: projectconfig.Digest(cfg),
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

	withoutJSON, err := render.ReportJSON(withoutReport)
	Expect(err).NotTo(HaveOccurred())
	withJSON, err := render.ReportJSON(withReport)
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

	dir := gitfixture.Init(GinkgoT())
	baseSHA := gitfixture.CommitFile(GinkgoT(), dir, "go.mod", "module example.com/app\n\ngo 1.25\n")
	headSHA := gitfixture.CommitFile(GinkgoT(), dir, "pkg/handlers/handlers.go", "package handlers\n\nfunc Handler() {}\n")
	files := []gitrepo.SelectedFile{{Path: "pkg/handlers/handlers.go", Language: "go", Status: "added"}}

	project := &ProjectAnalysis{
		ConfigPath:   "project.json",
		Language:     "go",
		Config:       goLayerBypassSearchConfigJSON,
		ConfigDigest: projectconfig.Digest(goLayerBypassSearchConfigJSON),
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

// goLayerBypassSearchConfigJSON declares a handlers/service layer pair with
// service as required_layer, matching goLayerBypassFakeWitness's
// RequiredLayer/Source/Sink below.
var goLayerBypassSearchConfigJSON = json.RawMessage(`{"schema_version":"1","roots":["."],"layers":[{"name":"handlers","prefixes":["pkg/handlers"]},{"name":"service","prefixes":["pkg/service"]}],"required_layer":"service"}`)

// goLayerBypassFakeWitness is a minimal, validly anchored high-confidence
// LayerBypassWitness: its Path has a step with a non-empty Path field, which
// EvaluateGoLayerBypass requires to compute a PrimaryAnchor (see
// pkg/codesignal/rule_layer_bypass.go) rather than dropping the witness as
// anchorless.
var goLayerBypassFakeWitness = projectmodel.LayerBypassWitness{
	ID:            "witness-1",
	Source:        "example.com/app/pkg/handlers.Handler",
	Sink:          "(*database/sql.DB).Query",
	RequiredLayer: "service",
	Path: []projectmodel.LayerBypassStep{
		{NodeID: "example.com/app/pkg/handlers.Handler", Path: "pkg/handlers/handlers.go", Line: 1},
		{NodeID: "(*database/sql.DB).Query"},
	},
	Confidence:       projectmodel.LayerBypassConfidenceHigh,
	AlgorithmVersion: "go-layer-bypass-registry@1",
}
