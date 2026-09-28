import type { Project } from "typescript/unstable/sync";

import { isWithinRoot, normalizeRoot } from "./discover.js";
import type { RootScopeFact } from "./protocol.js";
import { testHookDropRoot } from "./test-hook.js";
import { fromVirtualPath, type ProjectSnapshot } from "./vfs.js";

export function computeRootScopes(
  roots: readonly string[] | undefined,
  projects: readonly Project[],
  snapshot: ProjectSnapshot,
  visited: ReadonlySet<string>,
): RootScopeFact[] | undefined {
  if (!roots || roots.length === 0) return undefined;
  const normalizedRoots = [...new Set(roots.map(normalizeRoot))].sort();
  const dropRoot = testHookDropRoot !== undefined ? normalizeRoot(testHookDropRoot) : undefined;
  return normalizedRoots.filter((root) => root !== dropRoot).map((root) => rootScopeFor(root, projects, snapshot, visited));
}

function rootScopeFor(
  root: string,
  projects: readonly Project[],
  snapshot: ProjectSnapshot,
  visited: ReadonlySet<string>,
): RootScopeFact {
  const candidates = new Set<string>();
  for (const project of projects) {
    const configRepoPath = fromVirtualPath(project.configFileName);
    if (configRepoPath === undefined || !isWithinRoot(configRepoPath, root)) continue;
    for (const virtualPath of project.rootFiles) {
      candidates.add(snapshot.canonicalizeVirtualPath(virtualPath));
    }
  }
  const analyzedPaths: string[] = [];
  const unanalyzedPaths: string[] = [];
  for (const candidate of candidates) {
    const repoPath = fromVirtualPath(candidate) ?? candidate;
    (visited.has(candidate) ? analyzedPaths : unanalyzedPaths).push(repoPath);
  }
  analyzedPaths.sort();
  unanalyzedPaths.sort();
  return {
    root,
    candidate_files: candidates.size,
    analyzed_files: analyzedPaths.length,
    analyzed_paths: analyzedPaths.length > 0 ? analyzedPaths : undefined,
    unanalyzed_paths: unanalyzedPaths.length > 0 ? unanalyzedPaths : undefined,
  };
}
