import { type CallGraphEdgeFact, type Coverage, type ImportEdgeFact, type ProjectFile, type ReachabilityFactWire } from "./protocol.js";
/**
 * The exact resolved TypeScript compiler this request must run against
 * (coach#326 Task 2), assembled once by main.ts's dynamic-import bootstrap
 * from an explicit, host-provided compiler module location instead of this
 * package's own `typescript` devDependency. Every module below receives
 * the pieces it needs from this bundle as parameters rather than statically
 * importing "typescript/unstable/*" itself.
 */
export interface CompilerBundle {
    api: typeof import("typescript/unstable/sync").API;
    symbolFlags: typeof import("typescript/unstable/sync").SymbolFlags;
    ast: typeof import("typescript/unstable/ast");
    createVirtualFileSystem: typeof import("typescript/unstable/fs").createVirtualFileSystem;
}
/** Thrown only for genuine backend-startup failures (e.g. the bundled
 * native tsgo binary failing to spawn); main.ts turns this into a
 * whole-request Response.Error rather than crashing the process, since a
 * missing/unspawnable backend is an operational condition analogous to
 * pkg/projectmodel's own DiagBackendUnavailable, not a programming bug. */
export declare class SidecarBackendError extends Error {
}
export interface AnalyzeOptions {
    files: readonly ProjectFile[];
    roots?: readonly string[];
    timeoutMs?: number;
    /** Test-only: injects a fixed synchronous delay before processing each
     * discovered tsconfig project, so timeout_ms self-enforcement can be
     * exercised deterministically without depending on a genuinely slow
     * TypeScript analysis. Must never be set outside tests (see main.ts's
     * COACH_TS_SIDECAR_TEST_DELAY_MS gate). */
    testDelayMsPerProject?: number;
    compiler: CompilerBundle;
}
export interface AnalyzeResult {
    edges: ImportEdgeFact[];
    callGraph: CallGraphEdgeFact[];
    reachabilityFacts: ReachabilityFactWire[];
    coverage: Coverage;
}
export declare function analyzeProject(opts: AnalyzeOptions): AnalyzeResult;
//# sourceMappingURL=analyze.d.ts.map