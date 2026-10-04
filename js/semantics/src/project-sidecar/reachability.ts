import type * as astns from "typescript/unstable/ast";
import type { Project } from "typescript/unstable/sync";

import { ReachabilityAccumulator } from "./reachability-accumulator.js";
import { canonicalizeCallGraph, canonicalizeReachabilityFacts } from "./reachability-canonical.js";
import { isRouteRegistrationCall, processRouteRegistration } from "./reachability-routes.js";
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
export function extractReachabilityForProject(
  project: Project,
  snapshot: ProjectSnapshot,
  alreadyVisited: ReadonlySet<string>,
  alreadyVisitedSources: ReadonlySet<string>,
  compiler: AstCompiler,
): ReachabilityExtractionResult {
  const acc = new ReachabilityAccumulator(new Set(alreadyVisitedSources), new Set(), new Set());
  const newlyVisitedPaths: string[] = [];
  const seen = new Set(alreadyVisited);

  for (const virtualPath of project.rootFiles) {
    const canonicalVirtual = snapshot.canonicalizeVirtualPath(virtualPath);
    const repoPath = snapshot.toRepoPath(virtualPath);
    if (repoPath === undefined) continue;
    if (!(repoPath.endsWith(".ts") || repoPath.endsWith(".tsx"))) continue;
    if (seen.has(canonicalVirtual)) continue;
    seen.add(canonicalVirtual);
    newlyVisitedPaths.push(canonicalVirtual);

    const sf = project.program.getSourceFile(virtualPath) ?? project.program.getSourceFile(canonicalVirtual);
    if (!sf) continue;
    collectRouteRegistrations(sf, project, snapshot, acc, compiler);
  }

  return {
    callGraph: acc.callGraph,
    facts: acc.facts,
    diagnostics: acc.diagnostics,
    newlyVisitedPaths,
    visitedSources: [...acc.seenSources],
  };
}

function collectRouteRegistrations(
  sf: astns.SourceFile,
  project: Project,
  snapshot: ProjectSnapshot,
  acc: ReachabilityAccumulator,
  compiler: AstCompiler,
): void {
  const { ast } = compiler;
  const visit = (node: astns.Node): void => {
    if (ast.isCallExpression(node) && isRouteRegistrationCall(node, project, compiler)) {
      processRouteRegistration(node, sf, project, snapshot, acc, compiler);
    }
    node.forEachChild(visit);
  };
  visit(sf);
}
