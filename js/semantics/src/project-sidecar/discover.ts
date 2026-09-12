import type { ProjectFile } from "./protocol.js";

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

export function normalizeRoot(root: string): string {
  return root.replace(/^\.\/+/, "").replace(/\/+$/, "");
}

/** An empty root or "." matches every path (an unscoped request treats the whole repository as its root); not exercised by any current fixture. */
export function isWithinRoot(path: string, root: string): boolean {
  if (root === "" || root === ".") return true;
  return path === root || path.startsWith(`${root}/`);
}
