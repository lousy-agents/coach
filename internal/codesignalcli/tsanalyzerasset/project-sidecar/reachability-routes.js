import { containsDynamicImport } from "./reachability-dynamic.js";
import { GAP_DYNAMIC_IMPORT, GAP_UNRESOLVED_HANDLER, ROUTE_REGISTRATION_METHODS } from "./reachability-registry.js";
import { walkSourceForReachability } from "./reachability-walk.js";
export function isRouteRegistrationCall(call, project, compiler) {
    const { ast } = compiler;
    if (!ast.isPropertyAccessExpression(call.expression))
        return false;
    const callee = call.expression;
    if (!ast.isIdentifier(callee.name) || !ROUTE_REGISTRATION_METHODS.has(callee.name.text))
        return false;
    if (call.arguments.length < 2 || !ast.isStringLiteral(call.arguments[0]))
        return false;
    const receiverType = project.checker.getTypeAtLocation(callee.expression);
    if (!receiverType)
        return false;
    return project.checker.getPropertyOfType(receiverType, callee.name.text) !== undefined;
}
export function processRouteRegistration(call, sf, project, snapshot, acc, compiler) {
    const handlerArg = call.arguments[1];
    const fn = resolveHandlerFunction(handlerArg, project, compiler);
    if (fn) {
        const sourceId = functionSourceId(fn, snapshot);
        if (sourceId && acc.noteSource(sourceId)) {
            walkSourceForReachability(fn, sourceId, acc, snapshot, project, compiler);
        }
        return;
    }
    if (containsDynamicImport(handlerArg, compiler.ast)) {
        acc.recordGap(GAP_DYNAMIC_IMPORT, "route handler is resolved through a dynamic import, so it cannot be statically added as a reachability source", handlerArg, sf, snapshot);
        return;
    }
    acc.recordGap(GAP_UNRESOLVED_HANDLER, "route handler did not resolve to a locally declared named function (e.g. an inline arrow/function expression, or a handler bound through something other than a direct or re-exported function declaration), so it cannot be statically added as a reachability source", handlerArg, sf, snapshot);
}
/**
 * Resolves handlerArg to the FunctionDeclaration it names, following one
 * alias hop via Checker.getAliasedSymbol when the identifier's symbol
 * itself is an alias (e.g. `import { h } from "./handlers"` -- the
 * identifier's own declaration is the ImportSpecifier, not the function).
 */
function resolveHandlerFunction(handlerArg, project, compiler) {
    const { ast, symbolFlags } = compiler;
    if (!ast.isIdentifier(handlerArg))
        return undefined;
    const symbol = project.checker.getSymbolAtLocation(handlerArg);
    if (!symbol)
        return undefined;
    const resolvedSymbol = (symbol.flags & symbolFlags.Alias) !== 0 ? project.checker.getAliasedSymbol(symbol) : symbol;
    const declNode = resolvedSymbol.declarations?.[0]?.resolve(project);
    return declNode && ast.isFunctionDeclaration(declNode) && declNode.name ? declNode : undefined;
}
function functionSourceId(fn, snapshot) {
    if (!fn.name)
        return undefined;
    const repoPath = snapshot.toRepoPath(fn.getSourceFile().path);
    if (repoPath === undefined)
        return undefined;
    return `file:${repoPath}#${fn.name.text}`;
}
//# sourceMappingURL=reachability-routes.js.map