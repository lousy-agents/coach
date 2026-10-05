import type { Project } from "typescript/unstable/sync";
import { canonicalizeCallGraph, canonicalizeReachabilityFacts } from "./reachability-canonical.js";
import type { AstCompiler, ReachabilityExtractionResult } from "./reachability-types.js";
import type { ProjectSnapshot } from "./vfs.js";
export { canonicalizeCallGraph, canonicalizeReachabilityFacts };
export type { AstCompiler, ReachabilityExtractionResult };
/**
 * alreadyVisitedSources is seeded from every prior project's own walk in
 * this request, so a handler registered as a route from more than one
 * tsconfig project is walked exactly once -- without this,
 * ReachabilityAccumulator's factKeys/seenGapSites dedup only within a single
 * project's own walk, so the same handler walked again from a second
 * project would emit a second, duplicate ReachabilityFact/CallGraphEdgeFact
 * sharing the first one's ID, which
 * canonicalizeCallGraph/canonicalizeReachabilityFacts only sort, never
 * dedup. Absence of a fact is never a "verified safe" claim, only "not
 * found within this walk".
 */
export declare function extractReachabilityForProject(project: Project, snapshot: ProjectSnapshot, alreadyVisited: ReadonlySet<string>, alreadyVisitedSources: ReadonlySet<string>, compiler: AstCompiler): ReachabilityExtractionResult;
//# sourceMappingURL=reachability.d.ts.map