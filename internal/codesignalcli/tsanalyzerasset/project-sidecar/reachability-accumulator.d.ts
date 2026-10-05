import type * as astns from "typescript/unstable/ast";
import { type CallGraphEdgeFact, type Diagnostic, type ReachabilityFactWire } from "./protocol.js";
import type { ProjectSnapshot } from "./vfs.js";
export declare class ReachabilityAccumulator {
    readonly callGraph: CallGraphEdgeFact[];
    readonly facts: ReachabilityFactWire[];
    readonly diagnostics: Diagnostic[];
    readonly seenSources: Set<string>;
    private readonly factKeys;
    private readonly seenGapSites;
    constructor(seenSources: Set<string>, factKeys: Set<string>, seenGapSites: Set<string>);
    noteSource(sourceId: string): boolean;
    recordFact(sourceId: string, sinkId: string): void;
    recordGap(code: string, message: string, node: astns.Node, sf: astns.SourceFile, snapshot: ProjectSnapshot): void;
}
//# sourceMappingURL=reachability-accumulator.d.ts.map