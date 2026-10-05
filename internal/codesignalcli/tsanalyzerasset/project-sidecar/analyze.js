import { busyWaitMs, ProjectScan } from "./analyze-ingest.js";
import { computeRootScopes } from "./analyze-roots.js";
import { SidecarBackendError, } from "./analyze-types.js";
import { canonicalizeDiagnostics, canonicalizeEdges } from "./canonical.js";
import { describeErrorWithoutPaths } from "./describe-error.js";
import { discoverTsconfigPaths } from "./discover.js";
import { SIDECAR_PHASE, } from "./protocol.js";
import { canonicalizeCallGraph, canonicalizeReachabilityFacts } from "./reachability.js";
import { buildProjectSnapshot, toVirtualPath } from "./vfs.js";
export { SidecarBackendError };
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
//# sourceMappingURL=analyze.js.map