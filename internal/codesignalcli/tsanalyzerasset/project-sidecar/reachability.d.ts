import type * as astns from "typescript/unstable/ast";
import type { Project, SymbolFlags } from "typescript/unstable/sync";
import { type CallGraphEdgeFact, type Diagnostic, type ReachabilityFactWire } from "./protocol.js";
import type { ProjectSnapshot } from "./vfs.js";
export interface AstCompiler {
    ast: typeof astns;
    symbolFlags: typeof SymbolFlags;
}
export interface ReachabilityExtractionResult {
    callGraph: CallGraphEdgeFact[];
    facts: ReachabilityFactWire[];
    diagnostics: Diagnostic[];
    newlyVisitedPaths: string[];
    visitedSources: string[];
}
/**
 * alreadyVisitedSources is seeded from every prior project's own walk in
 * this request, so a handler registered as a route from more than one
 * tsconfig project is walked exactly once -- without this,
 * MutableAccumulator's factKeys/seenGapSites dedup only within a single
 * project's own walk, so the same handler walked again from a second
 * project would emit a second, duplicate ReachabilityFact/CallGraphEdgeFact
 * sharing the first one's ID, which
 * canonicalizeCallGraph/canonicalizeReachabilityFacts only sort, never
 * dedup. Absence of a fact is never a "verified safe" claim, only "not
 * found within this walk".
 */
export declare function extractReachabilityForProject(project: Project, snapshot: ProjectSnapshot, alreadyVisited: ReadonlySet<string>, alreadyVisitedSources: ReadonlySet<string>, compiler: AstCompiler): ReachabilityExtractionResult;
/** Sorts callGraph/facts by a stable key, mirroring canonical.ts's edge/diagnostic sorting so repeated runs are byte-identical. */
export declare function canonicalizeCallGraph(edges: readonly CallGraphEdgeFact[]): CallGraphEdgeFact[];
export declare function canonicalizeReachabilityFacts(facts: readonly ReachabilityFactWire[]): ReachabilityFactWire[];
//# sourceMappingURL=reachability.d.ts.map