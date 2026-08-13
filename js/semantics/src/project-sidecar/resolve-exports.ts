import { isRecord } from "./vfs.js";

const CONDITION_PRIORITY = ["import", "require", "node", "default"];

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
  if (exp === undefined && main !== undefined && subpath === ".") {
    return stripLeadingDot(main);
  }
  return undefined;
}

function resolveExportsObject(exp: Record<string, unknown>, subpath: string): string | undefined {
  if (exp[subpath] !== undefined) {
    return resolveConditions(exp[subpath]);
  }
  for (const [pattern, value] of Object.entries(exp)) {
    if (!pattern.includes("*")) continue;
    const match = matchWildcard(pattern, subpath);
    if (match === undefined) continue;
    const resolved = resolveConditions(value);
    if (resolved !== undefined) return resolved.replace("*", match);
  }
  return undefined;
}

function resolveConditions(value: unknown): string | undefined {
  if (typeof value === "string") return stripLeadingDot(value);
  if (!isRecord(value)) return undefined;
  for (const condition of CONDITION_PRIORITY) {
    const nested = value[condition];
    if (typeof nested === "string") return stripLeadingDot(nested);
    if (isRecord(nested)) {
      const resolved = resolveConditions(nested);
      if (resolved !== undefined) return resolved;
    }
  }
  return undefined;
}

function matchWildcard(pattern: string, subpath: string): string | undefined {
  const starIdx = pattern.indexOf("*");
  const prefix = pattern.slice(0, starIdx);
  const suffix = pattern.slice(starIdx + 1);
  if (!subpath.startsWith(prefix) || !subpath.endsWith(suffix)) return undefined;
  return subpath.slice(prefix.length, subpath.length - suffix.length);
}

function stripLeadingDot(p: string): string {
  return p.replace(/^\.\//, "");
}
