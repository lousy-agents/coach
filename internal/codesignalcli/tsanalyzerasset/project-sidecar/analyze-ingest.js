import { extractEdgesForProject } from "./edges.js";
import { extractReachabilityForProject } from "./reachability.js";
import { fromVirtualPath, VIRTUAL_ROOT } from "./vfs.js";
export class ProjectScan {
    visited;
    reachVisited;
    reachSourcesVisited;
    edges;
    callGraph;
    reachabilityFacts;
    diagnostics;
    seenConfigDiagnostics;
    complete = true;
    projectsProcessed = 0;
    constructor(visited, reachVisited, reachSourcesVisited, edges, callGraph, reachabilityFacts, diagnostics, seenConfigDiagnostics) {
        this.visited = visited;
        this.reachVisited = reachVisited;
        this.reachSourcesVisited = reachSourcesVisited;
        this.edges = edges;
        this.callGraph = callGraph;
        this.reachabilityFacts = reachabilityFacts;
        this.diagnostics = diagnostics;
        this.seenConfigDiagnostics = seenConfigDiagnostics;
    }
    timedOut(timeoutMs, projectCount) {
        this.complete = false;
        this.diagnostics.push({
            code: "ts_sidecar_timeout",
            message: `timeout_ms (${timeoutMs}) exceeded after processing ${this.projectsProcessed} of ${projectCount} project(s)`,
        });
    }
    ingest(project, snapshot, compiler) {
        const ingested = ingestProject(project, snapshot, this.visited, this.reachVisited, this.reachSourcesVisited, this.seenConfigDiagnostics, compiler);
        for (const key of ingested.configKeys)
            this.seenConfigDiagnostics.add(key);
        if (!ingested.configComplete)
            this.complete = false;
        this.diagnostics.push(...ingested.diagnostics);
        for (const path of ingested.newlyVisited)
            this.visited.add(path);
        this.edges.push(...ingested.edges);
        for (const path of ingested.reachNewlyVisited)
            this.reachVisited.add(path);
        for (const sourceId of ingested.reachSources)
            this.reachSourcesVisited.add(sourceId);
        this.callGraph.push(...ingested.callGraph);
        this.reachabilityFacts.push(...ingested.reachabilityFacts);
        this.projectsProcessed += 1;
    }
}
function ingestProject(project, snapshot, visited, reachVisited, reachSourcesVisited, seenConfigDiagnostics, compiler) {
    const configResult = collectConfigDiagnostics(project, seenConfigDiagnostics);
    const result = extractEdgesForProject(project, snapshot, visited, compiler.ast);
    const reachResult = processProjectReachability(project, snapshot, reachVisited, reachSourcesVisited, configResult.diagnostics.length > 0, compiler);
    return {
        configKeys: configResult.newKeys,
        configComplete: configResult.diagnostics.length === 0,
        diagnostics: [...configResult.diagnostics, ...result.diagnostics, ...reachResult.diagnostics],
        newlyVisited: result.newlyVisitedPaths,
        edges: result.edges,
        reachNewlyVisited: reachResult.newlyVisitedPaths,
        reachSources: reachResult.visitedSources,
        callGraph: reachResult.callGraph,
        reachabilityFacts: reachResult.facts,
    };
}
// A project whose own config failed to parse never got a real Program
// built, so no call-graph/reachability extraction is attempted for it.
function processProjectReachability(project, snapshot, reachVisited, reachSourcesVisited, configDiagnosticsPresent, compiler) {
    if (configDiagnosticsPresent) {
        return { callGraph: [], facts: [], diagnostics: [], newlyVisitedPaths: [], visitedSources: [] };
    }
    return extractReachabilityForProject(project, snapshot, reachVisited, reachSourcesVisited, {
        ast: compiler.ast,
        symbolFlags: compiler.symbolFlags,
    });
}
function collectConfigDiagnostics(project, seen) {
    const configPath = fromVirtualPath(project.configFileName);
    const diagnostics = [];
    const newKeys = [];
    for (const d of project.program.getConfigFileParsingDiagnostics()) {
        const key = `${d.code}|${configPath ?? ""}|${d.text}`;
        if (seen.has(key) || newKeys.includes(key))
            continue;
        newKeys.push(key);
        // TS embeds the synthetic VIRTUAL_ROOT absolute path in some messages.
        const message = d.text.split(`${VIRTUAL_ROOT}/`).join("");
        diagnostics.push({ code: "ts_config_diagnostic", message, path: configPath });
    }
    return { diagnostics, newKeys };
}
export function busyWaitMs(ms) {
    const end = Date.now() + ms;
    while (Date.now() < end) { }
}
//# sourceMappingURL=analyze-ingest.js.map