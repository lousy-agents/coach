import type { Diagnostic, ImportEdgeFact } from "./protocol.js";
/**
 * Sorts edges/diagnostics by a stable key so re-running the same snapshot
 * produces byte-identical output, mirroring pkg/projectmodel/
 * serialization.go's canonicalImportEdges/canonicalCoverage sort keys
 * (From/To/Kind/Site/Resolution, and Code/Path/Message respectively).
 */
export declare function canonicalizeEdges(edges: readonly ImportEdgeFact[]): ImportEdgeFact[];
export declare function canonicalizeDiagnostics(diagnostics: readonly Diagnostic[]): Diagnostic[];
//# sourceMappingURL=canonical.d.ts.map