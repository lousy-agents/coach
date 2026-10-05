import type { Project } from "typescript/unstable/sync";
import type { CompilerBundle } from "./analyze-types.js";
import type { CallGraphEdgeFact, Diagnostic, ImportEdgeFact, ReachabilityFactWire } from "./protocol.js";
import { type ProjectSnapshot } from "./vfs.js";
export declare class ProjectScan {
    readonly visited: Set<string>;
    readonly reachVisited: Set<string>;
    readonly reachSourcesVisited: Set<string>;
    readonly edges: ImportEdgeFact[];
    readonly callGraph: CallGraphEdgeFact[];
    readonly reachabilityFacts: ReachabilityFactWire[];
    readonly diagnostics: Diagnostic[];
    readonly seenConfigDiagnostics: Set<string>;
    complete: boolean;
    projectsProcessed: number;
    constructor(visited: Set<string>, reachVisited: Set<string>, reachSourcesVisited: Set<string>, edges: ImportEdgeFact[], callGraph: CallGraphEdgeFact[], reachabilityFacts: ReachabilityFactWire[], diagnostics: Diagnostic[], seenConfigDiagnostics: Set<string>);
    timedOut(timeoutMs: number | undefined, projectCount: number): void;
    ingest(project: Project, snapshot: ProjectSnapshot, compiler: CompilerBundle): void;
}
export declare function busyWaitMs(ms: number): void;
//# sourceMappingURL=analyze-ingest.d.ts.map