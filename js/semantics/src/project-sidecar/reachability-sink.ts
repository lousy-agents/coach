import type * as astns from "typescript/unstable/ast";

import { sinkNodeId } from "./reachability-registry.js";
import type { ProjectSnapshot } from "./vfs.js";

/**
 * Walks .parent from a resolved method declaration to the nearest
 * enclosing class/interface name -- e.g. `prisma.user.findMany()`
 * resolves to a MethodDeclaration nested inside an object-literal
 * property of class PrismaClient, so the enclosing class (not the
 * immediate object-literal type) names the sink -- then defers to
 * sinkNodeId for both the name and module-provenance match.
 */
export function sinkIdForDeclaration(declNode: astns.Node, ast: typeof astns): string | undefined {
  if (!ast.isMethodDeclaration(declNode) || !ast.isIdentifier(declNode.name)) return undefined;
  const className = enclosingClassName(declNode, ast);
  if (!className) return undefined;
  return sinkNodeId(className, declNode.name.text, declNode.getSourceFile().path);
}

function enclosingClassName(node: astns.Node, ast: typeof astns): string | undefined {
  let cur: astns.Node | undefined = node.parent;
  while (cur && !ast.isSourceFile(cur)) {
    if ((ast.isClassDeclaration(cur) || ast.isClassExpression(cur)) && cur.name) return cur.name.text;
    cur = cur.parent;
  }
  return undefined;
}

export function isUnfollowedLocalCallee(declNode: astns.Node, snapshot: ProjectSnapshot, ast: typeof astns): boolean {
  const isFunctionLike =
    ast.isFunctionDeclaration(declNode) ||
    ast.isFunctionExpression(declNode) ||
    ast.isArrowFunction(declNode) ||
    ast.isMethodDeclaration(declNode);
  if (!isFunctionLike) return false;
  return snapshot.toRepoPath(declNode.getSourceFile().path) !== undefined;
}
