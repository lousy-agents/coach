import { API } from "typescript/unstable/sync";

import { canonicalizeDiagnostics, canonicalizeEdges } from "./canonical.js";
import { discoverTsconfigPaths } from "./discover.js";
import { extractEdgesForProject } from "./edges.js";
import { SIDECAR_PHASE, type Coverage, type Diagnostic, type ImportEdgeFact, type ProjectFile } from "./protocol.js";
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
  coverage: Coverage;
}

export function analyzeProject(opts: AnalyzeOptions): AnalyzeResult {
  const deadline = opts.timeoutMs && opts.timeoutMs > 0 ? Date.now() + opts.timeoutMs : undefined;
  const snapshot = buildProjectSnapshot(opts.files);
  const tsconfigPaths = discoverTsconfigPaths(opts.files, opts.roots);
  const counts: Record<string, number> = { files_seen: opts.files.length, tsconfig_count: tsconfigPaths.length };

  if (tsconfigPaths.length === 0) {
    return emptyComplete(counts);
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

function emptyComplete(counts: Record<string, number>): AnalyzeResult {
  return { edges: [], coverage: { phase: SIDECAR_PHASE, complete: true, counts } };
}

function timeoutBeforeStart(counts: Record<string, number>, timeoutMs: number): AnalyzeResult {
  return {
    edges: [],
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
  const edges: ImportEdgeFact[] = [];
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
    projectsProcessed += 1;
  }

  counts.files_analyzed = visited.size;
  counts.projects_analyzed = projectsProcessed;
  return {
    edges: canonicalizeEdges(edges),
    coverage: {
      phase: SIDECAR_PHASE,
      complete,
      counts,
      budgets: deadline !== undefined ? { timeout_ms: opts.timeoutMs ?? 0 } : undefined,
      diagnostics: diagnostics.length > 0 ? canonicalizeDiagnostics(diagnostics) : undefined,
    },
  };
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
