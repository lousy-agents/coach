import { classifyCalleeGap, gapDiagnosticInfo } from "./reachability-gaps.js";
import { isUnfollowedLocalCallee, sinkIdForDeclaration } from "./reachability-sink.js";
import { GAP_LOCAL_CALL_NOT_FOLLOWED } from "./reachability-registry.js";
export function walkSourceForReachability(fn, sourceId, acc, snapshot, project, compiler) {
    if (!fn.body)
        return;
    const { ast } = compiler;
    const sf = fn.getSourceFile();
    const visit = (node) => {
        if (ast.isCallExpression(node)) {
            handleCallInSource(node, sourceId, sf, project, snapshot, acc, compiler);
        }
        node.forEachChild(visit);
    };
    visit(fn.body);
}
function handleCallInSource(call, sourceId, sf, project, snapshot, acc, compiler) {
    const declNode = resolvedCallDeclaration(call, project);
    const sinkId = declNode ? sinkIdForDeclaration(declNode, compiler.ast) : undefined;
    if (sinkId) {
        acc.recordFact(sourceId, sinkId);
        return;
    }
    const gap = classifyCalleeGap(call, project, snapshot, compiler);
    if (gap) {
        const { code, message } = gapDiagnosticInfo(gap);
        acc.recordGap(code, message, call, sf, snapshot);
        return;
    }
    if (declNode && isUnfollowedLocalCallee(declNode, snapshot, compiler.ast)) {
        acc.recordGap(GAP_LOCAL_CALL_NOT_FOLLOWED, "call target resolves to a function declared within this snapshot that this depth-1 walk does not follow further, so multi-hop reachability from here is unverified", call, sf, snapshot);
    }
}
function resolvedCallDeclaration(call, project) {
    const signature = project.checker.getResolvedSignature(call);
    return signature?.declaration?.resolve(project);
}
//# sourceMappingURL=reachability-walk.js.map