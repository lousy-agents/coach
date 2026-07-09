/**
 * Types mirroring pkg/semantics' frozen Result JSON shape (snake_case struct
 * tags in pkg/semantics/result.go). Field names are part of the Go library's
 * stability contract and are locked by golden files on both sides of the
 * bridge — never rename or camelCase them here.
 */

/** Source language a Result was produced from. */
export type Language = "go" | "typescript" | "tsx";

/** Whether Tree-sitter found syntax errors while parsing. */
export type ParseStatus = "ok" | "syntax_errors";

/** 0-based byte/row/col span as Tree-sitter reports it. */
export interface Location {
  start_byte: number;
  end_byte: number;
  start_row: number;
  start_col: number;
  end_row: number;
  end_col: number;
}

/** One syntax error or missing-node location found while parsing. */
export interface SyntaxIssue {
  kind: "error" | "missing";
  location: Location;
}

/** One import declaration. */
export interface ImportFeature {
  path: string;
  /** Alias ident, ".", or "_" (Go); absent when the import is unaliased. */
  alias?: string;
  location: Location;
}

/**
 * Branching/declaration construct counts for one file. type_switches and
 * selects have no TypeScript/TSX analog and are always 0 for those languages.
 */
export interface StructuralMetrics {
  ifs: number;
  fors: number;
  expr_switches: number;
  type_switches: number;
  selects: number;
  functions: number;
  methods: number;
  max_nesting_depth: number;
}

/** One detected pattern of interest, such as a constructor-like function. */
export interface Finding {
  /** "constructor_func" | "pointer_return" | "mutates_input" (Go); "tight_coupling" | "mutates_input" (TS/TSX). */
  kind: string;
  name: string;
  location: Location;
  /** Present only on findings that carry coaching metadata (e.g. "mutates_input"). */
  confidence?: string;
  evidence?: string;
  recommendation?: string;
  suggested_skill?: string;
}

/** Top-level output of analyzing one source file. */
export interface Result {
  path: string;
  language: Language;
  parse_status: ParseStatus;
  syntax_errors?: SyntaxIssue[];
  imports?: ImportFeature[];
  metrics: StructuralMetrics;
  findings?: Finding[];
}
