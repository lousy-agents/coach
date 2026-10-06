package projectmodel_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing/fstest"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func expectRootScopeMismatchMarksModelIncomplete(ctx context.Context, realOpts func() projectmodel.TSSidecarOptions) {

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

func expectModelFilesCoverEveryAnalyzedPath(ctx context.Context, realOpts func() projectmodel.TSSidecarOptions) {
	snapshot := fstest.MapFS{
		"tsconfig.json": tsconfigJSON(map[string]any{
			"compilerOptions": map[string]any{
				"module": "commonjs", "moduleResolution": "node10",
				"baseUrl": ".", "paths": map[string]any{"@lib/*": []string{"src/lib/*"}},
			},
		}),
		"src/a.ts":        file("import { helper } from \"@lib/util\";\nconsole.log(helper);\n"),
		"src/lib/util.ts": file("export const helper = 1;\n"),
	}

	model, err := projectmodel.BuildTypeScriptModelViaSidecar(ctx, snapshot, testMeta(), realOpts())
	Expect(err).NotTo(HaveOccurred())
	Expect(model.Coverage.Complete).To(BeTrue(), "%+v", model.Coverage)

	Expect(model.Files).NotTo(BeEmpty())
	paths := map[string]string{}
	for _, f := range model.Files {
		paths[f.Path] = f.Language
	}
	Expect(paths).To(HaveKeyWithValue("src/a.ts", "typescript"))
	Expect(paths).To(HaveKeyWithValue("src/lib/util.ts", "typescript"))

	expectImportEdgesHaveFileNodes(model)

	first, err := json.Marshal(model)
	Expect(err).NotTo(HaveOccurred())
	second, err := projectmodel.BuildTypeScriptModelViaSidecar(ctx, snapshot, testMeta(), realOpts())
	Expect(err).NotTo(HaveOccurred())
	secondJSON, err := json.Marshal(second)
	Expect(err).NotTo(HaveOccurred())
	Expect(first).To(Equal(secondJSON), "expected canonical Model.Files JSON to be byte-identical across two runs")
}

func expectConfigFileEdgeEndpointsInModelFiles(ctx context.Context, realOpts func() projectmodel.TSSidecarOptions) {
	snapshot := fstest.MapFS{
		"tsconfig.json": tsconfigJSON(map[string]any{
			"compilerOptions": map[string]any{"module": "commonjs", "moduleResolution": "node10"},
		}),
		"package.json": tsconfigJSON(map[string]any{"name": "fixture"}),
		"src/a.ts": file(strings.Join([]string{
			`const pkg = require("../package.json");`,
			`import cfg from "../tsconfig.json";`,
			`console.log(pkg, cfg);`,
			``,
		}, "\n")),
	}

	model, err := projectmodel.BuildTypeScriptModelViaSidecar(ctx, snapshot, testMeta(), realOpts())
	Expect(err).NotTo(HaveOccurred())
	Expect(model.Coverage.Complete).To(BeTrue(), "%+v", model.Coverage)

	pkgEdge, ok := edgeByTo(model.ImportEdges, "file:package.json")
	Expect(ok).To(BeTrue(), "expected an edge to file:package.json, got %+v", model.ImportEdges)
	Expect(pkgEdge.Kind).To(Equal("commonjs_require"))

	tsconfigEdge, ok := edgeByTo(model.ImportEdges, "file:tsconfig.json")
	Expect(ok).To(BeTrue(), "expected an edge to file:tsconfig.json, got %+v", model.ImportEdges)
	Expect(tsconfigEdge).NotTo(BeZero())

	expectImportEdgesHaveFileNodes(model)
}

func expectImportEdgesHaveFileNodes(model projectmodel.Model) {
	fileIDs := map[string]bool{}
	for _, f := range model.Files {
		fileIDs[f.ID] = true
	}
	for _, e := range model.ImportEdges {
		expectFileEndpointsKnown(fileIDs, model, e.From, e.To)
	}
}

func expectFileEndpointsKnown(fileIDs map[string]bool, model projectmodel.Model, endpoints ...string) {
	for _, endpoint := range endpoints {
		if strings.HasPrefix(endpoint, "file:") {
			Expect(fileIDs).To(HaveKey(endpoint), "edge endpoint %q has no corresponding Model.Files entry, got %+v", endpoint, model.Files)
		}
	}
}
