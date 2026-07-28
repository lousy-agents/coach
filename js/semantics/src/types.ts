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
  /** Max Cognitive Complexity score over all per-function records; 0 if none. */
  max_cognitive_complexity: number;
  /** Sum of Cognitive Complexity scores over top-level records only; 0 if none. */
  sum_cognitive_complexity: number;
}

/**
 * Cognitive Complexity score for one scored function body (declaration,
 * method, func lit, or arrow). kind is a closed set: Go uses function |
 * method | func_lit; TypeScript/TSX also allow arrow.
 */
export interface FunctionCognitiveComplexity {
  name: string;
  kind: string;
  location: Location;
  score: number;
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

/** One useState() call's binding/setter pair. */
export interface ReactUseStateBinding {
  binding: string;
  setter: string;
  location: Location;
}

/** A callback (e.g. a useEffect body or event handler) that updates more than one state binding together. */
export interface ReactCoordinatedTransition {
  name: string;
  kind: string;
  location: Location;
  updated_bindings: string[];
}

/** One branch of a multi-way conditional that renders a distinct panel/view. */
export interface ReactWorkspaceBranch {
  label: string;
  location: Location;
}

/** A call to an imperative DOM/UI API (e.g. document.getElementById, .focus()) inside a component body. */
export interface ReactImperativeUICall {
  api: string;
  location: Location;
}

/** A state binding passed as a prop to more than one distinct child panel component. */
export interface ReactSharedPanelDep {
  name: string;
  panels: string[];
}

/**
 * One detected React component's state and coordination shape, used by the
 * react_component_orchestration_density codesignal rule. Populated for
 * TypeScript/TSX only.
 */
export interface ReactComponentFacts {
  name: string;
  location: Location;
  client_kind: string;
  use_state?: ReactUseStateBinding[];
  coordinated_transitions?: ReactCoordinatedTransition[];
  workspace_branches?: ReactWorkspaceBranch[];
  imperative_ui?: ReactImperativeUICall[];
  shared_panel_deps?: ReactSharedPanelDep[];
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
  /** Per-function Cognitive Complexity records; omitted when empty. */
  cognitive_complexity?: FunctionCognitiveComplexity[];
  /** Per-component React orchestration facts; omitted when empty or non-TS/TSX. */
  react_components?: ReactComponentFacts[];
}
