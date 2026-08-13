import * as astns from "typescript/unstable/ast";
import type { Project } from "typescript/unstable/sync";

import {
  KIND_COMMONJS_REQUIRE,
  KIND_DYNAMIC_IMPORT,
  KIND_IMPORT,
  KIND_REEXPORT,
  KIND_TYPE_ONLY,
  type Diagnostic,
  type ImportEdgeFact,
} from "./protocol.js";
import { resolveTarget } from "./edges-resolve.js";
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
    return {
      visitedPath: canonicalVirtual,
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
  return {
    visitedPath: canonicalVirtual,
    edges: collectEdgesFromSourceFile(sf, repoPath, project, snapshot),
    diagnostics: [],
  };
}

function collectEdgesFromSourceFile(
  sf: astns.SourceFile,
  repoPath: string,
  project: Project,
  snapshot: ProjectSnapshot,
): ImportEdgeFact[] {
  const fromId = `file:${repoPath}`;
  const edges: ImportEdgeFact[] = [];
  const visit = (node: astns.Node): void => {
    const edge = edgeFromNode(node, fromId, sf, repoPath, project, snapshot);
    if (edge) edges.push(edge);
    node.forEachChild(visit);
  };
  visit(sf);
  return edges;
}

function edgeFromNode(
  node: astns.Node,
  fromId: string,
  sf: astns.SourceFile,
  fromRepoPath: string,
  project: Project,
  snapshot: ProjectSnapshot,
): ImportEdgeFact | undefined {
  if (astns.isImportDeclaration(node) && astns.isStringLiteral(node.moduleSpecifier)) {
    const kind = node.importClause?.phaseModifier === astns.SyntaxKind.TypeKeyword ? KIND_TYPE_ONLY : KIND_IMPORT;
    return buildEdge(fromId, node.moduleSpecifier, sf, fromRepoPath, kind, project, snapshot, true);
  }
  if (astns.isExportDeclaration(node) && node.moduleSpecifier && astns.isStringLiteral(node.moduleSpecifier)) {
    const kind = node.isTypeOnly ? KIND_TYPE_ONLY : KIND_REEXPORT;
    return buildEdge(fromId, node.moduleSpecifier, sf, fromRepoPath, kind, project, snapshot, true);
  }
  if (!astns.isCallExpression(node)) return undefined;
  const call = classifyCallExpression(node);
  if (!call) return undefined;
  return buildEdge(fromId, call.specifier, sf, fromRepoPath, call.kind, project, snapshot, call.useChecker);
}

interface ClassifiedCall {
  specifier: astns.StringLiteral;
  kind: string;
  useChecker: boolean;
}

function classifyCallExpression(node: astns.CallExpression): ClassifiedCall | undefined {
  if (node.arguments.length === 0 || !astns.isStringLiteral(node.arguments[0])) return undefined;
  const specifier = node.arguments[0];
  if (astns.isIdentifier(node.expression) && node.expression.text === "require") {
    return { specifier, kind: KIND_COMMONJS_REQUIRE, useChecker: false };
  }
  if (node.expression.kind === astns.SyntaxKind.ImportKeyword) {
    return { specifier, kind: KIND_DYNAMIC_IMPORT, useChecker: true };
  }
  return undefined;
}

function buildEdge(
  fromId: string,
  specifierNode: astns.StringLiteral,
  sf: astns.SourceFile,
  fromRepoPath: string,
  kind: string,
  project: Project,
  snapshot: ProjectSnapshot,
  useChecker: boolean,
): ImportEdgeFact {
  const specifierText = specifierNode.text;
  const { line } = sf.getLineAndCharacterOfPosition(specifierNode.getStart(sf));
  const site = `${fromRepoPath}:${line + 1}`;
  const target = resolveTarget(project, specifierNode, specifierText, snapshot, sf.path, useChecker);
  return { from: fromId, to: target.to, kind, site, resolution: target.resolution };
}
