import { type CallGraphEdgeFact, type Coverage, type ImportEdgeFact, type ProjectFile, type ReachabilityFactWire, type RootScopeFact } from "./protocol.js";
export interface CompilerBundle {
    api: typeof import("typescript/unstable/sync").API;
    symbolFlags: typeof import("typescript/unstable/sync").SymbolFlags;
    ast: typeof import("typescript/unstable/ast");
    createVirtualFileSystem: typeof import("typescript/unstable/fs").createVirtualFileSystem;
}
/** Thrown only for genuine backend-startup failures (e.g. the bundled native tsgo binary failing to spawn); main.ts turns this into a Response.Error rather than crashing the process. */
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
    tsserverPath: string;
}
export interface AnalyzeResult {
    edges: ImportEdgeFact[];
    callGraph: CallGraphEdgeFact[];
    reachabilityFacts: ReachabilityFactWire[];
    coverage: Coverage;
    rootScopes?: RootScopeFact[];
}
export declare function analyzeProject(opts: AnalyzeOptions): AnalyzeResult;
//# sourceMappingURL=analyze.d.ts.map