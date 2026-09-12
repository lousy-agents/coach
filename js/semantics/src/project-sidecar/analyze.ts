import type { Project } from "typescript/unstable/sync";

import { canonicalizeDiagnostics, canonicalizeEdges } from "./canonical.js";
import { describeErrorWithoutPaths } from "./describe-error.js";
import { discoverTsconfigPaths, isWithinRoot, normalizeRoot } from "./discover.js";
import { extractEdgesForProject } from "./edges.js";
import {
  SIDECAR_PHASE,
  type CallGraphEdgeFact,
  type Coverage,
  type Diagnostic,
  type ImportEdgeFact,
  type ProjectFile,
  type ReachabilityFactWire,
  type RootScopeFact,
} from "./protocol.js";
import { canonicalizeCallGraph, canonicalizeReachabilityFacts, extractReachabilityForProject } from "./reachability.js";
import { buildProjectSnapshot, fromVirtualPath, toVirtualPath, VIRTUAL_ROOT, type ProjectSnapshot } from "./vfs.js";

export interface CompilerBundle {
  api: typeof import("typescript/unstable/sync").API;
  symbolFlags: typeof import("typescript/unstable/sync").SymbolFlags;
  ast: typeof import("typescript/unstable/ast");
  createVirtualFileSystem: typeof import("typescript/unstable/fs").createVirtualFileSystem;
}

type ApiInstance = InstanceType<CompilerBundle["api"]>;

/** Thrown only for genuine backend-startup failures (e.g. the bundled native tsgo binary failing to spawn); main.ts turns this into a Response.Error rather than crashing the process. */
export class SidecarBackendError extends Error {}

export interface AnalyzeOptions {
  files: readonly ProjectFile[];
  roots?: readonly string[];
  timeoutMs?: number;
  /** Test-only: injects a fixed synchronous delay before processing each
   * discovered tsconfig project, so timeout_ms self-enforcement can be
   * exercised deterministically without depending on a genuinely slow
   * TypeScript analysis. Must never be set outside tests (see main.ts's
   * COACH_TS_SIDECAR_TEST_DELAY_MS gate). */
  testDelayMsPerProject?: number;
  compiler: CompilerBundle;
  tsserverPath: string;
}

export interface AnalyzeResult {
  edges: ImportEdgeFact[];
  callGraph: CallGraphEdgeFact[];
  reachabilityFacts: ReachabilityFactWire[];
  coverage: Coverage;
  rootScopes?: RootScopeFact[];
}

export function analyzeProject(opts: AnalyzeOptions): AnalyzeResult {
  const deadline = opts.timeoutMs && opts.timeoutMs > 0 ? Date.now() + opts.timeoutMs : undefined;
  const snapshot = buildProjectSnapshot(opts.files, opts.compiler.createVirtualFileSystem);
  const tsconfigPaths = discoverTsconfigPaths(opts.files, opts.roots);
  const counts: Record<string, number> = { files_seen: opts.files.length, tsconfig_count: tsconfigPaths.length };

  if (tsconfigPaths.length === 0) {
    return emptyComplete(counts, opts.files, opts.roots, snapshot);
  }
  if (deadline !== undefined && Date.now() >= deadline) {
    return timeoutBeforeStart(counts, opts.timeoutMs ?? 0, opts.roots, snapshot);
  }

  const api = startAnalysisAPI(snapshot, opts.compiler.api, opts.tsserverPath);
  try {
    return runProjects(api, snapshot, tsconfigPaths, opts, deadline, counts);
  } finally {
    api.close();
  }
}

function emptyComplete(
  counts: Record<string, number>,
  files: readonly ProjectFile[],
  roots: readonly string[] | undefined,
  snapshot: ProjectSnapshot,
): AnalyzeResult {
  const hasTsSources = files.some((f) => f.path.endsWith(".ts") || f.path.endsWith(".tsx"));
  return hasTsSources
    ? sourcesWithNoProjectConfigResult(counts, roots, snapshot)
    : vacuousProjectResult(counts, roots, snapshot);
}

function emptyAnalysisResult(
  coverage: Coverage,
  roots: readonly string[] | undefined,
  snapshot: ProjectSnapshot,
): AnalyzeResult {
  return {
    edges: [],
    callGraph: [],
    reachabilityFacts: [],
    coverage,
    rootScopes: computeRootScopes(roots, [], snapshot, new Set()),
  };
}

function vacuousProjectResult(
  counts: Record<string, number>,
  roots: readonly string[] | undefined,
  snapshot: ProjectSnapshot,
): AnalyzeResult {
  return emptyAnalysisResult({ phase: SIDECAR_PHASE, complete: true, counts }, roots, snapshot);
}

function sourcesWithNoProjectConfigResult(
  counts: Record<string, number>,
  roots: readonly string[] | undefined,
  snapshot: ProjectSnapshot,
): AnalyzeResult {
  return emptyAnalysisResult(
    {
      phase: SIDECAR_PHASE,
      complete: false,
      counts,
      diagnostics: [
        { code: "ts_no_project_config", message: "no tsconfig.json was discovered while .ts/.tsx sources were provided" },
      ],
    },
    roots,
    snapshot,
  );
}

function timeoutBeforeStart(
  counts: Record<string, number>,
  timeoutMs: number,
  roots: readonly string[] | undefined,
  snapshot: ProjectSnapshot,
): AnalyzeResult {
  return emptyAnalysisResult(
    {
      phase: SIDECAR_PHASE,
      complete: false,
      counts,
      budgets: { timeout_ms: timeoutMs },
      diagnostics: [{ code: "ts_sidecar_timeout", message: "timeout_ms exceeded before analysis started" }],
    },
    roots,
    snapshot,
  );
}

function startAnalysisAPI(snapshot: ProjectSnapshot, ApiCtor: CompilerBundle["api"], tsserverPath: string): ApiInstance {
  try {
    return new ApiCtor({ fs: snapshot.fs, tsserverPath });
  } catch (err) {
    throw new SidecarBackendError(`failed to start ts sidecar analysis backend: ${describeErrorWithoutPaths(err)}`);
  }
}

function runProjects(
  api: ApiInstance,
  snapshot: ProjectSnapshot,
  tsconfigPaths: readonly string[],
  opts: AnalyzeOptions,
  deadline: number | undefined,
  counts: Record<string, number>,
): AnalyzeResult {
  const snap = api.updateSnapshot({ openProjects: tsconfigPaths.map(toVirtualPath) });
  const projects = snap.getProjects();
  const visited = new Set<string>();
  const reachVisited = new Set<string>();
  const reachSourcesVisited = new Set<string>();
  const edges: ImportEdgeFact[] = [];
  const callGraph: CallGraphEdgeFact[] = [];
  const reachabilityFacts: ReachabilityFactWire[] = [];
  const diagnostics: Diagnostic[] = [];
  const seenConfigDiagnostics = new Set<string>();
  let complete = true;
  let projectsProcessed = 0;

  for (const project of projects) {
    if (opts.testDelayMsPerProject) busyWaitMs(opts.testDelayMsPerProject);
    if (deadline !== undefined && Date.now() >= deadline) {
      complete = false;
      diagnostics.push({
        code: "ts_sidecar_timeout",
        message: `timeout_ms (${opts.timeoutMs}) exceeded after processing ${projectsProcessed} of ${projects.length} project(s)`,
      });
      break;
    }
    const configResult = collectConfigDiagnostics(project, seenConfigDiagnostics);
    for (const key of configResult.newKeys) seenConfigDiagnostics.add(key);
    if (configResult.diagnostics.length > 0) complete = false;
    diagnostics.push(...configResult.diagnostics);
    const result = extractEdgesForProject(project, snapshot, visited, opts.compiler.ast);
    for (const path of result.newlyVisitedPaths) visited.add(path);
    edges.push(...result.edges);
    diagnostics.push(...result.diagnostics);
    const reachResult = processProjectReachability(
      project,
      snapshot,
      reachVisited,
      reachSourcesVisited,
      configResult.diagnostics.length > 0,
      opts.compiler,
    );
    callGraph.push(...reachResult.callGraph);
    reachabilityFacts.push(...reachResult.facts);
    diagnostics.push(...reachResult.diagnostics);
    projectsProcessed += 1;
  }

  return {
    edges: canonicalizeEdges(edges),
    callGraph: canonicalizeCallGraph(callGraph),
    reachabilityFacts: canonicalizeReachabilityFacts(reachabilityFacts),
    coverage: {
      phase: SIDECAR_PHASE,
      complete,
      counts: {
        ...counts,
        files_analyzed: visited.size,
        projects_analyzed: projectsProcessed,
      },
      budgets: deadline !== undefined ? { timeout_ms: opts.timeoutMs ?? 0 } : undefined,
      diagnostics: diagnostics.length > 0 ? canonicalizeDiagnostics(diagnostics) : undefined,
    },
    rootScopes: computeRootScopes(opts.roots, projects, snapshot, visited),
  };
}

function computeRootScopes(
  roots: readonly string[] | undefined,
  projects: readonly Project[],
  snapshot: ProjectSnapshot,
  visited: ReadonlySet<string>,
): RootScopeFact[] | undefined {
  if (!roots || roots.length === 0) return undefined;
  const normalizedRoots = [...new Set(roots.map(normalizeRoot))].sort();
  return normalizedRoots.map((root) => rootScopeFor(root, projects, snapshot, visited));
}

function rootScopeFor(
  root: string,
  projects: readonly Project[],
  snapshot: ProjectSnapshot,
  visited: ReadonlySet<string>,
): RootScopeFact {
  const candidates = new Set<string>();
  for (const project of projects) {
    const configRepoPath = fromVirtualPath(project.configFileName);
    if (configRepoPath === undefined || !isWithinRoot(configRepoPath, root)) continue;
    for (const virtualPath of project.rootFiles) {
      candidates.add(snapshot.canonicalizeVirtualPath(virtualPath));
    }
  }
  const analyzedPaths: string[] = [];
  const unanalyzedPaths: string[] = [];
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

// A project whose own config failed to parse never got a real Program
// built, so no call-graph/reachability extraction is attempted for it.
function processProjectReachability(
  project: Project,
  snapshot: ProjectSnapshot,
  reachVisited: Set<string>,
  reachSourcesVisited: Set<string>,
  configDiagnosticsPresent: boolean,
  compiler: CompilerBundle,
): { callGraph: CallGraphEdgeFact[]; facts: ReachabilityFactWire[]; diagnostics: Diagnostic[] } {
  if (configDiagnosticsPresent) return { callGraph: [], facts: [], diagnostics: [] };
  const reachResult = extractReachabilityForProject(project, snapshot, reachVisited, reachSourcesVisited, {
    ast: compiler.ast,
    symbolFlags: compiler.symbolFlags,
  });
  for (const path of reachResult.newlyVisitedPaths) reachVisited.add(path);
  for (const sourceId of reachResult.visitedSources) reachSourcesVisited.add(sourceId);
  return { callGraph: reachResult.callGraph, facts: reachResult.facts, diagnostics: reachResult.diagnostics };
}

function collectConfigDiagnostics(
  project: { configFileName: string; program: { getConfigFileParsingDiagnostics(): ReadonlyArray<{ code: number; text: string }> } },
  seen: ReadonlySet<string>,
): { diagnostics: Diagnostic[]; newKeys: string[] } {
  const configPath = fromVirtualPath(project.configFileName);
  const diagnostics: Diagnostic[] = [];
  const newKeys: string[] = [];
  for (const d of project.program.getConfigFileParsingDiagnostics()) {
    const key = `${d.code}|${configPath ?? ""}|${d.text}`;
    if (seen.has(key) || newKeys.includes(key)) continue;
    newKeys.push(key);
    // TS embeds the synthetic VIRTUAL_ROOT absolute path in some messages.
    const message = d.text.split(`${VIRTUAL_ROOT}/`).join("");
    diagnostics.push({ code: "ts_config_diagnostic", message, path: configPath });
  }
  return { diagnostics, newKeys };
}

function busyWaitMs(ms: number): void {
  const end = Date.now() + ms;
  while (Date.now() < end) {}
}
