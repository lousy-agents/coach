package projectmodel_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing/fstest"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/projectbridge"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

var fakeTSSidecarPath string

var _ = BeforeSuite(func() {
	dir, err := os.MkdirTemp("", "fake-ts-sidecar-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, dir)

	fakeTSSidecarPath = filepath.Join(dir, "fake-ts-sidecar")
	build := exec.Command("go", "build", "-o", fakeTSSidecarPath, "./testdata/fake_ts_sidecar")
	output, err := build.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "building fake ts sidecar: %s", output)
})

func tsSidecarSnapshot() fstest.MapFS {
	return fstest.MapFS{
		"src/a.ts":     &fstest.MapFile{Data: []byte("import { b } from './b';\nexport const a = b;\n")},
		"src/b.ts":     &fstest.MapFile{Data: []byte("export const b = 1;\n")},
		"src/c.tsx":    &fstest.MapFile{Data: []byte("export const C = () => null;\n")},
		"src/notes.md": &fstest.MapFile{Data: []byte("not a TypeScript source file\n")},
	}
}

func sidecarOptsWithMode(mode string) projectmodel.TSSidecarOptions {
	return projectmodel.TSSidecarOptions{
		BinaryPath: fakeTSSidecarPath,
		Args:       []string{"--mode=" + mode},
		Timeout:    5 * time.Second,
	}
}

func diagnosticWithCode(diags []projectmodel.Diagnostic, code string) (projectmodel.Diagnostic, bool) {
	for _, d := range diags {
		if d.Code == code {
			return d, true
		}
	}
	return projectmodel.Diagnostic{}, false
}

var _ = Describe("BuildTypeScriptModelViaSidecar", func() {
	When("the sidecar produces a valid response", func() {
		It("translates raw import facts into a Model without error, sending every snapshot file and preserving its exact content across the base64 boundary", func() {
			model, err := projectmodel.BuildTypeScriptModelViaSidecar(context.Background(), tsSidecarSnapshot(), testMeta(), sidecarOptsWithMode("happy"))
			Expect(err).NotTo(HaveOccurred())
			Expect(model.Coverage.Complete).To(BeTrue())
			Expect(model.Coverage.Diagnostics).To(BeEmpty())
			Expect(model.Coverage.Phase).To(Equal("ts_sidecar_build"), "expected the client's own phase, not the fake sidecar's reported Coverage.Phase, which must be ignored")
			Expect(model.Coverage.Counts).To(HaveKeyWithValue("files_seen", 3), "expected src/a.ts, src/b.ts, and src/c.tsx to be collected and sent to the sidecar, but src/notes.md excluded as neither a TypeScript source file nor a tsconfig*.json/package.json config file")
			Expect(model.ImportEdges).To(ContainElement(projectmodel.ImportEdge{
				From: "file:src/a.ts", To: "import { b } from './b';\nexport const a = b;\n", Kind: "internal", Site: "src/a.ts:1",
			}), "expected the sidecar's echo of the decoded first file's content to survive the base64 round-trip unchanged")
		})
	})

	When("Roots is set", func() {
		It("sends the op, timeout_ms, and roots fields on the outgoing request", func() {
			opts := sidecarOptsWithMode("request_probe")
			opts.Roots = []string{"src"}

			model, err := projectmodel.BuildTypeScriptModelViaSidecar(context.Background(), tsSidecarSnapshot(), testMeta(), opts)
			Expect(err).NotTo(HaveOccurred())
			Expect(model.Coverage.Complete).To(BeTrue())

			diag, ok := diagnosticWithCode(model.Coverage.Diagnostics, "request_probe")
			Expect(ok).To(BeTrue(), "expected a request_probe diagnostic, got %+v", model.Coverage.Diagnostics)
			Expect(diag.Message).To(ContainSubstring(fmt.Sprintf("op=%q", projectbridge.OpAnalyzeProject)), "expected the outgoing request's Op field to be sent")
			Expect(diag.Message).To(ContainSubstring(fmt.Sprintf("timeout_ms=%d", opts.Timeout.Milliseconds())), "expected the outgoing request's TimeoutMS field, derived from opts.Timeout, to be sent")
			Expect(diag.Message).To(ContainSubstring("roots=[src]"), "expected the outgoing request's Roots field to be sent")
		})

		It("scopes both file collection and Snapshot.SelectedRoots to the configured roots", func() {
			snapshot := fstest.MapFS{
				"src/a.ts":   &fstest.MapFile{Data: []byte("import { b } from './b';\nexport const a = b;\n")},
				"src/b.ts":   &fstest.MapFile{Data: []byte("export const b = 1;\n")},
				"other/c.ts": &fstest.MapFile{Data: []byte("export const c = 1;\n")},
			}
			opts := sidecarOptsWithMode("happy")
			opts.Roots = []string{"src"}

			model, err := projectmodel.BuildTypeScriptModelViaSidecar(context.Background(), snapshot, testMeta(), opts)
			Expect(err).NotTo(HaveOccurred())
			Expect(model.Coverage.Counts).To(HaveKeyWithValue("files_seen", 2), "expected other/c.ts outside the configured src root to be excluded from file collection")
			Expect(model.Snapshot.SelectedRoots).To(Equal([]string{"src"}))
		})

		It("deduplicates files collected once per overlapping/nested root instead of double-sending them", func() {
			snapshot := fstest.MapFS{
				"src/a.ts":        &fstest.MapFile{Data: []byte("import { b } from './nested/b';\nexport const a = b;\n")},
				"src/nested/b.ts": &fstest.MapFile{Data: []byte("export const b = 1;\n")},
			}
			opts := sidecarOptsWithMode("happy")
			opts.Roots = []string{"src", "src/nested"}

			model, err := projectmodel.BuildTypeScriptModelViaSidecar(context.Background(), snapshot, testMeta(), opts)
			Expect(err).NotTo(HaveOccurred())
			Expect(model.Coverage.Complete).To(BeTrue())
			Expect(model.Coverage.Counts).To(HaveKeyWithValue("files_seen", 2), "expected src/nested/b.ts, reachable from both the src and src/nested roots, to be counted once, not twice")
		})
	})

	When("Budgets bounds file collection", func() {
		ts3Snapshot := func() fstest.MapFS {
			return fstest.MapFS{
				"src/a.ts": &fstest.MapFile{Data: []byte("export const a = 'A';\n")},
				"src/b.ts": &fstest.MapFile{Data: []byte("export const b = 'B';\n")},
				"src/c.ts": &fstest.MapFile{Data: []byte("export const c = 'C';\n")},
			}
		}

		It("admits every candidate file when no budget is set", func() {
			model, err := projectmodel.BuildTypeScriptModelViaSidecar(context.Background(), ts3Snapshot(), testMeta(), sidecarOptsWithMode("happy"))
			Expect(err).NotTo(HaveOccurred())
			Expect(model.Coverage.Complete).To(BeTrue())
			Expect(model.Coverage.Counts).To(HaveKeyWithValue("files_seen", 3))
		})

		It("truncates deterministically to the sorted-first path, marks Complete false, and records a budget diagnostic", func() {
			opts := sidecarOptsWithMode("happy")
			opts.Budgets = projectmodel.GoBudgets{MaxInputFiles: 1}

			model, err := projectmodel.BuildTypeScriptModelViaSidecar(context.Background(), ts3Snapshot(), testMeta(), opts)
			Expect(err).NotTo(HaveOccurred())
			Expect(model.Coverage.Complete).To(BeFalse(), "expected the client-imposed input budget to mark the model incomplete even though the fake sidecar itself reports success")
			Expect(model.Coverage.Counts).To(HaveKeyWithValue("files_seen", 1), "expected only the single admitted file to be forwarded to the sidecar")
			Expect(model.ImportEdges).To(ContainElement(projectmodel.ImportEdge{
				From: "file:src/a.ts", To: "export const a = 'A';\n", Kind: "internal", Site: "src/a.ts:1",
			}), "expected the deterministic sorted-first path src/a.ts to be the sole admitted file, got %+v", model.ImportEdges)

			diag, ok := diagnosticWithCode(model.Coverage.Diagnostics, projectmodel.DiagFileBudgetExceeded)
			Expect(ok).To(BeTrue(), "expected a project_file_budget_exceeded diagnostic, got %+v", model.Coverage.Diagnostics)
			Expect(diag.Message).NotTo(BeEmpty())
		})

		It("truncates deterministically by MaxInputBytes, marks Complete false, and records a budget diagnostic", func() {
			opts := sidecarOptsWithMode("happy")
			opts.Budgets = projectmodel.GoBudgets{MaxInputBytes: 30}

			model, err := projectmodel.BuildTypeScriptModelViaSidecar(context.Background(), ts3Snapshot(), testMeta(), opts)
			Expect(err).NotTo(HaveOccurred())
			Expect(model.Coverage.Complete).To(BeFalse(), "expected the client-imposed byte budget to mark the model incomplete even though the fake sidecar itself reports success")
			Expect(model.Coverage.Counts).To(HaveKeyWithValue("files_seen", 1), "expected only the single admitted file (22 bytes, under the 30-byte budget) to be forwarded to the sidecar; a second 22-byte file would exceed it")
			Expect(model.ImportEdges).To(ContainElement(projectmodel.ImportEdge{
				From: "file:src/a.ts", To: "export const a = 'A';\n", Kind: "internal", Site: "src/a.ts:1",
			}), "expected the deterministic sorted-first path src/a.ts to be the sole admitted file, got %+v", model.ImportEdges)

			diag, ok := diagnosticWithCode(model.Coverage.Diagnostics, projectmodel.DiagFileBudgetExceeded)
			Expect(ok).To(BeTrue(), "expected a project_file_budget_exceeded diagnostic, got %+v", model.Coverage.Diagnostics)
			Expect(diag.Message).NotTo(BeEmpty())
		})

		It("produces a byte-identical truncation set across repeated calls with the same budget", func() {
			opts := sidecarOptsWithMode("happy")
			opts.Budgets = projectmodel.GoBudgets{MaxInputFiles: 1}

			first, err := projectmodel.BuildTypeScriptModelViaSidecar(context.Background(), ts3Snapshot(), testMeta(), opts)
			Expect(err).NotTo(HaveOccurred())
			Expect(first.Coverage.Complete).To(BeFalse(), "expected truncation to actually occur so this spec cannot trivially pass on two vacuously-equal untruncated runs")
			_, ok := diagnosticWithCode(first.Coverage.Diagnostics, projectmodel.DiagFileBudgetExceeded)
			Expect(ok).To(BeTrue(), "expected a project_file_budget_exceeded diagnostic confirming truncation happened, got %+v", first.Coverage.Diagnostics)

			second, err := projectmodel.BuildTypeScriptModelViaSidecar(context.Background(), ts3Snapshot(), testMeta(), opts)
			Expect(err).NotTo(HaveOccurred())

			firstJSON, err := json.Marshal(first)
			Expect(err).NotTo(HaveOccurred())
			secondJSON, err := json.Marshal(second)
			Expect(err).NotTo(HaveOccurred())
			Expect(firstJSON).To(Equal(secondJSON), "expected truncation to be deterministic across repeated calls with the same budget")
		})
	})

	When("the sidecar reports call-graph edges and reachability facts", func() {
		It("translates them into Model.CallFacts and Model.ReachabilityFacts with structured paths, confidence, and algorithm version", func() {
			model, err := projectmodel.BuildTypeScriptModelViaSidecar(context.Background(), tsSidecarSnapshot(), testMeta(), sidecarOptsWithMode("reachability"))
			Expect(err).NotTo(HaveOccurred())
			Expect(model.Coverage.Complete).To(BeTrue())
			Expect(model.Coverage.Phase).To(Equal("ts_sidecar_build"))

			Expect(model.CallFacts).To(ContainElement(projectmodel.CallFact{
				From: "file:src/app.ts#getUsers", To: "(PrismaClient).findMany",
			}))

			Expect(model.ReachabilityFacts).To(ContainElement(projectmodel.ReachabilityFact{
				ID:         "reach:file:src/app.ts#getUsers->(PrismaClient).findMany@ts-source-sink-registry@1",
				Kind:       projectmodel.KindPossibleCallReachability,
				Confidence: projectmodel.ReachabilityConfidenceResolvedDirect,
				Source:     "file:src/app.ts#getUsers",
				Sink:       "(PrismaClient).findMany",
				Path: []projectmodel.ReachabilityStep{
					{NodeID: "file:src/app.ts#getUsers"},
					{NodeID: "(PrismaClient).findMany"},
				},
				AlgorithmVersion: "ts-source-sink-registry@1",
			}), "expected the wire fact's ID/Kind/Confidence/Source/Sink/Path/AlgorithmVersion to translate unchanged, got %+v", model.ReachabilityFacts)
		})
	})

	When("the sidecar reports reachability coverage gaps alongside a resolved fact", func() {
		It("passes the gap diagnostic through unchanged while still translating the fact that was resolved", func() {
			model, err := projectmodel.BuildTypeScriptModelViaSidecar(context.Background(), tsSidecarSnapshot(), testMeta(), sidecarOptsWithMode("reachability_gap"))
			Expect(err).NotTo(HaveOccurred())
			Expect(model.Coverage.Complete).To(BeTrue(), "a routine reachability gap must not mark the whole model incomplete -- only reachability's own Coverage.Complete (BuildTypeScriptReachability/BuildTypeScriptLayerBypass) reflects it")

			diag, ok := diagnosticWithCode(model.Coverage.Diagnostics, "ts_reachability_type_only_gap")
			Expect(ok).To(BeTrue(), "expected the sidecar's reachability coverage-gap diagnostic to survive translation, got %+v", model.Coverage.Diagnostics)
			Expect(diag.Path).To(Equal("src/app.ts"))

			Expect(model.ReachabilityFacts).To(HaveLen(1), "expected the one resolved fact to still translate despite the coexisting gap")
			Expect(model.CallFacts).To(HaveLen(1))
		})
	})

	When("the sidecar's project analysis fails to load its tsconfig", func() {
		It("reports diagnostics and skips TS project signals instead of inventing a partial call/reachability graph", func() {
			model, err := projectmodel.BuildTypeScriptModelViaSidecar(context.Background(), tsSidecarSnapshot(), testMeta(), sidecarOptsWithMode("bad_tsconfig"))
			Expect(err).NotTo(HaveOccurred())
			Expect(model.Coverage.Complete).To(BeFalse())

			diag, ok := diagnosticWithCode(model.Coverage.Diagnostics, "ts_config_diagnostic")
			Expect(ok).To(BeTrue(), "expected a ts_config_diagnostic, got %+v", model.Coverage.Diagnostics)
			Expect(diag.Path).To(Equal("tsconfig.json"))

			Expect(model.CallFacts).To(BeEmpty(), "expected no fabricated call facts when tsconfig failed to load")
			Expect(model.ReachabilityFacts).To(BeEmpty(), "expected no fabricated reachability facts when tsconfig failed to load")

			_, hasBackendUnavailable := diagnosticWithCode(model.Coverage.Diagnostics, projectmodel.DiagBackendUnavailable)
			Expect(hasBackendUnavailable).To(BeFalse(), "a bad tsconfig is a degraded-but-successful analysis, not a transport/backend failure")
		})
	})

	When("the sidecar reports a whole-request error", func() {
		It("reports a project_backend_unavailable diagnostic carrying the error's message and kind, merged with any partial diagnostics the sidecar collected", func() {
			model, err := projectmodel.BuildTypeScriptModelViaSidecar(context.Background(), tsSidecarSnapshot(), testMeta(), sidecarOptsWithMode("request_error"))
			Expect(err).NotTo(HaveOccurred())
			Expect(model.Coverage.Complete).To(BeFalse())

			diag, ok := diagnosticWithCode(model.Coverage.Diagnostics, projectmodel.DiagBackendUnavailable)
			Expect(ok).To(BeTrue(), "expected a project_backend_unavailable diagnostic, got %+v", model.Coverage.Diagnostics)
			Expect(diag.Message).To(ContainSubstring("simulated tsconfig read failure"))
			Expect(diag.Message).To(ContainSubstring("kind: crashed"))

			partial, ok := diagnosticWithCode(model.Coverage.Diagnostics, "ts_partial_parse")
			Expect(ok).To(BeTrue(), "expected the sidecar's partial diagnostic to be merged into the model's diagnostics, got %+v", model.Coverage.Diagnostics)
			Expect(partial.Path).To(Equal("src/a.ts"))
		})
	})

	When("the sidecar reports a successful but incomplete response", func() {
		It("passes Coverage.Complete and Coverage.Budgets through unchanged instead of hardcoding completeness", func() {
			model, err := projectmodel.BuildTypeScriptModelViaSidecar(context.Background(), tsSidecarSnapshot(), testMeta(), sidecarOptsWithMode("partial"))
			Expect(err).NotTo(HaveOccurred())
			Expect(model.Coverage.Complete).To(BeFalse(), "expected the sidecar's reported Complete: false to survive, not be hardcoded to true")
			Expect(model.Coverage.Budgets).To(HaveKeyWithValue("wall_time_ms", 1234), "expected Coverage.Budgets to pass through unchanged")

			_, ok := diagnosticWithCode(model.Coverage.Diagnostics, projectmodel.DiagBackendUnavailable)
			Expect(ok).To(BeFalse(), "a partial-but-successful response is not a backend-unavailable condition")
		})
	})

	When("TSSidecarOptions.Dir is set", func() {
		It("spawns the child with that directory as its working directory", func() {
			dir, err := os.MkdirTemp("", "coach-ts-sidecar-cwd-*")
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(os.RemoveAll, dir)

			opts := sidecarOptsWithMode("cwd")
			opts.Dir = dir
			model, err := projectmodel.BuildTypeScriptModelViaSidecar(context.Background(), tsSidecarSnapshot(), testMeta(), opts)
			Expect(err).NotTo(HaveOccurred())

			diag, ok := diagnosticWithCode(model.Coverage.Diagnostics, "cwd_probe")
			Expect(ok).To(BeTrue(), "expected a cwd_probe diagnostic, got %+v", model.Coverage.Diagnostics)
			resolved, err := filepath.EvalSymlinks(dir)
			Expect(err).NotTo(HaveOccurred())
			Expect(diag.Message).To(Equal(resolved))
		})
	})

	When("the sidecar child process is spawned", func() {
		It("does not inherit ambient Node loader, proxy, package-manager, or HOME configuration", func() {
			GinkgoT().Setenv("COACH_TS_SIDECAR_ENV_PROBE", "leaked")
			GinkgoT().Setenv("NODE_OPTIONS", "--require /tmp/coach-confinement-spy.js")
			GinkgoT().Setenv("HTTP_PROXY", "http://127.0.0.1:9")
			GinkgoT().Setenv("npm_config_registry", "http://127.0.0.1:9/registry/")
			GinkgoT().Setenv("HOME", "/tmp/coach-confinement-home")

			model, err := projectmodel.BuildTypeScriptModelViaSidecar(context.Background(), tsSidecarSnapshot(), testMeta(), sidecarOptsWithMode("env"))
			Expect(err).NotTo(HaveOccurred())
			Expect(model.Coverage.Complete).To(BeTrue())

			diag, ok := diagnosticWithCode(model.Coverage.Diagnostics, "env_probe")
			Expect(ok).To(BeTrue(), "expected an env_probe diagnostic, got %+v", model.Coverage.Diagnostics)
			Expect(diag.Message).To(ContainSubstring(`probe=""`), "expected the child not to see the parent's COACH_TS_SIDECAR_ENV_PROBE value")
			Expect(diag.Message).To(ContainSubstring("path=set"), "expected PATH to still be forwarded so the resolved runtime can load dynamic linker search paths")
			Expect(diag.Message).To(ContainSubstring("home=unset"), "HOME must not be forwarded; it carries npmrc and other project-runtime config")
			Expect(diag.Message).To(ContainSubstring("node_options=unset"), "NODE_OPTIONS must not be forwarded; a leaked --require would run in the analyzer")
			Expect(diag.Message).To(ContainSubstring("http_proxy=unset"), "HTTP_PROXY must not be forwarded; a leaked proxy would be a network path")
			Expect(diag.Message).To(ContainSubstring("npm_config_registry=unset"), "npm_config_registry must not be forwarded; it is package-manager configuration")
		})
	})

	When("the configured sidecar binary does not exist", func() {
		It("reports a project_backend_unavailable diagnostic instead of a Go error", func() {
			opts := projectmodel.TSSidecarOptions{
				BinaryPath: "/nonexistent/path/to/coach-ts-sidecar",
				Timeout:    time.Second,
			}
			model, err := projectmodel.BuildTypeScriptModelViaSidecar(context.Background(), tsSidecarSnapshot(), testMeta(), opts)
			Expect(err).NotTo(HaveOccurred())
			Expect(model.Coverage.Complete).To(BeFalse())
			Expect(model.Coverage.Counts).To(HaveKeyWithValue("files_seen", 3), "expected the file-count fallback to still count all three TS/TSX snapshot files even though the sidecar was never reached")

			diag, ok := diagnosticWithCode(model.Coverage.Diagnostics, projectmodel.DiagBackendUnavailable)
			Expect(ok).To(BeTrue(), "expected a project_backend_unavailable diagnostic, got %+v", model.Coverage.Diagnostics)
			Expect(diag.Message).To(ContainSubstring("unavailable"))
		})
	})

	When("the sidecar process exits non-zero", func() {
		It("reports a project_backend_unavailable diagnostic describing the crash and including the sidecar's stderr", func() {
			model, err := projectmodel.BuildTypeScriptModelViaSidecar(context.Background(), tsSidecarSnapshot(), testMeta(), sidecarOptsWithMode("crash"))
			Expect(err).NotTo(HaveOccurred())
			Expect(model.Coverage.Complete).To(BeFalse())

			diag, ok := diagnosticWithCode(model.Coverage.Diagnostics, projectmodel.DiagBackendUnavailable)
			Expect(ok).To(BeTrue(), "expected a project_backend_unavailable diagnostic, got %+v", model.Coverage.Diagnostics)
			Expect(diag.Message).To(ContainSubstring("exited"))
			Expect(diag.Message).To(ContainSubstring("simulated crash"), "expected the sidecar's stderr to surface in the diagnostic message")
		})
	})

	When("the sidecar process exits non-zero after writing far more stderr than the retained-diagnostic budget", func() {
		It("caps the stderr included in the crash diagnostic instead of buffering it unbounded", func() {
			model, err := projectmodel.BuildTypeScriptModelViaSidecar(context.Background(), tsSidecarSnapshot(), testMeta(), sidecarOptsWithMode("crash_noisy"))
			Expect(err).NotTo(HaveOccurred())
			Expect(model.Coverage.Complete).To(BeFalse())

			diag, ok := diagnosticWithCode(model.Coverage.Diagnostics, projectmodel.DiagBackendUnavailable)
			Expect(ok).To(BeTrue(), "expected a project_backend_unavailable diagnostic, got %+v", model.Coverage.Diagnostics)
			Expect(diag.Message).To(ContainSubstring("exited"))
			Expect(len(diag.Message)).To(BeNumerically("<", 8<<10), "expected the child's 64 KiB of stderr to be capped in the diagnostic message, not buffered unbounded")
		})
	})

	When("the sidecar writes extra non-JSON output on stdout after its response line", func() {
		It("parses only the first response line and ignores the stray trailing output", func() {
			model, err := projectmodel.BuildTypeScriptModelViaSidecar(context.Background(), tsSidecarSnapshot(), testMeta(), sidecarOptsWithMode("trailing_output"))
			Expect(err).NotTo(HaveOccurred())
			Expect(model.Coverage.Complete).To(BeTrue())
			Expect(model.Coverage.Diagnostics).To(BeEmpty())
			Expect(model.ImportEdges).To(ContainElement(projectmodel.ImportEdge{
				From: "file:src/a.ts", To: "file:src/b.ts", Kind: "internal", Site: "src/a.ts:1",
			}), "expected the valid first response line to still parse even with stray trailing stdout after it")
		})
	})

	When("the sidecar writes malformed JSON", func() {
		It("reports a project_backend_unavailable diagnostic describing the malformed output", func() {
			model, err := projectmodel.BuildTypeScriptModelViaSidecar(context.Background(), tsSidecarSnapshot(), testMeta(), sidecarOptsWithMode("malformed"))
			Expect(err).NotTo(HaveOccurred())
			Expect(model.Coverage.Complete).To(BeFalse())

			diag, ok := diagnosticWithCode(model.Coverage.Diagnostics, projectmodel.DiagBackendUnavailable)
			Expect(ok).To(BeTrue(), "expected a project_backend_unavailable diagnostic, got %+v", model.Coverage.Diagnostics)
			Expect(diag.Message).To(ContainSubstring("malformed"))
		})
	})

	When("the sidecar writes output exceeding the response budget", func() {
		It("reports a project_backend_unavailable diagnostic instead of buffering unbounded output", func() {
			model, err := projectmodel.BuildTypeScriptModelViaSidecar(context.Background(), tsSidecarSnapshot(), testMeta(), sidecarOptsWithMode("oversized"))
			Expect(err).NotTo(HaveOccurred())
			Expect(model.Coverage.Complete).To(BeFalse())

			diag, ok := diagnosticWithCode(model.Coverage.Diagnostics, projectmodel.DiagBackendUnavailable)
			Expect(ok).To(BeTrue(), "expected a project_backend_unavailable diagnostic, got %+v", model.Coverage.Diagnostics)
			Expect(diag.Message).To(ContainSubstring("budget"))
		})
	})

	When("the sidecar answers with a mismatched protocol version but a correctly echoed correlation id", func() {
		It("reports a project_backend_unavailable diagnostic instead of trusting the mismatched response", func() {
			model, err := projectmodel.BuildTypeScriptModelViaSidecar(context.Background(), tsSidecarSnapshot(), testMeta(), sidecarOptsWithMode("version_mismatch"))
			Expect(err).NotTo(HaveOccurred())
			Expect(model.Coverage.Complete).To(BeFalse())
			Expect(model.ImportEdges).To(BeEmpty())

			diag, ok := diagnosticWithCode(model.Coverage.Diagnostics, projectmodel.DiagBackendUnavailable)
			Expect(ok).To(BeTrue(), "expected a project_backend_unavailable diagnostic, got %+v", model.Coverage.Diagnostics)
			Expect(diag.Message).To(ContainSubstring("protocol mismatch"))
		})
	})

	When("the sidecar answers with a mismatched correlation id but a correctly echoed protocol version", func() {
		It("reports a project_backend_unavailable diagnostic instead of trusting the mismatched response", func() {
			model, err := projectmodel.BuildTypeScriptModelViaSidecar(context.Background(), tsSidecarSnapshot(), testMeta(), sidecarOptsWithMode("id_mismatch"))
			Expect(err).NotTo(HaveOccurred())
			Expect(model.Coverage.Complete).To(BeFalse())
			Expect(model.ImportEdges).To(BeEmpty())

			diag, ok := diagnosticWithCode(model.Coverage.Diagnostics, projectmodel.DiagBackendUnavailable)
			Expect(ok).To(BeTrue(), "expected a project_backend_unavailable diagnostic, got %+v", model.Coverage.Diagnostics)
			Expect(diag.Message).To(ContainSubstring("protocol mismatch"))
		})
	})

	When("the configured sidecar path is not executable", func() {
		It("reports a project_backend_unavailable diagnostic describing the failed start", func() {
			dir, err := os.MkdirTemp("", "ts-sidecar-not-executable-*")
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(os.RemoveAll, dir)

			notExecutable := filepath.Join(dir, "not-executable")
			Expect(os.WriteFile(notExecutable, []byte("#!/bin/sh\necho hi\n"), 0o644)).To(Succeed())

			opts := projectmodel.TSSidecarOptions{
				BinaryPath: notExecutable,
				Timeout:    time.Second,
			}
			model, err := projectmodel.BuildTypeScriptModelViaSidecar(context.Background(), tsSidecarSnapshot(), testMeta(), opts)
			Expect(err).NotTo(HaveOccurred())
			Expect(model.Coverage.Complete).To(BeFalse())

			diag, ok := diagnosticWithCode(model.Coverage.Diagnostics, projectmodel.DiagBackendUnavailable)
			Expect(ok).To(BeTrue(), "expected a project_backend_unavailable diagnostic, got %+v", model.Coverage.Diagnostics)
			Expect(diag.Message).To(ContainSubstring("starting ts sidecar"))
		})
	})

	When("the caller's context is canceled mid-call", func() {
		It("reports a project_backend_unavailable diagnostic distinguishing cancellation from a crash", func() {
			opts := sidecarOptsWithMode("hang")
			opts.Timeout = 0

			ctx, cancel := context.WithCancel(context.Background())
			go func() {
				time.Sleep(200 * time.Millisecond)
				cancel()
			}()

			start := time.Now()
			model, err := projectmodel.BuildTypeScriptModelViaSidecar(ctx, tsSidecarSnapshot(), testMeta(), opts)
			elapsed := time.Since(start)

			Expect(err).NotTo(HaveOccurred())
			Expect(elapsed).To(BeNumerically("<", 5*time.Second), "expected cancellation well before the sidecar's simulated 30s hang ends")
			Expect(model.Coverage.Complete).To(BeFalse())

			diag, ok := diagnosticWithCode(model.Coverage.Diagnostics, projectmodel.DiagBackendUnavailable)
			Expect(ok).To(BeTrue(), "expected a project_backend_unavailable diagnostic, got %+v", model.Coverage.Diagnostics)
			Expect(diag.Message).To(ContainSubstring("ts sidecar canceled"))
			Expect(diag.Message).NotTo(ContainSubstring("exited"))
		})
	})

	When("the sidecar hangs past the configured timeout", func() {
		It("cancels the call well before the hang ends and reports a project_backend_unavailable diagnostic", func() {
			opts := sidecarOptsWithMode("hang")
			opts.Timeout = 300 * time.Millisecond

			start := time.Now()
			model, err := projectmodel.BuildTypeScriptModelViaSidecar(context.Background(), tsSidecarSnapshot(), testMeta(), opts)
			elapsed := time.Since(start)

			Expect(err).NotTo(HaveOccurred())
			Expect(elapsed).To(BeNumerically("<", 5*time.Second), "expected cancellation well before the sidecar's simulated 30s hang ends")
			Expect(model.Coverage.Complete).To(BeFalse())

			diag, ok := diagnosticWithCode(model.Coverage.Diagnostics, projectmodel.DiagBackendUnavailable)
			Expect(ok).To(BeTrue(), "expected a project_backend_unavailable diagnostic, got %+v", model.Coverage.Diagnostics)
			Expect(diag.Message).To(ContainSubstring("timed out"))
		})
	})

	When("the sidecar hangs past a deadline set on the caller's context rather than opts.Timeout", func() {
		It("reports a timeout diagnostic that does not misreport the zero opts.Timeout value", func() {
			opts := sidecarOptsWithMode("hang")
			opts.Timeout = 0

			ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
			defer cancel()

			start := time.Now()
			model, err := projectmodel.BuildTypeScriptModelViaSidecar(ctx, tsSidecarSnapshot(), testMeta(), opts)
			elapsed := time.Since(start)

			Expect(err).NotTo(HaveOccurred())
			Expect(elapsed).To(BeNumerically("<", 5*time.Second), "expected cancellation well before the sidecar's simulated 30s hang ends")
			Expect(model.Coverage.Complete).To(BeFalse())

			diag, ok := diagnosticWithCode(model.Coverage.Diagnostics, projectmodel.DiagBackendUnavailable)
			Expect(ok).To(BeTrue(), "expected a project_backend_unavailable diagnostic, got %+v", model.Coverage.Diagnostics)
			Expect(diag.Message).To(ContainSubstring("timed out"))
			Expect(diag.Message).NotTo(ContainSubstring("after 0s"), "opts.Timeout is 0 here; the deadline came from the caller's context, not opts.Timeout")
		})
	})
})
