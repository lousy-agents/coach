package codesignalcli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
	"github.com/lousy-agents/coach/internal/tstestutil"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Mutation testing showed that swapping filepath.Join(req.Dir, ...) for a
// bare repository-root-relative lookup left the whole cmd/coach acceptance
// suite green, because every existing test happens to run with the
// process's cwd equal to req.Dir. This spec calls tsProjectBackend.Analyze
// directly (bypassing the CLI's run() entrypoint) so the test process's own
// cwd differs from req.Dir.
var _ = Describe("tsProjectBackend compiler resolution", Label("ts-project-backend"), func() {
	BeforeEach(func() {
		body_projectAcceptanceTest_436()
	})

	When("ProjectBackendRequest.Dir differs from the test process's own working directory", func() {
		It("resolves the TypeScript compiler relative to req.Dir's repository root, not the process cwd", func() {
			cwd, err := os.Getwd()
			Expect(err).NotTo(HaveOccurred())

			repo := gitfixture.Init(GinkgoT())
			Expect(repo).NotTo(Equal(cwd), "the temp repo must differ from the test process's cwd for this assertion to be meaningful")

			version := tsAcceptanceRealTypescriptVersion()
			gitfixture.CommitFile(GinkgoT(), repo, "package.json", fmt.Sprintf(`{"devDependencies":{"typescript":%q}}`, version))
			gitfixture.CommitFile(GinkgoT(), repo, "tsconfig.json", `{"compilerOptions":{"module":"commonjs","moduleResolution":"node10"}}`)
			sha := gitfixture.CommitFile(GinkgoT(), repo, "a.ts", "export const a = 1;\n")
			installRealTypescriptCompilerAt(repo)

			cfg := json.RawMessage(`{"schema_version":"1","roots":["."]}`)
			backend := NewTSProjectBackend()

			result, err := backend.Analyze(context.Background(), ProjectBackendRequest{
				Dir:          repo,
				HeadRevision: sha,
				Baseline:     true,
				Config:       cfg,
				ConfigDigest: projectconfig.Digest(cfg),
				Language:     "typescript",
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(result.HeadCoverage).NotTo(BeNil())
			Expect(result.HeadCoverage.Complete).To(BeTrue(), "the compiler must have been resolved and the analyzer invoked via req.Dir's repository-root-relative resolution")
			Expect(result.RuntimeKind).To(Equal(runtimeKindNode))
			Expect(result.RuntimeVersion).To(MatchRegexp(`^v?\d+\.\d+\.\d+`))
			Expect(result.RuntimeOrigin).To(Equal(runtimeOriginPath))
			Expect(result.CompilerVersion).To(Equal(version))
			Expect(result.CompilerOrigin).To(Equal(tstoolchain.OriginProject))
		})
	})
})

var _ = Describe("tsProjectBackend compiler resolution from a subdirectory invocation", Label("ts-project-backend"), func() {
	BeforeEach(func() {
		body_projectAcceptanceTest_481()
	})

	When("ProjectBackendRequest.Dir is a subdirectory of the repository, not its root", func() {
		It("still resolves the compiler declared and installed at the repository root", func() {
			repo := gitfixture.Init(GinkgoT())
			version := tsAcceptanceRealTypescriptVersion()
			gitfixture.CommitFile(GinkgoT(), repo, "package.json", fmt.Sprintf(`{"devDependencies":{"typescript":%q}}`, version))
			gitfixture.CommitFile(GinkgoT(), repo, "tsconfig.json", `{"compilerOptions":{"module":"commonjs","moduleResolution":"node10"}}`)
			sha := gitfixture.CommitFile(GinkgoT(), repo, "a.ts", "export const a = 1;\n")
			installRealTypescriptCompilerAt(repo)

			subDir := filepath.Join(repo, "sub")
			Expect(os.MkdirAll(subDir, 0o755)).To(Succeed())

			cfg := json.RawMessage(`{"schema_version":"1","roots":["."]}`)
			backend := NewTSProjectBackend()

			result, err := backend.Analyze(context.Background(), ProjectBackendRequest{
				Dir:          subDir,
				HeadRevision: sha,
				Baseline:     true,
				Config:       cfg,
				ConfigDigest: projectconfig.Digest(cfg),
				Language:     "typescript",
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(result.HeadCoverage).NotTo(BeNil())
			Expect(result.HeadCoverage.Complete).To(BeTrue(), "the compiler must be resolved via repository-root-relative resolution regardless of invocation subdirectory")
		})
	})
})

var _ = Describe("tsProjectBackend compiler resolution from a nested TypeScript project", Label("ts-project-backend"), func() {
	BeforeEach(func() {
		body_projectAcceptanceTest_519()
	})

	When("the repository has no top-level package.json and the policy names a nested js/semantics-shaped root that pins an exact installed compiler", func() {
		It("resolves that nested manifest's compiler and produces a complete analysis rather than failing with no locatable compiler", func() {
			repo := gitfixture.Init(GinkgoT())
			version := tsAcceptanceRealTypescriptVersion()
			gitfixture.CommitFile(GinkgoT(), repo, "js/semantics/package.json", fmt.Sprintf(`{"devDependencies":{"typescript":%q}}`, version))
			gitfixture.CommitFile(GinkgoT(), repo, "js/semantics/tsconfig.json", `{"compilerOptions":{"module":"commonjs","moduleResolution":"node10"}}`)
			sha := gitfixture.CommitFile(GinkgoT(), repo, "js/semantics/a.ts", "export const a = 1;\n")
			installRealTypescriptCompilerAt(filepath.Join(repo, "js", "semantics"))

			_, statErr := os.Stat(filepath.Join(repo, "package.json"))
			Expect(os.IsNotExist(statErr)).To(BeTrue(), "nested-only fixture must not have a top-level package.json; that would exercise the already-green worktree-top origin")

			cfg := json.RawMessage(`{"schema_version":"1","roots":["js/semantics"]}`)
			backend := NewTSProjectBackend()

			result, err := backend.Analyze(context.Background(), ProjectBackendRequest{
				Dir:          repo,
				HeadRevision: sha,
				Baseline:     true,
				Config:       cfg,
				ConfigDigest: projectconfig.Digest(cfg),
				Language:     "typescript",
			})

			Expect(err).NotTo(HaveOccurred(), "nested js/semantics-shaped analysis must locate the compiler from the selected root's manifest, not fail with no locatable compiler")
			Expect(result.HeadCoverage).NotTo(BeNil())
			Expect(result.HeadCoverage.Complete).To(BeTrue(), "the compiler must be resolved from the nested project's package.json")
			Expect(result.CompilerVersion).To(Equal(version))
			Expect(result.CompilerOrigin).To(Equal(tstoolchain.OriginProject))
		})
	})
})

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

func tsAcceptanceRealTypescriptVersion() string {
	return tstestutil.TypeScriptVersion()
}

func ensureRealTypeScriptCompilerAvailable() string {
	return tstestutil.EnsureTypeScriptCompilerAvailable()
}

func installRealTypescriptCompilerAt(repoDir string) {
	tstestutil.InstallTypeScriptCompiler(repoDir, true)
}
