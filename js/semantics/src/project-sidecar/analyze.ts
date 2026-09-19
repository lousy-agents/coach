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
import { testHookDropRoot } from "./test-hook.js";
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
  const scan = new ProjectScan(visited, reachVisited, reachSourcesVisited, edges, callGraph, reachabilityFacts, diagnostics, seenConfigDiagnostics);

  for (const project of projects) {
    if (opts.testDelayMsPerProject) busyWaitMs(opts.testDelayMsPerProject);
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

function computeRootScopes(
  roots: readonly string[] | undefined,
  projects: readonly Project[],
  snapshot: ProjectSnapshot,
  visited: ReadonlySet<string>,
): RootScopeFact[] | undefined {
  if (!roots || roots.length === 0) return undefined;
  const normalizedRoots = [...new Set(roots.map(normalizeRoot))].sort();
  const dropRoot = testHookDropRoot !== undefined ? normalizeRoot(testHookDropRoot) : undefined;
  return normalizedRoots.filter((root) => root !== dropRoot).map((root) => rootScopeFor(root, projects, snapshot, visited));
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

class ProjectScan {
  complete = true;
  projectsProcessed = 0;

  constructor(
    readonly visited: Set<string>,
    readonly reachVisited: Set<string>,
    readonly reachSourcesVisited: Set<string>,
    readonly edges: ImportEdgeFact[],
    readonly callGraph: CallGraphEdgeFact[],
    readonly reachabilityFacts: ReachabilityFactWire[],
    readonly diagnostics: Diagnostic[],
    readonly seenConfigDiagnostics: Set<string>,
  ) {}

  timedOut(timeoutMs: number | undefined, projectCount: number): void {
    this.complete = false;
    this.diagnostics.push({
      code: "ts_sidecar_timeout",
      message: `timeout_ms (${timeoutMs}) exceeded after processing ${this.projectsProcessed} of ${projectCount} project(s)`,
    });
  }

  ingest(project: Project, snapshot: ProjectSnapshot, compiler: CompilerBundle): void {
    const ingested = ingestProject(
      project,
      snapshot,
      this.visited,
      this.reachVisited,
      this.reachSourcesVisited,
      this.seenConfigDiagnostics,
      compiler,
    );
    for (const key of ingested.configKeys) this.seenConfigDiagnostics.add(key);
    if (!ingested.configComplete) this.complete = false;
    this.diagnostics.push(...ingested.diagnostics);
    for (const path of ingested.newlyVisited) this.visited.add(path);
    this.edges.push(...ingested.edges);
    for (const path of ingested.reachNewlyVisited) this.reachVisited.add(path);
    for (const sourceId of ingested.reachSources) this.reachSourcesVisited.add(sourceId);
    this.callGraph.push(...ingested.callGraph);
    this.reachabilityFacts.push(...ingested.reachabilityFacts);
    this.projectsProcessed += 1;
  }
}

function ingestProject(
  project: Project,
  snapshot: ProjectSnapshot,
  visited: ReadonlySet<string>,
  reachVisited: ReadonlySet<string>,
  reachSourcesVisited: ReadonlySet<string>,
  seenConfigDiagnostics: ReadonlySet<string>,
  compiler: CompilerBundle,
): {
  configKeys: string[];
  configComplete: boolean;
  diagnostics: Diagnostic[];
  newlyVisited: string[];
  edges: ImportEdgeFact[];
  reachNewlyVisited: string[];
  reachSources: string[];
  callGraph: CallGraphEdgeFact[];
  reachabilityFacts: ReachabilityFactWire[];
} {
  const configResult = collectConfigDiagnostics(project, seenConfigDiagnostics);
  const result = extractEdgesForProject(project, snapshot, visited, compiler.ast);
  const reachResult = processProjectReachability(
    project,
    snapshot,
    reachVisited,
    reachSourcesVisited,
    configResult.diagnostics.length > 0,
    compiler,
  );
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
function processProjectReachability(
  project: Project,
  snapshot: ProjectSnapshot,
  reachVisited: ReadonlySet<string>,
  reachSourcesVisited: ReadonlySet<string>,
  configDiagnosticsPresent: boolean,
  compiler: CompilerBundle,
): { callGraph: CallGraphEdgeFact[]; facts: ReachabilityFactWire[]; diagnostics: Diagnostic[]; newlyVisitedPaths: string[]; visitedSources: string[] } {
  if (configDiagnosticsPresent) {
    return { callGraph: [], facts: [], diagnostics: [], newlyVisitedPaths: [], visitedSources: [] };
  }
  return extractReachabilityForProject(project, snapshot, reachVisited, reachSourcesVisited, {
    ast: compiler.ast,
    symbolFlags: compiler.symbolFlags,
  });
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
