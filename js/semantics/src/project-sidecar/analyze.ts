import { API, type Project } from "typescript/unstable/sync";

import { canonicalizeDiagnostics, canonicalizeEdges } from "./canonical.js";
import { discoverTsconfigPaths } from "./discover.js";
import { extractEdgesForProject } from "./edges.js";
import { SIDECAR_PHASE, type CallGraphEdgeFact, type Coverage, type Diagnostic, type ImportEdgeFact, type ProjectFile, type ReachabilityFactWire } from "./protocol.js";
import { canonicalizeCallGraph, canonicalizeReachabilityFacts, extractReachabilityForProject } from "./reachability.js";
import { buildProjectSnapshot, fromVirtualPath, toVirtualPath, VIRTUAL_ROOT, type ProjectSnapshot } from "./vfs.js";

/** Thrown only for genuine backend-startup failures (e.g. the bundled
 * native tsgo binary failing to spawn); main.ts turns this into a
 * whole-request Response.Error rather than crashing the process, since a
 * missing/unspawnable backend is an operational condition analogous to
 * pkg/projectmodel's own DiagBackendUnavailable, not a programming bug. */
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
}

export interface AnalyzeResult {
  edges: ImportEdgeFact[];
  callGraph: CallGraphEdgeFact[];
  reachabilityFacts: ReachabilityFactWire[];
  coverage: Coverage;
}

export function analyzeProject(opts: AnalyzeOptions): AnalyzeResult {
  const deadline = opts.timeoutMs && opts.timeoutMs > 0 ? Date.now() + opts.timeoutMs : undefined;
  const snapshot = buildProjectSnapshot(opts.files);
  const tsconfigPaths = discoverTsconfigPaths(opts.files, opts.roots);
  const counts: Record<string, number> = { files_seen: opts.files.length, tsconfig_count: tsconfigPaths.length };

  if (tsconfigPaths.length === 0) {
    return emptyComplete(counts, opts.files);
  }
  if (deadline !== undefined && Date.now() >= deadline) {
    return timeoutBeforeStart(counts, opts.timeoutMs ?? 0);
  }

  const api = startAnalysisAPI(snapshot);
  try {
    return runProjects(api, snapshot, tsconfigPaths, opts, deadline, counts);
  } finally {
    api.close();
  }
}

/**
 * Reports the vacuous-project result when no tsconfig.json was discovered.
 * A snapshot that genuinely contains no TypeScript/TSX sources has nothing
 * to analyze, so it stays Complete=true with empty edges. A snapshot that
 * does contain .ts/.tsx sources but no project config would otherwise
 * silently invent a "complete" empty graph over unanalyzed sources -- the
 * same false-complete failure mode this sidecar already guards against for
 * missing config forwarding -- so that case reports Complete=false with a
 * stable ts_no_project_config diagnostic instead.
 */
function emptyComplete(counts: Record<string, number>, files: readonly ProjectFile[]): AnalyzeResult {
  const hasTsSources = files.some((f) => f.path.endsWith(".ts") || f.path.endsWith(".tsx"));
  if (!hasTsSources) {
    return { edges: [], callGraph: [], reachabilityFacts: [], coverage: { phase: SIDECAR_PHASE, complete: true, counts } };
  }
  return {
    edges: [],
    callGraph: [],
    reachabilityFacts: [],
    coverage: {
      phase: SIDECAR_PHASE,
      complete: false,
      counts,
      diagnostics: [
        { code: "ts_no_project_config", message: "no tsconfig.json was discovered while .ts/.tsx sources were provided" },
      ],
    },
  };
}

function timeoutBeforeStart(counts: Record<string, number>, timeoutMs: number): AnalyzeResult {
  return {
    edges: [],
    callGraph: [],
    reachabilityFacts: [],
    coverage: {
      phase: SIDECAR_PHASE,
      complete: false,
      counts,
      budgets: { timeout_ms: timeoutMs },
      diagnostics: [{ code: "ts_sidecar_timeout", message: "timeout_ms exceeded before analysis started" }],
    },
  };
}

function startAnalysisAPI(snapshot: ProjectSnapshot): API {
  try {
    return new API({ fs: snapshot.fs });
  } catch (err) {
    throw new SidecarBackendError(`failed to start ts sidecar analysis backend: ${String(err)}`);
  }
}

function runProjects(
  api: API,
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
    const result = extractEdgesForProject(project, snapshot, visited);
    for (const path of result.visitedPaths) visited.add(path);
    edges.push(...result.edges);
    diagnostics.push(...result.diagnostics);
    const reachResult = processProjectReachability(project, snapshot, reachVisited, configResult.diagnostics.length > 0);
    callGraph.push(...reachResult.callGraph);
    reachabilityFacts.push(...reachResult.facts);
    // A ts_reachability_*_gap diagnostic (see processProjectReachability)
    // means one hop's reachability was deliberately left unverified, not
    // that import/config analysis for this project failed -- unlike Go's
    // Complete gate (unresolved interface/function-value/framework-
    // registration sites there are also counts+diagnostics, never a
    // Complete flip; see pkg/projectmodel/go_callgraph.go), a routine
    // one-hop delegation into a helper/service function is the ordinary
    // shape of layered code, not a rare failure. Folding it into this
    // project-wide Complete bit would mark most real TS trees incomplete
    // and, via internal/codesignalcli/project_ts_backend.go's passthrough,
    // degrade an unrelated already-shipped architecture.layer_violation to
    // lifecycle unknown. Reachability's own incompleteness is reported
    // independently on ReachabilityResult.Coverage/LayerBypassResult.Coverage
    // instead (see pkg/projectmodel/ts_reachability.go, ts_layer_bypass.go)
    // -- so, unlike configResult's diagnostics above, these never set
    // complete = false here.
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
  };
}

/**
 * Runs call-graph/reachability extraction for one project, unless
 * configDiagnosticsPresent -- a project whose own config failed to parse
 * never got a real Program built from the sender's intent, so no
 * call-graph/reachability extraction is attempted for it either, mirroring
 * runProjects's own complete=false gate on config diagnostics.
 */
function processProjectReachability(
  project: Project,
  snapshot: ProjectSnapshot,
  reachVisited: Set<string>,
  configDiagnosticsPresent: boolean,
): { callGraph: CallGraphEdgeFact[]; facts: ReachabilityFactWire[]; diagnostics: Diagnostic[] } {
  if (configDiagnosticsPresent) return { callGraph: [], facts: [], diagnostics: [] };
  const reachResult = extractReachabilityForProject(project, snapshot, reachVisited);
  for (const path of reachResult.visitedPaths) reachVisited.add(path);
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
  while (Date.now() < end) {
    // Deliberate synchronous busy-wait; test-only.
  }
}
