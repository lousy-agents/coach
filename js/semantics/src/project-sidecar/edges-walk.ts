import type * as astns from "typescript/unstable/ast";
import type { Project } from "typescript/unstable/sync";

import {
  KIND_COMMONJS_REQUIRE,
  KIND_DYNAMIC_IMPORT,
  KIND_IMPORT,
  KIND_REEXPORT,
  KIND_TYPE_ONLY,
  RESOLUTION_NON_LITERAL,
  type ImportEdgeFact,
} from "./protocol.js";
import { resolveTarget, type SpecifierNode } from "./edges-resolve.js";
import { isTypeOnlyExportDeclaration, isTypeOnlyImportClause } from "./type-only.js";
import type { ProjectSnapshot } from "./vfs.js";

export function collectEdgesFromSourceFile(
  sf: astns.SourceFile,
  repoPath: string,
  project: Project,
  snapshot: ProjectSnapshot,
  ast: typeof astns,
): ImportEdgeFact[] {
  const fromId = `file:${repoPath}`;
  const edges: ImportEdgeFact[] = [];
  const visit = (node: astns.Node): void => {
    const edge = edgeFromNode(node, fromId, sf, repoPath, project, snapshot, ast);
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
  ast: typeof astns,
): ImportEdgeFact | undefined {
  if (ast.isImportDeclaration(node) && ast.isStringLiteral(node.moduleSpecifier)) {
    const kind = isTypeOnlyImportClause(node.importClause, ast) ? KIND_TYPE_ONLY : KIND_IMPORT;
    return buildEdge(fromId, node.moduleSpecifier, sf, fromRepoPath, kind, project, snapshot, true);
  }
  if (ast.isExportDeclaration(node) && node.moduleSpecifier && ast.isStringLiteral(node.moduleSpecifier)) {
    const kind = isTypeOnlyExportDeclaration(node, ast) ? KIND_TYPE_ONLY : KIND_REEXPORT;
    return buildEdge(fromId, node.moduleSpecifier, sf, fromRepoPath, kind, project, snapshot, true);
  }
  return edgeFromCall(node, fromId, sf, fromRepoPath, project, snapshot, ast);
}

function edgeFromCall(
  node: astns.Node,
  fromId: string,
  sf: astns.SourceFile,
  fromRepoPath: string,
  project: Project,
  snapshot: ProjectSnapshot,
  ast: typeof astns,
): ImportEdgeFact | undefined {
  if (!ast.isCallExpression(node)) return undefined;
  const call = classifyCallExpression(node, ast);
  if (!call) return undefined;
  if ("expression" in call) return buildNonLiteralEdge(fromId, node, call.expression, sf, fromRepoPath, call.kind);
  return buildEdge(fromId, call.specifier, sf, fromRepoPath, call.kind, project, snapshot, call.useChecker);
}

interface StaticCall {
  specifier: SpecifierNode;
  kind: string;
  useChecker: boolean;
}

interface NonLiteralCall {
  expression: astns.Node;
  kind: string;
}

function classifyCallExpression(node: astns.CallExpression, ast: typeof astns): StaticCall | NonLiteralCall | undefined {
  const argument = node.arguments[0];
  if (argument === undefined) return undefined;
  const load = dynamicLoadOf(node, ast);
  if (!load) return undefined;
  if (ast.isStringLiteral(argument) || ast.isNoSubstitutionTemplateLiteral(argument)) {
    return { specifier: argument, ...load };
  }
  return { expression: argument, kind: load.kind };
}

function dynamicLoadOf(node: astns.CallExpression, ast: typeof astns): { kind: string; useChecker: boolean } | undefined {
  if (ast.isIdentifier(node.expression) && node.expression.text === "require") {
    return { kind: KIND_COMMONJS_REQUIRE, useChecker: false };
  }
  if (node.expression.kind === ast.SyntaxKind.ImportKeyword) {
    return { kind: KIND_DYNAMIC_IMPORT, useChecker: true };
  }
  return undefined;
}

const MAX_EXPRESSION_CODE_POINTS = 120;

/** Cuts on a code point so a surrogate pair is never split, and marks the cut. */
function boundedExpressionText(text: string): string {
  const codePoints = Array.from(text);
  if (codePoints.length <= MAX_EXPRESSION_CODE_POINTS) return text;
  return `${codePoints.slice(0, MAX_EXPRESSION_CODE_POINTS).join("")}…`;
}

function buildNonLiteralEdge(
  fromId: string,
  call: astns.CallExpression,
  expression: astns.Node,
  sf: astns.SourceFile,
  fromRepoPath: string,
  kind: string,
): ImportEdgeFact {
  const { line } = sf.getLineAndCharacterOfPosition(call.getStart(sf));
  const text = sf.text.slice(expression.getStart(sf), expression.end).replace(/\s+/g, " ").trim();
  return {
    from: fromId,
    to: `nonliteral:${boundedExpressionText(text)}`,
    kind,
    site: `${fromRepoPath}:${line + 1}`,
    resolution: RESOLUTION_NON_LITERAL,
  };
}

function buildEdge(
  fromId: string,
  specifierNode: SpecifierNode,
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
