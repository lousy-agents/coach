// A default import (clause.name) is always a value binding, so its
// presence forces a value edge regardless of any inline-typed named
// specifiers alongside it.
export function isTypeOnlyImportClause(clause, ast) {
    if (!clause)
        return false;
    if (clause.phaseModifier === ast.SyntaxKind.TypeKeyword)
        return true;
    if (clause.name)
        return false;
    const namedBindings = clause.namedBindings;
    if (namedBindings && ast.isNamedImports(namedBindings)) {
        return namedBindings.elements.length > 0 && namedBindings.elements.every((e) => e.isTypeOnly);
    }
    return false;
}
export function isTypeOnlyExportDeclaration(node, ast) {
    if (node.isTypeOnly)
        return true;
    const exportClause = node.exportClause;
    if (exportClause && ast.isNamedExports(exportClause)) {
        return exportClause.elements.length > 0 && exportClause.elements.every((e) => e.isTypeOnly);
    }
    return false;
}
//# sourceMappingURL=type-only.js.map