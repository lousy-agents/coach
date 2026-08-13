import * as astns from "typescript/unstable/ast";
import type { Project } from "typescript/unstable/sync";

import {
  KIND_COMMONJS_REQUIRE,
  KIND_DYNAMIC_IMPORT,
  KIND_IMPORT,
  KIND_REEXPORT,
  KIND_TYPE_ONLY,
  type ImportEdgeFact,
} from "./protocol.js";
import { resolveTarget } from "./edges-resolve.js";
import { isTypeOnlyExportDeclaration, isTypeOnlyImportClause } from "./type-only.js";
import type { ProjectSnapshot } from "./vfs.js";

export function collectEdgesFromSourceFile(
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
    const kind = isTypeOnlyImportClause(node.importClause) ? KIND_TYPE_ONLY : KIND_IMPORT;
    return buildEdge(fromId, node.moduleSpecifier, sf, fromRepoPath, kind, project, snapshot, true);
  }
  if (astns.isExportDeclaration(node) && node.moduleSpecifier && astns.isStringLiteral(node.moduleSpecifier)) {
    const kind = isTypeOnlyExportDeclaration(node) ? KIND_TYPE_ONLY : KIND_REEXPORT;
    return buildEdge(fromId, node.moduleSpecifier, sf, fromRepoPath, kind, project, snapshot, true);
  }
  return edgeFromCall(node, fromId, sf, fromRepoPath, project, snapshot);
}

function edgeFromCall(
  node: astns.Node,
  fromId: string,
  sf: astns.SourceFile,
  fromRepoPath: string,
  project: Project,
  snapshot: ProjectSnapshot,
): ImportEdgeFact | undefined {
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
