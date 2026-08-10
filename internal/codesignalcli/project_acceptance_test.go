package codesignalcli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
})

// F-002: typed project handoff must reach the builder for baseline and diff
// when a backend is selected, and must not run when project mode is omitted.
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
	Expect(os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o644)).To(Succeed())
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

		withProject, err := AnalyzeBaseline(context.Background(), dir, sha, files, nil, codesignal.Coverage{TrackedFilesDiscovered: 1}, project)
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
		withoutProject, err := AnalyzeBaseline(context.Background(), dir, sha, files, nil, codesignal.Coverage{TrackedFilesDiscovered: 1}, nil)
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
})

var _ = Describe("project-config boundary budgets", func() {
	It("rejects documents that exceed the config size budget before schema decode", func() {
		oversized := []byte(`{"schema_version":"1","roots":["` + strings.Repeat("a", maxProjectConfigBytes) + `"]}`)
		err := validateProjectConfigJSON(oversized)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("size budget"))
	})

	// F-006: many non-overlapping prefixes must validate without a long CPU stall
	// and exact overlap diagnostics must remain deterministic.
	It("validates a near-budget set of non-overlapping layer prefixes without stalling", func() {
		doc := layerPrefixConfigJSON(512)
		started := time.Now()
		err := validateProjectConfigJSON(doc)
		elapsed := time.Since(started)
		Expect(err).NotTo(HaveOccurred(), "non-overlapping prefixes within budget must validate")
		Expect(elapsed).To(BeNumerically("<", 2*time.Second), "prefix validation must stay sub-quadratic; elapsed=%s", elapsed)
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
