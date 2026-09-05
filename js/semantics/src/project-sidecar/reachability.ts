import type * as astns from "typescript/unstable/ast";
import type { Project, SymbolFlags } from "typescript/unstable/sync";

import {
  KIND_POSSIBLE_CALL_REACHABILITY,
  RESOLUTION_SNAPSHOT,
  type CallGraphEdgeFact,
  type Diagnostic,
  type ReachabilityFactWire,
} from "./protocol.js";
import { resolveTarget } from "./edges-resolve.js";
import {
  CONFIDENCE_RESOLVED_DIRECT,
  GAP_DYNAMIC_IMPORT,
  GAP_LOCAL_CALL_NOT_FOLLOWED,
  GAP_TYPE_ONLY,
  GAP_UNRESOLVED_HANDLER,
  GAP_UNRESOLVED_TYPE,
  REACHABILITY_ALGORITHM,
  REACHABILITY_BACKEND,
  ROUTE_REGISTRATION_METHODS,
  sinkNodeId,
} from "./reachability-registry.js";
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

interface MutableAccumulator {
  callGraph: CallGraphEdgeFact[];
  facts: ReachabilityFactWire[];
  diagnostics: Diagnostic[];
  /** `${sourceId}->${sinkId}` pairs already emitted, so a source calling
   * the same sink more than once yields exactly one fact/edge, matching
   * Go's ReachabilityFact dedup (pkg/projectmodel/go_reachability.go). */
  factKeys: Set<string>;
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
export function extractReachabilityForProject(
  project: Project,
  snapshot: ProjectSnapshot,
  alreadyVisited: ReadonlySet<string>,
  alreadyVisitedSources: ReadonlySet<string>,
  compiler: AstCompiler,
): ReachabilityExtractionResult {
  const out: MutableAccumulator = { callGraph: [], facts: [], diagnostics: [], factKeys: new Set() };
  const newlyVisitedPaths: string[] = [];
  const seen = new Set(alreadyVisited);
  const seenSources = new Set(alreadyVisitedSources);
  const seenGapSites = new Set<string>();

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
    collectRouteRegistrations(sf, project, snapshot, out, seenSources, seenGapSites, compiler);
  }

  return { ...out, newlyVisitedPaths, visitedSources: [...seenSources] };
}

function collectRouteRegistrations(
  sf: astns.SourceFile,
  project: Project,
  snapshot: ProjectSnapshot,
  out: MutableAccumulator,
  seenSources: Set<string>,
  seenGapSites: Set<string>,
  compiler: AstCompiler,
): void {
  const { ast } = compiler;
  const visit = (node: astns.Node): void => {
    if (ast.isCallExpression(node) && isRouteRegistrationCall(node, project, compiler)) {
      processRouteRegistration(node, sf, project, snapshot, out, seenSources, seenGapSites, compiler);
    }
    node.forEachChild(visit);
  };
  visit(sf);
}

function isRouteRegistrationCall(call: astns.CallExpression, project: Project, compiler: AstCompiler): boolean {
  const { ast } = compiler;
  if (!ast.isPropertyAccessExpression(call.expression)) return false;
  const callee = call.expression;
  if (!ast.isIdentifier(callee.name) || !ROUTE_REGISTRATION_METHODS.has(callee.name.text)) return false;
  if (call.arguments.length < 2 || !ast.isStringLiteral(call.arguments[0])) return false;
  const receiverType = project.checker.getTypeAtLocation(callee.expression);
  if (!receiverType) return false;
  return project.checker.getPropertyOfType(receiverType, callee.name.text) !== undefined;
}

function processRouteRegistration(
  call: astns.CallExpression,
  sf: astns.SourceFile,
  project: Project,
  snapshot: ProjectSnapshot,
  out: MutableAccumulator,
  seenSources: Set<string>,
  seenGapSites: Set<string>,
  compiler: AstCompiler,
): void {
  const handlerArg = call.arguments[1];
  const fn = resolveHandlerFunction(handlerArg, project, compiler);
  if (fn) {
    const sourceId = functionSourceId(fn, snapshot);
    if (sourceId && !seenSources.has(sourceId)) {
      seenSources.add(sourceId);
      walkSourceForReachability(fn, sourceId, out, snapshot, project, seenGapSites, compiler);
    }
    return;
  }
  if (containsDynamicImport(handlerArg, compiler.ast)) {
    recordGapDiagnostic(
      GAP_DYNAMIC_IMPORT,
      "route handler is resolved through a dynamic import, so it cannot be statically added as a reachability source",
      handlerArg,
      sf,
      snapshot,
      out,
      seenGapSites,
    );
    return;
  }
  recordGapDiagnostic(
    GAP_UNRESOLVED_HANDLER,
    "route handler did not resolve to a locally declared named function (e.g. an inline arrow/function expression, or a handler bound through something other than a direct or re-exported function declaration), so it cannot be statically added as a reachability source",
    handlerArg,
    sf,
    snapshot,
    out,
    seenGapSites,
  );
}

/**
 * Resolves handlerArg to the FunctionDeclaration it names, following one
 * alias hop via Checker.getAliasedSymbol when the identifier's symbol
 * itself is an alias (e.g. `import { h } from "./handlers"` -- the
 * identifier's own declaration is the ImportSpecifier, not the function).
 */
function resolveHandlerFunction(handlerArg: astns.Expression, project: Project, compiler: AstCompiler): astns.FunctionDeclaration | undefined {
  const { ast, symbolFlags } = compiler;
  if (!ast.isIdentifier(handlerArg)) return undefined;
  const symbol = project.checker.getSymbolAtLocation(handlerArg);
  if (!symbol) return undefined;
  const resolvedSymbol = (symbol.flags & symbolFlags.Alias) !== 0 ? project.checker.getAliasedSymbol(symbol) : symbol;
  const declNode = resolvedSymbol.declarations?.[0]?.resolve(project);
  return declNode && ast.isFunctionDeclaration(declNode) && declNode.name ? declNode : undefined;
}

function containsDynamicImport(node: astns.Node, ast: typeof astns): boolean {
  if (ast.isCallExpression(node) && node.expression.kind === ast.SyntaxKind.ImportKeyword) return true;
  let found = false;
  node.forEachChild((child) => {
    if (!found) found = containsDynamicImport(child, ast);
  });
  return found;
}

function functionSourceId(fn: astns.FunctionDeclaration, snapshot: ProjectSnapshot): string | undefined {
  if (!fn.name) return undefined;
  const repoPath = snapshot.toRepoPath(fn.getSourceFile().path);
  if (repoPath === undefined) return undefined;
  return `file:${repoPath}#${fn.name.text}`;
}

function walkSourceForReachability(
  fn: astns.FunctionDeclaration,
  sourceId: string,
  out: MutableAccumulator,
  snapshot: ProjectSnapshot,
  project: Project,
  seenGapSites: Set<string>,
  compiler: AstCompiler,
): void {
  if (!fn.body) return;
  const { ast } = compiler;
  const sf = fn.getSourceFile();
  const visit = (node: astns.Node): void => {
    if (ast.isCallExpression(node)) {
      handleCallInSource(node, sourceId, sf, project, snapshot, out, seenGapSites, compiler);
    }
    node.forEachChild(visit);
  };
  visit(fn.body);
}

function handleCallInSource(
  call: astns.CallExpression,
  sourceId: string,
  sf: astns.SourceFile,
  project: Project,
  snapshot: ProjectSnapshot,
  out: MutableAccumulator,
  seenGapSites: Set<string>,
  compiler: AstCompiler,
): void {
  const declNode = resolvedCallDeclaration(call, project);
  const sinkId = declNode ? sinkIdForDeclaration(declNode, compiler.ast) : undefined;
  if (sinkId) {
    recordReachabilityFact(sourceId, sinkId, out);
    return;
  }
  const gap = classifyCalleeGap(call, project, snapshot, compiler);
  if (gap) {
    const { code, message } = gapDiagnosticInfo(gap);
    recordGapDiagnostic(code, message, call, sf, snapshot, out, seenGapSites);
    return;
  }
  if (declNode && isUnfollowedLocalCallee(declNode, snapshot, compiler.ast)) {
    recordGapDiagnostic(
      GAP_LOCAL_CALL_NOT_FOLLOWED,
      "call target resolves to a function declared within this snapshot that this depth-1 walk does not follow further, so multi-hop reachability from here is unverified",
      call,
      sf,
      snapshot,
      out,
      seenGapSites,
    );
  }
}

function recordReachabilityFact(sourceId: string, sinkId: string, out: MutableAccumulator): void {
  const factKey = `${sourceId}->${sinkId}`;
  if (out.factKeys.has(factKey)) return;
  out.factKeys.add(factKey);
  out.callGraph.push({ from: sourceId, to: sinkId });
  out.facts.push({
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

function resolvedCallDeclaration(call: astns.CallExpression, project: Project): astns.Node | undefined {
  const signature = project.checker.getResolvedSignature(call);
  return signature?.declaration?.resolve(project);
}

/**
 * Walks .parent from a resolved method declaration to the nearest
 * enclosing class/interface name -- e.g. `prisma.user.findMany()`
 * resolves to a MethodDeclaration nested inside an object-literal
 * property of class PrismaClient, so the enclosing class (not the
 * immediate object-literal type) names the sink -- then defers to
 * sinkNodeId for both the name and module-provenance match.
 */
function sinkIdForDeclaration(declNode: astns.Node, ast: typeof astns): string | undefined {
  if (!ast.isMethodDeclaration(declNode) || !ast.isIdentifier(declNode.name)) return undefined;
  const className = enclosingClassName(declNode, ast);
  if (!className) return undefined;
  return sinkNodeId(className, declNode.name.text, declNode.getSourceFile().path);
}

function enclosingClassName(node: astns.Node, ast: typeof astns): string | undefined {
  let cur: astns.Node | undefined = node.parent;
  while (cur && !ast.isSourceFile(cur)) {
    if ((ast.isClassDeclaration(cur) || ast.isClassExpression(cur)) && cur.name) return cur.name.text;
    cur = cur.parent;
  }
  return undefined;
}

function isUnfollowedLocalCallee(declNode: astns.Node, snapshot: ProjectSnapshot, ast: typeof astns): boolean {
  const isFunctionLike =
    ast.isFunctionDeclaration(declNode) ||
    ast.isFunctionExpression(declNode) ||
    ast.isArrowFunction(declNode) ||
    ast.isMethodDeclaration(declNode);
  if (!isFunctionLike) return false;
  return snapshot.toRepoPath(declNode.getSourceFile().path) !== undefined;
}

type GapKind = "type_only" | "unresolved_external" | "dynamic_import";

function gapDiagnosticInfo(gap: GapKind): { code: string; message: string } {
  switch (gap) {
    case "type_only":
      return {
        code: GAP_TYPE_ONLY,
        message: "call target resolves through a type-only import binding, so further reachability from here is unverified",
      };
    case "dynamic_import":
      return {
        code: GAP_DYNAMIC_IMPORT,
        message: "call target is bound through a dynamic import, so it cannot be statically added as a reachability sink",
      };
    case "unresolved_external":
      return {
        code: GAP_UNRESOLVED_TYPE,
        message:
          "call target resolves through an import that did not resolve within the snapshot, so further reachability from here is unverified",
      };
  }
}

// A symbol with no local import (e.g. an ambient global like `console`
// that resolves to no declaration at all under this snapshot's lib set)
// is not a gap -- there is nothing to extend from, so it is silently
// ignored rather than misreported as an unresolved import.
function classifyCalleeGap(call: astns.CallExpression, project: Project, snapshot: ProjectSnapshot, compiler: AstCompiler): GapKind | undefined {
  const { ast } = compiler;
  const baseIdent = leftmostIdentifier(call.expression, ast);
  if (!baseIdent) return undefined;
  const symbol = project.checker.getSymbolAtLocation(baseIdent);
  const declNode = symbol?.declarations?.[0]?.resolve(project);
  if (!declNode) return undefined;

  if (ast.isImportSpecifier(declNode) && declNode.isTypeOnly) return "type_only";
  if (ast.isImportClause(declNode) && declNode.phaseModifier === ast.SyntaxKind.TypeKeyword) return "type_only";
  if (ast.isVariableDeclaration(declNode) && declNode.initializer && containsDynamicImport(declNode.initializer, ast)) {
    return "dynamic_import";
  }

  const importDecl = enclosingImportDeclaration(declNode, ast);
  if (!importDecl || !ast.isStringLiteral(importDecl.moduleSpecifier)) return undefined;
  const fromVirtualPath = importDecl.getSourceFile().path;
  const target = resolveTarget(project, importDecl.moduleSpecifier, importDecl.moduleSpecifier.text, snapshot, fromVirtualPath, true);
  return target.resolution === RESOLUTION_SNAPSHOT ? undefined : "unresolved_external";
}

function leftmostIdentifier(expr: astns.Expression, ast: typeof astns): astns.Identifier | undefined {
  let cur: astns.Expression = expr;
  while (ast.isPropertyAccessExpression(cur)) cur = cur.expression;
  return ast.isIdentifier(cur) ? cur : undefined;
}

function enclosingImportDeclaration(node: astns.Node, ast: typeof astns): astns.ImportDeclaration | undefined {
  let cur: astns.Node | undefined = node;
  while (cur && !ast.isSourceFile(cur)) {
    if (ast.isImportDeclaration(cur)) return cur;
    cur = cur.parent;
  }
  return undefined;
}

function recordGapDiagnostic(
  code: string,
  message: string,
  node: astns.Node,
  sf: astns.SourceFile,
  snapshot: ProjectSnapshot,
  out: MutableAccumulator,
  seenGapSites: Set<string>,
): void {
  const repoPath = snapshot.toRepoPath(sf.path);
  const { line } = sf.getLineAndCharacterOfPosition(node.getStart(sf));
  const site = `${repoPath ?? sf.fileName}:${line + 1}`;
  const key = `${code}|${site}`;
  if (seenGapSites.has(key)) return;
  seenGapSites.add(key);
  out.diagnostics.push({ code, message, path: repoPath });
}

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
