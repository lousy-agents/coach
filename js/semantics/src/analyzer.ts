import type { Backend } from "./backend.js";
import { SemanticsError } from "./errors.js";
import { allocateRequestID, decodeResponse, encodeContent, type WireRequest } from "./protocol.js";
import type { Language, Result } from "./types.js";

/** Mirrors semantics.AnalyzerOptions. */
export interface AnalyzerOptions {
  /** Restrict analysis to this set; empty/absent means all supported. */
  languages?: Language[];
  /** Cap on content size in bytes; 0/absent uses the Go default (2 MiB). */
  maxFileBytes?: number;
}

/** Mirrors semantics.FileInput. Path is opaque metadata, never opened. */
export interface FileInput {
  path?: string;
  language: Language;
  /** Strings are encoded as UTF-8; byte inputs pass through exactly. */
  content: Uint8Array | string;
  /** Bounds the analysis; maps to context.WithTimeout on the Go side. */
  timeoutMs?: number;
}

export interface Analyzer {
  /**
   * Analyze one file. Resolves with the frozen Result JSON shape. Rejects
   * with SemanticsSyntaxError (carrying the partial Result) when the source
   * has syntax errors, or SemanticsError for every other failure —
   * mirroring Go's AnalyzeBytes double return.
   */
  analyzeBytes(input: FileInput): Promise<Result>;
  /** Release the analyzer's hold on the backend. Idempotent. */
  dispose(): void;
}

const KNOWN_LANGUAGES: ReadonlySet<string> = new Set(["go", "typescript", "tsx"]);

/**
 * Fail-fast mirror of NewAnalyzer's two validation rules (non-negative
 * MaxFileBytes, recognized languages) so createAnalyzer rejects immediately,
 * matching Go semantics. The Go side re-validates per request and remains
 * the authority.
 */
export function validateOptions(opts: AnalyzerOptions): void {
  if (opts.maxFileBytes !== undefined && opts.maxFileBytes < 0) {
    throw new SemanticsError("invalid_options", `maxFileBytes must be >= 0, got ${opts.maxFileBytes}`);
  }
  for (const lang of opts.languages ?? []) {
    if (!KNOWN_LANGUAGES.has(lang)) {
      throw new SemanticsError("unsupported_language", `unsupported language: ${JSON.stringify(lang)}`);
    }
  }
}

/**
 * Build an Analyzer on an explicit Backend. The public createAnalyzer wires
 * in the default backend; this seam exists so tests (and unusual embeddings)
 * can supply their own transport. On dispose, onDispose runs if given —
 * createAnalyzer passes a refcounted release so a shared backend outlives
 * any one analyzer — otherwise dispose() falls through to backend.dispose()
 * directly, so a caller-supplied Backend is never leaked. Runs once.
 */
export function createAnalyzerWithBackend(
  backend: Backend,
  opts: AnalyzerOptions = {},
  onDispose?: () => void,
): Analyzer {
  validateOptions(opts);
  let disposed = false;
  return {
    async analyzeBytes(input: FileInput): Promise<Result> {
      if (disposed) {
        throw new SemanticsError("internal", "analyzer has been disposed");
      }
      const request: WireRequest = {
        id: allocateRequestID(),
        op: "analyze",
        language: input.language,
        content_b64: encodeContent(input.content),
        options: {},
      };
      if (input.path !== undefined) {
        request.path = input.path;
      }
      if (opts.languages !== undefined && opts.languages.length > 0) {
        request.options.languages = opts.languages;
      }
      if (opts.maxFileBytes !== undefined && opts.maxFileBytes > 0) {
        request.options.max_file_bytes = opts.maxFileBytes;
      }
      if (input.timeoutMs !== undefined && input.timeoutMs > 0) {
        request.timeout_ms = input.timeoutMs;
      }
      const responseJson = await backend.analyze(JSON.stringify(request));
      return decodeResponse(responseJson);
    },
    dispose(): void {
      if (disposed) {
        return;
      }
      disposed = true;
      if (onDispose) {
        onDispose();
      } else {
        backend.dispose();
      }
    },
  };
}

/**
 * Map a file extension (with leading dot, e.g. ".go") to its Language,
 * case-insensitively. Mirrors semantics.LanguageForExtension; analyzeBytes
 * never calls it — language selection stays explicit.
 */
export function languageForExtension(ext: string): Language | undefined {
  switch (ext.toLowerCase()) {
    case ".go":
      return "go";
    case ".ts":
      return "typescript";
    case ".tsx":
      return "tsx";
    default:
      return undefined;
  }
}
