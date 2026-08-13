import type { Project } from "typescript/unstable/sync";

import type { Diagnostic, ImportEdgeFact } from "./protocol.js";
import { collectEdgesFromSourceFile } from "./edges-walk.js";
import type { ProjectSnapshot } from "./vfs.js";

export interface EdgeExtractionResult {
  edges: ImportEdgeFact[];
  diagnostics: Diagnostic[];
  /** Canonical virtual paths newly walked by this call (caller merges into its visit set). */
  visitedPaths: string[];
}

/**
 * Walks every .ts/.tsx file owned by `project` that has not already been
 * visited by another project in this request, extracting one ImportEdgeFact
 * per import/re-export/require/dynamic-import site.
 */
export function extractEdgesForProject(
  project: Project,
  snapshot: ProjectSnapshot,
  alreadyVisited: ReadonlySet<string>,
): EdgeExtractionResult {
  const edges: ImportEdgeFact[] = [];
  const diagnostics: Diagnostic[] = [];
  const visitedPaths: string[] = [];
  const seen = new Set(alreadyVisited);

  for (const virtualPath of project.rootFiles) {
    const fileResult = extractEdgesFromRootFile(project, snapshot, virtualPath, seen);
    if (!fileResult) continue;
    seen.add(fileResult.visitedPath);
    visitedPaths.push(fileResult.visitedPath);
    edges.push(...fileResult.edges);
    diagnostics.push(...fileResult.diagnostics);
  }

  return { edges, diagnostics, visitedPaths };
}

function extractEdgesFromRootFile(
  project: Project,
  snapshot: ProjectSnapshot,
  virtualPath: string,
  seen: ReadonlySet<string>,
): (Omit<EdgeExtractionResult, "visitedPaths"> & { visitedPath: string }) | undefined {
  const canonicalVirtual = snapshot.canonicalizeVirtualPath(virtualPath);
  const repoPath = snapshot.toRepoPath(virtualPath);
  if (repoPath === undefined) return undefined;
  if (!(repoPath.endsWith(".ts") || repoPath.endsWith(".tsx"))) return undefined;
  if (seen.has(canonicalVirtual)) return undefined;

  const sf = project.program.getSourceFile(virtualPath) ?? project.program.getSourceFile(canonicalVirtual);
  if (!sf) {
    return missingSourceFile(canonicalVirtual, repoPath);
  }
  return {
    visitedPath: canonicalVirtual,
    edges: collectEdgesFromSourceFile(sf, repoPath, project, snapshot),
    diagnostics: [],
  };
}

function missingSourceFile(visitedPath: string, repoPath: string): Omit<EdgeExtractionResult, "visitedPaths"> & { visitedPath: string } {
  return {
    visitedPath,
    edges: [],
    diagnostics: [
      {
        code: "ts_source_file_missing",
        message: `project reports ${repoPath} as a root file but its Program has no SourceFile for it`,
        path: repoPath,
      },
    ],
  };
}
