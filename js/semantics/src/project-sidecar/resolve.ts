import { resolvePackageExports } from "./resolve-exports.js";
import { VIRTUAL_ROOT, type ProjectSnapshot } from "./vfs.js";

export type ResolvedKind = "snapshot" | "external" | "unresolved";

export interface ResolvedSpecifier {
  kind: ResolvedKind;
  /** Set only when kind === "snapshot". */
  virtualPath?: string;
}

/**
 * tsconfig `paths` as the checker sees them: the pattern table plus the
 * virtual directory its targets are relative to (the declaring config's
 * directory, which differs from the project's own when `paths` is inherited
 * through `extends`).
 */
export interface PathAliases {
  paths: Readonly<Record<string, readonly string[]>>;
  baseDirVirtual: string;
}

/**
 * Manual, hand-rolled specifier resolver used only where the real TS
 * checker cannot help: require() calls (TS never tracks these without an
 * ambient `require` typing from @types/node, which snapshots do not ship)
 * and as a fallback when the checker's own resolution comes back empty.
 * Static import/export declarations are resolved primarily through the
 * real TS checker (see edges.ts).
 *
 * A bare specifier is tried against `aliases` first, then against workspace
 * packages. A specifier that an alias pattern claims but whose targets are
 * missing is "unresolved" rather than "external": the snapshot has no
 * node_modules, so the only way to tell a broken alias from a package is that
 * a package is never claimed by a specific pattern. A catch-all `*` pattern
 * claims every package, so its misses stay "external".
 */
export function resolveSpecifier(
  fromVirtualPath: string,
  specifier: string,
  snapshot: ProjectSnapshot,
  aliases?: PathAliases,
): ResolvedSpecifier {
  if (isRelativeSpecifier(specifier)) {
    const resolved = resolveRelative(fromVirtualPath, specifier, snapshot.nonMirrorVirtualPaths);
    return resolved ? { kind: "snapshot", virtualPath: resolved } : { kind: "unresolved" };
  }
  const viaAlias = aliases ? matchPathAlias(specifier, aliases, snapshot.nonMirrorVirtualPaths) : undefined;
  if (viaAlias?.virtualPath) return { kind: "snapshot", virtualPath: viaAlias.virtualPath };
  const resolved = resolvePackageSpecifier(specifier, snapshot);
  if (resolved) return { kind: "snapshot", virtualPath: resolved };
  return viaAlias?.claimedBySpecificPattern ? { kind: "unresolved" } : { kind: "external" };
}

function isRelativeSpecifier(spec: string): boolean {
  return spec === "." || spec === ".." || spec.startsWith("./") || spec.startsWith("../");
}

function resolveRelative(fromVirtualPath: string, specifier: string, paths: ReadonlySet<string>): string | undefined {
  const dir = fromVirtualPath.slice(0, fromVirtualPath.lastIndexOf("/"));
  const joined = normalizeVirtualPath(`${dir}/${specifier}`);
  return probeCandidates(joined, paths);
}

/**
 * Tries, in order: the exact path; a `.js`/`.jsx` specifier rewritten to
 * `.ts`/`.tsx`; `.ts`/`.tsx`/`.d.ts` appended; and `/index.ts`/`/index.tsx`.
 */
function probeCandidates(base: string, paths: ReadonlySet<string>): string | undefined {
  const rewritten = base.replace(/\.jsx$/, ".tsx").replace(/\.js$/, ".ts");
  const candidates = [
    rewritten,
    base,
    `${rewritten}.ts`,
    `${rewritten}.tsx`,
    `${rewritten}.d.ts`,
    `${rewritten}/index.ts`,
    `${rewritten}/index.tsx`,
  ];
  for (const candidate of candidates) {
    if (paths.has(candidate)) return candidate;
  }
  return undefined;
}

interface AliasMatch {
  virtualPath?: string;
  claimedBySpecificPattern: boolean;
}

function matchPathAlias(specifier: string, aliases: PathAliases, paths: ReadonlySet<string>): AliasMatch | undefined {
  const pattern = bestMatchingPattern(specifier, Object.keys(aliases.paths));
  if (pattern === undefined) return undefined;
  const star = pattern.indexOf("*");
  const capture = star < 0 ? "" : specifier.slice(star, specifier.length - (pattern.length - star - 1));
  for (const target of aliases.paths[pattern] ?? []) {
    const substituted = star < 0 ? target : target.replace("*", capture);
    const hit = probeCandidates(normalizeVirtualPath(`${aliases.baseDirVirtual}/${substituted}`), paths);
    if (hit) return { virtualPath: hit, claimedBySpecificPattern: true };
  }
  return { claimedBySpecificPattern: pattern !== "*" };
}

/**
 * Mirrors the checker's choice: an exact (wildcard-free) pattern wins
 * outright, otherwise the matching wildcard pattern with the longest prefix.
 */
function bestMatchingPattern(specifier: string, patterns: readonly string[]): string | undefined {
  let best: string | undefined;
  let bestPrefixLength = -1;
  for (const pattern of patterns) {
    const star = pattern.indexOf("*");
    if (star < 0) {
      if (pattern === specifier) return pattern;
      continue;
    }
    const prefix = pattern.slice(0, star);
    const suffix = pattern.slice(star + 1);
    const matches =
      specifier.length >= prefix.length + suffix.length && specifier.startsWith(prefix) && specifier.endsWith(suffix);
    if (matches && prefix.length > bestPrefixLength) {
      best = pattern;
      bestPrefixLength = prefix.length;
    }
  }
  return best;
}

function resolvePackageSpecifier(specifier: string, snapshot: ProjectSnapshot): string | undefined {
  for (const info of snapshot.packagesByName.values()) {
    if (specifier !== info.name && !specifier.startsWith(`${info.name}/`)) continue;
    const subpath = specifier === info.name ? "." : `.${specifier.slice(info.name.length)}`;
    const target = resolvePackageExports(info.exports, info.main, subpath);
    if (target === undefined) continue;
    const full = normalizeVirtualPath(`${info.dirVirtual}/${target}`);
    if (snapshot.nonMirrorVirtualPaths.has(full)) return full;
    const probed = probeCandidates(full, snapshot.nonMirrorVirtualPaths);
    if (probed) return probed;
  }
  return undefined;
}

function normalizeVirtualPath(p: string): string {
  const parts = p.split("/");
  const out: string[] = [];
  for (const part of parts) {
    if (part === "" || part === ".") continue;
    if (part === "..") {
      out.pop();
      continue;
    }
    out.push(part);
  }
  return out.length === 0 ? VIRTUAL_ROOT : `/${out.join("/")}`;
}
