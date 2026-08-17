/**
 * Acceptance coverage for the pinned Node/TypeScript project sidecar
 * (issue #214 Task 2): spawns the compiled bin/coach-ts-project-sidecar
 * binary as a real child process -- the same way Task 1's Go client
 * (pkg/projectmodel/ts_sidecar.go) invokes it -- writes one
 * internal/projectbridge.Request NDJSON line to its stdin, and reads one
 * Response line back from stdout.
 */
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { before, test } from "node:test";

import { edgesTo, file, runSidecar, type WireFile } from "./project-sidecar-harness.js";

const PACKAGE_ROOT = fileURLToPath(new URL("..", import.meta.url));

before(() => {
  execFileSync("npm", ["run", "build:project-sidecar"], { cwd: PACKAGE_ROOT, stdio: "pipe" });
});

test("aliases: compilerOptions.paths/baseUrl resolve an aliased import to its target file", async () => {
  const { response, exitCode } = await runSidecar({
    files: [
      file(
        "tsconfig.json",
        JSON.stringify({
          compilerOptions: { module: "commonjs", moduleResolution: "node10", baseUrl: ".", paths: { "@lib/*": ["src/lib/*"] } },
        }),
      ),
      file("src/a.ts", `import { helper } from "@lib/util";\nconsole.log(helper);\n`),
      file("src/lib/util.ts", `export const helper = 1;\n`),
    ],
  });
  assert.equal(exitCode, 0);
  assert.equal(response.error, undefined);
  const edges = edgesTo(response, "file:src/lib/util.ts");
  assert.equal(edges.length, 1, JSON.stringify(response));
  assert.equal(edges[0]?.from, "file:src/a.ts");
  assert.equal(edges[0]?.kind, "import");
  assert.equal(edges[0]?.resolution, "snapshot");
});

test("stable IDs: mixed-case snapshot paths are preserved byte-for-byte in from/to/site", async () => {
  // On case-insensitive hosts, TS's checker lowercases declaration paths.
  // Stable file: IDs must still match the request inventory exactly so
  // Mac and Linux emit byte-identical facts for the same snapshot.
  const { response, exitCode } = await runSidecar({
    files: [
      file(
        "tsconfig.json",
        JSON.stringify({ compilerOptions: { module: "commonjs", moduleResolution: "node10" } }),
      ),
      file("Src/App.ts", `import { helper } from "./Lib/Util";\nexport { onlyY } from "./onlyY";\nconsole.log(helper);\n`),
      file("Src/Lib/Util.ts", `export const helper = 1;\n`),
      file("Src/onlyY.ts", `export const onlyY = 1;\n`),
    ],
  });
  assert.equal(exitCode, 0);
  assert.equal(response.error, undefined, JSON.stringify(response));

  const utilEdge = edgesTo(response, "file:Src/Lib/Util.ts");
  assert.equal(utilEdge.length, 1, JSON.stringify(response));
  assert.equal(utilEdge[0]?.from, "file:Src/App.ts");
  assert.equal(utilEdge[0]?.site, "Src/App.ts:1");
  assert.equal(utilEdge[0]?.resolution, "snapshot");

  const onlyYEdge = edgesTo(response, "file:Src/onlyY.ts");
  assert.equal(onlyYEdge.length, 1, JSON.stringify(response));
  assert.equal(onlyYEdge[0]?.from, "file:Src/App.ts");
  assert.equal(onlyYEdge[0]?.kind, "reexport");
  assert.equal(onlyYEdge[0]?.resolution, "snapshot");
});

test("references: an import crossing a tsconfig project-reference boundary resolves to the referenced project's file", async () => {
  // The alias target ("../libpkg/dist/*") only exists via reference-driven
  // redirection: libpkg's tsconfig declares rootDir "src"/outDir "dist" but
  // the snapshot never contains a built libpkg/dist/*, so this can only
  // resolve to file:libpkg/src/helper.ts through TS's own project-reference
  // source-redirect, not through paths/baseUrl matching a real snapshot
  // path (which the "aliases" spec above already covers on its own).
  const { response, exitCode } = await runSidecar({
    files: [
      file(
        "app/tsconfig.json",
        JSON.stringify({
          compilerOptions: {
            composite: true,
            module: "commonjs",
            moduleResolution: "node10",
            baseUrl: ".",
            paths: { "libpkg/*": ["../libpkg/dist/*"] },
          },
          references: [{ path: "../libpkg" }],
          include: ["src/**/*"],
        }),
      ),
      file("app/src/a.ts", `import { helper } from "libpkg/helper";\nconsole.log(helper);\n`),
      file(
        "libpkg/tsconfig.json",
        JSON.stringify({
          compilerOptions: { composite: true, module: "commonjs", moduleResolution: "node10", rootDir: "src", outDir: "dist" },
          include: ["src/**/*"],
        }),
      ),
      file("libpkg/src/helper.ts", `export const helper = 1;\n`),
    ],
  });
  assert.equal(exitCode, 0);
  assert.equal(response.error, undefined);
  const edges = edgesTo(response, "file:libpkg/src/helper.ts");
  assert.equal(edges.length, 1, JSON.stringify(response));
  assert.equal(edges[0]?.from, "file:app/src/a.ts");
  assert.equal(edges[0]?.resolution, "snapshot");
  // Both projects were actually opened and walked by the sidecar (not just
  // found as tsconfig.json files in the snapshot -- see counts.tsconfig_count
  // for that, which is set before any project is opened).
  assert.equal(response.coverage.counts?.projects_analyzed, 2, JSON.stringify(response.coverage));
});

test("exports/re-exports: a package.json exports map and barrel re-exports both resolve", async () => {
  const { response, exitCode } = await runSidecar({
    files: [
      file("tsconfig.json", JSON.stringify({ compilerOptions: { module: "nodenext", moduleResolution: "nodenext" } })),
      file(
        "src/a.ts",
        `import { z } from "@acme/lib";\nimport { w } from "@acme/lib/sub";\nconsole.log(z, w);\n`,
      ),
      file(
        "packages/lib/package.json",
        JSON.stringify({ name: "@acme/lib", exports: { ".": "./src/index.ts", "./sub": "./src/sub.ts" } }),
      ),
      // Barrel: index.ts re-exports through both `export * from` and `export { x } from`.
      file("packages/lib/src/index.ts", `export * from "./inner";\nexport { onlyY } from "./onlyY";\n`),
      file("packages/lib/src/inner.ts", `export const z = 42;\n`),
      file("packages/lib/src/onlyY.ts", `export const onlyY = 1;\n`),
      file("packages/lib/src/sub.ts", `export const w = 7;\n`),
    ],
  });
  assert.equal(exitCode, 0, JSON.stringify(response));
  assert.equal(response.error, undefined);

  const mainImport = edgesTo(response, "file:packages/lib/src/index.ts");
  assert.equal(mainImport.length, 1, JSON.stringify(response));
  assert.equal(mainImport[0]?.resolution, "snapshot");

  const subImport = edgesTo(response, "file:packages/lib/src/sub.ts");
  assert.equal(subImport.length, 1, JSON.stringify(response));

  const barrelStarReexport = edgesTo(response, "file:packages/lib/src/inner.ts");
  assert.equal(barrelStarReexport.length, 1, JSON.stringify(response));
  assert.equal(barrelStarReexport[0]?.from, "file:packages/lib/src/index.ts");
  assert.equal(barrelStarReexport[0]?.kind, "reexport");

  const barrelNamedReexport = edgesTo(response, "file:packages/lib/src/onlyY.ts");
  assert.equal(barrelNamedReexport.length, 1, JSON.stringify(response));
  assert.equal(barrelNamedReexport[0]?.kind, "reexport");
});

test("commonjs/dynamic/type-only imports each produce an edge with a distinct Kind", async () => {
  const { response, exitCode } = await runSidecar({
    files: [
      file("tsconfig.json", JSON.stringify({ compilerOptions: { module: "commonjs", moduleResolution: "node10" } })),
      file(
        "src/a.ts",
        [
          `const cjs = require("./cjs-target");`,
          `import type { T } from "./type-only-target";`,
          `async function f(): Promise<void> {`,
          `  const dyn = await import("./dynamic-target");`,
          `  console.log(cjs, dyn);`,
          `}`,
          `export type UsesT = T;`,
          ``,
        ].join("\n"),
      ),
      file("src/cjs-target.ts", `export const cjsValue = 1;\n`),
      file("src/dynamic-target.ts", `export const dynValue = 1;\n`),
      file("src/type-only-target.ts", `export type T = number;\n`),
    ],
  });
  assert.equal(exitCode, 0, JSON.stringify(response));
  assert.equal(response.error, undefined);

  const cjsEdges = edgesTo(response, "file:src/cjs-target.ts");
  assert.equal(cjsEdges.length, 1, JSON.stringify(response));
  assert.equal(cjsEdges[0]?.kind, "commonjs_require");

  const dynEdges = edgesTo(response, "file:src/dynamic-target.ts");
  assert.equal(dynEdges.length, 1, JSON.stringify(response));
  assert.equal(dynEdges[0]?.kind, "dynamic_import");

  const typeOnlyEdges = edgesTo(response, "file:src/type-only-target.ts");
  assert.equal(typeOnlyEdges.length, 1, JSON.stringify(response));
  assert.equal(typeOnlyEdges[0]?.kind, "type_only");

  const kinds = new Set(response.import_edges?.map((e) => e.kind));
  assert.equal(kinds.size, 3, `expected 3 distinct kinds, got ${JSON.stringify([...kinds])}`);
});

test("inline type-only named imports: `import { type T }` classifies as type_only, not a value import", async () => {
  const { response, exitCode } = await runSidecar({
    files: [
      file("tsconfig.json", JSON.stringify({ compilerOptions: { module: "commonjs", moduleResolution: "node10" } })),
      file("src/a.ts", `import { type T } from "./b";\nexport type UsesT = T;\n`),
      file("src/b.ts", `export type T = number;\nexport const v = 1;\n`),
    ],
  });
  assert.equal(exitCode, 0, JSON.stringify(response));
  assert.equal(response.error, undefined);
  const edges = edgesTo(response, "file:src/b.ts");
  assert.equal(edges.length, 1, JSON.stringify(response));
  assert.equal(edges[0]?.kind, "type_only", JSON.stringify(response));
  assert.equal(edges[0]?.resolution, "snapshot");
});

test("mixed inline type-only and value named imports: `import { type T, v }` classifies as a value import", async () => {
  const { response, exitCode } = await runSidecar({
    files: [
      file("tsconfig.json", JSON.stringify({ compilerOptions: { module: "commonjs", moduleResolution: "node10" } })),
      file("src/a.ts", `import { type T, v } from "./b";\nexport type UsesT = T;\nconsole.log(v);\n`),
      file("src/b.ts", `export type T = number;\nexport const v = 1;\n`),
    ],
  });
  assert.equal(exitCode, 0, JSON.stringify(response));
  assert.equal(response.error, undefined);
  const edges = edgesTo(response, "file:src/b.ts");
  assert.equal(edges.length, 1, JSON.stringify(response));
  assert.equal(edges[0]?.kind, "import", JSON.stringify(response));
});

test("inline type-only named re-exports: `export { type T } from` classifies as type_only, not a value reexport", async () => {
  const { response, exitCode } = await runSidecar({
    files: [
      file("tsconfig.json", JSON.stringify({ compilerOptions: { module: "commonjs", moduleResolution: "node10" } })),
      file("src/a.ts", `export { type T } from "./b";\n`),
      file("src/b.ts", `export type T = number;\nexport const v = 1;\n`),
    ],
  });
  assert.equal(exitCode, 0, JSON.stringify(response));
  assert.equal(response.error, undefined);
  const edges = edgesTo(response, "file:src/b.ts");
  assert.equal(edges.length, 1, JSON.stringify(response));
  assert.equal(edges[0]?.kind, "type_only", JSON.stringify(response));
});

test("false-complete guard: .ts sources with no discovered tsconfig.json report Complete=false with a diagnostic", async () => {
  const { response, exitCode } = await runSidecar({
    files: [file("src/a.ts", `export const a = 1;\n`), file("src/b.ts", `export const b = 1;\n`)],
  });
  assert.equal(exitCode, 0, JSON.stringify(response));
  assert.equal(response.error, undefined);
  assert.equal(response.coverage.complete, false, JSON.stringify(response.coverage));
  assert.ok(
    response.coverage.diagnostics?.some((d) => d.code === "ts_no_project_config"),
    JSON.stringify(response.coverage),
  );
});

test("vacuous snapshot: no .ts/.tsx sources and no tsconfig.json stays Complete=true with empty edges", async () => {
  const { response, exitCode } = await runSidecar({
    files: [file("README.md", "not typescript\n")],
  });
  assert.equal(exitCode, 0, JSON.stringify(response));
  assert.equal(response.error, undefined);
  assert.equal(response.coverage.complete, true, JSON.stringify(response.coverage));
  assert.equal(response.import_edges, undefined, JSON.stringify(response));
});

test("snapshot confinement: an import resolving only via real disk is reported unresolved, never read", async () => {
  const realDir = mkdtempSync(join(tmpdir(), "coach-ts-sidecar-leak-"));
  const realFile = join(realDir, "leak-marker.ts");
  const marker = "LEAKED_MARKER_SHOULD_NEVER_APPEAR_IN_SIDECAR_OUTPUT";
  writeFileSync(realFile, `export const LEAKED_MARKER = "${marker}";\n`);
  try {
    const realSpecifier = realFile.slice(0, -3); // strip ".ts"; TS resolvers append extensions themselves
    const { response, rawLine, exitCode } = await runSidecar({
      files: [
        file("tsconfig.json", JSON.stringify({ compilerOptions: { module: "commonjs", moduleResolution: "node10" } })),
        file("src/a.ts", `import { LEAKED_MARKER } from ${JSON.stringify(realSpecifier)};\nconsole.log(LEAKED_MARKER);\n`),
      ],
    });
    assert.equal(exitCode, 0);
    assert.ok(!rawLine.includes(marker), `sidecar output leaked real-disk content: ${rawLine}`);
    assert.ok(!rawLine.includes("LEAKED_MARKER_SHOULD"), rawLine);
    const edges = response.import_edges ?? [];
    assert.equal(edges.length, 1, JSON.stringify(response));
    // An absolute, non-relative specifier is classified "external" by this
    // sidecar's resolver (see resolve.ts) -- the safety property under test
    // is that it is never classified "snapshot"/`file:...`, i.e. never
    // reported as if it were read from the real filesystem.
    assert.notEqual(edges[0]?.resolution, "snapshot", JSON.stringify(response));
    assert.ok(!edges[0]?.to.startsWith("file:"), JSON.stringify(response));
  } finally {
    rmSync(realDir, { recursive: true, force: true });
  }
});

test("snapshot confinement: tsconfig 'extends' targeting a real, existing disk path is never silently read", async () => {
  // Unlike the specifier-resolution case above, this drives content that
  // genuinely exists on disk outside the snapshot straight at vfs.ts's
  // FileSystem.readFile during tsgo's own config-file loading (the "extends"
  // resolution path), which is exactly the fall-through guard vfs.ts's
  // comment calls out as load-bearing for snapshot confinement. A
  // real-disk fixture is required here: an in-snapshot-only fixture never
  // reaches this code path, because tsgo's fileExists check on the
  // synthetic VFS already reports the extended config as absent before
  // readFile would even be attempted with an ambiguous return value.
  const realDir = mkdtempSync(join(tmpdir(), "coach-ts-sidecar-extends-"));
  const baseConfigPath = join(realDir, "base.json");
  writeFileSync(baseConfigPath, JSON.stringify({ compilerOptions: { strict: true } }));
  try {
    const { response, exitCode } = await runSidecar({
      files: [
        file(
          "tsconfig.json",
          JSON.stringify({ extends: baseConfigPath, compilerOptions: { module: "commonjs", moduleResolution: "node10" } }),
        ),
        file("src/a.ts", `export const a = 1;\n`),
      ],
    });
    assert.equal(exitCode, 0);
    // If vfs.ts's readFile fell through to the real filesystem instead of
    // confining reads to the snapshot, "extends" would resolve cleanly and
    // no diagnostic would be produced at all -- see the Implementer Report
    // for the reproduced before/after with vfs.ts's fall-through guard
    // deliberately removed.
    assert.ok(
      response.coverage.diagnostics?.some((d) => d.code === "ts_config_diagnostic" && d.message.includes("Cannot read file")),
      JSON.stringify(response.coverage),
    );
  } finally {
    rmSync(realDir, { recursive: true, force: true });
  }
});

test("bounded output: a large synthetic snapshot still produces a single valid response", async () => {
  const fileCount = 150;
  const files: WireFile[] = [
    file("tsconfig.json", JSON.stringify({ compilerOptions: { module: "commonjs", moduleResolution: "node10" } })),
  ];
  for (let i = 0; i < fileCount; i++) {
    const isLast = i === fileCount - 1;
    const body = isLast
      ? `export const v${i} = ${i};\n`
      : `export { v${i + 1} } from "./m${i + 1}";\nexport const v${i} = ${i};\n`;
    files.push(file(`src/m${i}.ts`, body));
  }
  const started = Date.now();
  const { response, exitCode } = await runSidecar({ files });
  const elapsedMs = Date.now() - started;

  assert.equal(exitCode, 0);
  assert.equal(response.error, undefined);
  assert.equal(response.coverage.complete, true, JSON.stringify(response.coverage));
  assert.equal(response.import_edges?.length, fileCount - 1, `expected ${fileCount - 1} re-export edges`);
  // Not a hard product guarantee -- just proves this snapshot size does not
  // pathologically blow up; see the Implementer Report for what "bounded"
  // means for this design (timeout_ms self-enforcement, not a hardcoded cap).
  assert.ok(elapsedMs < 20000, `analysis took ${elapsedMs}ms, expected well under 20s for ${fileCount} trivial files`);
});

test("cancellation via timeout_ms: a small timeout_ms stops analysis early; 0 means no enforcement", async () => {
  const twoProjectFiles: WireFile[] = [
    file("proj1/tsconfig.json", JSON.stringify({ compilerOptions: { module: "commonjs", moduleResolution: "node10" } })),
    file("proj1/src/a.ts", `import { b } from "./b";\nconsole.log(b);\n`),
    file("proj1/src/b.ts", `export const b = 1;\n`),
    file("proj2/tsconfig.json", JSON.stringify({ compilerOptions: { module: "commonjs", moduleResolution: "node10" } })),
    file("proj2/src/a.ts", `import { b } from "./b";\nconsole.log(b);\n`),
    file("proj2/src/b.ts", `export const b = 1;\n`),
  ];

  const delayed = await runSidecar(
    { files: twoProjectFiles, timeout_ms: 5 },
    { COACH_TS_SIDECAR_TEST_DELAY_MS: "200" },
  );
  assert.equal(delayed.exitCode, 0);
  assert.equal(delayed.response.coverage.complete, false, JSON.stringify(delayed.response.coverage));
  assert.ok(
    delayed.response.coverage.diagnostics?.some((d) => d.code === "ts_sidecar_timeout"),
    JSON.stringify(delayed.response.coverage),
  );
  assert.ok(
    (delayed.response.coverage.counts?.projects_analyzed ?? 0) < 2,
    JSON.stringify(delayed.response.coverage),
  );
  // A deadline was in effect for this request, so it must be reported
  // regardless of which of the two timeout-enforcement checks (pre-analysis
  // vs. mid-loop) actually tripped -- this run trips the mid-loop one.
  assert.equal(delayed.response.coverage.budgets?.timeout_ms, 5, JSON.stringify(delayed.response.coverage));

  const unbounded = await runSidecar(
    { files: twoProjectFiles, timeout_ms: 0 },
    { COACH_TS_SIDECAR_TEST_DELAY_MS: "200" },
  );
  assert.equal(unbounded.exitCode, 0);
  assert.equal(unbounded.response.coverage.complete, true, JSON.stringify(unbounded.response.coverage));
  assert.equal(unbounded.response.coverage.counts?.projects_analyzed, 2, JSON.stringify(unbounded.response.coverage));
});

test("invalid config: an unparseable tsconfig.json degrades to a diagnostic, not a crash", async () => {
  const { response, exitCode } = await runSidecar({
    files: [
      file("tsconfig.json", "{ this is not valid json !!! "),
      file("src/a.ts", `export const a = 1;\n`),
    ],
  });
  assert.equal(exitCode, 0);
  assert.equal(response.error, undefined, JSON.stringify(response));
  assert.ok(
    response.coverage.diagnostics?.some((d) => d.code === "ts_config_diagnostic"),
    JSON.stringify(response.coverage),
  );
  // A tsconfig that failed to parse was never honored, so this response
  // must not claim a complete model even though the sidecar didn't crash.
  assert.equal(response.coverage.complete, false, JSON.stringify(response.coverage));
});

/**
 * A `@prisma/client` package mirrored into the snapshot's node_modules
 * (see vfs.ts's package-mirroring), so PrismaClient has real module
 * provenance -- REACHABILITY_SINK_CLASSES only matches a class declared
 * under this module specifier, not any in-snapshot class sharing the name
 * (see the "sink matching requires module provenance" spec below).
 */
function prismaPackageFiles(): WireFile[] {
  return [
    file("vendor/prisma-client/package.json", JSON.stringify({ name: "@prisma/client", main: "index" })),
    file(
      "vendor/prisma-client/index.ts",
      [
        `export class PrismaClient {`,
        `  user = {`,
        `    findMany(): Promise<unknown[]> {`,
        `      return Promise.resolve([]);`,
        `    },`,
        `    delete(): void {},`,
        `  };`,
        `}`,
        ``,
      ].join("\n"),
    ),
  ];
}

function reachabilityTsconfig(): WireFile {
  return file("tsconfig.json", JSON.stringify({ compilerOptions: { module: "commonjs", moduleResolution: "node10" } }));
}

test("TS reachability: resolved route-to-query facts, explicit coverage gaps, and a bad-tsconfig skip", async () => {
  const { response, exitCode } = await runSidecar({
    files: [
      reachabilityTsconfig(),
      ...prismaPackageFiles(),
      file(
        "src/db.ts",
        [`import { PrismaClient } from "@prisma/client";`, `export const prisma = new PrismaClient();`, ``].join("\n"),
      ),
      file("src/helpers.ts", `export function helperFn(): void {}\n`),
      file(
        "src/dynamic-handler.ts",
        `export function handler(req: unknown, res: unknown): void {\n  console.log(req, res);\n}\n`,
      ),
      file(
        "src/app.ts",
        [
          `import { prisma } from "./db";`,
          `import { type helperFn } from "./helpers";`,
          `import unknownThing from "unresolved-external-package";`,
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
          `export function callsTypeOnly(req: unknown, res: unknown): void {`,
          `  helperFn();`,
          `  console.log(req, res);`,
          `}`,
          `app.get("/type-only", callsTypeOnly);`,
          ``,
          `export function callsUnresolvedExternal(req: unknown, res: unknown): void {`,
          `  unknownThing.doSomething();`,
          `  console.log(req, res);`,
          `}`,
          `app.get("/external", callsUnresolvedExternal);`,
          ``,
          `async function registerDynamicRoute(): Promise<void> {`,
          `  app.get("/dynamic", (await import("./dynamic-handler")).handler);`,
          `}`,
          `void registerDynamicRoute();`,
          ``,
        ].join("\n"),
      ),
    ],
  });

  assert.equal(exitCode, 0, JSON.stringify(response));
  assert.equal(response.error, undefined, JSON.stringify(response));

  // Resolved case: getUsers -> prisma.user.findMany() is a fully resolved
  // possible-call-reachability fact, using the same vocabulary as Go's
  // ReachabilityFact (Kind/Confidence/AlgorithmVersion), not an active Signal.
  const facts = response.reachability_facts ?? [];
  const resolved = facts.filter((f) => f.source.endsWith("#getUsers"));
  assert.equal(resolved.length, 1, JSON.stringify(response.reachability_facts));
  assert.equal(resolved[0]?.sink, "(PrismaClient).findMany", JSON.stringify(resolved[0]));
  assert.equal(resolved[0]?.confidence, "resolved_direct", JSON.stringify(resolved[0]));
  assert.equal(resolved[0]?.kind, "possible_call_reachability", JSON.stringify(resolved[0]));
  assert.ok(resolved[0]?.algorithm_version, JSON.stringify(resolved[0]));
  assert.ok((resolved[0]?.path.length ?? 0) >= 2, JSON.stringify(resolved[0]));
  // backend carries this sidecar's own language/backend provenance, the
  // same way Coverage.phase does, so a fact can be attributed to "this TS
  // sidecar" rather than looking indistinguishable from a Go-side fact.
  assert.equal(resolved[0]?.backend, "ts_project_sidecar", JSON.stringify(resolved[0]));

  // No fabricated facts for the dynamic-import, unresolved-external-type, or
  // type-only handlers -- their absence must come with an explicit coverage
  // gap below, not silence.
  assert.equal(facts.some((f) => f.source.endsWith("#callsTypeOnly")), false, JSON.stringify(facts));
  assert.equal(facts.some((f) => f.source.endsWith("#callsUnresolvedExternal")), false, JSON.stringify(facts));
  assert.equal(facts.some((f) => f.source.includes("dynamic")), false, JSON.stringify(facts));

  const diagCodes = new Set((response.coverage.diagnostics ?? []).map((d) => d.code));
  assert.ok(diagCodes.has("ts_reachability_type_only_gap"), JSON.stringify(response.coverage));
  assert.ok(diagCodes.has("ts_reachability_unresolved_type_gap"), JSON.stringify(response.coverage));
  assert.ok(diagCodes.has("ts_reachability_dynamic_import_gap"), JSON.stringify(response.coverage));

  // A response carrying acknowledged ts_reachability_*_gap diagnostics must
  // not also claim Complete=true -- a consumer reading only Coverage.Complete
  // would otherwise see a fully-covered model despite the gaps above.
  assert.equal(response.coverage.complete, false, JSON.stringify(response.coverage));

  // The raw call graph carries the same resolved edge the fact's path used.
  const callGraph = response.call_graph ?? [];
  assert.ok(
    callGraph.some((e) => e.from.endsWith("#getUsers") && e.to === "(PrismaClient).findMany"),
    JSON.stringify(callGraph),
  );

  const badConfig = await runSidecar({
    files: [file("tsconfig.json", "{ this is not valid json !!! "), file("src/a.ts", `export const a = 1;\n`)],
  });
  assert.equal(badConfig.exitCode, 0);
  assert.equal(badConfig.response.coverage.complete, false, JSON.stringify(badConfig.response.coverage));
  assert.ok(
    badConfig.response.coverage.diagnostics?.some((d) => d.code === "ts_config_diagnostic"),
    JSON.stringify(badConfig.response.coverage),
  );
  assert.equal(badConfig.response.call_graph, undefined, JSON.stringify(badConfig.response));
  assert.equal(badConfig.response.reachability_facts, undefined, JSON.stringify(badConfig.response));
});

test("TS reachability: a handler imported from another file still resolves as a source", async () => {
  const { response, exitCode } = await runSidecar({
    files: [
      reachabilityTsconfig(),
      ...prismaPackageFiles(),
      file(
        "src/db.ts",
        [`import { PrismaClient } from "@prisma/client";`, `export const prisma = new PrismaClient();`, ``].join("\n"),
      ),
      file(
        "src/handlers.ts",
        [
          `import { prisma } from "./db";`,
          `export async function h(req: unknown, res: unknown): Promise<void> {`,
          `  await prisma.user.findMany();`,
          `  console.log(req, res);`,
          `}`,
        ].join("\n"),
      ),
      file(
        "src/app.ts",
        [
          `import { h } from "./handlers";`,
          `interface App {`,
          `  get(path: string, handler: (req: unknown, res: unknown) => void): void;`,
          `}`,
          `declare const app: App;`,
          `app.get("/a", h);`,
          ``,
        ].join("\n"),
      ),
    ],
  });
  assert.equal(exitCode, 0, JSON.stringify(response));
  assert.equal(response.error, undefined, JSON.stringify(response));

  const facts = response.reachability_facts ?? [];
  assert.equal(facts.length, 1, JSON.stringify(response));
  assert.equal(facts[0]?.source, "file:src/handlers.ts#h", JSON.stringify(facts));
  assert.equal(facts[0]?.sink, "(PrismaClient).findMany", JSON.stringify(facts));
});

test("TS reachability: a non-identifier handler (inline arrow) is an explicit gap, not silence", async () => {
  const { response, exitCode } = await runSidecar({
    files: [
      reachabilityTsconfig(),
      ...prismaPackageFiles(),
      file(
        "src/db.ts",
        [`import { PrismaClient } from "@prisma/client";`, `export const prisma = new PrismaClient();`, ``].join("\n"),
      ),
      file(
        "src/app.ts",
        [
          `import { prisma } from "./db";`,
          `interface App {`,
          `  get(path: string, handler: (req: unknown, res: unknown) => void): void;`,
          `}`,
          `declare const app: App;`,
          `app.get("/a", async (req, res) => {`,
          `  await prisma.user.findMany();`,
          `  console.log(req, res);`,
          `});`,
          ``,
        ].join("\n"),
      ),
    ],
  });
  assert.equal(exitCode, 0, JSON.stringify(response));
  assert.equal(response.error, undefined, JSON.stringify(response));

  assert.equal(response.reachability_facts, undefined, JSON.stringify(response.reachability_facts));
  assert.ok(
    response.coverage.diagnostics?.some((d) => d.code === "ts_reachability_unresolved_handler_gap"),
    JSON.stringify(response.coverage),
  );
  assert.equal(response.coverage.complete, false, JSON.stringify(response.coverage));
});

test("TS reachability: a dynamic import() inside a handler body is a gap, not silence", async () => {
  const { response, exitCode } = await runSidecar({
    files: [
      reachabilityTsconfig(),
      ...prismaPackageFiles(),
      file(
        "src/db.ts",
        [`import { PrismaClient } from "@prisma/client";`, `export const prisma = new PrismaClient();`, ``].join("\n"),
      ),
      file(
        "src/mod.ts",
        [
          `import { prisma } from "./db";`,
          `export async function doThing(): Promise<void> {`,
          `  await prisma.user.findMany();`,
          `}`,
        ].join("\n"),
      ),
      file(
        "src/app.ts",
        [
          `interface App {`,
          `  get(path: string, handler: (req: unknown, res: unknown) => void): void;`,
          `}`,
          `declare const app: App;`,
          `export async function h(req: unknown, res: unknown): Promise<void> {`,
          `  const m = await import("./mod");`,
          `  m.doThing();`,
          `  console.log(req, res);`,
          `}`,
          `app.get("/a", h);`,
          ``,
        ].join("\n"),
      ),
    ],
  });
  assert.equal(exitCode, 0, JSON.stringify(response));
  assert.equal(response.error, undefined, JSON.stringify(response));

  assert.equal(response.reachability_facts, undefined, JSON.stringify(response.reachability_facts));
  assert.ok(
    response.coverage.diagnostics?.some((d) => d.code === "ts_reachability_dynamic_import_gap"),
    JSON.stringify(response.coverage),
  );
  assert.equal(response.coverage.complete, false, JSON.stringify(response.coverage));
});

test("TS reachability: calling the same sink twice from one source yields exactly one fact", async () => {
  const { response, exitCode } = await runSidecar({
    files: [
      reachabilityTsconfig(),
      ...prismaPackageFiles(),
      file(
        "src/db.ts",
        [`import { PrismaClient } from "@prisma/client";`, `export const prisma = new PrismaClient();`, ``].join("\n"),
      ),
      file(
        "src/app.ts",
        [
          `import { prisma } from "./db";`,
          `interface App {`,
          `  get(path: string, handler: (req: unknown, res: unknown) => void): void;`,
          `}`,
          `declare const app: App;`,
          `export async function h(req: unknown, res: unknown): Promise<void> {`,
          `  await prisma.user.findMany();`,
          `  await prisma.user.findMany();`,
          `  console.log(req, res);`,
          `}`,
          `app.get("/a", h);`,
          ``,
        ].join("\n"),
      ),
    ],
  });
  assert.equal(exitCode, 0, JSON.stringify(response));
  assert.equal(response.error, undefined, JSON.stringify(response));

  const facts = response.reachability_facts ?? [];
  assert.equal(facts.length, 1, JSON.stringify(facts));
  assert.equal(new Set(facts.map((f) => f.id)).size, facts.length, JSON.stringify(facts));

  const callGraph = response.call_graph ?? [];
  assert.equal(callGraph.length, 1, JSON.stringify(callGraph));
});

test("TS reachability: sink matching requires module provenance, not just a matching class name", async () => {
  const { response, exitCode } = await runSidecar({
    files: [
      reachabilityTsconfig(),
      file(
        "src/domain.ts",
        [
          `export class PrismaClient {`,
          `  user = { delete(): void {} };`,
          `}`,
        ].join("\n"),
      ),
      file(
        "src/app.ts",
        [
          `import { PrismaClient } from "./domain";`,
          `const prisma = new PrismaClient();`,
          `interface App {`,
          `  get(path: string, handler: (req: unknown, res: unknown) => void): void;`,
          `}`,
          `declare const app: App;`,
          `export function h(req: unknown, res: unknown): void {`,
          `  prisma.user.delete();`,
          `  console.log(req, res);`,
          `}`,
          `app.get("/a", h);`,
          ``,
        ].join("\n"),
      ),
    ],
  });
  assert.equal(exitCode, 0, JSON.stringify(response));
  assert.equal(response.error, undefined, JSON.stringify(response));

  // A class named PrismaClient that isn't imported from the pinned
  // @prisma/client module specifier must never be treated as a resolved
  // sink -- only its module provenance (not its name) makes it a sink.
  const facts = response.reachability_facts ?? [];
  assert.equal(facts.some((f) => f.sink === "(PrismaClient).delete"), false, JSON.stringify(facts));
  const callGraph = response.call_graph ?? [];
  assert.equal(callGraph.some((e) => e.to === "(PrismaClient).delete"), false, JSON.stringify(callGraph));
  // The call is not silently dropped either: since it resolves to an
  // in-snapshot method this depth-1 walk does not follow, it surfaces as
  // an explicit truncation gap instead of a false "nothing reachable" claim.
  assert.ok(
    response.coverage.diagnostics?.some((d) => d.code === "ts_reachability_local_call_not_followed_gap"),
    JSON.stringify(response.coverage),
  );
});

test("diagnostic messages never leak the synthetic virtual-root path", async () => {
  // A root-scoped monorepo tsconfig whose "include" matches nothing in the
  // snapshot drives TS's own "No inputs were found" config diagnostic,
  // whose message text embeds the *absolute* path of the config file --
  // vfs.ts's internal VIRTUAL_ROOT synthetic prefix, not a repo-relative
  // one -- unless the sidecar strips it before putting the message on the
  // wire.
  const { response, exitCode } = await runSidecar({
    files: [
      file(
        "packages/api/tsconfig.json",
        JSON.stringify({
          compilerOptions: { module: "commonjs", moduleResolution: "node10" },
          include: ["src/**/*"],
        }),
      ),
    ],
  });
  assert.equal(exitCode, 0);
  assert.equal(response.error, undefined, JSON.stringify(response));
  const configDiagnostics = response.coverage.diagnostics?.filter((d) => d.code === "ts_config_diagnostic") ?? [];
  assert.ok(configDiagnostics.length > 0, JSON.stringify(response.coverage));
  for (const d of configDiagnostics) {
    assert.ok(!d.message.includes("/coach-snapshot"), `diagnostic message leaked the virtual root: ${d.message}`);
  }
  // The diagnostic's own path field stays repo-relative regardless (see the
  // "invalid config" spec above); this spec is specifically about message text.
  assert.ok(
    configDiagnostics.some((d) => d.path === "packages/api/tsconfig.json"),
    JSON.stringify(response.coverage),
  );
});
