import type * as astns from "typescript/unstable/ast";

export function containsDynamicImport(node: astns.Node, ast: typeof astns): boolean {
  if (ast.isCallExpression(node) && node.expression.kind === ast.SyntaxKind.ImportKeyword) return true;
  let found = false;
  node.forEachChild((child) => {
    if (!found) found = containsDynamicImport(child, ast);
  });
  return found;
}
