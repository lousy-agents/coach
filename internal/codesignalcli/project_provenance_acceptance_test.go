package codesignalcli

import (
	"context"
	"encoding/json"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

var _ = Describe("applyProjectBackend dirty-worktree diagnostic and package manager provenance", func() {
	When("the worktree has relevant uncommitted changes and the project backend returns results", func() {
		It("appends a worktree_changes_not_analyzed diagnostic to input", func() {
			original := runDirtyWorktreeGit
			runDirtyWorktreeGit = func(dir string, args ...string) ([]byte, error) {
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

var _ = Describe("applyProjectBackend dirty-worktree diagnostic: error handling", func() {
	When("the git status seam returns an error", func() {
		It("still emits worktree_changes_not_analyzed rather than silently omitting the AC-VER-4 warning", func() {
			original := runDirtyWorktreeGit
			runDirtyWorktreeGit = func(dir string, args ...string) ([]byte, error) {
				return nil, errors.New("simulated git status failure")
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

			kinds := make([]string, len(input.Diagnostics))
			for i, d := range input.Diagnostics {
				kinds[i] = d.Kind
			}
			Expect(kinds).To(ContainElement(codesignal.DiagKindWorktreeChangesNotAnalyzed),
				"a git status error must conservatively emit the worktree_changes_not_analyzed diagnostic; diagnostics=%v", input.Diagnostics)
		})
	})
})

var _ = Describe("applyProjectBackend diagnostics mutation contract", func() {
	When("the backend returns no diagnostics and the worktree is dirty", func() {
		It("does not mutate the caller's Input.Diagnostics backing array when appending the worktree diagnostic", func() {
			original := runDirtyWorktreeGit
			runDirtyWorktreeGit = func(dir string, args ...string) ([]byte, error) {
				return []byte("M  bun.lock\x00"), nil
			}
			DeferCleanup(func() { runDirtyWorktreeGit = original })

			dir := acceptanceTempGitRepo()
			acceptanceCommitFile(dir, "a.ts", "export const x = 1;\n")

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
				ConfigDigest: ConfigDigest(cfg),
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

var _ = Describe("snapshotReadPackageManagerField git error handling", func() {
	When("git show fails for package.json but ls-tree and cat-file succeed", func() {
		It("returns ambiguous rather than treating the blob read failure as a clean absent field", func() {
			originalRunner := runProjectConfigGit
			runProjectConfigGit = func(dir string, args ...string) ([]byte, error) {
				if len(args) > 0 && args[0] == "show" {
					return nil, errors.New("simulated blob read failure")
				}
				return originalRunner(dir, args...)
			}
			DeferCleanup(func() { runProjectConfigGit = originalRunner })

			dir := acceptanceTempGitRepo()
			acceptanceCommitFile(dir, "package.json", `{"name":"x","packageManager":"pnpm@10.0.0"}`)
			revision := acceptanceCommitFile(dir, "package-lock.json", `{"lockfileVersion":3}`)

			kind, _, _ := snapshotPackageManagerAtRevision(context.Background(), dir, revision, []string{"."})
			Expect(kind).To(BeEmpty(),
				"a git show failure reading package.json must suppress detection rather than falling through to the npm lockfile kind")
		})
	})
})

var _ = Describe("snapshotDetectLockfileAtRoot git error handling", func() {
	When("fileExistsAtRevision returns an error for a lockfile basename", func() {
		It("returns ambiguous rather than treating the error as a clean absent lockfile", func() {
			originalRunner := runProjectConfigGit
			runProjectConfigGit = func(dir string, args ...string) ([]byte, error) {
				// Fail ls-tree calls so fileExistsAtRevision errors on lockfile checks.
				if len(args) > 0 && args[0] == "ls-tree" {
					return nil, errors.New("simulated transient git failure")
				}
				return originalRunner(dir, args...)
			}
			DeferCleanup(func() { runProjectConfigGit = originalRunner })

			dir := acceptanceTempGitRepo()
			revision := acceptanceCommitFile(dir, "package.json", `{"name":"x","packageManager":"npm@11.0.0"}`)
			acceptanceCommitFile(dir, "package-lock.json", `{"lockfileVersion":3}`)

			kind, _, _ := snapshotPackageManagerAtRevision(context.Background(), dir, revision, []string{"."})
			Expect(kind).To(BeEmpty(),
				"a transient git error during lockfile detection must suppress detection rather than returning a potentially wrong kind")
		})
	})
})

var _ = Describe("snapshotPackageManagerAtRevision context threading", func() {
	When("a supported package manager lockfile is committed", func() {
		It("passes the scan context to the version probe rather than context.Background", func() {
			scanCtx, scanCancel := context.WithCancel(context.Background())
			DeferCleanup(scanCancel)

			var capturedCtx context.Context
			originalProbe := snapshotProbePackageManagerVersion
			snapshotProbePackageManagerVersion = func(ctx context.Context, kind string) (string, bool) {
				capturedCtx = ctx
				return "", false
			}
			DeferCleanup(func() { snapshotProbePackageManagerVersion = originalProbe })

			dir := acceptanceTempGitRepo()
			revision := acceptanceCommitFile(dir, "package-lock.json", `{"lockfileVersion":3}`)

			snapshotPackageManagerAtRevision(scanCtx, dir, revision, []string{"."})

			Expect(capturedCtx).NotTo(BeNil(), "probe must have been called")
			Expect(capturedCtx).To(BeIdenticalTo(scanCtx),
				"probe must receive the scan ctx, not a detached context.Background()")
		})
	})
})
