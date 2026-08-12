import type { ProjectFile } from "./protocol.js";

/**
 * Finds repository-relative tsconfig.json paths in the snapshot, scoped to
 * `roots` when non-empty (matching how pkg/projectmodel's Go client scopes
 * file collection to roots -- here it scopes which tsconfig(s) are treated
 * as project entry points, per Task 2's brief). Each discovered path is
 * opened as its own project (see analyze.ts); TypeScript's own project
 * reference following (proven in this package's design spikes) resolves
 * cross-project imports without this sidecar needing to parse `references`
 * arrays itself.
 */
export function discoverTsconfigPaths(files: readonly ProjectFile[], roots: readonly string[] | undefined): string[] {
  const scopedRoots = roots && roots.length > 0 ? roots.map(normalizeRoot) : undefined;
  const found = new Set<string>();
  for (const f of files) {
    if (basename(f.path) !== "tsconfig.json") continue;
    if (scopedRoots && !scopedRoots.some((root) => isWithinRoot(f.path, root))) continue;
    found.add(f.path);
  }
  return [...found].sort();
}

function basename(p: string): string {
  const idx = p.lastIndexOf("/");
  return idx === -1 ? p : p.slice(idx + 1);
}

function normalizeRoot(root: string): string {
  return root.replace(/^\.\/+/, "").replace(/\/+$/, "");
}

function isWithinRoot(path: string, root: string): boolean {
  if (root === "" || root === ".") return true;
  return path === root || path.startsWith(`${root}/`);
}
