export function containsDynamicImport(node, ast) {
    if (ast.isCallExpression(node) && node.expression.kind === ast.SyntaxKind.ImportKeyword)
        return true;
    let found = false;
    node.forEachChild((child) => {
        if (!found)
            found = containsDynamicImport(child, ast);
    });
    return found;
}
//# sourceMappingURL=reachability-dynamic.js.map