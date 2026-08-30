/** ReachabilityFactWire.algorithm_version's fixed value for this sidecar's
 * possible-call-reachability extraction, distinct from Go's
 * "go-source-sink-registry@1" (pkg/projectmodel.ReachabilityAlgorithm)
 * since the two traversal implementations evolve independently. */
export declare const REACHABILITY_ALGORITHM = "ts-source-sink-registry@1";
/** ReachabilityFactWire.backend's fixed value for every fact this sidecar
 * emits, kept identical to Coverage.phase (SIDECAR_PHASE) so a consumer can
 * attribute a fact to "this TS sidecar" the same way it attributes
 * coverage. */
export declare const REACHABILITY_BACKEND = "ts_project_sidecar";
/** ReachabilityFactWire.confidence's only value this sidecar produces:
 * every Path hop is a single directly-resolved call (Checker.getResolvedSignature
 * succeeded), never a guess through a dynamic import, an unresolved external
 * type, or a type-only binding -- see reachability.ts's gap diagnostics for
 * those excluded paths. The one caveat: a union-typed receiver (e.g.
 * `PrismaClient | Other`) still resolves to a single signature via
 * TypeScript's own overload-resolution "best guess", so a call through such
 * a receiver is reported the same as any other resolved_direct call. */
export declare const CONFIDENCE_RESOLVED_DIRECT = "resolved_direct";
/**
 * Property names on a route-registration receiver this sidecar recognizes
 * as an HTTP-verb-shaped registration call, e.g. `app.get(path, handler)`.
 * Matching is structural (the receiver's checker-resolved type must
 * actually have a property of this name -- see isRouteRegistrationCall in
 * reachability.ts), not a literal import of any specific framework, so a
 * locally declared interface shaped like Express/Koa/Fastify's router
 * still matches without importing those packages.
 */
export declare const ROUTE_REGISTRATION_METHODS: ReadonlySet<string>;
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
export declare const REACHABILITY_SINK_CLASSES: readonly SinkClassPattern[];
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
export declare function sinkNodeId(className: string, methodName: string, declaringVirtualPath: string): string | undefined;
export declare const GAP_DYNAMIC_IMPORT = "ts_reachability_dynamic_import_gap";
export declare const GAP_UNRESOLVED_HANDLER = "ts_reachability_unresolved_handler_gap";
export declare const GAP_LOCAL_CALL_NOT_FOLLOWED = "ts_reachability_local_call_not_followed_gap";
export declare const GAP_TYPE_ONLY = "ts_reachability_type_only_gap";
export declare const GAP_UNRESOLVED_TYPE = "ts_reachability_unresolved_type_gap";
/** Every REACHABILITY_GAP_DIAGNOSTIC_CODES member above, for the Go-side
 * parity test to enumerate (see the doc comment above). */
export declare const REACHABILITY_GAP_DIAGNOSTIC_CODES: readonly ["ts_reachability_dynamic_import_gap", "ts_reachability_unresolved_handler_gap", "ts_reachability_local_call_not_followed_gap", "ts_reachability_type_only_gap", "ts_reachability_unresolved_type_gap"];
export {};
//# sourceMappingURL=reachability-registry.d.ts.map