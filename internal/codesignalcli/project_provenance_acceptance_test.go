package codesignalcli

import (
	"context"
	"encoding/json"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/pkg/codesignal"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("applyProjectBackend dirty-worktree diagnostic and package manager provenance", func() {
	When("the worktree has relevant uncommitted changes and the project backend returns results", func() {
		It("appends a worktree_report_reflects_committed_head diagnostic to input", func() {
			body_projectProvenanceAcceptanceTest_appendsAWorktreeReportReflectsCommittedHeadDiagn_16()
		})

		It("worktree_report_reflects_committed_head message references committed HEAD", func() {
			body_projectProvenanceAcceptanceTest_worktreeReportReflectsCommittedHeadMessageRefere_52()
		})

		It("does not emit a worktree provenance diagnostic when the worktree is clean", func() {
			body_projectProvenanceAcceptanceTest_doesNotEmitAWorktreeProvenanceDiagnosticWhenTheW_85()
		})
	})

	When("the backend result carries analyzer identity fields", func() {
		It("copies AnalyzerVersion and AnalyzerDigest from ProjectBackendResult onto codesignal.Input", func() {
			backend := identityHandoffBackend{result: &ProjectBackendResult{
				AnalyzerVersion: "coach-ts-project-sidecar",
				AnalyzerDigest:  "sha256:abc123",
			}}
			input, _, err := applyProjectBackend(context.Background(), codesignal.Input{}, codesignal.Options{}, &ProjectAnalysis{
				Backend:  backend,
				Language: "typescript",
				Config:   json.RawMessage(`{"schema_version":"1","roots":["."]}`)},
				".", "HEAD", "", true)
			Expect(err).NotTo(HaveOccurred())
			Expect(input.AnalyzerVersion).To(Equal("coach-ts-project-sidecar"))
			Expect(input.AnalyzerDigest).To(Equal("sha256:abc123"))
		})

		It("copies PackageManager* fields from ProjectBackendResult onto codesignal.Input", func() {
			backend := identityHandoffBackend{result: &ProjectBackendResult{
				PackageManagerKind:    "npm",
				PackageManagerVersion: "11.0.0",
				PackageManagerOrigin:  "lockfile",
			}}
			input, _, err := applyProjectBackend(context.Background(), codesignal.Input{}, codesignal.Options{}, &ProjectAnalysis{
				Backend:  backend,
				Language: "typescript",
				Config:   json.RawMessage(`{"schema_version":"1","roots":["."]}`)},
				".", "HEAD", "", true)
			Expect(err).NotTo(HaveOccurred())
			Expect(input.PackageManagerKind).To(Equal("npm"))
			Expect(input.PackageManagerVersion).To(Equal("11.0.0"))
			Expect(input.PackageManagerOrigin).To(Equal("lockfile"))
		})
	})
})

var _ = Describe("applyProjectBackend dirty-worktree diagnostic: error handling", func() {
	When("the git status seam returns an error", func() {
		It("emits worktree_status_check_failed rather than describing the error as skipped analysis", func() {
			body_projectProvenanceAcceptanceTest_emitsWorktreeStatusCheckFailedRatherThanDescribi_154()
		})
	})
})

var _ = Describe("applyProjectBackend diagnostics mutation contract", func() {
	When("the backend returns no diagnostics and the worktree is dirty", func() {
		It("does not mutate the caller's Input.Diagnostics backing array when appending the worktree diagnostic", func() {
			original := listWorktreeStatus
			listWorktreeStatus = func(string) ([]gitrepo.WorktreeEntry, error) {
				return gitrepo.ParseWorktreeStatus([]byte("M  bun.lock\x00")), nil
			}
			DeferCleanup(func() { listWorktreeStatus = original })

			dir := gitfixture.Init(GinkgoT())
			gitfixture.CommitFile(GinkgoT(), dir, "a.ts", "export const x = 1;\n")

			existing := codesignal.Diagnostic{Kind: "pre_existing", Message: "must survive"}
			// Spare capacity lets append write into the backing array if it aliases.
			callerSlice := make([]codesignal.Diagnostic, 1, 4)
			callerSlice[0] = existing

			backend := identityHandoffBackend{result: &ProjectBackendResult{}}
			cfg := json.RawMessage(`{"schema_version":"1","roots":["."]}`)
			project := &ProjectAnalysis{
				ConfigPath:   "project.json",
				Language:     "typescript",
				Config:       cfg,
				ConfigDigest: projectconfig.Digest(cfg),
				Backend:      backend,
			}

			input := codesignal.Input{Diagnostics: callerSlice}
			returned, _, err := applyProjectBackend(context.Background(), input, codesignal.Options{}, project, dir, "HEAD", "", true)
			Expect(err).NotTo(HaveOccurred())

			// Expose the backing array beyond callerSlice's len to see whether
			// applyProjectBackend's append wrote into it.
			backing := callerSlice[:cap(callerSlice)]
			Expect(backing[1].Kind).To(BeEmpty(),
				"backing array slot 1 must not have been written by applyProjectBackend; in-place mutation contradicts the no-mutation contract at project_analysis.go:110")
			Expect(returned.Diagnostics).To(HaveLen(2), "returned diagnostics must include both the pre-existing entry and the worktree diagnostic")
		})
	})
})
