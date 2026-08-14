import type { Diagnostic, ImportEdgeFact } from "./protocol.js";

/**
 * Sorts edges/diagnostics by a stable key so re-running the same snapshot
 * produces byte-identical output, mirroring pkg/projectmodel/
 * serialization.go's canonicalImportEdges/canonicalCoverage sort keys
 * (From/To/Kind/Site/Resolution, and Code/Path/Message respectively).
 */
export function canonicalizeEdges(edges: readonly ImportEdgeFact[]): ImportEdgeFact[] {
  return [...edges].sort(
    (a, b) =>
      compare(a.from, b.from) ||
      compare(a.to, b.to) ||
      compare(a.kind, b.kind) ||
      compare(a.site ?? "", b.site ?? "") ||
      compare(a.resolution ?? "", b.resolution ?? ""),
  );
}

export function canonicalizeDiagnostics(diagnostics: readonly Diagnostic[]): Diagnostic[] {
  return [...diagnostics].sort(
    (a, b) => compare(a.code, b.code) || compare(a.path ?? "", b.path ?? "") || compare(a.message, b.message),
  );
}

function compare(a: string, b: string): number {
  if (a < b) return -1;
  if (a > b) return 1;
  return 0;
}
