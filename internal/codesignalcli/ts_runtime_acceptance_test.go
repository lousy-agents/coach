package codesignalcli

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("PrepareTSRuntime resolved-runtime provenance", Label("ts-project-backend"), func() {
	BeforeEach(func() {
		body_projectAcceptanceTest_559()
	})

	It("records node version, compiler version, compiler origin, and the materialized analyzer directory on the prepared runtime", func() {
		repo := gitfixture.Init(GinkgoT())
		version := tsAcceptanceRealTypescriptVersion()
		gitfixture.CommitFile(GinkgoT(), repo, "package.json", fmt.Sprintf(`{"devDependencies":{"typescript":%q}}`, version))
		gitfixture.CommitFile(GinkgoT(), repo, "tsconfig.json", `{"compilerOptions":{"module":"commonjs","moduleResolution":"node10"}}`)
		gitfixture.CommitFile(GinkgoT(), repo, "a.ts", "export const a = 1;\n")
		installRealTypescriptCompilerAt(repo)

		rt, cleanup, err := PrepareTSRuntime(context.Background(), repo, nil)
		Expect(err).NotTo(HaveOccurred())
		defer cleanup()

		Expect(rt.ExecPath).NotTo(BeEmpty())
		Expect(filepath.IsAbs(rt.ExecPath)).To(BeTrue())
		Expect(rt.ExecArgs).To(Equal([]string{
			rt.AnalyzerShimPath,
			"--compiler-module=" + rt.CompilerModulePath,
			"--native-package=" + rt.NativePackagePath,
		}))
		Expect(rt.Version).NotTo(BeEmpty())
		Expect(rt.Version).To(MatchRegexp(`^v?\d+\.\d+\.\d+`), "expected a real `node --version` output, got %q", rt.Version)
		Expect(rt.Kind).To(Equal(runtimeKindNode))
		Expect(rt.Origin).To(Equal(runtimeOriginPath))
		Expect(rt.CompilerVersion).To(Equal(version), "CompilerVersion must match the installed compiler's own package.json version")
		Expect(rt.CompilerOrigin).To(Equal(tstoolchain.OriginProject), "the manifest/installed-version match must resolve via the project origin")
		wantCompiler, err := filepath.EvalSymlinks(filepath.Join(repo, "node_modules", "typescript"))
		Expect(err).NotTo(HaveOccurred())
		gotCompiler, err := filepath.EvalSymlinks(rt.CompilerModulePath)
		Expect(err).NotTo(HaveOccurred())
		Expect(gotCompiler).To(Equal(wantCompiler))
		Expect(rt.AnalyzerDir).NotTo(BeEmpty())
		Expect(rt.AnalyzerShimPath).To(HavePrefix(rt.AnalyzerDir))
		wantNative, err := filepath.EvalSymlinks(filepath.Join(repo, "node_modules", "@typescript", tstoolchain.NativeTypescriptUnscopedName()))
		Expect(err).NotTo(HaveOccurred())
		gotNative, err := filepath.EvalSymlinks(rt.NativePackagePath)
		Expect(err).NotTo(HaveOccurred())
		Expect(gotNative).To(Equal(wantNative))
		Expect(tstoolchain.NativeTypescriptPackageName()).To(Equal("@typescript/" + tstoolchain.NativeTypescriptUnscopedName()))
	})
})

func body_projectAcceptanceTest_559() {
	if reason := ensureRealTypeScriptCompilerAvailable(); reason != "" {
		Skip(reason)
	}
}
