import { stat } from "node:fs/promises";
import { isAbsolute, join, resolve as resolvePath } from "node:path";
import { pathToFileURL } from "node:url";

import { CompilerLoadError } from "./compiler-load.js";

export const COMPILER_MODULE_FLAG_PREFIX = "--compiler-module=";
export const NATIVE_PACKAGE_FLAG_PREFIX = "--native-package=";

/**
 * An argv flag rather than a wire Request field, so
 * internal/projectbridge/protocol.go's frozen Request/Response shape stays
 * untouched.
 */
export async function resolveCompilerRootURL(argv: readonly string[]): Promise<URL> {
  const flag = argv.find((a) => a.startsWith(COMPILER_MODULE_FLAG_PREFIX));
  if (!flag) {
    throw new CompilerLoadError("missing required --compiler-module argument");
  }
  return normalizePackageRootURL(flag.slice(COMPILER_MODULE_FLAG_PREFIX.length));
}

export async function resolveNativeTsserverPath(argv: readonly string[]): Promise<string> {
  const flag = argv.find((a) => a.startsWith(NATIVE_PACKAGE_FLAG_PREFIX));
  const raw = flag?.slice(NATIVE_PACKAGE_FLAG_PREFIX.length) ?? "";
  if (!raw || !isAbsolute(raw)) {
    throw new CompilerLoadError("missing required --native-package argument");
  }
  try {
    const info = await stat(raw);
    if (!info.isDirectory()) {
      throw new CompilerLoadError("missing required --native-package argument");
    }
  } catch (err) {
    if (err instanceof CompilerLoadError) throw err;
    throw new CompilerLoadError("missing required --native-package argument");
  }
  const tsserverPath = join(raw, "lib", "tsc");
  try {
    await stat(tsserverPath);
  } catch {
    throw new CompilerLoadError("native TypeScript executable is missing");
  }
  return tsserverPath;
}

function normalizePackageRootURL(raw: string): URL {
  const url = raw.startsWith("file:") ? new URL(raw) : pathToFileURL(resolvePath(raw));
  return url.href.endsWith("/") ? url : new URL(`${url.href}/`);
}
