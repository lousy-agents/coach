import type { Project } from "typescript/unstable/sync";

import type { CompilerBundle } from "./analyze-types.js";
import { extractEdgesForProject } from "./edges.js";
import type { CallGraphEdgeFact, Diagnostic, ImportEdgeFact, ReachabilityFactWire } from "./protocol.js";
import { extractReachabilityForProject } from "./reachability.js";
import { fromVirtualPath, VIRTUAL_ROOT, type ProjectSnapshot } from "./vfs.js";

export class ProjectScan {
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

export function busyWaitMs(ms: number): void {
  const end = Date.now() + ms;
  while (Date.now() < end) {}
}
