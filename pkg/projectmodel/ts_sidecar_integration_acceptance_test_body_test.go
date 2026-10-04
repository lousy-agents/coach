package projectmodel_test

import (
	"context"

	"fmt"
	"path/filepath"
	"runtime"

	"strings"
	"testing/fstest"
	"time"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func body_tsSidecarIntegrationAcceptanceTest_47(sidecarPath string) projectmodel.TSSidecarOptions {
	compilerModule := filepath.Join(jsSemanticsRoot(), "node_modules", "typescript")
	arch := runtime.GOARCH
	switch arch {
	case "amd64":
		arch = "x64"
	case "386":
		arch = "ia32"
	}
	nativePackage := filepath.Join(jsSemanticsRoot(), "node_modules", "@typescript", fmt.Sprintf("typescript-%s-%s", runtime.GOOS, arch))
	return projectmodel.TSSidecarOptions{
		BinaryPath: sidecarPath,
		Args:       []string{"--compiler-module=" + compilerModule, "--native-package=" + nativePackage},
		Timeout:    20 * time.Second,
	}
}

func body_tsSidecarIntegrationAcceptanceTest_reportsARealRootScopeCandidateAnalyzedMismatchTh_183(ctx context.Context, realOpts func() projectmodel.TSSidecarOptions) {

	snapshot := fstest.MapFS{
		"tsconfig.json": tsconfigJSON(map[string]any{
			"compilerOptions": map[string]any{
				"module": "commonjs", "moduleResolution": "node10", "resolveJsonModule": true,
			},
			"files": []string{"package.json", "src/a.ts"},
		}),
		"package.json": tsconfigJSON(map[string]any{"name": "fixture"}),
		"src/a.ts":     file("export const a = 1;\n"),
	}
	opts := realOpts()
	opts.Roots = []string{"."}

	model, err := projectmodel.BuildTypeScriptModelViaSidecar(ctx, snapshot, testMeta(), opts)
	Expect(err).NotTo(HaveOccurred())

	var rootScope projectmodel.RootScope
	found := false
	for _, rs := range model.RootScopes {
		if rs.Root == "." {
			rootScope, found = rs, true
		}
	}
	Expect(found).To(BeTrue(), "expected a root_scopes entry for \".\", got %+v", model.RootScopes)
	Expect(rootScope.AnalyzedFiles).To(BeNumerically("<", rootScope.CandidateFiles), "expected package.json to be counted as a candidate root file but never analyzed, got %+v", rootScope)

	Expect(model.Coverage.Complete).To(BeFalse(), "expected the real candidate/analyzed mismatch to mark model coverage incomplete per SA-280-025, got %+v", model.Coverage)
	Expect(rootScope.UnanalyzedPaths).To(ConsistOf("package.json"), "expected package.json identified by path as the one candidate never analyzed, got %+v", rootScope)
	diag, ok := diagnosticWithCode(model.Coverage.Diagnostics, projectmodel.DiagRootScopeIncomplete)
	Expect(ok).To(BeTrue(), "expected a root-scope-incomplete diagnostic, got %+v", model.Coverage.Diagnostics)
	Expect(diag.Path).To(Equal("package.json"), "expected the diagnostic's Path to name the specific unanalyzed file, not the root")
}

func body_tsSidecarIntegrationAcceptanceTest_producesThreeEdgesWithThreeDistinctKindValues_329(ctx context.Context, realOpts func() projectmodel.TSSidecarOptions) {
	snapshot := fstest.MapFS{
		"tsconfig.json": tsconfigJSON(map[string]any{
			"compilerOptions": map[string]any{"module": "commonjs", "moduleResolution": "node10"},
		}),
		"src/a.ts": file(strings.Join([]string{
			`const cjs = require("./cjs-target");`,
			`import type { T } from "./type-only-target";`,
			`async function f(): Promise<void> {`,
			`  const dyn = await import("./dynamic-target");`,
			`  console.log(cjs, dyn);`,
			`}`,
			`export type UsesT = T;`,
			``,
		}, "\n")),
		"src/cjs-target.ts":       file("export const cjsValue = 1;\n"),
		"src/dynamic-target.ts":   file("export const dynValue = 1;\n"),
		"src/type-only-target.ts": file("export type T = number;\n"),
	}

	model, err := projectmodel.BuildTypeScriptModelViaSidecar(ctx, snapshot, testMeta(), realOpts())
	Expect(err).NotTo(HaveOccurred())
	Expect(model.Coverage.Complete).To(BeTrue(), "%+v", model.Coverage)

	cjsEdge, ok := edgeByTo(model.ImportEdges, "file:src/cjs-target.ts")
	Expect(ok).To(BeTrue(), "%+v", model.ImportEdges)
	Expect(cjsEdge.Kind).To(Equal("commonjs_require"))

	dynEdge, ok := edgeByTo(model.ImportEdges, "file:src/dynamic-target.ts")
	Expect(ok).To(BeTrue(), "%+v", model.ImportEdges)
	Expect(dynEdge.Kind).To(Equal("dynamic_import"))

	typeEdge, ok := edgeByTo(model.ImportEdges, "file:src/type-only-target.ts")
	Expect(ok).To(BeTrue(), "%+v", model.ImportEdges)
	Expect(typeEdge.Kind).To(Equal("type_only"))

	kinds := map[string]struct{}{}
	for _, e := range model.ImportEdges {
		kinds[e.Kind] = struct{}{}
	}
	Expect(kinds).To(HaveLen(3), "expected 3 distinct kinds, got %+v", kinds)
}

func body_tsSidecarIntegrationAcceptanceTest_classifiesTheEdgeTypeOnlyNotAValueImportAndAMixe_374(ctx context.Context, realOpts func() projectmodel.TSSidecarOptions) {
	snapshot := fstest.MapFS{
		"tsconfig.json": tsconfigJSON(map[string]any{
			"compilerOptions": map[string]any{"module": "commonjs", "moduleResolution": "node10"},
		}),
		"src/a.ts": file("import { type T } from \"./b\";\nexport type UsesT = T;\n"),
		"src/b.ts": file("export type T = number;\nexport const v = 1;\n"),
		"src/c.ts": file("import { type T, v } from \"./b\";\nexport type UsesT = T;\nconsole.log(v);\n"),
	}

	model, err := projectmodel.BuildTypeScriptModelViaSidecar(ctx, snapshot, testMeta(), realOpts())
	Expect(err).NotTo(HaveOccurred())
	Expect(model.Coverage.Complete).To(BeTrue(), "%+v", model.Coverage)

	var pureTypeOnly, mixed []projectmodel.ImportEdge
	for _, e := range model.ImportEdges {
		if e.To != "file:src/b.ts" {
			continue
		}
		switch e.From {
		case "file:src/a.ts":
			pureTypeOnly = append(pureTypeOnly, e)
		case "file:src/c.ts":
			mixed = append(mixed, e)
		}
	}
	Expect(pureTypeOnly).To(HaveLen(1), "%+v", model.ImportEdges)
	Expect(pureTypeOnly[0].Kind).To(Equal("type_only"), "expected `import { type T } from \"./b\"` to classify as type_only, got %+v", pureTypeOnly[0])

	Expect(mixed).To(HaveLen(1), "%+v", model.ImportEdges)
	Expect(mixed[0].Kind).To(Equal("import"), "expected `import { type T, v } from \"./b\"` to classify as a value import, got %+v", mixed[0])
}
