import { resolveConditions, stripLeadingDot } from "./resolve-conditions.js";
import { isRecord } from "./vfs.js";

/**
 * Minimal package.json "exports" resolution: a plain string (only for
 * subpath "."), a subpath-keyed object (with one-level condition objects
 * and a single "*" wildcard per pattern), or -- absent "exports" -- the
 * "main" field for subpath ".".
 */
export function resolvePackageExports(exp: unknown, main: string | undefined, subpath: string): string | undefined {
  if (typeof exp === "string") {
    return subpath === "." ? stripLeadingDot(exp) : undefined;
  }
  if (isRecord(exp)) {
    return resolveExportsObject(exp, subpath);
  }
  return mainFallback(exp, main, subpath);
}

function mainFallback(exp: unknown, main: string | undefined, subpath: string): string | undefined {
  if (exp !== undefined || main === undefined || subpath !== ".") return undefined;
  return stripLeadingDot(main);
}

function resolveExportsObject(exp: Record<string, unknown>, subpath: string): string | undefined {
  const direct = exp[subpath];
  if (direct !== undefined) return resolveConditions(direct);
  return resolveWildcardExport(exp, subpath);
}

function resolveWildcardExport(exp: Record<string, unknown>, subpath: string): string | undefined {
  for (const [pattern, value] of Object.entries(exp)) {
    const match = tryWildcard(pattern, subpath);
    if (match === undefined) continue;
    const resolved = resolveConditions(value);
    if (resolved !== undefined) return resolved.replace("*", match);
  }
  return undefined;
}

function tryWildcard(pattern: string, subpath: string): string | undefined {
  if (!pattern.includes("*")) return undefined;
  const starIdx = pattern.indexOf("*");
  const prefix = pattern.slice(0, starIdx);
  const suffix = pattern.slice(starIdx + 1);
  if (!subpath.startsWith(prefix) || !subpath.endsWith(suffix)) return undefined;
  return subpath.slice(prefix.length, subpath.length - suffix.length);
}
