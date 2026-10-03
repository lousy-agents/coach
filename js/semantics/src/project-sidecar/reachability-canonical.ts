import type { CallGraphEdgeFact, ReachabilityFactWire } from "./protocol.js";

/** Sorts callGraph/facts by a stable key, mirroring canonical.ts's edge/diagnostic sorting so repeated runs are byte-identical. */
export function canonicalizeCallGraph(edges: readonly CallGraphEdgeFact[]): CallGraphEdgeFact[] {
  return [...edges].sort((a, b) => compare(a.from, b.from) || compare(a.to, b.to));
}

export function canonicalizeReachabilityFacts(facts: readonly ReachabilityFactWire[]): ReachabilityFactWire[] {
  return [...facts].sort((a, b) => compare(a.source, b.source) || compare(a.sink, b.sink) || compare(a.id, b.id));
}

function compare(a: string, b: string): number {
  if (a < b) return -1;
  if (a > b) return 1;
  return 0;
}
