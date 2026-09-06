import type { Project } from "typescript/unstable/sync";

import type { Diagnostic, ImportEdgeFact } from "./protocol.js";
import { collectEdgesFromSourceFile } from "./edges-walk.js";
import type { ProjectSnapshot } from "./vfs.js";

type AstModule = typeof import("typescript/unstable/ast");

export interface EdgeExtractionResult {
  edges: ImportEdgeFact[];
  diagnostics: Diagnostic[];
  newlyVisitedPaths: string[];
}

export function extractEdgesForProject(
  project: Project,
  snapshot: ProjectSnapshot,
  alreadyVisited: ReadonlySet<string>,
  ast: AstModule,
): EdgeExtractionResult {
  const edges: ImportEdgeFact[] = [];
  const diagnostics: Diagnostic[] = [];
  const newlyVisitedPaths: string[] = [];
  const seen = new Set(alreadyVisited);

  for (const virtualPath of project.rootFiles) {
    const fileResult = extractEdgesFromRootFile(project, snapshot, virtualPath, seen, ast);
    if (!fileResult) continue;
    seen.add(fileResult.visitedPath);
    newlyVisitedPaths.push(fileResult.visitedPath);
    edges.push(...fileResult.edges);
    diagnostics.push(...fileResult.diagnostics);
  }

  return { edges, diagnostics, newlyVisitedPaths };
}

function extractEdgesFromRootFile(
  project: Project,
  snapshot: ProjectSnapshot,
  virtualPath: string,
  seen: ReadonlySet<string>,
  ast: AstModule,
): (Omit<EdgeExtractionResult, "newlyVisitedPaths"> & { visitedPath: string }) | undefined {
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
    edges: collectEdgesFromSourceFile(sf, repoPath, project, snapshot, ast),
    diagnostics: [],
  };
}

function missingSourceFile(visitedPath: string, repoPath: string): Omit<EdgeExtractionResult, "newlyVisitedPaths"> & { visitedPath: string } {
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
