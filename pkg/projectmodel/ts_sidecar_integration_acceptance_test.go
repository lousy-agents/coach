package projectmodel_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing/fstest"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/projectbridge"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

// This file is issue #214 Task 3's end-to-end proof: Task 1's Go client
// (pkg/projectmodel/ts_sidecar.go) driving Task 2's real compiled
// Node/TypeScript sidecar (js/semantics/src/project-sidecar/), not the fake
// stand-in ts_sidecar_acceptance_test.go builds, and not the sidecar's own
// direct-spawn tests in js/semantics/test/project-sidecar.test.ts. It
// therefore needs Node to build and run the real sidecar; see
// ensureRealTSSidecarBinary for the graceful-skip contract that keeps this
// requirement from leaking into the core Go path (see
// node_absent_acceptance_test.go in cmd/coach for that guarantee's own
// proof).

var (
	realTSSidecarOnce sync.Once
	realTSSidecarPath string
	realTSSidecarSkip string
)

// repoRootFromThisFile locates the repository root relative to this test
// file's own path, mirroring pgMigrationFiles' runtime.Caller(0) convention
// in internal/coachapi/store_postgres_acceptance_test.go.
func repoRootFromThisFile() string {
	_, thisFile, _, ok := runtime.Caller(0)
	Expect(ok).To(BeTrue(), "runtime.Caller(0) failed")
	return filepath.Join(filepath.Dir(thisFile), "..", "..")
}

func jsSemanticsRoot() string {
	return filepath.Join(repoRootFromThisFile(), "js", "semantics")
}

// ensureRealTSSidecarBinary builds the real compiled sidecar
// (`npm run build:project-sidecar`, i.e. `mise run project-sidecar-build`)
// at most once per test binary run and memoizes the outcome. It cannot be a
// second Ginkgo BeforeSuite -- Ginkgo permits exactly one per suite, and
// ts_sidecar_acceptance_test.go (frozen, Task 1) already declares the
// suite's only one, for the fake sidecar -- so every spec below calls this
// from its own BeforeEach and Skips with skipReason when Node/npm are
// unavailable or the build fails, degrading this suite gracefully instead
// of failing Go-only environments (issue #214's explicit requirement).
func ensureRealTSSidecarBinary() (path string, skipReason string) {
	realTSSidecarOnce.Do(func() {
		if _, err := exec.LookPath("node"); err != nil {
			realTSSidecarSkip = fmt.Sprintf("node not found on PATH; skipping real ts sidecar integration suite (%s)", err)
			return
		}
		if _, err := exec.LookPath("npm"); err != nil {
			realTSSidecarSkip = fmt.Sprintf("npm not found on PATH; skipping real ts sidecar integration suite (%s)", err)
			return
		}
		root := jsSemanticsRoot()
		build := exec.Command("npm", "run", "build:project-sidecar")
		build.Dir = root
		output, err := build.CombinedOutput()
		if err != nil {
			realTSSidecarSkip = fmt.Sprintf("building the real ts sidecar failed; skipping real ts sidecar integration suite: %s: %s", err, output)
			return
		}
		binPath := filepath.Join(root, "bin", "coach-ts-project-sidecar")
		if _, statErr := os.Stat(binPath); statErr != nil {
			realTSSidecarSkip = fmt.Sprintf("real ts sidecar binary missing after build; skipping: %s", statErr)
			return
		}
		realTSSidecarPath = binPath
	})
	return realTSSidecarPath, realTSSidecarSkip
}

func file(content string) *fstest.MapFile {
	return &fstest.MapFile{Data: []byte(content)}
}

func tsconfigJSON(v any) *fstest.MapFile {
	data, err := json.Marshal(v)
	Expect(err).NotTo(HaveOccurred())
	return &fstest.MapFile{Data: data}
}

// sidecarSourceDigest hashes the sorted relative path and content of every
// .js file under vendoredDir (the real sidecar implementation copied to
// bin/project-sidecar/ by scripts/build-project-sidecar.mjs). This is
// deliberately not a hash of bin/coach-ts-project-sidecar itself: that
// file is a fixed-size, hand-written ESM shim whose bytes never change
// across sidecar versions (see the Implementer Report), so hashing it
// would produce a "version identity" that never actually varies with the
// sidecar's version.
func sidecarSourceDigest(vendoredDir string) string {
	var relPaths []string
	Expect(filepath.WalkDir(vendoredDir, func(p string, d fs.DirEntry, err error) error {
		Expect(err).NotTo(HaveOccurred())
		if d.IsDir() || !strings.HasSuffix(p, ".js") {
			return nil
		}
		rel, relErr := filepath.Rel(vendoredDir, p)
		Expect(relErr).NotTo(HaveOccurred())
		relPaths = append(relPaths, filepath.ToSlash(rel))
		return nil
	})).To(Succeed())
	sort.Strings(relPaths)

	h := sha256.New()
	for _, rel := range relPaths {
		content, readErr := os.ReadFile(filepath.Join(vendoredDir, filepath.FromSlash(rel)))
		Expect(readErr).NotTo(HaveOccurred())
		fmt.Fprintf(h, "%s\x00", rel)
		h.Write(content)
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// copyDirRecursive copies every file and directory under src into dst,
// used by the coverage-identity spec to mutate a private copy of the
// vendored sidecar tree without touching the real build output.
func copyDirRecursive(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(src, p)
		if relErr != nil {
			return relErr
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		content, readErr := os.ReadFile(p)
		if readErr != nil {
			return readErr
		}
		return os.WriteFile(target, content, 0o644)
	})
}

// pathExcludingExecutables returns the current process's PATH with any
// directory containing one of names removed, used by the "missing runtime"
// spec below to construct a child environment where the sidecar's
// `#!/usr/bin/env node` shebang cannot locate node.
func pathExcludingExecutables(names ...string) string {
	var kept []string
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" {
			continue
		}
		excluded := false
		for _, name := range names {
			if info, err := os.Stat(filepath.Join(dir, name)); err == nil && !info.IsDir() {
				excluded = true
				break
			}
		}
		if !excluded {
			kept = append(kept, dir)
		}
	}
	return strings.Join(kept, string(os.PathListSeparator))
}

var _ = Describe("BuildTypeScriptModelViaSidecar against the real compiled Node/TypeScript sidecar", Label("ts-sidecar-integration"), func() {
	var (
		sidecarPath string
		ctx         context.Context
	)

	BeforeEach(func() {
		path, skip := ensureRealTSSidecarBinary()
		if skip != "" {
			Skip(skip)
		}
		sidecarPath = path
		ctx = context.Background()
	})

	realOpts := func() projectmodel.TSSidecarOptions {
		return projectmodel.TSSidecarOptions{BinaryPath: sidecarPath, Timeout: 20 * time.Second}
	}

	When("a snapshot contains a tsconfig.json alongside .ts files", func() {
		It("forwards tsconfig.json to the real sidecar so it opens the project and analyzes the snapshot's files", func() {
			snapshot := fstest.MapFS{
				"tsconfig.json": tsconfigJSON(map[string]any{
					"compilerOptions": map[string]any{"module": "commonjs", "moduleResolution": "node10"},
				}),
				"src/a.ts": file("export const a = 1;\n"),
			}

			model, err := projectmodel.BuildTypeScriptModelViaSidecar(ctx, snapshot, testMeta(), realOpts())
			Expect(err).NotTo(HaveOccurred())
			Expect(model.Coverage.Complete).To(BeTrue(), "%+v", model.Coverage)

			// files_seen now counts both the .ts source file and the forwarded
			// tsconfig.json config file (collectTSSidecarFiles's fixed contract,
			// proven in ts_sidecar_acceptance_test.go).
			Expect(model.Coverage.Counts).To(HaveKeyWithValue("files_seen", 2))
			Expect(model.Coverage.Counts).To(HaveKeyWithValue("tsconfig_count", 1), "expected the forwarded tsconfig.json to be discovered by the real sidecar")
			Expect(model.Coverage.Counts).To(HaveKeyWithValue("projects_analyzed", 1), "expected the discovered project to actually be opened and walked")
		})
	})

	When("a snapshot contains a root tsconfig.json and a nested workspace tsconfig.json, alongside decoy config files", func() {
		It("populates Model.Workspaces with one sorted, deduped entry per discovered tsconfig.json, excluding tsconfig.base.json/package.json", func() {
			snapshot := fstest.MapFS{
				"tsconfig.json": tsconfigJSON(map[string]any{
					"compilerOptions": map[string]any{"module": "commonjs", "moduleResolution": "node10"},
				}),
				"tsconfig.base.json": tsconfigJSON(map[string]any{
					"compilerOptions": map[string]any{"module": "commonjs", "moduleResolution": "node10"},
				}),
				"package.json": tsconfigJSON(map[string]any{"name": "root-fixture"}),
				"src/a.ts":     file("export const a = 1;\n"),
				"packages/x/tsconfig.json": tsconfigJSON(map[string]any{
					"compilerOptions": map[string]any{"module": "commonjs", "moduleResolution": "node10"},
				}),
				"packages/x/src/b.ts": file("export const b = 1;\n"),
			}

			model, err := projectmodel.BuildTypeScriptModelViaSidecar(ctx, snapshot, testMeta(), realOpts())
			Expect(err).NotTo(HaveOccurred())
			Expect(model.Coverage.Complete).To(BeTrue(), "%+v", model.Coverage)
			Expect(model.Coverage.Counts).To(HaveKeyWithValue("projects_analyzed", 2), "expected both discovered tsconfig.json roots to be opened and walked")

			Expect(model.Workspaces).To(Equal([]projectmodel.Workspace{
				{ID: "workspace:.", Language: "typescript", Root: "."},
				{ID: "workspace:packages/x", Language: "typescript", Root: "packages/x"},
			}), "expected one Workspace per forwarded tsconfig.json, sorted and deduped, excluding the tsconfig.base.json/package.json decoys, got %+v", model.Workspaces)
		})
	})

	When("compilerOptions.paths/baseUrl alias an import", func() {
		It("produces an ImportEdge resolving the aliased import to its target file", func() {
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

			edge, ok := edgeByTo(model.ImportEdges, "file:src/lib/util.ts")
			Expect(ok).To(BeTrue(), "expected an edge to file:src/lib/util.ts, got %+v", model.ImportEdges)
			Expect(edge.From).To(Equal("file:src/a.ts"))
			Expect(edge.Kind).To(Equal("import"))
			Expect(edge.Resolution).To(Equal("snapshot"))
		})
	})

	When("a tsconfig.json extends a sibling tsconfig.base.json that supplies the path aliases", func() {
		It("resolves the aliased import via the extended config, pinning the tsconfig* glob (not just the exact tsconfig.json name)", func() {
			snapshot := fstest.MapFS{
				"tsconfig.base.json": tsconfigJSON(map[string]any{
					"compilerOptions": map[string]any{
						"module": "commonjs", "moduleResolution": "node10",
						"baseUrl": ".", "paths": map[string]any{"@lib/*": []string{"src/lib/*"}},
					},
				}),
				"tsconfig.json":   tsconfigJSON(map[string]any{"extends": "./tsconfig.base.json"}),
				"src/a.ts":        file("import { helper } from \"@lib/util\";\nconsole.log(helper);\n"),
				"src/lib/util.ts": file("export const helper = 1;\n"),
			}

			model, err := projectmodel.BuildTypeScriptModelViaSidecar(ctx, snapshot, testMeta(), realOpts())
			Expect(err).NotTo(HaveOccurred())
			Expect(model.Coverage.Complete).To(BeTrue(), "%+v", model.Coverage)

			edge, ok := edgeByTo(model.ImportEdges, "file:src/lib/util.ts")
			Expect(ok).To(BeTrue(), "expected an edge to file:src/lib/util.ts (requires tsconfig.base.json to have been forwarded), got %+v", model.ImportEdges)
			Expect(edge.From).To(Equal("file:src/a.ts"))
			Expect(edge.Resolution).To(Equal("snapshot"))
		})
	})

	When("the snapshot contains files under node_modules/ alongside a directory whose name merely starts with node_modules", func() {
		It("excludes node_modules/** from files_seen while still collecting the non-node_modules directory", func() {
			snapshot := fstest.MapFS{
				"tsconfig.json": tsconfigJSON(map[string]any{
					"compilerOptions": map[string]any{"module": "commonjs", "moduleResolution": "node10"},
				}),
				"src/a.ts":                            file("export const a = 1;\n"),
				"node_modules/some-pkg/package.json":  tsconfigJSON(map[string]any{"name": "some-pkg"}),
				"node_modules/some-pkg/tsconfig.json": tsconfigJSON(map[string]any{}),
				"node_modules/some-pkg/index.ts":      file("export const x = 1;\n"),
				"my-node_modules/tsconfig.json":       tsconfigJSON(map[string]any{}),
			}

			model, err := projectmodel.BuildTypeScriptModelViaSidecar(ctx, snapshot, testMeta(), realOpts())
			Expect(err).NotTo(HaveOccurred())

			// 3 = tsconfig.json + src/a.ts + my-node_modules/tsconfig.json; the
			// three node_modules/** entries must be dropped, while
			// my-node_modules/tsconfig.json (a non-node_modules directory that
			// merely shares a name prefix) must be kept, pinning the exclusion's
			// segment-wise (not substring) matching.
			Expect(model.Coverage.Counts).To(HaveKeyWithValue("files_seen", 3), "expected node_modules/** excluded but my-node_modules/tsconfig.json kept, got %+v", model.Coverage.Counts)
		})
	})

	When("an import crosses a tsconfig project-reference boundary", func() {
		It("resolves to the referenced project's source file via project-reference redirection", func() {
			snapshot := fstest.MapFS{
				"app/tsconfig.json": tsconfigJSON(map[string]any{
					"compilerOptions": map[string]any{
						"composite": true, "module": "commonjs", "moduleResolution": "node10",
						"baseUrl": ".", "paths": map[string]any{"libpkg/*": []string{"../libpkg/dist/*"}},
					},
					"references": []map[string]string{{"path": "../libpkg"}},
					"include":    []string{"src/**/*"},
				}),
				"app/src/a.ts": file("import { helper } from \"libpkg/helper\";\nconsole.log(helper);\n"),
				"libpkg/tsconfig.json": tsconfigJSON(map[string]any{
					"compilerOptions": map[string]any{
						"composite": true, "module": "commonjs", "moduleResolution": "node10",
						"rootDir": "src", "outDir": "dist",
					},
					"include": []string{"src/**/*"},
				}),
				"libpkg/src/helper.ts": file("export const helper = 1;\n"),
			}

			model, err := projectmodel.BuildTypeScriptModelViaSidecar(ctx, snapshot, testMeta(), realOpts())
			Expect(err).NotTo(HaveOccurred())
			Expect(model.Coverage.Complete).To(BeTrue(), "%+v", model.Coverage)

			edge, ok := edgeByTo(model.ImportEdges, "file:libpkg/src/helper.ts")
			Expect(ok).To(BeTrue(), "expected an edge to file:libpkg/src/helper.ts, got %+v", model.ImportEdges)
			Expect(edge.From).To(Equal("file:app/src/a.ts"))
			Expect(edge.Resolution).To(Equal("snapshot"))
			Expect(model.Coverage.Counts).To(HaveKeyWithValue("projects_analyzed", 2), "expected both projects to actually be opened and walked")
		})
	})

	When("a package.json exports map and barrel re-exports are both present", func() {
		It("resolves the exports-map import and both star/named barrel re-exports", func() {
			snapshot := fstest.MapFS{
				"tsconfig.json": tsconfigJSON(map[string]any{
					"compilerOptions": map[string]any{"module": "nodenext", "moduleResolution": "nodenext"},
				}),
				"src/a.ts": file("import { z } from \"@acme/lib\";\nimport { w } from \"@acme/lib/sub\";\nconsole.log(z, w);\n"),
				"packages/lib/package.json": tsconfigJSON(map[string]any{
					"name":    "@acme/lib",
					"exports": map[string]any{".": "./src/index.ts", "./sub": "./src/sub.ts"},
				}),
				"packages/lib/src/index.ts": file("export * from \"./inner\";\nexport { onlyY } from \"./onlyY\";\n"),
				"packages/lib/src/inner.ts": file("export const z = 42;\n"),
				"packages/lib/src/onlyY.ts": file("export const onlyY = 1;\n"),
				"packages/lib/src/sub.ts":   file("export const w = 7;\n"),
			}

			model, err := projectmodel.BuildTypeScriptModelViaSidecar(ctx, snapshot, testMeta(), realOpts())
			Expect(err).NotTo(HaveOccurred())
			Expect(model.Coverage.Complete).To(BeTrue(), "%+v", model.Coverage)

			mainImport, ok := edgeByTo(model.ImportEdges, "file:packages/lib/src/index.ts")
			Expect(ok).To(BeTrue(), "%+v", model.ImportEdges)
			Expect(mainImport.Resolution).To(Equal("snapshot"))

			subImport, ok := edgeByTo(model.ImportEdges, "file:packages/lib/src/sub.ts")
			Expect(ok).To(BeTrue(), "%+v", model.ImportEdges)
			Expect(subImport.Resolution).To(Equal("snapshot"))

			starReexport, ok := edgeByTo(model.ImportEdges, "file:packages/lib/src/inner.ts")
			Expect(ok).To(BeTrue(), "%+v", model.ImportEdges)
			Expect(starReexport.From).To(Equal("file:packages/lib/src/index.ts"))
			Expect(starReexport.Kind).To(Equal("reexport"))

			namedReexport, ok := edgeByTo(model.ImportEdges, "file:packages/lib/src/onlyY.ts")
			Expect(ok).To(BeTrue(), "%+v", model.ImportEdges)
			Expect(namedReexport.Kind).To(Equal("reexport"))
		})
	})

	When("snapshot paths use mixed case", func() {
		It("preserves inventory path casing byte-for-byte in ImportEdge from/to/site", func() {
			// On case-insensitive hosts, TS's checker lowercases declaration
			// paths. Stable file: IDs must still match the request inventory
			// exactly so Mac and Linux emit byte-identical facts.
			snapshot := fstest.MapFS{
				"tsconfig.json": tsconfigJSON(map[string]any{
					"compilerOptions": map[string]any{"module": "commonjs", "moduleResolution": "node10"},
				}),
				"Src/App.ts": file(strings.Join([]string{
					`import { helper } from "./Lib/Util";`,
					`export { onlyY } from "./onlyY";`,
					`console.log(helper);`,
					``,
				}, "\n")),
				"Src/Lib/Util.ts": file("export const helper = 1;\n"),
				"Src/onlyY.ts":    file("export const onlyY = 1;\n"),
			}

			model, err := projectmodel.BuildTypeScriptModelViaSidecar(ctx, snapshot, testMeta(), realOpts())
			Expect(err).NotTo(HaveOccurred())
			Expect(model.Coverage.Complete).To(BeTrue(), "%+v", model.Coverage)

			utilEdge, ok := edgeByTo(model.ImportEdges, "file:Src/Lib/Util.ts")
			Expect(ok).To(BeTrue(), "expected exact-case to file:Src/Lib/Util.ts, got %+v", model.ImportEdges)
			Expect(utilEdge.From).To(Equal("file:Src/App.ts"))
			Expect(utilEdge.Site).To(Equal("Src/App.ts:1"))
			Expect(utilEdge.Resolution).To(Equal("snapshot"))

			onlyYEdge, ok := edgeByTo(model.ImportEdges, "file:Src/onlyY.ts")
			Expect(ok).To(BeTrue(), "expected exact-case to file:Src/onlyY.ts, got %+v", model.ImportEdges)
			Expect(onlyYEdge.From).To(Equal("file:Src/App.ts"))
			Expect(onlyYEdge.Kind).To(Equal("reexport"))
			Expect(onlyYEdge.Resolution).To(Equal("snapshot"))
		})
	})

	When("a file mixes a CommonJS require, a dynamic import, and a type-only import", func() {
		It("produces three edges with three distinct Kind values", func() {
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
		})
	})

	When("a source file imports a pure inline type-only named binding", func() {
		It("classifies the edge type_only, not a value import, and a mixed named import still classifies as a value import", func() {
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
		})
	})

	When("a snapshot contains .ts sources but no tsconfig.json anywhere", func() {
		It("reports Complete=false with a stable diagnostic instead of a vacuous complete empty graph", func() {
			snapshot := fstest.MapFS{
				"src/a.ts": file("export const a = 1;\n"),
				"src/b.ts": file("export const b = 1;\n"),
			}

			model, err := projectmodel.BuildTypeScriptModelViaSidecar(ctx, snapshot, testMeta(), realOpts())
			Expect(err).NotTo(HaveOccurred())
			Expect(model.Coverage.Complete).To(BeFalse(), "%+v", model.Coverage)
			Expect(model.ImportEdges).To(BeEmpty())

			_, hasDiag := diagnosticWithCode(model.Coverage.Diagnostics, "ts_no_project_config")
			Expect(hasDiag).To(BeTrue(), "expected a ts_no_project_config diagnostic, got %+v", model.Coverage.Diagnostics)

			_, hasBackendUnavailable := diagnosticWithCode(model.Coverage.Diagnostics, projectmodel.DiagBackendUnavailable)
			Expect(hasBackendUnavailable).To(BeFalse(), "missing project config is a degraded-but-successful analysis, not a transport/backend failure")
		})
	})

	When("the snapshot has no TypeScript/TSX sources and no tsconfig.json", func() {
		It("stays Complete=true with an empty, vacuous model", func() {
			snapshot := fstest.MapFS{
				"README.md": file("not typescript\n"),
			}

			model, err := projectmodel.BuildTypeScriptModelViaSidecar(ctx, snapshot, testMeta(), realOpts())
			Expect(err).NotTo(HaveOccurred())
			Expect(model.Coverage.Complete).To(BeTrue(), "%+v", model.Coverage)
			Expect(model.ImportEdges).To(BeEmpty())
		})
	})

	When("a successful analysis inventories the snapshot's TS/TSX sources", func() {
		It("populates Model.Files for every analyzed path, with every file: edge endpoint present in Model.Files", func() {
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

			fileIDs := map[string]bool{}
			for _, f := range model.Files {
				fileIDs[f.ID] = true
			}
			for _, e := range model.ImportEdges {
				for _, endpoint := range []string{e.From, e.To} {
					if strings.HasPrefix(endpoint, "file:") {
						Expect(fileIDs).To(HaveKey(endpoint), "edge endpoint %q has no corresponding Model.Files entry, got %+v", endpoint, model.Files)
					}
				}
			}

			first, err := json.Marshal(model)
			Expect(err).NotTo(HaveOccurred())
			second, err := projectmodel.BuildTypeScriptModelViaSidecar(ctx, snapshot, testMeta(), realOpts())
			Expect(err).NotTo(HaveOccurred())
			secondJSON, err := json.Marshal(second)
			Expect(err).NotTo(HaveOccurred())
			Expect(first).To(Equal(secondJSON), "expected canonical Model.Files JSON to be byte-identical across two runs")
		})
	})

	When("a source file directly requires/imports a forwarded config file (package.json, tsconfig.json)", func() {
		It("includes those config-file edge endpoints in Model.Files too, not just .ts/.tsx sources", func() {
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

			fileIDs := map[string]bool{}
			for _, f := range model.Files {
				fileIDs[f.ID] = true
			}
			for _, e := range model.ImportEdges {
				for _, endpoint := range []string{e.From, e.To} {
					if strings.HasPrefix(endpoint, "file:") {
						Expect(fileIDs).To(HaveKey(endpoint), "edge endpoint %q has no corresponding Model.Files entry, got %+v", endpoint, model.Files)
					}
				}
			}
		})
	})

	When("tsconfig.json is not valid JSON", func() {
		It("degrades to an incomplete Model with a diagnostic instead of a Go error or a backend-unavailable diagnostic", func() {
			snapshot := fstest.MapFS{
				"tsconfig.json": file("{ this is not valid json !!! "),
				"src/a.ts":      file("export const a = 1;\n"),
			}

			model, err := projectmodel.BuildTypeScriptModelViaSidecar(ctx, snapshot, testMeta(), realOpts())
			Expect(err).NotTo(HaveOccurred())
			Expect(model.Coverage.Complete).To(BeFalse(), "an unparseable tsconfig must never be silently honored")

			_, hasConfigDiag := diagnosticWithCode(model.Coverage.Diagnostics, "ts_config_diagnostic")
			Expect(hasConfigDiag).To(BeTrue(), "expected a ts_config_diagnostic, got %+v", model.Coverage.Diagnostics)

			_, hasBackendUnavailable := diagnosticWithCode(model.Coverage.Diagnostics, projectmodel.DiagBackendUnavailable)
			Expect(hasBackendUnavailable).To(BeFalse(), "an invalid config is a degraded-but-successful analysis, not a transport/backend failure")
		})
	})

	When("an import resolves only via the real filesystem, outside the snapshot", func() {
		It("never reads real-disk content and never reports it as a snapshot-resolved edge", func() {
			realDir, err := os.MkdirTemp("", "coach-ts-sidecar-integration-leak-*")
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(os.RemoveAll, realDir)

			marker := "LEAKED_MARKER_SHOULD_NEVER_APPEAR_IN_INTEGRATION_OUTPUT"
			realFile := filepath.Join(realDir, "leak-marker.ts")
			Expect(os.WriteFile(realFile, []byte(fmt.Sprintf("export const LEAKED_MARKER = %q;\n", marker)), 0o644)).To(Succeed())

			realSpecifier := strings.TrimSuffix(realFile, ".ts")
			snapshot := fstest.MapFS{
				"tsconfig.json": tsconfigJSON(map[string]any{
					"compilerOptions": map[string]any{"module": "commonjs", "moduleResolution": "node10"},
				}),
				"src/a.ts": file(fmt.Sprintf("import { LEAKED_MARKER } from %q;\nconsole.log(LEAKED_MARKER);\n", realSpecifier)),
			}

			model, err := projectmodel.BuildTypeScriptModelViaSidecar(ctx, snapshot, testMeta(), realOpts())
			Expect(err).NotTo(HaveOccurred())

			encoded, marshalErr := json.Marshal(model)
			Expect(marshalErr).NotTo(HaveOccurred())
			Expect(string(encoded)).NotTo(ContainSubstring(marker), "sidecar output leaked real-disk content: %s", encoded)

			Expect(model.ImportEdges).To(HaveLen(1), "%+v", model.ImportEdges)
			Expect(model.ImportEdges[0].Resolution).NotTo(Equal("snapshot"))
			Expect(model.ImportEdges[0].To).NotTo(HavePrefix("file:"))
		})
	})

	When("the snapshot is large enough to exercise bounded output handling", func() {
		It("still produces one complete, valid Model instead of truncating or hanging", func() {
			const fileCount = 80
			snapshot := fstest.MapFS{
				"tsconfig.json": tsconfigJSON(map[string]any{
					"compilerOptions": map[string]any{"module": "commonjs", "moduleResolution": "node10"},
				}),
			}
			for i := 0; i < fileCount; i++ {
				var body string
				if i == fileCount-1 {
					body = fmt.Sprintf("export const v%d = %d;\n", i, i)
				} else {
					body = fmt.Sprintf("export { v%d } from \"./m%d\";\nexport const v%d = %d;\n", i+1, i+1, i, i)
				}
				snapshot[fmt.Sprintf("src/m%d.ts", i)] = file(body)
			}

			start := time.Now()
			model, err := projectmodel.BuildTypeScriptModelViaSidecar(ctx, snapshot, testMeta(), realOpts())
			elapsed := time.Since(start)

			Expect(err).NotTo(HaveOccurred())
			Expect(model.Coverage.Complete).To(BeTrue(), "%+v", model.Coverage)
			Expect(model.ImportEdges).To(HaveLen(fileCount-1), "expected one re-export edge per chained file")
			Expect(elapsed).To(BeNumerically("<", 20*time.Second), "analysis of %d trivial files took %s", fileCount, elapsed)
		})
	})

	When("opts.Timeout is far smaller than the real sidecar's startup and analysis time", func() {
		It("cancels the real child process and reports the same DiagBackendUnavailable timeout diagnostic proven against the fake sidecar", func() {
			snapshot := fstest.MapFS{
				"tsconfig.json": tsconfigJSON(map[string]any{
					"compilerOptions": map[string]any{"module": "commonjs", "moduleResolution": "node10"},
				}),
				"src/a.ts": file("export const a = 1;\n"),
			}
			opts := realOpts()
			// Empirically the real sidecar (Node startup + a trivial single-file
			// analysis) takes at least ~95ms end to end (measured floor across 8
			// runs; see the Implementer Report for samples); 30ms is comfortably
			// below that floor while still well above zero, so this reliably
			// exercises the timeout path rather than racing it.
			opts.Timeout = 30 * time.Millisecond

			start := time.Now()
			model, err := projectmodel.BuildTypeScriptModelViaSidecar(ctx, snapshot, testMeta(), opts)
			elapsed := time.Since(start)

			Expect(err).NotTo(HaveOccurred())
			Expect(elapsed).To(BeNumerically("<", 10*time.Second))
			Expect(model.Coverage.Complete).To(BeFalse())

			diag, ok := diagnosticWithCode(model.Coverage.Diagnostics, projectmodel.DiagBackendUnavailable)
			Expect(ok).To(BeTrue(), "expected a project_backend_unavailable diagnostic, got %+v", model.Coverage.Diagnostics)
			Expect(diag.Message).To(ContainSubstring("timed out"))
		})
	})

	When("the compiled sidecar binary's #!/usr/bin/env node shebang cannot find node on PATH", func() {
		It("fails open to project_backend_unavailable instead of hanging or panicking", func() {
			strippedPath := pathExcludingExecutables("node", "npm")

			// Belt-and-suspenders: prove node and npm are genuinely unreachable
			// under strippedPath before trusting the assertion below.
			probe := exec.Command("sh", "-c", "command -v node || command -v npm")
			probe.Env = []string{"PATH=" + strippedPath}
			Expect(probe.Run()).To(HaveOccurred(), "expected neither node nor npm to be found on the stripped PATH used for this spec")

			GinkgoT().Setenv("PATH", strippedPath)

			snapshot := fstest.MapFS{
				"tsconfig.json": tsconfigJSON(map[string]any{
					"compilerOptions": map[string]any{"module": "commonjs", "moduleResolution": "node10"},
				}),
				"src/a.ts": file("export const a = 1;\n"),
			}
			opts := realOpts()
			opts.Timeout = 10 * time.Second

			model, err := projectmodel.BuildTypeScriptModelViaSidecar(ctx, snapshot, testMeta(), opts)
			Expect(err).NotTo(HaveOccurred())
			Expect(model.Coverage.Complete).To(BeFalse())

			diag, ok := diagnosticWithCode(model.Coverage.Diagnostics, projectmodel.DiagBackendUnavailable)
			Expect(ok).To(BeTrue(), "expected a project_backend_unavailable diagnostic, got %+v", model.Coverage.Diagnostics)
			Expect(diag.Message).To(ContainSubstring("exited"))
			Expect(strings.ToLower(diag.Message)).To(ContainSubstring("node"), "expected the shebang's own failure to find node to surface in the diagnostic: %s", diag.Message)
		})
	})

	When("the same real snapshot and options are analyzed twice", func() {
		It("produces byte-identical canonical JSON both times", func() {
			// This spec needs a fixture that actually produces multiple
			// ImportEdges (so canonical sorting genuinely matters, not a
			// vacuous zero-edge comparison), hence the package.json exports
			// map and barrel re-exports below.
			snapshot := fstest.MapFS{
				"tsconfig.json": tsconfigJSON(map[string]any{
					"compilerOptions": map[string]any{"module": "nodenext", "moduleResolution": "nodenext"},
				}),
				"src/a.ts": file("import { z } from \"@acme/lib\";\nimport { w } from \"@acme/lib/sub\";\nconsole.log(z, w);\n"),
				"packages/lib/package.json": tsconfigJSON(map[string]any{
					"name":    "@acme/lib",
					"exports": map[string]any{".": "./src/index.ts", "./sub": "./src/sub.ts"},
				}),
				"packages/lib/src/index.ts": file("export * from \"./inner\";\nexport { onlyY } from \"./onlyY\";\n"),
				"packages/lib/src/inner.ts": file("export const z = 42;\n"),
				"packages/lib/src/onlyY.ts": file("export const onlyY = 1;\n"),
				"packages/lib/src/sub.ts":   file("export const w = 7;\n"),
			}

			first, err := projectmodel.BuildTypeScriptModelViaSidecar(ctx, snapshot, testMeta(), realOpts())
			Expect(err).NotTo(HaveOccurred())
			Expect(first.Coverage.Complete).To(BeTrue(), "%+v", first.Coverage)

			second, err := projectmodel.BuildTypeScriptModelViaSidecar(ctx, snapshot, testMeta(), realOpts())
			Expect(err).NotTo(HaveOccurred())
			Expect(second.Coverage.Complete).To(BeTrue(), "%+v", second.Coverage)

			firstJSON, err := json.Marshal(first)
			Expect(err).NotTo(HaveOccurred())
			secondJSON, err := json.Marshal(second)
			Expect(err).NotTo(HaveOccurred())
			Expect(firstJSON).To(Equal(secondJSON), "expected canonical Model JSON to be byte-identical across two identical real-sidecar runs")
			Expect(len(first.ImportEdges)).To(BeNumerically(">", 0))
		})
	})

	When("a caller resolves a sidecar/protocol/toolchain identity and supplies it as SnapshotMeta.BackendDigest", func() {
		It("carries that identity through to Model.Snapshot.BackendDigest unchanged and stably across calls", func() {
			// pkg/projectmodel.SnapshotMeta is documented (go_imports.go) as
			// caller-resolved identity the build functions do not compute
			// themselves; this is the extension point a future CLI wiring layer
			// would use to record which sidecar implementation, wire protocol
			// version, and TypeScript toolchain version produced a Model,
			// without reopening internal/projectbridge/protocol.go (frozen) or
			// ts_sidecar.go's transport (frozen) -- see the Implementer Report
			// for why no frozen file needed to change for this.
			vendoredDir := filepath.Join(jsSemanticsRoot(), "bin", "project-sidecar")
			sourceDigest := sidecarSourceDigest(vendoredDir)

			typescriptVersion := readJSSemanticsTypescriptDevDependency()
			backendDigest := fmt.Sprintf("ts-sidecar:sha256:%s;protocol:%d;typescript:%s", sourceDigest, projectbridge.ProtocolVersion, typescriptVersion)

			meta := testMeta()
			meta.BackendDigest = backendDigest

			snapshot := fstest.MapFS{
				"tsconfig.json": tsconfigJSON(map[string]any{
					"compilerOptions": map[string]any{"module": "commonjs", "moduleResolution": "node10"},
				}),
				"src/a.ts": file("export const a = 1;\n"),
			}

			first, err := projectmodel.BuildTypeScriptModelViaSidecar(ctx, snapshot, meta, realOpts())
			Expect(err).NotTo(HaveOccurred())
			Expect(first.Snapshot.BackendDigest).To(Equal(backendDigest))
			Expect(first.Snapshot.BackendDigest).To(ContainSubstring(fmt.Sprintf("protocol:%d", projectbridge.ProtocolVersion)))
			Expect(first.Snapshot.BackendDigest).To(ContainSubstring("typescript:" + typescriptVersion))

			second, err := projectmodel.BuildTypeScriptModelViaSidecar(ctx, snapshot, meta, realOpts())
			Expect(err).NotTo(HaveOccurred())
			Expect(second.Snapshot.BackendDigest).To(Equal(first.Snapshot.BackendDigest), "expected the sidecar/protocol/toolchain identity to be stable across calls")

			// Prove the digest genuinely tracks sidecar source content rather
			// than being a constant that merely looks like a version identity
			// (the failure mode this spec exists to catch: hashing the fixed-
			// size ESM shim at bin/coach-ts-project-sidecar instead, which never
			// changes across sidecar versions).
			mutatedDir, mkErr := os.MkdirTemp("", "ts-sidecar-source-mutation-*")
			Expect(mkErr).NotTo(HaveOccurred())
			DeferCleanup(os.RemoveAll, mutatedDir)
			Expect(copyDirRecursive(vendoredDir, mutatedDir)).To(Succeed())

			var firstJSFile string
			Expect(filepath.WalkDir(mutatedDir, func(p string, d fs.DirEntry, walkErr error) error {
				Expect(walkErr).NotTo(HaveOccurred())
				if !d.IsDir() && strings.HasSuffix(p, ".js") && firstJSFile == "" {
					firstJSFile = p
				}
				return nil
			})).To(Succeed())
			Expect(firstJSFile).NotTo(BeEmpty(), "expected at least one vendored .js file to mutate")

			original, readErr := os.ReadFile(firstJSFile)
			Expect(readErr).NotTo(HaveOccurred())
			Expect(os.WriteFile(firstJSFile, append(original, []byte("\n// mutated for coverage-identity test\n")...), 0o644)).To(Succeed())

			mutatedDigest := sidecarSourceDigest(mutatedDir)
			Expect(mutatedDigest).NotTo(Equal(sourceDigest), "expected the digest to change when a vendored sidecar source file changes")
		})
	})

	When("a Model is produced from a real sidecar run", func() {
		It("never carries any Signal/layer/violation-shaped data, matching pkg/projectmodel's facts-only package contract", func() {
			// Deliberately does not include a tsconfig.json -- this spec's claim
			// is about JSON shape, not about exercising alias/reference/export
			// resolution, so a minimal snapshot keeps it focused.
			snapshot := fstest.MapFS{
				"src/a.ts": file("export const a = 1;\n"),
			}
			model, err := projectmodel.BuildTypeScriptModelViaSidecar(ctx, snapshot, testMeta(), realOpts())
			Expect(err).NotTo(HaveOccurred())

			encoded, marshalErr := json.Marshal(model)
			Expect(marshalErr).NotTo(HaveOccurred())
			lower := strings.ToLower(string(encoded))
			Expect(lower).NotTo(ContainSubstring("\"signal"))
			Expect(lower).NotTo(ContainSubstring("\"layer"))
			Expect(lower).NotTo(ContainSubstring("\"violation"))
		})
	})
})

// readJSSemanticsTypescriptDevDependency reads js/semantics/package.json's
// devDependencies.typescript version string directly from disk so the
// coverage-identity spec above cannot drift from the actual pinned
// toolchain version.
func readJSSemanticsTypescriptDevDependency() string {
	data, err := os.ReadFile(filepath.Join(jsSemanticsRoot(), "package.json"))
	Expect(err).NotTo(HaveOccurred())

	var pkg struct {
		DevDependencies struct {
			TypeScript string `json:"typescript"`
		} `json:"devDependencies"`
	}
	Expect(json.Unmarshal(data, &pkg)).To(Succeed())
	Expect(pkg.DevDependencies.TypeScript).NotTo(BeEmpty())
	return pkg.DevDependencies.TypeScript
}

var _ = Describe("pkg/projectmodel's dependency boundary", func() {
	It("never imports pkg/codesignal, so this facts-only slice cannot emit a Signal", func() {
		cmd := exec.Command("go", "list", "-deps", "./pkg/projectmodel/...")
		cmd.Dir = repoRootFromThisFile()
		output, err := cmd.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), "go list -deps: %s", output)
		Expect(string(output)).NotTo(ContainSubstring("coach/pkg/codesignal"), "pkg/projectmodel must never depend on pkg/codesignal (see model.go's package doc)")
	})
})
