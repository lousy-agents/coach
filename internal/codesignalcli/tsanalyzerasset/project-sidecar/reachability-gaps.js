import { resolveTarget } from "./edges-resolve.js";
import { RESOLUTION_SNAPSHOT } from "./protocol.js";
import { containsDynamicImport } from "./reachability-dynamic.js";
import { GAP_DYNAMIC_IMPORT, GAP_TYPE_ONLY, GAP_UNRESOLVED_TYPE, } from "./reachability-registry.js";
export function gapDiagnosticInfo(gap) {
    switch (gap) {
        case "type_only":
            return {
                code: GAP_TYPE_ONLY,
                message: "call target resolves through a type-only import binding, so further reachability from here is unverified",
            };
        case "dynamic_import":
            return {
                code: GAP_DYNAMIC_IMPORT,
                message: "call target is bound through a dynamic import, so it cannot be statically added as a reachability sink",
            };
        case "unresolved_external":
            return {
                code: GAP_UNRESOLVED_TYPE,
                message: "call target resolves through an import that did not resolve within the snapshot, so further reachability from here is unverified",
            };
    }
}
// A symbol with no local import (e.g. an ambient global like `console`
// that resolves to no declaration at all under this snapshot's lib set)
// is not a gap -- there is nothing to extend from, so it is silently
// ignored rather than misreported as an unresolved import.
export function classifyCalleeGap(call, project, snapshot, compiler) {
    const { ast } = compiler;
    const baseIdent = leftmostIdentifier(call.expression, ast);
    if (!baseIdent)
        return undefined;
    const symbol = project.checker.getSymbolAtLocation(baseIdent);
    const declNode = symbol?.declarations?.[0]?.resolve(project);
    if (!declNode)
        return undefined;
    if (ast.isImportSpecifier(declNode) && declNode.isTypeOnly)
        return "type_only";
    if (ast.isImportClause(declNode) && declNode.phaseModifier === ast.SyntaxKind.TypeKeyword)
        return "type_only";
    if (ast.isVariableDeclaration(declNode) && declNode.initializer && containsDynamicImport(declNode.initializer, ast)) {
        return "dynamic_import";
    }
    const importDecl = enclosingImportDeclaration(declNode, ast);
    if (!importDecl || !ast.isStringLiteral(importDecl.moduleSpecifier))
        return undefined;
    const fromVirtualPath = importDecl.getSourceFile().path;
    const target = resolveTarget(project, importDecl.moduleSpecifier, importDecl.moduleSpecifier.text, snapshot, fromVirtualPath, true);
    return target.resolution === RESOLUTION_SNAPSHOT ? undefined : "unresolved_external";
}
function leftmostIdentifier(expr, ast) {
    let cur = expr;
    while (ast.isPropertyAccessExpression(cur))
        cur = cur.expression;
    return ast.isIdentifier(cur) ? cur : undefined;
}
function enclosingImportDeclaration(node, ast) {
    let cur = node;
    while (cur && !ast.isSourceFile(cur)) {
        if (ast.isImportDeclaration(cur))
            return cur;
        cur = cur.parent;
    }
    return undefined;
}
//# sourceMappingURL=reachability-gaps.js.map