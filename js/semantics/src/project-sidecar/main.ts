#!/usr/bin/env node
import { readFile } from "node:fs/promises";
import { resolve as resolvePath } from "node:path";
import { pathToFileURL } from "node:url";

import { analyzeProject, SidecarBackendError, type CompilerBundle } from "./analyze.js";
import { describeErrorWithoutPaths } from "./describe-error.js";
import { KIND_INTERNAL, OP_ANALYZE_PROJECT, PROTOCOL_VERSION, SIDECAR_PHASE, type Request, type Response } from "./protocol.js";
import { readRequestLine, writeResponseLine } from "./stdio.js";

const COMPILER_MODULE_FLAG_PREFIX = "--compiler-module=";

/**
 * Every named export from `typescript/unstable/ast` this sidecar calls at
 * runtime (coach#326 Task 2 review round 3): the full set of `ast.<name>`
 * call sites across reachability.ts, edges-walk.ts, and type-only.ts, found
 * via `grep -rn "ast\.\w\+" src/project-sidecar/*.ts`. A resolved compiler
 * whose ast module is missing any of these -- e.g. version skew dropping a
 * single guard function -- must surface as a CompilerLoadError, not a
 * `TypeError: ast.isX is not a function` crash mid-analysis. Keep this list
 * in sync with the grep above when a new `ast.*` call site is added.
 */
const AST_REQUIRED_EXPORTS = [
  "isArrowFunction",
  "isCallExpression",
  "isClassDeclaration",
  "isClassExpression",
  "isExportDeclaration",
  "isFunctionDeclaration",
  "isFunctionExpression",
  "isIdentifier",
  "isImportClause",
  "isImportDeclaration",
  "isImportSpecifier",
  "isMethodDeclaration",
  "isNamedExports",
  "isNamedImports",
  "isPropertyAccessExpression",
  "isSourceFile",
  "isStringLiteral",
  "isVariableDeclaration",
  "SyntaxKind",
] as const;

/**
 * Reads exactly one internal/projectbridge.Request line from stdin and
 * writes exactly one Response line to stdout. Genuine internal bugs
 * (anything not one of the handled operational conditions below) are
 * deliberately left to propagate to the top-level catch, which fails
 * loudly -- non-zero exit, stderr -- rather than emitting a response that
 * might misrepresent a broken analysis as a clean one.
 */
async function main(): Promise<void> {
  const line = await readRequestLine(process.stdin);

  let req: Request;
  try {
    req = JSON.parse(line) as Request;
  } catch (err) {
    process.stderr.write(`coach-ts-project-sidecar: malformed request JSON: ${String(err)}\n`);
    process.exitCode = 1;
    return;
  }

  if (req.op !== OP_ANALYZE_PROJECT) {
    writeErrorResponse(req, `unsupported op ${JSON.stringify(req.op)}`);
    return;
  }

  let compiler: CompilerBundle;
  try {
    compiler = await loadCompiler(await resolveCompilerRootURL(process.argv.slice(2)));
  } catch (err) {
    // A CompilerLoadError's own .message is already constructed to be
    // path-free (its throw sites already run any wrapped raw fs/import
    // error through describeErrorWithoutPaths below), so it is reported
    // verbatim; anything else reaching here is an unanticipated failure
    // whose .message is not trusted to be path-free.
    const detail = err instanceof CompilerLoadError ? err.message : describeErrorWithoutPaths(err);
    writeErrorResponse(req, `failed to load resolved TypeScript compiler module: ${detail}`);
    return;
  }

  try {
    const { edges, callGraph, reachabilityFacts, coverage } = analyzeProject({
      files: req.files ?? [],
      roots: req.roots,
      timeoutMs: req.timeout_ms,
      testDelayMsPerProject: readTestDelayHook(),
      compiler,
    });
    const response: Response = {
      version: PROTOCOL_VERSION,
      id: req.id,
      import_edges: edges.length > 0 ? edges : undefined,
      call_graph: callGraph.length > 0 ? callGraph : undefined,
      reachability_facts: reachabilityFacts.length > 0 ? reachabilityFacts : undefined,
      coverage,
    };
    writeResponseLine(process.stdout, response);
  } catch (err) {
    if (err instanceof SidecarBackendError) {
      writeErrorResponse(req, err.message);
      return;
    }
    throw err;
  }
}

function writeErrorResponse(req: Request, message: string): void {
  const response: Response = {
    version: PROTOCOL_VERSION,
    id: req.id,
    coverage: { phase: SIDECAR_PHASE, complete: false },
    error: { kind: KIND_INTERNAL, message },
  };
  writeResponseLine(process.stdout, response);
}

// See analyze.ts's AnalyzeOptions.testDelayMsPerProject for what this
// injects and why. Forwarding an environment variable here, rather than a
// request field, keeps the test hook entirely out of the wire protocol.
function readTestDelayHook(): number | undefined {
  const raw = process.env.COACH_TS_SIDECAR_TEST_DELAY_MS;
  if (!raw) return undefined;
  const n = Number(raw);
  return Number.isFinite(n) && n > 0 ? n : undefined;
}

/**
 * Resolves the compiler package root to load (coach#326 Task 3): an
 * explicit `--compiler-module=<path-or-file-URL>` argv flag naming an
 * absolute, host-resolved `typescript` package root. The flag is
 * mandatory — there is no bare-specifier fallback from the private
 * analyzer directory. This argv-based extension point (rather than a
 * wire Request field) keeps internal/projectbridge/protocol.go's frozen
 * Request/Response shape untouched.
 */
async function resolveCompilerRootURL(argv: readonly string[]): Promise<URL> {
  const flag = argv.find((a) => a.startsWith(COMPILER_MODULE_FLAG_PREFIX));
  if (!flag) {
    throw new CompilerLoadError("missing required --compiler-module argument");
  }
  return normalizePackageRootURL(flag.slice(COMPILER_MODULE_FLAG_PREFIX.length));
}

function normalizePackageRootURL(raw: string): URL {
  const url = raw.startsWith("file:") ? new URL(raw) : pathToFileURL(resolvePath(raw));
  return url.href.endsWith("/") ? url : new URL(`${url.href}/`);
}

/** Thrown only for a compiler module that failed to resolve/load/declare
 * the required unstable API surface -- distinct from SidecarBackendError,
 * which covers a resolved-and-loaded compiler whose native platform
 * package is missing (that failure only surfaces once the compiler
 * actually tries to spawn its backend; see analyze.ts's startAnalysisAPI). */
class CompilerLoadError extends Error {}

/**
 * Dynamically loads the three `typescript/unstable/*` subpaths this
 * sidecar needs directly from `rootURL`'s own package.json `exports` map,
 * rather than via bare-specifier resolution (which Node would always
 * satisfy from this package's own node_modules, regardless of rootURL).
 * A subpath missing from `exports`, or a resolved module missing a
 * required named export, is reported as a CompilerLoadError -- a
 * qualified incomplete report the caller turns into a structured Response
 * error, not a crash.
 */
async function loadCompiler(rootURL: URL): Promise<CompilerBundle> {
  const pkg = await readCompilerPackageJson(rootURL);
  const syncURL = resolveExportURL(pkg, rootURL, "./unstable/sync");
  const astURL = resolveExportURL(pkg, rootURL, "./unstable/ast");
  const fsURL = resolveExportURL(pkg, rootURL, "./unstable/fs");

  const [sync, ast, fsMod] = await Promise.all([
    importCompilerModule(syncURL, "typescript/unstable/sync"),
    importCompilerModule(astURL, "typescript/unstable/ast"),
    importCompilerModule(fsURL, "typescript/unstable/fs"),
  ]);

  for (const name of AST_REQUIRED_EXPORTS) requireExport(ast, name);

  return {
    api: requireExport(sync, "API") as CompilerBundle["api"],
    symbolFlags: requireExport(sync, "SymbolFlags") as CompilerBundle["symbolFlags"],
    ast: ast as CompilerBundle["ast"],
    createVirtualFileSystem: requireExport(fsMod, "createVirtualFileSystem") as CompilerBundle["createVirtualFileSystem"],
  };
}

async function readCompilerPackageJson(rootURL: URL): Promise<Record<string, unknown>> {
  const pkgURL = new URL("package.json", rootURL);
  let raw: string;
  try {
    raw = await readFile(pkgURL, "utf8");
  } catch (err) {
    throw new CompilerLoadError(`could not read the resolved TypeScript compiler's package.json: ${describeErrorWithoutPaths(err)}`);
  }
  try {
    return JSON.parse(raw) as Record<string, unknown>;
  } catch (err) {
    throw new CompilerLoadError(`could not parse the resolved TypeScript compiler's package.json: ${describeErrorWithoutPaths(err)}`);
  }
}

function resolveExportURL(pkg: Record<string, unknown>, rootURL: URL, subpath: string): URL {
  const exportsField = pkg.exports;
  const mapped =
    exportsField !== null && typeof exportsField === "object" ? (exportsField as Record<string, unknown>)[subpath] : undefined;
  const relative = typeof mapped === "string" ? mapped : pickConditionalExport(mapped);
  if (!relative) {
    throw new CompilerLoadError(`resolved TypeScript compiler does not declare a "${subpath}" export (required unstable API)`);
  }
  return new URL(relative, rootURL);
}

function pickConditionalExport(mapped: unknown): string | undefined {
  if (mapped === null || typeof mapped !== "object") return undefined;
  const record = mapped as Record<string, unknown>;
  for (const key of ["node", "import", "default"]) {
    const value = record[key];
    if (typeof value === "string") return value;
  }
  return undefined;
}

async function importCompilerModule(url: URL, subpath: string): Promise<Record<string, unknown>> {
  try {
    return (await import(url.href)) as Record<string, unknown>;
  } catch (err) {
    throw new CompilerLoadError(`failed to load ${subpath} from the resolved TypeScript compiler: ${describeErrorWithoutPaths(err)}`);
  }
}

function requireExport<T>(mod: Record<string, unknown>, name: string): T {
  const value = mod[name];
  if (value === undefined) {
    throw new CompilerLoadError(`resolved TypeScript compiler module does not export required API "${name}"`);
  }
  return value as T;
}

main().catch((err: unknown) => {
  process.stderr.write(`coach-ts-project-sidecar: fatal: ${describeErrorWithoutPaths(err)}\n`);
  process.exitCode = 1;
});
