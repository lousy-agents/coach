package codesignalcli

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/projectmodel"
	"github.com/lousy-agents/coach/pkg/semantics"
)

func layerPrefixConfigJSON(n int) []byte {
	var b strings.Builder
	b.WriteString(`{"schema_version":"1","roots":["."],"layers":[{"name":"L","prefixes":[`)
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, `"p%04d"`, i)
	}
	b.WriteString(`]}]}`)
	return []byte(b.String())
}

func TestProjectTextAcceptance(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "project text renderer acceptance suite")
}

var _ = Describe("project-analysis text rendering", func() {
	It("renders active project observations, structured paths, and project coverage", func() {
		report := &codesignal.Report{
			SchemaVersion: "2",
			Scope:         codesignal.Scope{AppliedScope: "all"},
			Summary:       codesignal.Summary{FilesAnalyzed: 1},
			ProjectChanges: []codesignal.ProjectChange{{
				SemanticKey: "cycle:pkg/a<->pkg/b",
				RuleID:      "project.import_cycle",
				Lifecycle:   codesignal.Lifecycle("introduced"),
				Changed:     true,
				Evidence:    "pkg/a -> pkg/b -> pkg/a",
				PrimaryAnchor: codesignal.ProjectLocation{
					Path:     "pkg/a/a.go",
					Location: semantics.Location{StartRow: 2},
				},
				PathSteps: []codesignal.ProjectPathStep{{
					NodeID:      "package:pkg/a",
					DisplayName: "pkg/a",
					Resolution:  "resolved",
					Confidence:  codesignal.Confidence("high"),
					SourceLocations: []codesignal.ProjectLocation{{
						Path: "pkg/a/a.go", Location: semantics.Location{StartRow: 2},
					}},
				}},
			}},
			ProjectSummary:  &codesignal.ProjectSummary{ActiveChanges: 1, IntroducedChanges: 1},
			ProjectCoverage: &projectmodel.Coverage{Phase: "full", Complete: true, Counts: map[string]int{"packages": 2}},
		}

		text := RenderText(report)
		Expect(text).To(ContainSubstring("Project findings:"))
		Expect(text).To(ContainSubstring("semantic_key: cycle:pkg/a<->pkg/b"))
		Expect(text).To(ContainSubstring("path: pkg/a/a.go"))
		Expect(text).To(ContainSubstring("path step: package:pkg/a (pkg/a), resolution: resolved, confidence: high"))
		Expect(text).To(ContainSubstring("Project coverage: phase=full, complete=true"))
	})

	It("renders machine_evidence keys in sorted order under Project findings", func() {
		report := &codesignal.Report{
			SchemaVersion: "2",
			Scope:         codesignal.Scope{AppliedScope: "all"},
			ProjectChanges: []codesignal.ProjectChange{{
				SemanticKey: "layer:domain->infra",
				RuleID:      "architecture.layer_violation",
				Lifecycle:   codesignal.Lifecycle("baseline"),
				PrimaryAnchor: codesignal.ProjectLocation{
					Path:     "pkg/domain/d.go",
					Location: semantics.Location{StartRow: 0},
				},
				MachineEvidence: map[string]string{
					"importer":  "pkg/domain",
					"edge_kind": "internal",
					"importee":  "pkg/infra",
				},
			}},
			ProjectSummary: &codesignal.ProjectSummary{ActiveChanges: 1, BaselineChanges: 1},
		}

		text := RenderText(report)
		edge := strings.Index(text, "machine_evidence.edge_kind: internal")
		importee := strings.Index(text, "machine_evidence.importee: pkg/infra")
		importer := strings.Index(text, "machine_evidence.importer: pkg/domain")
		Expect(edge).To(BeNumerically(">=", 0))
		Expect(importee).To(BeNumerically(">", edge))
		Expect(importer).To(BeNumerically(">", importee))
	})

	It("renders facts-only observations under Facts without counting them as project findings", func() {
		report := &codesignal.Report{
			SchemaVersion: "2",
			Scope:         codesignal.Scope{AppliedScope: "all"},
			Summary:       codesignal.Summary{},
			ProjectFacts: []codesignal.ProjectFact{{
				Kind:        "possible_call_reachability",
				SemanticKey: "reach:handler->query",
				Evidence:    "Handler may reach Query",
				PathSteps: []codesignal.ProjectPathStep{{
					NodeID:      "func:Handler",
					DisplayName: "Handler",
					Resolution:  "static",
					Confidence:  codesignal.Confidence("medium"),
				}},
				Provenance: codesignal.Provenance{Producer: "projectmodel"},
			}},
			ProjectSummary: &codesignal.ProjectSummary{},
		}

		text := RenderText(report)
		Expect(text).To(ContainSubstring("No active CodeSignal findings."))
		Expect(text).To(ContainSubstring("Facts:"))
		Expect(text).To(ContainSubstring("kind: possible_call_reachability"))
		Expect(text).To(ContainSubstring("path step: func:Handler (Handler), resolution: static, confidence: medium"))
		Expect(text).NotTo(ContainSubstring("Project findings:"))
	})

	It("presents a Build-projected project observation once in text with structured fields", func() {
		builder, err := codesignal.New(codesignal.Options{ProjectEnabled: true, Baseline: true})
		Expect(err).NotTo(HaveOccurred())
		report, err := builder.Build(context.Background(), codesignal.Input{
			Files: []codesignal.FileChange{{
				Path:   "file_local.go",
				Status: "modified",
				Head: &semantics.Result{
					Path:        "file_local.go",
					Language:    semantics.LanguageGo,
					ParseStatus: "ok",
					Findings: []semantics.Finding{{
						Kind:     "mutates_input",
						Name:     "Update",
						Location: semantics.Location{StartRow: 0, EndRow: 0},
						Evidence: "input.value = 1",
					}},
				},
			}},
			ProjectChanges: []codesignal.ProjectChange{{
				SemanticKey: "cycle:pkg/a<->pkg/b",
				RuleID:      "architecture.layer_violation",
				RuleVersion: "1",
				Kind:        "project_layer_violation",
				Category:    "structure",
				Severity:    "medium",
				Confidence:  "high",
				Evidence:    "domain imports infrastructure",
				PrimaryAnchor: codesignal.ProjectLocation{
					Path:     "pkg/a/a.go",
					Location: semantics.Location{StartRow: 2},
				},
				RelatedLocations: []codesignal.ProjectLocation{{
					Path: "pkg/b/b.go", Location: semantics.Location{StartRow: 4},
				}},
				PathSteps: []codesignal.ProjectPathStep{{
					NodeID:      "package:pkg/a",
					DisplayName: "pkg/a",
					Resolution:  "resolved",
					Confidence:  codesignal.Confidence("high"),
				}},
				Provenance: codesignal.Provenance{Producer: "projectmodel"},
			}},
			ProjectCoverage: &projectmodel.Coverage{Phase: "full", Complete: true},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(report.Signals).NotTo(BeEmpty(), "Build must still project anchored project observations onto signals for JSON")
		Expect(report.ProjectChanges).To(HaveLen(1))

		text := RenderText(report)
		Expect(text).To(ContainSubstring("path: file_local.go"), "file-local signals must still render")
		Expect(strings.Count(text, "path: pkg/a/a.go")).To(Equal(1), "project primary path must appear once; text=\n%s", text)
		Expect(strings.Count(text, "evidence: domain imports infrastructure")).To(Equal(1), "project evidence must appear once; text=\n%s", text)
		Expect(strings.Count(text, "lifecycle:")).To(Equal(2), "one file-local + one project lifecycle line; text=\n%s", text)
		Expect(text).To(ContainSubstring("Project findings:"))
		Expect(text).To(ContainSubstring("semantic_key: cycle:pkg/a<->pkg/b"))
		Expect(text).To(ContainSubstring("related: pkg/b/b.go:5"))
		Expect(text).To(ContainSubstring("path step: package:pkg/a (pkg/a), resolution: resolved, confidence: high"))
	})
})

type recordingProjectBackend struct {
	requests []ProjectBackendRequest
	result   *ProjectBackendResult
}

func (b *recordingProjectBackend) Analyze(_ context.Context, req ProjectBackendRequest) (*ProjectBackendResult, error) {
	b.requests = append(b.requests, req)
	return b.result, nil
}

func acceptanceTempGitRepo() string {
	dir := GinkgoT().TempDir()
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "git init: %s", output)
	return dir
}

func acceptanceCommitFile(dir, name, contents string) string {
	target := filepath.Join(dir, name)
	Expect(os.MkdirAll(filepath.Dir(target), 0o755)).To(Succeed())
	Expect(os.WriteFile(target, []byte(contents), 0o644)).To(Succeed())
	addCmd := exec.Command("git", "add", name)
	addCmd.Dir = dir
	output, err := addCmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "git add: %s", output)
	commitCmd := exec.Command("git", "commit", "-m", "commit "+name)
	commitCmd.Dir = dir
	commitCmd.Env = commitTestEnv
	output, err = commitCmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "git commit: %s", output)
	revCmd := exec.Command("git", "rev-parse", "HEAD")
	revCmd.Dir = dir
	sha, err := revCmd.Output()
	Expect(err).NotTo(HaveOccurred())
	return strings.TrimSpace(string(sha))
}

var _ = Describe("project-analysis handoff into AnalyzeBaseline/AnalyzeChanges", func() {
	It("threads baseline project results into a schema-2 report and skips the seam when project is nil", func() {
		dir := acceptanceTempGitRepo()
		sha := acceptanceCommitFile(dir, "a.go", "package a\n\nfunc A() {}\n")
		files := []SelectedFile{{Path: "a.go", Language: "go", Status: "added"}}

		backend := &recordingProjectBackend{result: &ProjectBackendResult{
			HeadChanges: []codesignal.ProjectChange{{
				SemanticKey: "cycle:pkg/a<->pkg/b",
				RuleID:      "architecture.layer_violation",
				RuleVersion: "1",
				Kind:        "project_layer_violation",
				Category:    "structure",
				Severity:    "medium",
				Confidence:  "high",
				PrimaryAnchor: codesignal.ProjectLocation{
					Path:     "a.go",
					Location: semantics.Location{StartRow: 1},
				},
				Provenance: codesignal.Provenance{Producer: "fake-backend"},
			}},
			HeadCoverage: &projectmodel.Coverage{Phase: "full", Complete: true, Counts: map[string]int{"packages": 1}},
		}}
		cfg := json.RawMessage(`{"schema_version":"1","roots":["."]}`)
		project := &ProjectAnalysis{
			ConfigPath:   "project.json",
			Language:     "go",
			Config:       cfg,
			ConfigDigest: ConfigDigest(cfg),
			Backend:      backend,
		}

		withProject, err := AnalyzeBaseline(context.Background(), dir, sha, files, nil, "", codesignal.Coverage{TrackedFilesDiscovered: 1}, project)
		Expect(err).NotTo(HaveOccurred())
		Expect(withProject.SchemaVersion).To(Equal("2"))
		Expect(withProject.ProjectChanges).To(HaveLen(1))
		Expect(withProject.ProjectCoverage).NotTo(BeNil())
		Expect(withProject.ProjectCoverage.Counts["packages"]).To(Equal(1))
		Expect(withProject.Signals).To(HaveLen(1))
		Expect(backend.requests).To(HaveLen(1))
		Expect(backend.requests[0].HeadRevision).To(Equal(sha))
		Expect(backend.requests[0].Baseline).To(BeTrue())
		Expect(backend.requests[0].ConfigDigest).To(Equal(project.ConfigDigest))

		backend.requests = nil
		withoutProject, err := AnalyzeBaseline(context.Background(), dir, sha, files, nil, "", codesignal.Coverage{TrackedFilesDiscovered: 1}, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(withoutProject.SchemaVersion).To(Equal("1"))
		Expect(withoutProject.ProjectChanges).To(BeEmpty())
		Expect(backend.requests).To(BeEmpty(), "no-config runs must never invoke the project backend seam")
	})

	It("threads diff head/base project observations and config identity into the builder", func() {
		dir := acceptanceTempGitRepo()
		baseSHA := acceptanceCommitFile(dir, "a.go", "package a\n\nfunc A() {}\n")
		headSHA := acceptanceCommitFile(dir, "a.go", "package a\n\nfunc A() { println(1) }\n")
		files := []SelectedFile{{Path: "a.go", Language: "go", Status: "modified"}}

		backend := &recordingProjectBackend{result: &ProjectBackendResult{
			HeadChanges: []codesignal.ProjectChange{{
				SemanticKey: "cycle:pkg/a<->pkg/b",
				RuleID:      "architecture.layer_violation",
				RuleVersion: "1",
				Kind:        "project_layer_violation",
				PrimaryAnchor: codesignal.ProjectLocation{
					Path:     "a.go",
					Location: semantics.Location{StartRow: 1},
				},
				Provenance: codesignal.Provenance{Producer: "fake-backend"},
			}},
			BaseChanges: []codesignal.ProjectChange{{
				SemanticKey: "cycle:pkg/a<->pkg/b",
				RuleID:      "architecture.layer_violation",
				RuleVersion: "1",
				Kind:        "project_layer_violation",
				PrimaryAnchor: codesignal.ProjectLocation{
					Path:     "a.go",
					Location: semantics.Location{StartRow: 1},
				},
				Provenance: codesignal.Provenance{Producer: "fake-backend"},
			}},
			HeadCoverage: &projectmodel.Coverage{Phase: "full", Complete: true},
			BaseCoverage: &projectmodel.Coverage{Phase: "full", Complete: true},
			BaseAnalyzed: true,
		}}
		project := &ProjectAnalysis{
			ConfigPath:   "project.json",
			Language:     "go",
			Config:       json.RawMessage(`{"schema_version":"1","roots":["."]}`),
			ConfigDigest: "pcfg_test",
			Backend:      backend,
		}

		report, err := AnalyzeChanges(context.Background(), dir, headSHA, baseSHA, files, nil, "all", nil, project)
		Expect(err).NotTo(HaveOccurred())
		Expect(report.SchemaVersion).To(Equal("2"))
		Expect(report.ProjectChanges).To(HaveLen(1))
		Expect(report.ProjectChanges[0].Lifecycle).To(Equal(codesignal.Lifecycle("existing")))
		Expect(backend.requests).To(HaveLen(1))
		Expect(backend.requests[0].HeadRevision).To(Equal(headSHA))
		Expect(backend.requests[0].BaseRevision).To(Equal(baseSHA))
		Expect(backend.requests[0].ConfigDigest).To(Equal("pcfg_test"))
		Expect(backend.requests[0].Baseline).To(BeFalse())
	})

	// Proves the CLI-facing AnalyzeBaseline wiring genuinely surfaces a
	// backend's incomplete HeadCoverage rather than silently claiming
	// completeness: codesignal.Build already contracts that incomplete head
	// coverage forces Lifecycle "unknown" plus a project_lifecycle_indeterminate
	// diagnostic (see pkg/codesignal/project_contract_acceptance_test.go); this
	// asserts AnalyzeBaseline threads ProjectBackendResult.HeadCoverage into
	// that same contract instead of re-deriving or dropping it.
	It("threads an incomplete HeadCoverage into a lifecycle-indeterminate report rather than claiming baseline", func() {
		dir := acceptanceTempGitRepo()
		sha := acceptanceCommitFile(dir, "a.go", "package a\n\nfunc A() {}\n")
		files := []SelectedFile{{Path: "a.go", Language: "go", Status: "added"}}

		backend := &recordingProjectBackend{result: &ProjectBackendResult{
			HeadChanges: []codesignal.ProjectChange{{
				SemanticKey: "cycle:pkg/a<->pkg/b",
				RuleID:      "architecture.layer_violation",
				RuleVersion: "1",
				Kind:        "project_layer_violation",
				Category:    "structure",
				Severity:    "medium",
				Confidence:  "high",
				PrimaryAnchor: codesignal.ProjectLocation{
					Path:     "a.go",
					Location: semantics.Location{StartRow: 1},
				},
				Provenance: codesignal.Provenance{Producer: "fake-backend"},
			}},
			HeadCoverage: &projectmodel.Coverage{Phase: "go_model_build", Complete: false},
		}}
		cfg := json.RawMessage(`{"schema_version":"1","roots":["."]}`)
		project := &ProjectAnalysis{
			ConfigPath:   "project.json",
			Language:     "go",
			Config:       cfg,
			ConfigDigest: ConfigDigest(cfg),
			Backend:      backend,
		}

		report, err := AnalyzeBaseline(context.Background(), dir, sha, files, nil, "", codesignal.Coverage{TrackedFilesDiscovered: 1}, project)
		Expect(err).NotTo(HaveOccurred())
		Expect(report.ProjectChanges).To(HaveLen(1))
		Expect(report.ProjectChanges[0].Lifecycle).To(Equal(codesignal.Lifecycle("unknown")), "an incomplete HeadCoverage must never be reported as lifecycle baseline")
		Expect(report.ProjectSummary.BaselineChanges).To(Equal(0))
		// ProjectCoverage itself (not just the lifecycle outcome) must reach
		// the report and stay Complete:false: Lifecycle "unknown" plus
		// project_lifecycle_indeterminate alone would also fire for a
		// completely dropped (nil) HeadCoverage, so asserting those two
		// facts alone would not prove HeadCoverage was genuinely threaded
		// through rather than silently discarded.
		Expect(report.ProjectCoverage).NotTo(BeNil(), "HeadCoverage must reach the report, not be dropped as nil")
		Expect(report.ProjectCoverage.Complete).To(BeFalse())
		Expect(report.Diagnostics).To(ContainElement(HaveField("Kind", "project_coverage_incomplete")))
		Expect(report.Diagnostics).To(ContainElement(HaveField("Kind", "project_lifecycle_indeterminate")), "the CLI-facing report must surface the same indeterminate diagnostic codesignal.Build contracts for partial coverage")
	})

	// Through the real Build pipeline: incomplete project coverage with
	// no project changes and no file-local signals must reach the
	// zero-active-finding verdict, stating project analysis did not complete,
	// without claiming any path was skipped -- project_coverage_incomplete and
	// project_lifecycle_indeterminate diagnostics carry no Path.
	It("renders the no-active-findings verdict as project-incomplete, not path-skipped, for a real incomplete-coverage report", func() {
		dir := acceptanceTempGitRepo()
		sha := acceptanceCommitFile(dir, "a.go", "package a\n\nfunc A() {}\n")
		files := []SelectedFile{{Path: "a.go", Language: "go", Status: "added"}}

		backend := &recordingProjectBackend{result: &ProjectBackendResult{
			HeadCoverage: &projectmodel.Coverage{Phase: "go_model_build", Complete: false},
		}}
		cfg := json.RawMessage(`{"schema_version":"1","roots":["."]}`)
		project := &ProjectAnalysis{
			ConfigPath:   "project.json",
			Language:     "go",
			Config:       cfg,
			ConfigDigest: ConfigDigest(cfg),
			Backend:      backend,
		}

		report, err := AnalyzeBaseline(context.Background(), dir, sha, files, nil, "", codesignal.Coverage{TrackedFilesDiscovered: 1}, project)
		Expect(err).NotTo(HaveOccurred())
		Expect(report.ProjectChanges).To(BeEmpty(), "no project changes were returned by the backend")
		Expect(report.Signals).To(BeEmpty(), "the fixture file triggers no file-local finding")
		Expect(report.Diagnostics).To(ContainElement(HaveField("Kind", "project_coverage_incomplete")))

		text := RenderText(report)
		Expect(text).To(ContainSubstring("No active CodeSignal findings, but the analysis is incomplete"))
		Expect(text).To(ContainSubstring("project analysis did not complete"))
		Expect(text).NotTo(ContainSubstring("not analyzed"), "project_coverage_incomplete/project_lifecycle_indeterminate diagnostics carry no Path, so no path count must be claimed")
	})

	// Regression: Report only exposes head-side ProjectCoverage, so a report
	// whose HEAD coverage is complete but whose BASE coverage was incomplete
	// (a real, reachable diff-mode state -- see pkg/codesignal/codesignal.go's
	// projectLifecycleState, which marks lifecycle indeterminate and appends a
	// project_lifecycle_indeterminate diagnostic when the base side is
	// expected but incomplete) must still render "project analysis did not
	// complete", not the generic "additional diagnostics were recorded"
	// fallback -- the render layer must consult the diagnostic itself, not
	// only report.ProjectCoverage.Complete.
	It("renders project-incomplete for a real report whose base-side coverage was incomplete even though head coverage is complete", func() {
		dir := acceptanceTempGitRepo()
		baseSHA := acceptanceCommitFile(dir, "a.go", "package a\n\nfunc A() {}\n")
		headSHA := acceptanceCommitFile(dir, "a.go", "package a\n\n// note\nfunc A() {}\n")
		files := []SelectedFile{{Path: "a.go", Language: "go", Status: "modified"}}

		backend := &recordingProjectBackend{result: &ProjectBackendResult{
			HeadCoverage: &projectmodel.Coverage{Phase: "go_model_build", Complete: true},
			BaseCoverage: &projectmodel.Coverage{Phase: "go_model_build", Complete: false},
			BaseAnalyzed: true,
		}}
		cfg := json.RawMessage(`{"schema_version":"1","roots":["."]}`)
		project := &ProjectAnalysis{
			ConfigPath:   "project.json",
			Language:     "go",
			Config:       cfg,
			ConfigDigest: ConfigDigest(cfg),
			Backend:      backend,
		}

		report, err := AnalyzeChanges(context.Background(), dir, headSHA, baseSHA, files, nil, "all", nil, project)
		Expect(err).NotTo(HaveOccurred())
		Expect(report.ProjectChanges).To(BeEmpty(), "the backend returned no observations on either side")
		Expect(report.Signals).To(BeEmpty(), "a comment-only change triggers no file-local finding")
		Expect(report.ProjectCoverage.Complete).To(BeTrue(), "head coverage alone is complete")
		Expect(report.Diagnostics).To(ContainElement(HaveField("Kind", "project_lifecycle_indeterminate")), "base-side incompleteness must still surface as a diagnostic")

		text := RenderText(report)
		Expect(text).To(ContainSubstring("No active CodeSignal findings, but the analysis is incomplete"))
		Expect(text).To(ContainSubstring("project analysis did not complete"), "the verdict must not fall back to the generic 'additional diagnostics were recorded' clause when the diagnostic itself names project-analysis incompleteness")
	})

	// Regression: a real backend can return a ProjectChange with no
	// PrimaryAnchor.Path (filterAnchorlessProjectChanges in
	// pkg/codesignal/codesignal.go drops it and appends a pathless
	// project_observation_missing_primary_path diagnostic), while
	// ProjectCoverage is otherwise complete. That diagnostic is neither a
	// path-count cause nor a project-incompleteness cause, so it must reach
	// the render layer's generic "additional diagnostics were recorded"
	// fallback through the real Build pipeline, not only a hand-built Report.
	It("renders the generic incomplete-analysis fallback for a real report whose only diagnostic is an anchorless project observation", func() {
		dir := acceptanceTempGitRepo()
		sha := acceptanceCommitFile(dir, "a.go", "package a\n\nfunc A() {}\n")
		files := []SelectedFile{{Path: "a.go", Language: "go", Status: "added"}}

		backend := &recordingProjectBackend{result: &ProjectBackendResult{
			HeadChanges: []codesignal.ProjectChange{{
				SemanticKey: "cycle:pkg/a<->pkg/b",
				RuleID:      "architecture.layer_violation",
				Kind:        "project_layer_violation",
				Provenance:  codesignal.Provenance{Producer: "fake-backend"},
				// PrimaryAnchor deliberately left zero-value: Path == "".
			}},
			HeadCoverage: &projectmodel.Coverage{Phase: "full", Complete: true},
		}}
		cfg := json.RawMessage(`{"schema_version":"1","roots":["."]}`)
		project := &ProjectAnalysis{
			ConfigPath:   "project.json",
			Language:     "go",
			Config:       cfg,
			ConfigDigest: ConfigDigest(cfg),
			Backend:      backend,
		}

		report, err := AnalyzeBaseline(context.Background(), dir, sha, files, nil, "", codesignal.Coverage{TrackedFilesDiscovered: 1}, project)
		Expect(err).NotTo(HaveOccurred())
		Expect(report.ProjectChanges).To(BeEmpty(), "the anchorless observation must be dropped, not rendered as an active finding")
		Expect(report.Signals).To(BeEmpty(), "the fixture file triggers no file-local finding")
		Expect(report.ProjectCoverage.Complete).To(BeTrue())
		Expect(report.Diagnostics).To(ContainElement(HaveField("Kind", "project_observation_missing_primary_path")))

		text := RenderText(report)
		Expect(text).To(ContainSubstring("No active CodeSignal findings, but the analysis is incomplete: additional diagnostics were recorded."))
	})
})

// fakeTSSidecarBinary holds the compiled
// cmd/coach/testdata/fake_ts_sidecar bytes, built lazily on first use via
// buildFakeTSSidecarBinaryOnce. It is built by full module import path
// rather than a relative "./testdata/..." path so the build works
// regardless of the test binary's own working directory. The build-scratch
// dir is removed via DeferCleanup when the first spec that builds it ends,
// not at suite teardown; later specs write the cached bytes, not that path.
var (
	fakeTSSidecarBinary      []byte
	buildFakeTSSidecarBinary sync.Once
)

func ensureFakeTSSidecarBinaryBuilt() {
	buildFakeTSSidecarBinary.Do(func() {
		dir, err := os.MkdirTemp("", "fake-ts-project-sidecar-*")
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(os.RemoveAll, dir)

		binPath := filepath.Join(dir, "fake-ts-project-sidecar")
		build := exec.Command("go", "build", "-o", binPath, "github.com/lousy-agents/coach/cmd/coach/testdata/fake_ts_sidecar")
		output, err := build.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), "building fake ts project sidecar: %s", output)

		data, err := os.ReadFile(binPath)
		Expect(err).NotTo(HaveOccurred())
		fakeTSSidecarBinary = data
	})
}

// installFakeTSSidecarAt writes the compiled fake sidecar binary to
// repoDir/js/semantics/bin/coach-ts-project-sidecar -- the fixed
// tsSidecarRelativePath tsProjectBackend resolves against the repository
// root (via repositoryRoot(req.Dir)), so repoDir must be the repository
// root here.
func installFakeTSSidecarAt(repoDir string) {
	ensureFakeTSSidecarBinaryBuilt()
	binDir := filepath.Join(repoDir, "js", "semantics", "bin")
	Expect(os.MkdirAll(binDir, 0o755)).To(Succeed())
	Expect(os.WriteFile(filepath.Join(binDir, "coach-ts-project-sidecar"), fakeTSSidecarBinary, 0o755)).To(Succeed())
}

// Mutation testing showed that swapping
// filepath.Join(req.Dir, tsSidecarRelativePath) for the bare
// tsSidecarRelativePath left the whole cmd/coach acceptance suite green,
// because every existing test happens to run with the process's cwd equal
// to req.Dir. This spec calls tsProjectBackend.Analyze directly (bypassing
// the CLI's run() entrypoint) so the test process's own cwd -- this
// package's source directory -- differs from req.Dir, the temporary repo
// below (which is also that repo's own root, so repository-root-relative
// resolution still finds the sidecar here). See the "from a subdirectory
// invocation" spec below for the case where req.Dir is not the repository
// root.
var _ = Describe("tsProjectBackend sidecar binary resolution", func() {
	When("ProjectBackendRequest.Dir differs from the test process's own working directory", func() {
		It("resolves the sidecar binary relative to req.Dir, not the process cwd", func() {
			cwd, err := os.Getwd()
			Expect(err).NotTo(HaveOccurred())

			repo := acceptanceTempGitRepo()
			Expect(repo).NotTo(Equal(cwd), "the temp repo must differ from the test process's cwd for this assertion to be meaningful")

			sha := acceptanceCommitFile(repo, "a.ts", "export const a = 1;\n")
			installFakeTSSidecarAt(repo)

			cfg := json.RawMessage(`{"schema_version":"1","roots":["."]}`)
			backend := NewTSProjectBackend()

			result, err := backend.Analyze(context.Background(), ProjectBackendRequest{
				Dir:          repo,
				HeadRevision: sha,
				Baseline:     true,
				Config:       cfg,
				ConfigDigest: ConfigDigest(cfg),
				Language:     "typescript",
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(result.HeadCoverage).NotTo(BeNil())
			Expect(result.HeadCoverage.Complete).To(BeTrue(), "the sidecar must have been found and invoked via req.Dir-relative resolution")
		})
	})
})

// ProjectBackendRequest.Dir is
// os.Getwd() at CLI invocation time, not necessarily the repository
// checkout root -- coach codesignal run from a subdirectory of a repo must
// still find a sidecar vendored at the repository root's
// js/semantics/bin/coach-ts-project-sidecar. This spec sets req.Dir to a
// subdirectory of the temp repo (not the repo root itself), so only
// repository-root-anchored (not req.Dir-anchored) resolution can find it.
var _ = Describe("tsProjectBackend sidecar binary resolution from a subdirectory invocation", func() {
	When("ProjectBackendRequest.Dir is a subdirectory of the repository, not its root", func() {
		It("still finds the sidecar vendored at the repository root", func() {
			repo := acceptanceTempGitRepo()
			sha := acceptanceCommitFile(repo, "a.ts", "export const a = 1;\n")
			installFakeTSSidecarAt(repo)

			subDir := filepath.Join(repo, "sub")
			Expect(os.MkdirAll(subDir, 0o755)).To(Succeed())

			cfg := json.RawMessage(`{"schema_version":"1","roots":["."]}`)
			backend := NewTSProjectBackend()

			result, err := backend.Analyze(context.Background(), ProjectBackendRequest{
				Dir:          subDir,
				HeadRevision: sha,
				Baseline:     true,
				Config:       cfg,
				ConfigDigest: ConfigDigest(cfg),
				Language:     "typescript",
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(result.HeadCoverage).NotTo(BeNil())
			Expect(result.HeadCoverage.Complete).To(BeTrue(), "the sidecar must be found via repository-root-relative resolution regardless of invocation subdirectory")
		})
	})
})

var _ = Describe("project-config boundary budgets", func() {
	It("rejects documents that exceed the config size budget before schema decode", func() {
		oversized := []byte(`{"schema_version":"1","roots":["` + strings.Repeat("a", maxProjectConfigBytes) + `"]}`)
		err := validateProjectConfigJSON(oversized)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("size budget"))
	})

	It("validates a near-budget set of non-overlapping layer prefixes without stalling", func() {
		doc := layerPrefixConfigJSON(512)
		started := time.Now()
		err := validateProjectConfigJSON(doc)
		elapsed := time.Since(started)
		Expect(err).NotTo(HaveOccurred(), "non-overlapping prefixes within budget must validate")
		Expect(elapsed).To(BeNumerically("<", 2*time.Second), "prefix validation must stay sub-quadratic; elapsed=%s", elapsed)
	})

	// forbidden_imports entries are an explicit user claim that a layer pair
	// exists; a typo'd from/to that names no declared layer must never
	// silently validate and reach the evaluator as a permanent no-op edge.
	It("rejects forbidden_imports entries that reference an undeclared layer name", func() {
		doc := []byte(`{"schema_version":"1","roots":["."],"layers":[{"name":"handlers","prefixes":["pkg/handlers"]},{"name":"db","prefixes":["pkg/db"]}],"forbidden_imports":[{"from":"handler","to":"db"}]}`)
		err := validateProjectConfigJSON(doc)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("undefined layer"))
		Expect(err.Error()).To(ContainSubstring("handler"))
	})

	It("rejects overlapping layer prefixes with the stable diagnostic", func() {
		doc := []byte(`{"schema_version":"1","roots":["."],"layers":[{"name":"L","prefixes":["services","services/payments"]}]}`)
		err := validateProjectConfigJSON(doc)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("layer prefixes must be unique and non-overlapping"))
	})

	It("rejects layer prefix counts above the explicit budget", func() {
		err := validateProjectConfigJSON(layerPrefixConfigJSON(maxProjectConfigLayerPrefixes + 1))
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("layer prefixes exceed budget"))
	})

	It("rejects documents that exceed the JSON nesting budget", func() {
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
	})

	It("surfaces a timed-out git child as project_config_invalid", func() {
		originalRunner := runProjectConfigGit
		originalGit := gitCommandContext
		DeferCleanup(func() {
			runProjectConfigGit = originalRunner
			gitCommandContext = originalGit
		})

		gitCommandContext = func(ctx context.Context, dir string, args ...string) *exec.Cmd {
			return exec.CommandContext(ctx, "sleep", "60")
		}
		runProjectConfigGit = func(dir string, args ...string) ([]byte, error) {
			return runGitBytesBounded(dir, maxProjectConfigBytes, maxProjectConfigGitStderr, 50*time.Millisecond, args...)
		}

		_, err := LoadProjectConfig(".", "HEAD", "project.json")
		Expect(err).To(HaveOccurred())
		var cfgErr *ProjectConfigError
		Expect(err).To(BeAssignableToTypeOf(cfgErr))
		Expect(err.Error()).To(ContainSubstring("project_config_invalid"))
		Expect(err.Error()).To(ContainSubstring("timed out"))
	})

	It("accepts a required_layer that names a declared layer", func() {
		doc := []byte(`{"schema_version":"1","roots":["."],"layers":[{"name":"handlers","prefixes":["pkg/handlers"]},{"name":"db","prefixes":["pkg/db"]}],"required_layer":"db"}`)
		Expect(validateProjectConfigJSON(doc)).To(Succeed(), "a required_layer naming a declared layer must be a valid config")
	})

	It("treats an empty required_layer the same as omitting the field entirely", func() {
		withEmpty := []byte(`{"schema_version":"1","roots":["."],"required_layer":""}`)
		withoutField := []byte(`{"schema_version":"1","roots":["."]}`)
		Expect(validateProjectConfigJSON(withEmpty)).To(Succeed(), "an empty required_layer must not be treated as naming a (nonexistent) layer")
		Expect(validateProjectConfigJSON(withoutField)).To(Succeed())
	})

	// A required_layer is an explicit user claim that a declared layer exists,
	// mirroring the forbidden_imports precedent above: a typo'd name
	// that matches no declared layer must never validate cleanly and become a
	// permanent no-op for the downstream backend wiring.
	It("rejects a required_layer that references an undeclared layer name", func() {
		doc := []byte(`{"schema_version":"1","roots":["."],"layers":[{"name":"handlers","prefixes":["pkg/handlers"]}],"required_layer":"database"}`)
		err := validateProjectConfigJSON(doc)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("undefined layer"))
		Expect(err.Error()).To(ContainSubstring("database"))
	})

	It("rejects oversized git stdout without buffering unbounded output", func() {
		originalGit := gitCommandContext
		DeferCleanup(func() {
			gitCommandContext = originalGit
		})

		gitCommandContext = func(ctx context.Context, dir string, args ...string) *exec.Cmd {
			// Emit more than the tiny budget without relying on a real repo.
			return exec.CommandContext(ctx, "python3", "-c", "print('x'*100)")
		}

		_, err := runGitBytesBounded(".", 10, maxProjectConfigGitStderr, time.Second, "show", "HEAD:project.json")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("stdout exceeded"))
	})

	// Sequential stdout-then-stderr reads deadlock when the child fills the
	// stderr pipe before closing stdout. Bounded project-config I/O must
	// drain both pipes concurrently so a chatty git child cannot stall the
	// CLI until the wall-time budget fires.
	It("drains git stdout and stderr concurrently so a large stderr write cannot deadlock", func() {
		originalGit := gitCommandContext
		DeferCleanup(func() {
			gitCommandContext = originalGit
		})

		gitCommandContext = func(ctx context.Context, dir string, args ...string) *exec.Cmd {
			return exec.CommandContext(ctx, "python3", "-c", `
import sys
sys.stderr.write("e" * (1024 * 1024))
sys.stderr.flush()
sys.stdout.write("ok")
sys.stdout.flush()
`)
		}

		started := time.Now()
		data, err := runGitBytesBounded(".", 1024, 2<<20, 2*time.Second, "show", "HEAD:project.json")
		elapsed := time.Since(started)

		Expect(err).NotTo(HaveOccurred(), "concurrent pipe drain must succeed without waiting for the wall-time budget; elapsed=%s", elapsed)
		Expect(elapsed).To(BeNumerically("<", 1500*time.Millisecond), "sequential pipe reads hang until timeout; elapsed=%s", elapsed)
		Expect(string(data)).To(Equal("ok"))
	})
})

var _ = Describe("source_sink_pack config field disposition", func() {
	It("never changes which findings the real Go backend produces or their content, but does change config_digest/id/fingerprint, and README documents both halves", func() {
		dir := acceptanceTempGitRepo()
		acceptanceCommitFile(dir, "go.mod", "module example.com/app\n\ngo 1.25\n")
		acceptanceCommitFile(dir, "pkg/db/db.go", "package db\n\nvar Name = \"db\"\n")
		sha := acceptanceCommitFile(dir, "pkg/handlers/handlers.go", "package handlers\n\nimport \"example.com/app/pkg/db\"\n\nfunc Use() string {\n\treturn db.Name\n}\n")
		files := []SelectedFile{
			{Path: "pkg/db/db.go", Language: "go", Status: "added"},
			{Path: "pkg/handlers/handlers.go", Language: "go", Status: "added"},
		}

		// handlers -> db is a real forbidden_imports violation (mirrors
		// cmd/coach's goLayerPolicyConfigJSON fixture), so this run produces
		// at least one ProjectChange -- unlike an empty-result fixture, that
		// makes the comparison below capable of catching source_sink_pack
		// actually being wired to select a different sink pack.
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

			// source_sink_pack is still part of the raw config bytes
			// ConfigDigest hashes, and ConfigDigest feeds both
			// ProjectChange.ID and ProjectChange.Fingerprint (see
			// pkg/codesignal/project_fingerprint.go), so identity fields
			// must genuinely differ even though evaluation content above
			// does not.
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

		readme, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(readme)).To(ContainSubstring("`source_sink_pack`"), "README must document source_sink_pack")
		Expect(string(readme)).To(ContainSubstring("reserved field"), "README must document source_sink_pack as reserved, not a live policy knob")
		Expect(string(readme)).To(ContainSubstring("config_digest"), "README must disclose that source_sink_pack still affects config_digest/id/fingerprint")
		Expect(string(readme)).NotTo(ContainSubstring("byte-identical `coach codesignal` output"), "README must not claim source_sink_pack produces byte-identical output -- config_digest/id/fingerprint genuinely differ")
	})
})

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

func countDiagnosticsOfKind(diagnostics []codesignal.Diagnostic, kind string) int {
	count := 0
	for _, d := range diagnostics {
		if d.Kind == kind {
			count++
		}
	}
	return count
}

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

		dir := acceptanceTempGitRepo()
		acceptanceCommitFile(dir, "go.mod", "module example.com/app\n\ngo 1.25\n")
		sha := acceptanceCommitFile(dir, "pkg/handlers/handlers.go", "package handlers\n\nfunc Handler() {}\n")
		files := []SelectedFile{{Path: "pkg/handlers/handlers.go", Language: "go", Status: "added"}}

		project := &ProjectAnalysis{
			ConfigPath:   "project.json",
			Language:     "go",
			Config:       goLayerBypassSearchConfigJSON,
			ConfigDigest: ConfigDigest(goLayerBypassSearchConfigJSON),
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

		dir := acceptanceTempGitRepo()
		acceptanceCommitFile(dir, "go.mod", "module example.com/app\n\ngo 1.25\n")
		sha := acceptanceCommitFile(dir, "pkg/handlers/handlers.go", "package handlers\n\nfunc Handler() {}\n")
		files := []SelectedFile{{Path: "pkg/handlers/handlers.go", Language: "go", Status: "added"}}

		project := &ProjectAnalysis{
			ConfigPath:   "project.json",
			Language:     "go",
			Config:       goLayerBypassSearchConfigJSON,
			ConfigDigest: ConfigDigest(goLayerBypassSearchConfigJSON),
			Backend:      NewGoProjectBackend(),
		}
		report, err := AnalyzeBaseline(context.Background(), dir, sha, files, nil, "", codesignal.Coverage{TrackedFilesDiscovered: 1}, project)
		Expect(err).NotTo(HaveOccurred())

		Expect(calls).To(Equal(1), "the bypass seam must actually have been invoked, or the MaxSearchNodes assertion below is vacuous")
		// Pinned at 0; see goLayerBypassMaxSearchNodes's doc comment in
		// project_go_backend.go before making this finite.
		Expect(capturedMaxSearchNodes).To(Equal(0))
		Expect(countDiagnosticsOfKind(report.Diagnostics, "project_layer_bypass_coverage_incomplete")).To(Equal(0))
	})

	It("degrades only the base-side coverage-incomplete diagnostic to base_-prefixed and still marks the report indeterminate when only the base revision's search is incomplete", func() {
		callCount := 0
		buildGoLayerBypass = func(ctx context.Context, snapshot fs.FS, opts projectmodel.LayerBypassOptions) (projectmodel.LayerBypassResult, error) {
			callCount++
			if callCount == 1 {
				// First call is always the head revision (goProjectBackend.Analyze
				// evaluates HeadRevision before BaseRevision).
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
	})

	It("keeps head- and base-side coverage-incomplete diagnostics distinct when both revisions' searches are incomplete", func() {
		buildGoLayerBypass = func(ctx context.Context, snapshot fs.FS, opts projectmodel.LayerBypassOptions) (projectmodel.LayerBypassResult, error) {
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

		Expect(countDiagnosticsOfKind(report.Diagnostics, "project_layer_bypass_coverage_incomplete")).To(Equal(1), "the head-side incompleteness must not be collapsed into or duplicated by the base-side one")
		Expect(countDiagnosticsOfKind(report.Diagnostics, "base_project_layer_bypass_coverage_incomplete")).To(Equal(1))
	})
})
