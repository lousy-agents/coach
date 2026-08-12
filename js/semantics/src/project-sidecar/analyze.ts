import { API } from "typescript/unstable/sync";

import { canonicalizeDiagnostics, canonicalizeEdges } from "./canonical.js";
import { discoverTsconfigPaths } from "./discover.js";
import { extractEdgesForProject } from "./edges.js";
import { SIDECAR_PHASE, type Coverage, type Diagnostic, type ImportEdgeFact, type ProjectFile } from "./protocol.js";
import { buildProjectSnapshot, fromVirtualPath, toVirtualPath, VIRTUAL_ROOT } from "./vfs.js";

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
    return { edges: [], coverage: { phase: SIDECAR_PHASE, complete: true, counts } };
  }
  if (deadline !== undefined && Date.now() >= deadline) {
    return {
      edges: [],
      coverage: {
        phase: SIDECAR_PHASE,
        complete: false,
        counts,
        budgets: { timeout_ms: opts.timeoutMs ?? 0 },
        diagnostics: [{ code: "ts_sidecar_timeout", message: "timeout_ms exceeded before analysis started" }],
      },
    };
  }

  let api: API;
  try {
    api = new API({ fs: snapshot.fs });
  } catch (err) {
    throw new SidecarBackendError(`failed to start ts sidecar analysis backend: ${String(err)}`);
  }

  const edges: ImportEdgeFact[] = [];
  const diagnostics: Diagnostic[] = [];
  // Dedupes identical (numeric TS code, path, message) config-parsing
  // diagnostics: a single malformed tsconfig can otherwise repeat the same
  // parser message many times, and the wire Diagnostic carries no position
  // to distinguish them for a consumer anyway.
  const seenConfigDiagnostics = new Set<string>();
  let complete = true;

  try {
    const snap = api.updateSnapshot({ openProjects: tsconfigPaths.map(toVirtualPath) });
    const projects = snap.getProjects();
    const visited = new Set<string>();
    let projectsProcessed = 0;

    for (const project of projects) {
      if (opts.testDelayMsPerProject) {
        busyWaitMs(opts.testDelayMsPerProject);
      }
      if (deadline !== undefined && Date.now() >= deadline) {
        complete = false;
        diagnostics.push({
          code: "ts_sidecar_timeout",
          message: `timeout_ms (${opts.timeoutMs}) exceeded after processing ${projectsProcessed} of ${projects.length} project(s)`,
        });
        break;
      }
      const configPath = fromVirtualPath(project.configFileName);
      for (const d of project.program.getConfigFileParsingDiagnostics()) {
        // A config that failed to parse cleanly was never fully honored
        // (aliases/references it declared may have been silently lost), so
        // this response cannot claim `complete: true` even though analysis
        // otherwise ran to completion.
        complete = false;
        const key = `${d.code}|${configPath ?? ""}|${d.text}`;
        if (seenConfigDiagnostics.has(key)) continue;
        seenConfigDiagnostics.add(key);
        // TS's own config-parsing diagnostics embed the config file's
        // *absolute* path in the message text (e.g. "No inputs were found
        // in config file '<path>'"), which -- unlike the diagnostic's own
        // `path` field -- is the synthetic VIRTUAL_ROOT, not a
        // repo-relative one. Strip it so the wire message never leaks this
        // internal implementation detail; genuinely-external paths (e.g. a
        // real-disk "extends" target) are untouched.
        const message = d.text.split(`${VIRTUAL_ROOT}/`).join("");
        diagnostics.push({ code: "ts_config_diagnostic", message, path: configPath });
      }
      const result = extractEdgesForProject(project, snapshot, visited);
      edges.push(...result.edges);
      diagnostics.push(...result.diagnostics);
      projectsProcessed += 1;
    }
    counts.files_analyzed = visited.size;
    counts.projects_analyzed = projectsProcessed;
  } finally {
    api.close();
  }

  const coverage: Coverage = {
    phase: SIDECAR_PHASE,
    complete,
    counts,
    // Reported consistently for both timeout paths (the pre-analysis check
    // above, and the mid-loop check in this function): present whenever a
    // deadline was in effect, regardless of which path enforced it.
    budgets: deadline !== undefined ? { timeout_ms: opts.timeoutMs ?? 0 } : undefined,
    diagnostics: diagnostics.length > 0 ? canonicalizeDiagnostics(diagnostics) : undefined,
  };
  return { edges: canonicalizeEdges(edges), coverage };
}

/** Test-only: deliberately blocks the event loop for `ms` milliseconds to
 * simulate a slow analysis phase (see AnalyzeOptions.testDelayMsPerProject). */
function busyWaitMs(ms: number): void {
  const end = Date.now() + ms;
  while (Date.now() < end) {
    // Deliberate synchronous busy-wait; test-only.
  }
}
