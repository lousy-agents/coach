import { canonicalizeDiagnostics, canonicalizeEdges } from "./canonical.js";
import { describeErrorWithoutPaths } from "./describe-error.js";
import { discoverTsconfigPaths, isWithinRoot, normalizeRoot } from "./discover.js";
import { extractEdgesForProject } from "./edges.js";
import { SIDECAR_PHASE, } from "./protocol.js";
import { canonicalizeCallGraph, canonicalizeReachabilityFacts, extractReachabilityForProject } from "./reachability.js";
import { testHookDropRoot } from "./test-hook.js";
import { buildProjectSnapshot, fromVirtualPath, toVirtualPath, VIRTUAL_ROOT } from "./vfs.js";
/** Thrown only for genuine backend-startup failures (e.g. the bundled native tsgo binary failing to spawn); main.ts turns this into a Response.Error rather than crashing the process. */
export class SidecarBackendError extends Error {
}
export function analyzeProject(opts) {
    const deadline = opts.timeoutMs && opts.timeoutMs > 0 ? Date.now() + opts.timeoutMs : undefined;
    const snapshot = buildProjectSnapshot(opts.files, opts.compiler.createVirtualFileSystem);
    const tsconfigPaths = discoverTsconfigPaths(opts.files, opts.roots);
    const counts = { files_seen: opts.files.length, tsconfig_count: tsconfigPaths.length };
    if (tsconfigPaths.length === 0) {
        return emptyComplete(counts, opts.files, opts.roots, snapshot);
    }
    if (deadline !== undefined && Date.now() >= deadline) {
        return timeoutBeforeStart(counts, opts.timeoutMs ?? 0, opts.roots, snapshot);
    }
    const api = startAnalysisAPI(snapshot, opts.compiler.api, opts.tsserverPath);
    try {
        return runProjects(api, snapshot, tsconfigPaths, opts, deadline, counts);
    }
    finally {
        api.close();
    }
}
function emptyComplete(counts, files, roots, snapshot) {
    const hasTsSources = files.some((f) => f.path.endsWith(".ts") || f.path.endsWith(".tsx"));
    return hasTsSources
        ? sourcesWithNoProjectConfigResult(counts, roots, snapshot)
        : vacuousProjectResult(counts, roots, snapshot);
}
function emptyAnalysisResult(coverage, roots, snapshot) {
    return {
        edges: [],
        callGraph: [],
        reachabilityFacts: [],
        coverage,
        rootScopes: computeRootScopes(roots, [], snapshot, new Set()),
    };
}
function vacuousProjectResult(counts, roots, snapshot) {
    return emptyAnalysisResult({ phase: SIDECAR_PHASE, complete: true, counts }, roots, snapshot);
}
function sourcesWithNoProjectConfigResult(counts, roots, snapshot) {
    return emptyAnalysisResult({
        phase: SIDECAR_PHASE,
        complete: false,
        counts,
        diagnostics: [
            { code: "ts_no_project_config", message: "no tsconfig.json was discovered while .ts/.tsx sources were provided" },
        ],
    }, roots, snapshot);
}
function timeoutBeforeStart(counts, timeoutMs, roots, snapshot) {
    return emptyAnalysisResult({
        phase: SIDECAR_PHASE,
        complete: false,
        counts,
        budgets: { timeout_ms: timeoutMs },
        diagnostics: [{ code: "ts_sidecar_timeout", message: "timeout_ms exceeded before analysis started" }],
    }, roots, snapshot);
}
function startAnalysisAPI(snapshot, ApiCtor, tsserverPath) {
    try {
        return new ApiCtor({ fs: snapshot.fs, tsserverPath });
    }
    catch (err) {
        throw new SidecarBackendError(`failed to start ts sidecar analysis backend: ${describeErrorWithoutPaths(err)}`);
    }
}
function runProjects(api, snapshot, tsconfigPaths, opts, deadline, counts) {
    const snap = api.updateSnapshot({ openProjects: tsconfigPaths.map(toVirtualPath) });
    const projects = snap.getProjects();
    const visited = new Set();
    const reachVisited = new Set();
    const reachSourcesVisited = new Set();
    const edges = [];
    const callGraph = [];
    const reachabilityFacts = [];
    const diagnostics = [];
    const seenConfigDiagnostics = new Set();
    const scan = new ProjectScan(visited, reachVisited, reachSourcesVisited, edges, callGraph, reachabilityFacts, diagnostics, seenConfigDiagnostics);
    for (const project of projects) {
        if (opts.testDelayMsPerProject)
            busyWaitMs(opts.testDelayMsPerProject);
        if (deadline !== undefined && Date.now() >= deadline) {
            scan.timedOut(opts.timeoutMs, projects.length);
            break;
        }
        scan.ingest(project, snapshot, opts.compiler);
    }
    return {
        edges: canonicalizeEdges(edges),
        callGraph: canonicalizeCallGraph(callGraph),
        reachabilityFacts: canonicalizeReachabilityFacts(reachabilityFacts),
        coverage: {
            phase: SIDECAR_PHASE,
            complete: scan.complete,
            counts: {
                ...counts,
                files_analyzed: visited.size,
                projects_analyzed: scan.projectsProcessed,
            },
            budgets: deadline !== undefined ? { timeout_ms: opts.timeoutMs ?? 0 } : undefined,
            diagnostics: diagnostics.length > 0 ? canonicalizeDiagnostics(diagnostics) : undefined,
        },
        rootScopes: computeRootScopes(opts.roots, projects, snapshot, visited),
    };
}
function computeRootScopes(roots, projects, snapshot, visited) {
    if (!roots || roots.length === 0)
        return undefined;
    const normalizedRoots = [...new Set(roots.map(normalizeRoot))].sort();
    const dropRoot = testHookDropRoot !== undefined ? normalizeRoot(testHookDropRoot) : undefined;
    return normalizedRoots.filter((root) => root !== dropRoot).map((root) => rootScopeFor(root, projects, snapshot, visited));
}
function rootScopeFor(root, projects, snapshot, visited) {
    const candidates = new Set();
    for (const project of projects) {
        const configRepoPath = fromVirtualPath(project.configFileName);
        if (configRepoPath === undefined || !isWithinRoot(configRepoPath, root))
            continue;
        for (const virtualPath of project.rootFiles) {
            candidates.add(snapshot.canonicalizeVirtualPath(virtualPath));
        }
    }
    const analyzedPaths = [];
    const unanalyzedPaths = [];
    for (const candidate of candidates) {
        const repoPath = fromVirtualPath(candidate) ?? candidate;
        (visited.has(candidate) ? analyzedPaths : unanalyzedPaths).push(repoPath);
    }
    analyzedPaths.sort();
    unanalyzedPaths.sort();
    return {
        root,
        candidate_files: candidates.size,
        analyzed_files: analyzedPaths.length,
        analyzed_paths: analyzedPaths.length > 0 ? analyzedPaths : undefined,
        unanalyzed_paths: unanalyzedPaths.length > 0 ? unanalyzedPaths : undefined,
    };
}
class ProjectScan {
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
function busyWaitMs(ms) {
    const end = Date.now() + ms;
    while (Date.now() < end) { }
}
//# sourceMappingURL=analyze.js.map