/**
 * End-to-end parity: replays every case of internal/jsbridge's parity
 * manifest through the public createAnalyzer().analyzeBytes() API and
 * compares against the .expected.json files. The Go-side fixture-lock test
 * (TestParityFixtures) guarantees those files are exactly what Go emits, so
 * agreement here proves Go-native and JS-boundary results are identical.
 */
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { test } from "node:test";

import {
  createAnalyzer,
  SemanticsError,
  SemanticsSyntaxError,
  type AnalyzerOptions,
  type Language,
  type Result,
} from "../dist/index.js";

const PARITY_DIR = new URL("../../../internal/jsbridge/testdata/parity/", import.meta.url);

interface ParityCase {
  name: string;
  src: string;
  path: string;
  language: string;
  options?: { languages?: string[]; max_file_bytes?: number };
  expected: string;
}

interface ExpectedResponse {
  id: number;
  result?: Result;
  error?: { kind: string; message: string };
}

const manifest = JSON.parse(
  await readFile(new URL("manifest.json", PARITY_DIR), "utf-8"),
) as ParityCase[];
assert.ok(manifest.length > 0, "parity manifest is empty");

for (const parityCase of manifest) {
  test(`parity: ${parityCase.name}`, async () => {
    const content = new Uint8Array(await readFile(new URL(parityCase.src, PARITY_DIR)));
    const expected = JSON.parse(
      await readFile(new URL(parityCase.expected, PARITY_DIR), "utf-8"),
    ) as ExpectedResponse;

    const opts: AnalyzerOptions = {};
    if (parityCase.options?.languages !== undefined) {
      opts.languages = parityCase.options.languages as Language[];
    }
    if (parityCase.options?.max_file_bytes !== undefined) {
      opts.maxFileBytes = parityCase.options.max_file_bytes;
    }

    let outcome: { result: Result } | { error: SemanticsError };
    let analyzer;
    try {
      analyzer = await createAnalyzer(opts);
      outcome = {
        result: await analyzer.analyzeBytes({
          path: parityCase.path,
          language: parityCase.language as Language,
          content,
        }),
      };
    } catch (err) {
      // createAnalyzer's eager local validation may fire before the request
      // ever reaches Go (e.g. invalid_options); both paths must agree on the
      // error kind, so a SemanticsError from either place counts.
      if (!(err instanceof SemanticsError)) {
        throw err;
      }
      outcome = { error: err };
    } finally {
      analyzer?.dispose();
    }

    if (expected.error !== undefined) {
      assert.ok("error" in outcome, `expected error kind ${expected.error.kind}, got a result`);
      assert.equal(outcome.error.kind, expected.error.kind);
      if (expected.result !== undefined) {
        // Syntax double return: the partial Result must match Go's exactly.
        assert.ok(outcome.error instanceof SemanticsSyntaxError);
        assert.deepEqual(outcome.error.partialResult, expected.result);
        assert.deepEqual(outcome.error.issues, expected.result.syntax_errors ?? []);
      }
    } else {
      assert.ok("result" in outcome, `expected a result, got error: ${"error" in outcome ? outcome.error.message : ""}`);
      assert.deepEqual(outcome.result, expected.result);
    }
  });
}
