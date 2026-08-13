import * as astns from "typescript/unstable/ast";
import type { Project } from "typescript/unstable/sync";

import {
  KIND_COMMONJS_REQUIRE,
  KIND_DYNAMIC_IMPORT,
  KIND_IMPORT,
  KIND_REEXPORT,
  KIND_TYPE_ONLY,
  RESOLUTION_EXTERNAL,
  RESOLUTION_SNAPSHOT,
  RESOLUTION_UNRESOLVED,
  type Diagnostic,
  type ImportEdgeFact,
} from "./protocol.js";
import { resolveSpecifier } from "./resolve.js";
import type { ProjectSnapshot } from "./vfs.js";

export interface EdgeExtractionResult {
  edges: ImportEdgeFact[];
  diagnostics: Diagnostic[];
}

/**
 * Walks every .ts/.tsx file owned by `project` (its own tsconfig's root
 * files, not files pulled in only via project references or the default
 * lib) that has not already been visited by another project in this
 * request, extracting one ImportEdgeFact per import/re-export/require/
 * dynamic-import site. `visited` is shared across all projects opened for
 * one request so a file reachable from more than one tsconfig (e.g. via a
 * project reference) is only walked once.
 */
export function extractEdgesForProject(project: Project, snapshot: ProjectSnapshot, visited: Set<string>): EdgeExtractionResult {
  const edges: ImportEdgeFact[] = [];
  const diagnostics: Diagnostic[] = [];

  for (const virtualPath of project.rootFiles) {
    const canonicalVirtual = snapshot.canonicalizeVirtualPath(virtualPath);
    const repoPath = snapshot.toRepoPath(virtualPath);
    if (repoPath === undefined) continue;
    if (!(repoPath.endsWith(".ts") || repoPath.endsWith(".tsx"))) continue;
    // Visit-key on the canonical path so case-folded duplicates from TS do
    // not re-walk the same inventory file.
    if (visited.has(canonicalVirtual)) continue;
    visited.add(canonicalVirtual);

    const sf = project.program.getSourceFile(virtualPath) ?? project.program.getSourceFile(canonicalVirtual);
    if (!sf) {
      diagnostics.push({
        code: "ts_source_file_missing",
        message: `project reports ${repoPath} as a root file but its Program has no SourceFile for it`,
        path: repoPath,
      });
      continue;
    }
    walkSourceFile(sf, repoPath, project, snapshot, edges);
  }

  return { edges, diagnostics };
}

function walkSourceFile(sf: astns.SourceFile, repoPath: string, project: Project, snapshot: ProjectSnapshot, edges: ImportEdgeFact[]): void {
  const fromId = `file:${repoPath}`;

  const visit = (node: astns.Node): void => {
    if (astns.isImportDeclaration(node) && astns.isStringLiteral(node.moduleSpecifier)) {
      const kind = node.importClause?.phaseModifier === astns.SyntaxKind.TypeKeyword ? KIND_TYPE_ONLY : KIND_IMPORT;
      edges.push(buildEdge(fromId, node.moduleSpecifier, sf, repoPath, kind, project, snapshot, true));
    } else if (astns.isExportDeclaration(node) && node.moduleSpecifier && astns.isStringLiteral(node.moduleSpecifier)) {
      const kind = node.isTypeOnly ? KIND_TYPE_ONLY : KIND_REEXPORT;
      edges.push(buildEdge(fromId, node.moduleSpecifier, sf, repoPath, kind, project, snapshot, true));
    } else if (astns.isCallExpression(node)) {
      const call = classifyCallExpression(node);
      if (call) {
        edges.push(buildEdge(fromId, call.specifier, sf, repoPath, call.kind, project, snapshot, call.useChecker));
      }
    }
    node.forEachChild(visit);
  };
  visit(sf);
}

interface ClassifiedCall {
  specifier: astns.StringLiteral;
  kind: string;
  /** require() is never tracked by the checker without an ambient
   * `require` typing (see resolve.ts's module comment), so it always uses
   * the manual fallback resolver; dynamic import() is checker-resolvable
   * when TS's own binder picked it up (see SourceFile.imports). */
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

function resolveTarget(
  project: Project,
  specifierNode: astns.StringLiteral,
  specifierText: string,
  snapshot: ProjectSnapshot,
  fromVirtualFsPath: string,
  useChecker: boolean,
): { to: string; resolution: string } {
  if (useChecker) {
    const symbol = project.checker.getSymbolAtLocation(specifierNode);
    const declPath = symbol?.declarations?.[0]?.path;
    if (declPath) {
      const repoPath = snapshot.toRepoPath(declPath);
      if (repoPath !== undefined) {
        return { to: `file:${repoPath}`, resolution: RESOLUTION_SNAPSHOT };
      }
    }
  }

  const manualFrom = snapshot.canonicalizeVirtualPath(fromVirtualFsPath);
  const manual = resolveSpecifier(manualFrom, specifierText, snapshot);
  if (manual.kind === "snapshot" && manual.virtualPath) {
    const repoPath = snapshot.toRepoPath(manual.virtualPath);
    if (repoPath !== undefined) {
      return { to: `file:${repoPath}`, resolution: RESOLUTION_SNAPSHOT };
    }
  }
  if (manual.kind === "external") {
    return { to: `external:${specifierText}`, resolution: RESOLUTION_EXTERNAL };
  }
  return { to: `unresolved:${specifierText}`, resolution: RESOLUTION_UNRESOLVED };
}
