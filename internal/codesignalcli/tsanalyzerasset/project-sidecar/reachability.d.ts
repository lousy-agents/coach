import { type Project } from "typescript/unstable/sync";
import { type CallGraphEdgeFact, type Diagnostic, type ReachabilityFactWire } from "./protocol.js";
import type { ProjectSnapshot } from "./vfs.js";
export interface ReachabilityExtractionResult {
    callGraph: CallGraphEdgeFact[];
    facts: ReachabilityFactWire[];
    diagnostics: Diagnostic[];
    /** Canonical virtual paths newly walked by this call (caller merges into its visit set), mirroring EdgeExtractionResult.visitedPaths. */
    visitedPaths: string[];
    /**
     * functionSourceId values walked by this call, including ones seeded via
     * alreadyVisitedSources -- the caller merges this into a request-wide
     * set (mirroring visitedPaths) so a handler function registered as a
     * route from more than one tsconfig project (e.g. a monorepo service
     * sharing one handler file) is walked, and its facts/diagnostics
     * emitted, exactly once across the whole request, not once per project.
     */
    visitedSources: string[];
}
/**
 * Walks every .ts/.tsx root file owned by `project` that has not already
 * been visited by another project's reachability extraction in this
 * request, finding `<receiver>.<verb>(path, handler)`-shaped route
 * registrations (see ROUTE_REGISTRATION_METHODS) and, for every handler
 * that resolves to a locally declared or re-exported function and whose
 * functionSourceId is not in alreadyVisitedSources, walking that
 * function's body (one level deep) for calls into the pinned sink
 * registry (REACHABILITY_SINK_CLASSES). alreadyVisitedSources is seeded
 * from every prior project's own walk in this request, so a handler
 * registered as a route from more than one tsconfig project is walked
 * exactly once -- without this, MutableAccumulator's factKeys/seenGapSites
 * dedup only within a single project's own walk, so the same handler
 * walked again from a second project would emit a second, duplicate
 * ReachabilityFact/CallGraphEdgeFact sharing the first one's ID, which
 * canonicalizeCallGraph/canonicalizeReachabilityFacts only sort, never
 * dedup. A call this walk cannot classify
 * as a resolved sink, a recognized coverage gap, or an unfollowed local
 * callee is left unreported only when it resolves to nothing this
 * snapshot owns at all (e.g. `console.log`) -- absence of a fact is never
 * a "verified safe" claim, only "not found within this walk".
 */
export declare function extractReachabilityForProject(project: Project, snapshot: ProjectSnapshot, alreadyVisited: ReadonlySet<string>, alreadyVisitedSources: ReadonlySet<string>): ReachabilityExtractionResult;
/** Sorts callGraph/facts by a stable key, mirroring canonical.ts's edge/diagnostic sorting so repeated runs are byte-identical. */
export declare function canonicalizeCallGraph(edges: readonly CallGraphEdgeFact[]): CallGraphEdgeFact[];
export declare function canonicalizeReachabilityFacts(facts: readonly ReachabilityFactWire[]): ReachabilityFactWire[];
//# sourceMappingURL=reachability.d.ts.map