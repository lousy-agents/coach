import type { CallGraphEdgeFact, ReachabilityFactWire } from "./protocol.js";
/** Sorts callGraph/facts by a stable key, mirroring canonical.ts's edge/diagnostic sorting so repeated runs are byte-identical. */
export declare function canonicalizeCallGraph(edges: readonly CallGraphEdgeFact[]): CallGraphEdgeFact[];
export declare function canonicalizeReachabilityFacts(facts: readonly ReachabilityFactWire[]): ReachabilityFactWire[];
//# sourceMappingURL=reachability-canonical.d.ts.map