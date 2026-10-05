package projectmodel_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing/fstest"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

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
		return realSidecarOptions(sidecarPath)
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

			Expect(model.Coverage.Counts).To(HaveKeyWithValue("files_seen", 2), "expected files_seen to count both the .ts source and the forwarded tsconfig.json")
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

			Expect(model.Coverage.Counts).To(HaveKeyWithValue("files_seen", 3), "expected the three node_modules/** entries excluded (segment-wise, not substring match) but tsconfig.json, src/a.ts, and my-node_modules/tsconfig.json kept, got %+v", model.Coverage.Counts)
		})
	})

	When("a tsconfig lists a JSON file as an explicit root file (resolveJsonModule) alongside a normal .ts source, and Roots is set", func() {
		It("reports a real root-scope candidate/analyzed mismatch that marks model coverage incomplete per SA-280-025", func() {
			expectRootScopeMismatchMarksModelIncomplete(ctx, realOpts)
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
			expectRequireDynamicAndTypeOnlyImportKinds(ctx, realOpts)
		})
	})

	When("a source file imports a pure inline type-only named binding", func() {
		It("classifies the edge type_only, not a value import, and a mixed named import still classifies as a value import", func() {
			expectInlineTypeOnlyBindingClassifiedTypeOnly(ctx, realOpts)
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
			expectModelFilesCoverEveryAnalyzedPath(ctx, realOpts)
		})
	})

	When("a source file directly requires/imports a forwarded config file (package.json, tsconfig.json)", func() {
		It("includes those config-file edge endpoints in Model.Files too, not just .ts/.tsx sources", func() {
			expectConfigFileEdgeEndpointsInModelFiles(ctx, realOpts)
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

			Expect(model.CallFacts).To(BeEmpty(), "expected no fabricated call facts when tsconfig failed to load")
			Expect(model.ReachabilityFacts).To(BeEmpty(), "expected no fabricated reachability facts when tsconfig failed to load")
		})
	})

	When("a route registration's handler calls a resolved ORM sink method", func() {

		It("populates Model.CallFacts and Model.ReachabilityFacts with structured paths and resolved-direct confidence", func() {
			expectRouteHandlerReachabilityFacts(ctx, realOpts)
		})
	})

	When("BuildTypeScriptLayerBypass runs against a snapshot with one compliant handler (whose depth-1 walk hits a routine local-call-not-followed gap) and one unrelated, fully-resolved, genuinely direct bypass handler", func() {

		It("still produces the unrelated bypass witness instead of suppressing it project-wide", func() {
			expectUnrelatedBypassWitnessSurvivesReachabilityGap(ctx, realOpts)
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
			expectLargeSnapshotYieldsOneCompleteModel(ctx, realOpts)
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
			expectBackendDigestCarriedThroughUnchanged(ctx, realOpts)
		})
	})

	When("a Model is produced from a real sidecar run", func() {
		It("never carries any Signal/layer/violation-shaped data, matching pkg/projectmodel's facts-only package contract", func() {

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

var _ = Describe("pkg/projectmodel's dependency boundary", func() {
	It("never imports pkg/codesignal, so this facts-only slice cannot emit a Signal", func() {
		cmd := exec.Command("go", "list", "-deps", "./pkg/projectmodel/...")
		cmd.Dir = repoRootFromThisFile()
		output, err := cmd.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), "go list -deps: %s", output)
		Expect(string(output)).NotTo(ContainSubstring("coach/pkg/codesignal"), "pkg/projectmodel must never depend on pkg/codesignal (see doc.go's package doc)")
	})
})
