package projectmodel_test

import (
	"context"

	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"strings"
	"testing/fstest"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/projectbridge"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func body_tsSidecarIntegrationAcceptanceTest_populatesModelCallFactsAndModelReachabilityFacts_553(ctx context.Context, realOpts func() projectmodel.TSSidecarOptions) {
	snapshot := fstest.MapFS{
		"tsconfig.json": tsconfigJSON(map[string]any{
			"compilerOptions": map[string]any{"module": "commonjs", "moduleResolution": "node10"},
		}),
		"vendor/prisma-client/package.json": tsconfigJSON(map[string]any{"name": "@prisma/client", "main": "index"}),
		"vendor/prisma-client/index.ts": file(strings.Join([]string{
			`export class PrismaClient {`,
			`  user = {`,
			`    findMany(): Promise<unknown[]> {`,
			`      return Promise.resolve([]);`,
			`    },`,
			`  };`,
			`}`,
			``,
		}, "\n")),
		"src/db.ts": file(strings.Join([]string{
			`import { PrismaClient } from "@prisma/client";`,
			`export const prisma = new PrismaClient();`,
			``,
		}, "\n")),
		"src/app.ts": file(strings.Join([]string{
			`import { prisma } from "./db";`,
			``,
			`interface App {`,
			`  get(path: string, handler: (req: unknown, res: unknown) => void): void;`,
			`}`,
			`declare const app: App;`,
			``,
			`export async function getUsers(req: unknown, res: unknown): Promise<void> {`,
			`  const users = await prisma.user.findMany();`,
			`  console.log(users, req, res);`,
			`}`,
			`app.get("/users", getUsers);`,
			``,
		}, "\n")),
	}

	model, err := projectmodel.BuildTypeScriptModelViaSidecar(ctx, snapshot, testMeta(), realOpts())
	Expect(err).NotTo(HaveOccurred())
	Expect(model.Coverage.Complete).To(BeTrue(), "%+v", model.Coverage)

	const source = "file:src/app.ts#getUsers"
	const sink = "(PrismaClient).findMany"

	var callFact projectmodel.CallFact
	foundCallFact := false
	for _, f := range model.CallFacts {
		if f.From == source && f.To == sink {
			callFact = f
			foundCallFact = true
			break
		}
	}
	Expect(foundCallFact).To(BeTrue(), "expected a CallFact from %q to %q, got %+v", source, sink, model.CallFacts)
	Expect(callFact).To(Equal(projectmodel.CallFact{From: source, To: sink}))

	var reachFact projectmodel.ReachabilityFact
	foundReachFact := false
	for _, f := range model.ReachabilityFacts {
		if f.Source == source && f.Sink == sink {
			reachFact = f
			foundReachFact = true
			break
		}
	}
	Expect(foundReachFact).To(BeTrue(), "expected a ReachabilityFact from %q to %q, got %+v", source, sink, model.ReachabilityFacts)
	Expect(reachFact.Kind).To(Equal(projectmodel.KindPossibleCallReachability))
	Expect(reachFact.Confidence).To(Equal(projectmodel.ReachabilityConfidenceResolvedDirect))
	Expect(reachFact.AlgorithmVersion).To(Equal(projectmodel.TSReachabilityAlgorithm))
	Expect(reachFact.Path).To(HaveLen(2), "expected a structured source-to-sink path, got %+v", reachFact.Path)
	Expect(reachFact.Path[0].NodeID).To(Equal(source))
	Expect(reachFact.Path[len(reachFact.Path)-1].NodeID).To(Equal(sink))
}

func body_tsSidecarIntegrationAcceptanceTest_stillProducesTheUnrelatedBypassWitnessInsteadOfS_631(ctx context.Context, realOpts func() projectmodel.TSSidecarOptions) {
	snapshot := fstest.MapFS{
		"tsconfig.json": tsconfigJSON(map[string]any{
			"compilerOptions": map[string]any{"module": "commonjs", "moduleResolution": "node10"},
		}),
		"vendor/prisma-client/package.json": tsconfigJSON(map[string]any{"name": "@prisma/client", "main": "index"}),
		"vendor/prisma-client/index.ts": file(strings.Join([]string{
			`export class PrismaClient {`,
			`  user = {`,
			`    findMany(): Promise<unknown[]> {`,
			`      return Promise.resolve([]);`,
			`    },`,
			`  };`,
			`}`,
			``,
		}, "\n")),
		"src/db.ts": file(strings.Join([]string{
			`import { PrismaClient } from "@prisma/client";`,
			`export const prisma = new PrismaClient();`,
			``,
		}, "\n")),
		"service/userService.ts": file(strings.Join([]string{
			`export function loadUsers(): unknown[] {`,
			`  return [];`,
			`}`,
			``,
		}, "\n")),
		"handlers/compliant.ts": file(strings.Join([]string{
			`import { loadUsers } from "../service/userService";`,
			``,
			`interface App {`,
			`  get(path: string, handler: (req: unknown, res: unknown) => void): void;`,
			`}`,
			`declare const app: App;`,
			``,
			`export function getUsersCompliant(req: unknown, res: unknown): void {`,
			`  const users = loadUsers();`,
			`  console.log(users, req, res);`,
			`}`,
			`app.get("/users-compliant", getUsersCompliant);`,
			``,
		}, "\n")),
		"handlers/bypass.ts": file(strings.Join([]string{
			`import { prisma } from "../src/db";`,
			``,
			`interface App {`,
			`  get(path: string, handler: (req: unknown, res: unknown) => void): void;`,
			`}`,
			`declare const app: App;`,
			``,
			`export async function getUsersBypass(req: unknown, res: unknown): Promise<void> {`,
			`  const users = await prisma.user.findMany();`,
			`  console.log(users, req, res);`,
			`}`,
			`app.get("/users-bypass", getUsersBypass);`,
			``,
		}, "\n")),
	}

	model, err := projectmodel.BuildTypeScriptModelViaSidecar(ctx, snapshot, testMeta(), realOpts())
	Expect(err).NotTo(HaveOccurred())
	Expect(model.Coverage.Complete).To(BeTrue(), "a routine depth-1 reachability gap must not mark the whole model incomplete -- that bit is what coach codesignal --project-language typescript publishes as project_coverage, and it must not degrade an unrelated architecture.layer_violation to lifecycle unknown for the ordinary shape of layered code; got %+v", model.Coverage)
	_, hasGapDiag := diagnosticWithCode(model.Coverage.Diagnostics, "ts_reachability_local_call_not_followed_gap")
	Expect(hasGapDiag).To(BeTrue(), "expected this fixture to genuinely reproduce the routine local-call-not-followed gap, got %+v", model.Coverage.Diagnostics)

	const bypassSource = "file:handlers/bypass.ts#getUsersBypass"
	const sink = "(PrismaClient).findMany"
	foundBypassEdge := false
	for _, f := range model.CallFacts {
		if f.From == bypassSource && f.To == sink {
			foundBypassEdge = true
		}
	}
	Expect(foundBypassEdge).To(BeTrue(), "expected the real sidecar to resolve the direct bypass edge despite the unrelated gap elsewhere, got %+v", model.CallFacts)

	result, err := projectmodel.BuildTypeScriptLayerBypass(ctx, snapshot, testMeta(), realOpts(), projectmodel.BypassLayer{Name: "service", Prefixes: []string{"service"}})
	Expect(err).NotTo(HaveOccurred())

	Expect(result.Witnesses).To(HaveLen(1), "expected the unrelated, fully-resolved bypass witness to survive an unrelated compliant handler's routine gap, got %+v (Coverage: %+v)", result.Witnesses, result.Coverage)
	witness := result.Witnesses[0]
	Expect(witness.Source).To(Equal(bypassSource))
	Expect(witness.Sink).To(Equal(sink))
	Expect(witness.RequiredLayer).To(Equal("service"))
	Expect(witness.Confidence).To(Equal(projectmodel.LayerBypassConfidenceHigh))

	Expect(result.Coverage.Complete).To(BeFalse(), "expected the unrelated gap to still surface as aggregate incompleteness -- the per-pair gate must not erase the signal that some other part of this run was truncated, got %+v", result.Coverage)
}

func body_tsSidecarIntegrationAcceptanceTest_stillProducesOneCompleteValidModelInsteadOfTrunc_752(ctx context.Context, realOpts func() projectmodel.TSSidecarOptions) {
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
}

func body_tsSidecarIntegrationAcceptanceTest_carriesThatIdentityThroughToModelSnapshotBackend_872(ctx context.Context, realOpts func() projectmodel.TSSidecarOptions) {

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
}
