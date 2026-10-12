package projectmodel_test

import (
	"context"
	"strings"
	"testing/fstest"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func expectRouteHandlerReachabilityFacts(ctx context.Context, realOpts func() projectmodel.TSSidecarOptions) {
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

func expectUnrelatedBypassWitnessSurvivesReachabilityGap(ctx context.Context, realOpts func() projectmodel.TSSidecarOptions) {
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
