import { collectEdgesFromSourceFile } from "./edges-walk.js";
export function extractEdgesForProject(project, snapshot, alreadyVisited, ast) {
    const edges = [];
    const diagnostics = [];
    const newlyVisitedPaths = [];
    const seen = new Set(alreadyVisited);
    for (const virtualPath of project.rootFiles) {
        const fileResult = extractEdgesFromRootFile(project, snapshot, virtualPath, seen, ast);
        if (!fileResult)
            continue;
        seen.add(fileResult.visitedPath);
        newlyVisitedPaths.push(fileResult.visitedPath);
        edges.push(...fileResult.edges);
        diagnostics.push(...fileResult.diagnostics);
    }
    return { edges, diagnostics, newlyVisitedPaths };
}
function extractEdgesFromRootFile(project, snapshot, virtualPath, seen, ast) {
    const canonicalVirtual = snapshot.canonicalizeVirtualPath(virtualPath);
    const repoPath = snapshot.toRepoPath(virtualPath);
    if (repoPath === undefined)
        return undefined;
    if (!(repoPath.endsWith(".ts") || repoPath.endsWith(".tsx")))
        return undefined;
    if (seen.has(canonicalVirtual))
        return undefined;
    const sf = project.program.getSourceFile(virtualPath) ?? project.program.getSourceFile(canonicalVirtual);
    if (!sf) {
        return missingSourceFile(canonicalVirtual, repoPath);
    }
    return {
        visitedPath: canonicalVirtual,
        edges: collectEdgesFromSourceFile(sf, repoPath, project, snapshot, ast),
        diagnostics: [],
    };
}
function missingSourceFile(visitedPath, repoPath) {
    return {
        visitedPath,
        edges: [],
        diagnostics: [
            {
                code: "ts_source_file_missing",
                message: `project reports ${repoPath} as a root file but its Program has no SourceFile for it`,
                path: repoPath,
            },
        ],
    };
}
//# sourceMappingURL=edges.js.map