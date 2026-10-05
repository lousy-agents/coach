package projectmodel_test

import (
	"context"
	"strings"
	"testing/fstest"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func expectRequireDynamicAndTypeOnlyImportKinds(ctx context.Context, realOpts func() projectmodel.TSSidecarOptions) {
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

func expectInlineTypeOnlyBindingClassifiedTypeOnly(ctx context.Context, realOpts func() projectmodel.TSSidecarOptions) {
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
