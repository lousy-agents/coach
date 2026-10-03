import type * as astns from "typescript/unstable/ast";
import type { SymbolFlags } from "typescript/unstable/sync";

import type { CallGraphEdgeFact, Diagnostic, ReachabilityFactWire } from "./protocol.js";

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
