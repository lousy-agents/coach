package codesignalcli

import (
	"context"
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

var _ = Describe("applyProjectBackend dirty-worktree diagnostic and package manager provenance", func() {
	When("the worktree has relevant uncommitted changes and the project backend returns results", func() {
		It("appends a worktree_changes_not_analyzed diagnostic to input", func() {
			// Replace the git seam to report a dirty relevant path.
			original := runDirtyWorktreeGit
			runDirtyWorktreeGit = func(dir string, args ...string) ([]byte, error) {
				// Simulate `git status --porcelain=v1 -z` output: one modified bun.lock
				return []byte("M  bun.lock\x00"), nil
			}
			DeferCleanup(func() { runDirtyWorktreeGit = original })

			dir := acceptanceTempGitRepo()
			acceptanceCommitFile(dir, "a.go", "package a\n")

			backend := identityHandoffBackend{result: &ProjectBackendResult{
				RuntimeKind:   runtimeKindNode,
				RuntimeOrigin: runtimeOriginPath,
			}}
			cfg := json.RawMessage(`{"schema_version":"1","roots":["."]}`)
			project := &ProjectAnalysis{
				ConfigPath:   "project.json",
				Language:     "typescript",
				Config:       cfg,
				ConfigDigest: ConfigDigest(cfg),
				Backend:      backend,
			}

			input, _, err := applyProjectBackend(context.Background(), codesignal.Input{}, codesignal.Options{}, project, dir, "HEAD", "", true)
			Expect(err).NotTo(HaveOccurred())

			kinds := make([]string, len(input.Diagnostics))
			for i, d := range input.Diagnostics {
				kinds[i] = d.Kind
			}
			Expect(kinds).To(ContainElement(codesignal.DiagKindWorktreeChangesNotAnalyzed),
				"a dirty relevant path must trigger the worktree_changes_not_analyzed diagnostic; diagnostics=%v", input.Diagnostics)
		})

		It("worktree_changes_not_analyzed message references committed HEAD", func() {
			original := runDirtyWorktreeGit
			runDirtyWorktreeGit = func(dir string, args ...string) ([]byte, error) {
				return []byte("?? bun.lock\x00"), nil
			}
			DeferCleanup(func() { runDirtyWorktreeGit = original })

			dir := acceptanceTempGitRepo()
			acceptanceCommitFile(dir, "a.ts", "export const x = 1;\n")

			backend := identityHandoffBackend{result: &ProjectBackendResult{}}
			cfg := json.RawMessage(`{"schema_version":"1","roots":["."]}`)
			project := &ProjectAnalysis{
				ConfigPath:   "project.json",
				Language:     "typescript",
				Config:       cfg,
				ConfigDigest: ConfigDigest(cfg),
				Backend:      backend,
			}

			input, _, err := applyProjectBackend(context.Background(), codesignal.Input{}, codesignal.Options{}, project, dir, "HEAD", "", true)
			Expect(err).NotTo(HaveOccurred())

			var msg string
			for _, d := range input.Diagnostics {
				if d.Kind == codesignal.DiagKindWorktreeChangesNotAnalyzed {
					msg = d.Message
				}
			}
			Expect(msg).NotTo(BeEmpty(), "diagnostic message must be set")
			Expect(msg).To(ContainSubstring("HEAD"), "message must reference committed HEAD; got %q", msg)
		})

		It("does not emit worktree_changes_not_analyzed when the worktree is clean", func() {
			original := runDirtyWorktreeGit
			runDirtyWorktreeGit = func(dir string, args ...string) ([]byte, error) {
				return []byte{}, nil
			}
			DeferCleanup(func() { runDirtyWorktreeGit = original })

			dir := acceptanceTempGitRepo()
			acceptanceCommitFile(dir, "a.ts", "export const x = 1;\n")

			backend := identityHandoffBackend{result: &ProjectBackendResult{}}
			cfg := json.RawMessage(`{"schema_version":"1","roots":["."]}`)
			project := &ProjectAnalysis{
				ConfigPath:   "project.json",
				Language:     "typescript",
				Config:       cfg,
				ConfigDigest: ConfigDigest(cfg),
				Backend:      backend,
			}

			input, _, err := applyProjectBackend(context.Background(), codesignal.Input{}, codesignal.Options{}, project, dir, "HEAD", "", true)
			Expect(err).NotTo(HaveOccurred())

			for _, d := range input.Diagnostics {
				Expect(d.Kind).NotTo(Equal(codesignal.DiagKindWorktreeChangesNotAnalyzed),
					"clean worktree must not trigger the diagnostic")
			}
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
