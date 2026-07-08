import type { Result, SyntaxIssue } from "./types.js";

/**
 * Error kinds crossing the JS boundary. All but "backend_unavailable" mirror
 * internal/jsbridge's Kind* constants (which in turn map pkg/semantics'
 * sentinel errors); "backend_unavailable" is raised purely on the JS side
 * when the compiled backend artifact is missing.
 */
export type SemanticsErrorKind =
  | "syntax"
  | "empty_content"
  | "unsupported_language"
  | "file_too_large"
  | "binary_content"
  | "parse_failure"
  | "invalid_options"
  | "canceled"
  | "internal"
  | "backend_unavailable";

const KNOWN_KINDS: ReadonlySet<string> = new Set([
  "syntax",
  "empty_content",
  "unsupported_language",
  "file_too_large",
  "binary_content",
  "parse_failure",
  "invalid_options",
  "canceled",
  "internal",
  "backend_unavailable",
]);

/** Coerce a wire kind to the union, treating anything unknown as internal. */
export function toErrorKind(kind: string): SemanticsErrorKind {
  return KNOWN_KINDS.has(kind) ? (kind as SemanticsErrorKind) : "internal";
}

/**
 * Error thrown by analyzeBytes and createAnalyzer. The kind field replaces
 * Go-side errors.Is matching: switch on it instead of parsing messages.
 */
export class SemanticsError extends Error {
  readonly kind: SemanticsErrorKind;

  constructor(kind: SemanticsErrorKind, message: string) {
    super(message);
    this.name = "SemanticsError";
    this.kind = kind;
  }
}

/**
 * Thrown when the source parsed but contains syntax errors. Mirrors Go's
 * double return: AnalyzeBytes yields both a partial Result and a
 * *SyntaxError, so this error carries the partial Result (parse_status
 * "syntax_errors") and derives issues from its syntax_errors field. Named
 * SemanticsSyntaxError to avoid shadowing JavaScript's global SyntaxError.
 */
export class SemanticsSyntaxError extends SemanticsError {
  readonly issues: SyntaxIssue[];
  readonly partialResult: Result;

  constructor(message: string, partialResult: Result) {
    super("syntax", message);
    this.name = "SemanticsSyntaxError";
    this.partialResult = partialResult;
    this.issues = partialResult.syntax_errors ?? [];
  }
}
