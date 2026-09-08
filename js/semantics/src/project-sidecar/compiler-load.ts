import { readFile } from "node:fs/promises";

import type { CompilerBundle } from "./analyze.js";
import { describeErrorWithoutPaths } from "./describe-error.js";

/**
 * Every `ast.<name>` this sidecar calls, so version skew that drops one
 * surfaces as a CompilerLoadError rather than a mid-analysis TypeError.
 * Keep in sync with `grep -rn "ast\.\w\+" src/project-sidecar/*.ts`.
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

/** Thrown for a compiler module that failed to resolve/load/declare the
 * required unstable API surface, or for a missing/invalid --native-package
 * argument. Distinct from SidecarBackendError, which covers a loaded
 * compiler whose native backend then fails to spawn. */
export class CompilerLoadError extends Error {}

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
export async function loadCompiler(rootURL: URL): Promise<CompilerBundle> {
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
