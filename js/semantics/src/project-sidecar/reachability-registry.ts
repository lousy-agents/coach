/**
 * Pinned, deterministic registries reachability.ts matches against -- the
 * TypeScript-side counterpart of pkg/projectmodel's
 * ReachabilitySinkPatterns (go_reachability.go). This is registry policy,
 * not data derived from a snapshot: extending it is a deliberate
 * versioning decision (bump REACHABILITY_ALGORITHM alongside any change
 * here), matching the Go side's own doc comment.
 */
import { SIDECAR_PHASE } from "./protocol.js";
import { VIRTUAL_ROOT } from "./vfs.js";

/** ReachabilityFactWire.algorithm_version's fixed value for this sidecar's
 * possible-call-reachability extraction, distinct from Go's
 * "go-source-sink-registry@1" (pkg/projectmodel.ReachabilityAlgorithm)
 * since the two traversal implementations evolve independently. */
export const REACHABILITY_ALGORITHM = "ts-source-sink-registry@1";

/** ReachabilityFactWire.backend's fixed value for every fact this sidecar
 * emits, kept identical to Coverage.phase (SIDECAR_PHASE) so a consumer can
 * attribute a fact to "this TS sidecar" the same way it attributes
 * coverage. */
export const REACHABILITY_BACKEND = SIDECAR_PHASE;

/** ReachabilityFactWire.confidence's only value this sidecar produces:
 * every Path hop is a single directly-resolved call (Checker.getResolvedSignature
 * succeeded), never a guess through a dynamic import, an unresolved external
 * type, or a type-only binding -- see reachability.ts's gap diagnostics for
 * those excluded paths. The one caveat: a union-typed receiver (e.g.
 * `PrismaClient | Other`) still resolves to a single signature via
 * TypeScript's own overload-resolution "best guess", so a call through such
 * a receiver is reported the same as any other resolved_direct call. */
export const CONFIDENCE_RESOLVED_DIRECT = "resolved_direct";

/**
 * Property names on a route-registration receiver this sidecar recognizes
 * as an HTTP-verb-shaped registration call, e.g. `app.get(path, handler)`.
 * Matching is structural (the receiver's checker-resolved type must
 * actually have a property of this name -- see isRouteRegistrationCall in
 * reachability.ts), not a literal import of any specific framework, so a
 * locally declared interface shaped like Express/Koa/Fastify's router
 * still matches without importing those packages.
 */
export const ROUTE_REGISTRATION_METHODS: ReadonlySet<string> = new Set(["get", "post", "put", "patch", "delete", "all"]);

interface SinkClassPattern {
  className: string;
  /** The declaring class's source file must resolve through the
   * snapshot's node_modules mirror for a package of this name (see
   * vfs.ts's package-mirroring) -- an unrelated in-snapshot class that
   * merely shares className is never mistaken for the real client. */
  moduleSpecifier: string;
  methods: ReadonlySet<string>;
}

/**
 * Known ORM/DB-client-shaped sinks, keyed by declaring class name plus the
 * module it must resolve through. A resolved method call whose
 * Checker.getResolvedSignature declaration lives inside one of these
 * classes, imported from the matching moduleSpecifier, under one of the
 * listed method names, is a reachability sink -- mirroring how Go's
 * ReachabilitySinkPatterns pins concrete `(*database/sql.DB).Exec`-shaped
 * strings rather than deriving them from the snapshot.
 */
export const REACHABILITY_SINK_CLASSES: readonly SinkClassPattern[] = [
  {
    className: "PrismaClient",
    moduleSpecifier: "@prisma/client",
    methods: new Set([
      "findMany",
      "findFirst",
      "findFirstOrThrow",
      "findUnique",
      "findUniqueOrThrow",
      "create",
      "createMany",
      "createManyAndReturn",
      "update",
      "updateMany",
      "updateManyAndReturn",
      "upsert",
      "delete",
      "deleteMany",
      "aggregate",
      "count",
      "groupBy",
      "$queryRaw",
      "$queryRawUnsafe",
      "$executeRaw",
      "$executeRawUnsafe",
    ]),
  },
];

/**
 * Renders (className, methodName) as a sink node ID when the pair matches
 * REACHABILITY_SINK_CLASSES *and* declaringVirtualPath (the resolved
 * method declaration's own source file, pre-unmirror) resolves through
 * the pattern's moduleSpecifier in the snapshot's node_modules mirror --
 * e.g. `(PrismaClient).findMany`. The `(<class>).<method>` shape is the
 * general convention for every resolved method-call sink this sidecar
 * emits, not a fixture-specific literal -- it falls out of whichever
 * class/method pair the registry matches.
 */
export function sinkNodeId(className: string, methodName: string, declaringVirtualPath: string): string | undefined {
  const pattern = REACHABILITY_SINK_CLASSES.find((p) => p.className === className);
  if (!pattern || !pattern.methods.has(methodName)) return undefined;
  if (!declaringVirtualPath.startsWith(`${VIRTUAL_ROOT}/node_modules/${pattern.moduleSpecifier}/`)) return undefined;
  return `(${className}).${methodName}`;
}
