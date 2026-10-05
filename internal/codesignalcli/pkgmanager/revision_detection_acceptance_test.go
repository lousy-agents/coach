package pkgmanager

import (
	"context"
	"errors"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("snapshotReadPackageManagerField git error handling", func() {
	When("git show fails for package.json but ls-tree and cat-file succeed", func() {
		It("returns ambiguous rather than treating the blob read failure as a clean absent field", func() {
			originalRunner := runPackageManagerRevisionGit
			runPackageManagerRevisionGit = func(dir string, args ...string) ([]byte, error) {
				return body_projectProvenanceAcceptanceTest_234(dir, args, originalRunner)
			}
			DeferCleanup(func() { runPackageManagerRevisionGit = originalRunner })

			dir := gitfixture.Init(GinkgoT())
			gitfixture.CommitFile(GinkgoT(), dir, "package.json", `{"name":"x","packageManager":"pnpm@10.0.0"}`)
			revision := gitfixture.CommitFile(GinkgoT(), dir, "package-lock.json", `{"lockfileVersion":3}`)

			kind, _, _ := AtRevision(context.Background(), dir, revision, []string{"."})
			Expect(kind).To(BeEmpty(),
				"a git show failure reading package.json must suppress detection rather than falling through to the npm lockfile kind")
		})
	})
})

var _ = Describe("snapshotDetectLockfileAtRoot git error handling", func() {
	When("fileExistsAtRevision returns an error for a lockfile basename", func() {
		It("returns ambiguous rather than treating the error as a clean absent lockfile", func() {
			originalRunner := runPackageManagerRevisionGit
			runPackageManagerRevisionGit = func(dir string, args ...string) ([]byte, error) {
				return body_projectProvenanceAcceptanceTest_257(dir, args, originalRunner)
			}
			DeferCleanup(func() { runPackageManagerRevisionGit = originalRunner })

			dir := gitfixture.Init(GinkgoT())
			revision := gitfixture.CommitFile(GinkgoT(), dir, "package.json", `{"name":"x","packageManager":"npm@11.0.0"}`)
			gitfixture.CommitFile(GinkgoT(), dir, "package-lock.json", `{"lockfileVersion":3}`)

			kind, _, _ := AtRevision(context.Background(), dir, revision, []string{"."})
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

			dir := gitfixture.Init(GinkgoT())
			revision := gitfixture.CommitFile(GinkgoT(), dir, "package-lock.json", `{"lockfileVersion":3}`)

			AtRevision(scanCtx, dir, revision, []string{"."})

			Expect(capturedCtx).NotTo(BeNil(), "probe must have been called")
			Expect(capturedCtx).To(BeIdenticalTo(scanCtx),
				"probe must receive the scan ctx, not a detached context.Background()")
		})
	})
})

func body_projectProvenanceAcceptanceTest_234(dir string, args []string, originalRunner func(dir string, args ...string) ([]byte, error)) ([]byte, error) {
	if len(args) > 0 && args[0] == "show" {
		return nil, errors.New("simulated blob read failure")
	}
	return originalRunner(dir, args...)
}

func body_projectProvenanceAcceptanceTest_257(dir string, args []string, originalRunner func(dir string, args ...string) ([]byte, error)) ([]byte, error) {
	// Fail ls-tree calls so gitrepo.FileExistsAtRevision errors on lockfile checks.
	if len(args) > 0 && args[0] == "ls-tree" {
		return nil, errors.New("simulated transient git failure")
	}
	return originalRunner(dir, args...)
}
