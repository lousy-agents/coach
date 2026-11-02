import { KIND_COMMONJS_REQUIRE, KIND_DYNAMIC_IMPORT, KIND_IMPORT, KIND_REEXPORT, KIND_TYPE_ONLY, RESOLUTION_NON_LITERAL, } from "./protocol.js";
import { resolveTarget } from "./edges-resolve.js";
import { isTypeOnlyExportDeclaration, isTypeOnlyImportClause } from "./type-only.js";
export function collectEdgesFromSourceFile(sf, repoPath, project, snapshot, ast) {
    const fromId = `file:${repoPath}`;
    const edges = [];
    const visit = (node) => {
        const edge = edgeFromNode(node, fromId, sf, repoPath, project, snapshot, ast);
        if (edge)
            edges.push(edge);
        node.forEachChild(visit);
    };
    visit(sf);
    return edges;
}
function edgeFromNode(node, fromId, sf, fromRepoPath, project, snapshot, ast) {
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
function edgeFromCall(node, fromId, sf, fromRepoPath, project, snapshot, ast) {
    if (!ast.isCallExpression(node))
        return undefined;
    const call = classifyCallExpression(node, ast);
    if (!call)
        return undefined;
    if ("expression" in call)
        return buildNonLiteralEdge(fromId, node, call.expression, sf, fromRepoPath, call.kind);
    return buildEdge(fromId, call.specifier, sf, fromRepoPath, call.kind, project, snapshot, call.useChecker);
}
function classifyCallExpression(node, ast) {
    const argument = node.arguments[0];
    if (argument === undefined)
        return undefined;
    const load = dynamicLoadOf(node, ast);
    if (!load)
        return undefined;
    if (ast.isStringLiteral(argument) || ast.isNoSubstitutionTemplateLiteral(argument)) {
        return { specifier: argument, ...load };
    }
    return { expression: argument, kind: load.kind };
}
function dynamicLoadOf(node, ast) {
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
function boundedExpressionText(text) {
    const codePoints = Array.from(text);
    if (codePoints.length <= MAX_EXPRESSION_CODE_POINTS)
        return text;
    return `${codePoints.slice(0, MAX_EXPRESSION_CODE_POINTS).join("")}…`;
}
function buildNonLiteralEdge(fromId, call, expression, sf, fromRepoPath, kind) {
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
function buildEdge(fromId, specifierNode, sf, fromRepoPath, kind, project, snapshot, useChecker) {
    const specifierText = specifierNode.text;
    const { line } = sf.getLineAndCharacterOfPosition(specifierNode.getStart(sf));
    const site = `${fromRepoPath}:${line + 1}`;
    const target = resolveTarget(project, specifierNode, specifierText, snapshot, sf.path, useChecker);
    return { from: fromId, to: target.to, kind, site, resolution: target.resolution };
}
//# sourceMappingURL=edges-walk.js.map