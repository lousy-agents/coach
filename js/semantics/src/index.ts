/**
 * @lousy-agents/coach-semantics — typed Node.js bindings for coach's
 * pkg/semantics structural source analyzer.
 *
 * Usage:
 *
 *   const analyzer = await createAnalyzer();
 *   try {
 *     const result = await analyzer.analyzeBytes({
 *       path: "widget.go",
 *       language: "go",
 *       content: await fs.readFile("widget.go"),
 *     });
 *     console.log(result.metrics);
 *   } catch (err) {
 *     if (err instanceof SemanticsSyntaxError) {
 *       console.log(err.partialResult.syntax_errors);
 *     } else {
 *       throw err;
 *     }
 *   } finally {
 *     analyzer.dispose();
 *   }
 */

import {
  createAnalyzerWithBackend,
  validateOptions,
  type Analyzer,
  type AnalyzerOptions,
} from "./analyzer.js";
import { acquireDefaultBackend } from "./backend-default.js";

export type { Backend } from "./backend.js";
export {
  createAnalyzerWithBackend,
  languageForExtension,
  type Analyzer,
  type AnalyzerOptions,
  type FileInput,
} from "./analyzer.js";
export { SemanticsError, SemanticsSyntaxError, type SemanticsErrorKind } from "./errors.js";
export type {
  Finding,
  ImportFeature,
  Language,
  Location,
  ParseStatus,
  Result,
  StructuralMetrics,
  SyntaxIssue,
} from "./types.js";

/**
 * Create an Analyzer backed by the package's default transport. Options are
 * validated eagerly (mirroring NewAnalyzer); the underlying backend is
 * shared per process, and analyzer.dispose() releases the hold on it.
 */
export async function createAnalyzer(opts: AnalyzerOptions = {}): Promise<Analyzer> {
  validateOptions(opts);
  const { backend, release } = await acquireDefaultBackend();
  return createAnalyzerWithBackend(backend, opts, release);
}
