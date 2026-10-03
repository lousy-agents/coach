import type * as astns from "typescript/unstable/ast";

import {
  KIND_POSSIBLE_CALL_REACHABILITY,
  type CallGraphEdgeFact,
  type Diagnostic,
  type ReachabilityFactWire,
} from "./protocol.js";
import {
  CONFIDENCE_RESOLVED_DIRECT,
  REACHABILITY_ALGORITHM,
  REACHABILITY_BACKEND,
} from "./reachability-registry.js";
import type { ProjectSnapshot } from "./vfs.js";

export class ReachabilityAccumulator {
  readonly callGraph: CallGraphEdgeFact[] = [];
  readonly facts: ReachabilityFactWire[] = [];
  readonly diagnostics: Diagnostic[] = [];
  readonly seenSources: Set<string>;
  private readonly factKeys: Set<string>;
  private readonly seenGapSites: Set<string>;

  constructor(seenSources: Set<string>, factKeys: Set<string>, seenGapSites: Set<string>) {
    this.seenSources = seenSources;
    this.factKeys = factKeys;
    this.seenGapSites = seenGapSites;
  }

  noteSource(sourceId: string): boolean {
    if (this.seenSources.has(sourceId)) return false;
    this.seenSources.add(sourceId);
    return true;
  }

  recordFact(sourceId: string, sinkId: string): void {
    const factKey = `${sourceId}->${sinkId}`;
    if (this.factKeys.has(factKey)) return;
    this.factKeys.add(factKey);
    this.callGraph.push({ from: sourceId, to: sinkId });
    this.facts.push({
      id: `reach:${factKey}@${REACHABILITY_ALGORITHM}`,
      kind: KIND_POSSIBLE_CALL_REACHABILITY,
      confidence: CONFIDENCE_RESOLVED_DIRECT,
      source: sourceId,
      sink: sinkId,
      path: [{ node_id: sourceId }, { node_id: sinkId }],
      algorithm_version: REACHABILITY_ALGORITHM,
      backend: REACHABILITY_BACKEND,
    });
  }

  recordGap(code: string, message: string, node: astns.Node, sf: astns.SourceFile, snapshot: ProjectSnapshot): void {
    const repoPath = snapshot.toRepoPath(sf.path);
    const { line } = sf.getLineAndCharacterOfPosition(node.getStart(sf));
    const site = `${repoPath ?? sf.fileName}:${line + 1}`;
    const key = `${code}|${site}`;
    if (this.seenGapSites.has(key)) return;
    this.seenGapSites.add(key);
    this.diagnostics.push({ code, message, path: repoPath });
  }
}
