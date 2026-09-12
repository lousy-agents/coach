/**
 * Wire types mirroring internal/projectbridge/protocol.go exactly (field
 * names, JSON shape, constant values) -- do not rename a field here without
 * updating the Go side, since the two are not generated from a shared
 * schema.
 */

export const PROTOCOL_VERSION = 1;
export const OP_ANALYZE_PROJECT = "analyze_project";

export const KIND_BACKEND_UNAVAILABLE = "backend_unavailable";
export const KIND_CRASHED = "crashed";
export const KIND_INTERNAL = "internal";
export const KIND_CANCELED = "canceled";

export interface ProjectFile {
  path: string;
  content_b64: string;
}

export interface Request {
  version: number;
  op: string;
  id: number;
  files: ProjectFile[];
  roots?: string[];
  timeout_ms?: number;
}

/**
 * Kind vocabulary for ImportEdgeFact.kind, stable wire values other code
 * matches on (issue #214):
 *  - "import": a value-level `import ... from "spec"` declaration.
 *  - "reexport": a value-level `export ... from "spec"` declaration
 *    (`export * from`, `export { x } from`).
 *  - "type_only": an `import type ...` declaration, or an
 *    `export type ... from`/`export ... from` with isTypeOnly set.
 *  - "commonjs_require": a `require("spec")` call.
 *  - "dynamic_import": an `import("spec")` call expression.
 */
export const KIND_IMPORT = "import";
export const KIND_REEXPORT = "reexport";
export const KIND_TYPE_ONLY = "type_only";
export const KIND_COMMONJS_REQUIRE = "commonjs_require";
export const KIND_DYNAMIC_IMPORT = "dynamic_import";

/**
 * Resolution vocabulary for ImportEdgeFact.resolution, paired with the `to`
 * field's stable-ID convention:
 *  - "snapshot": resolved to a file present in the request's snapshot;
 *    `to` is `file:<repository-relative path>`.
 *  - "external": a non-relative specifier that did not resolve to any
 *    snapshot file (a real external package); `to` is `external:<specifier>`.
 *  - "unresolved": a relative or aliased specifier that should have
 *    resolved within the snapshot but did not (e.g. a broken import, or a
 *    file genuinely missing from the request); `to` is
 *    `unresolved:<specifier>`.
 */
export const RESOLUTION_SNAPSHOT = "snapshot";
export const RESOLUTION_EXTERNAL = "external";
export const RESOLUTION_UNRESOLVED = "unresolved";

export interface ImportEdgeFact {
  from: string;
  to: string;
  kind: string;
  site?: string;
  resolution?: string;
}

/** One raw call-graph edge, mirroring internal/projectbridge.CallGraphEdgeFact. */
export interface CallGraphEdgeFact {
  from: string;
  to: string;
}

/** Mirrors internal/projectbridge.RootScopeFact exactly. */
export interface RootScopeFact {
  root: string;
  candidate_files: number;
  analyzed_files: number;
  analyzed_paths?: string[];
  unanalyzed_paths?: string[];
}

export const KIND_POSSIBLE_CALL_REACHABILITY = "possible_call_reachability";

/** One node in a ReachabilityFactWire.path, mirroring internal/projectbridge.ReachabilityStepFact. */
export interface ReachabilityStepFact {
  node_id: string;
}

/** Mirrors internal/projectbridge.ReachabilityFactWire exactly, including the `backend` field carrying language provenance. */
export interface ReachabilityFactWire {
  id: string;
  kind: string;
  confidence: string;
  source: string;
  sink: string;
  path: ReachabilityStepFact[];
  algorithm_version: string;
  backend?: string;
}

export interface Diagnostic {
  code: string;
  message: string;
  path?: string;
}

export interface Coverage {
  phase: string;
  complete: boolean;
  counts?: Record<string, number>;
  budgets?: Record<string, number>;
  diagnostics?: Diagnostic[];
}

export interface ErrorPayload {
  kind: string;
  message: string;
}

export interface Response {
  version: number;
  id: number;
  import_edges?: ImportEdgeFact[];
  call_graph?: CallGraphEdgeFact[];
  reachability_facts?: ReachabilityFactWire[];
  root_scopes?: RootScopeFact[];
  coverage: Coverage;
  error?: ErrorPayload;
}

export const SIDECAR_PHASE = "ts_project_sidecar";
