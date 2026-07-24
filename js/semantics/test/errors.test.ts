import assert from "node:assert/strict";
import { Buffer } from "node:buffer";
import { test } from "node:test";

import {
  createAnalyzerWithBackend,
  SemanticsError,
  SemanticsSyntaxError,
  languageForExtension,
  type Backend,
  type Result,
} from "../dist/index.js";

const okResult: Result = {
  path: "main.go",
  language: "go",
  parse_status: "ok",
  metrics: {
    ifs: 1,
    fors: 0,
    expr_switches: 0,
    type_switches: 0,
    selects: 0,
    functions: 1,
    methods: 0,
    max_nesting_depth: 1,
  },
};

const syntaxResult: Result = {
  path: "broken.go",
  language: "go",
  parse_status: "syntax_errors",
  syntax_errors: [
    {
      kind: "missing",
      location: { start_byte: 26, end_byte: 26, start_row: 2, start_col: 10, end_row: 2, end_col: 10 },
    },
  ],
  metrics: {
    ifs: 0,
    fors: 0,
    expr_switches: 0,
    type_switches: 0,
    selects: 0,
    functions: 0,
    methods: 0,
    max_nesting_depth: 0,
  },
};

/** A Backend that replays canned protocol responses and records requests. */
function fakeBackend(respond: (request: Record<string, unknown>) => Record<string, unknown>): Backend & {
  requests: Record<string, unknown>[];
  disposed: boolean;
} {
  const backend = {
    requests: [] as Record<string, unknown>[],
    disposed: false,
    analyze(requestJson: string): Promise<string> {
      const request = JSON.parse(requestJson) as Record<string, unknown>;
      backend.requests.push(request);
      return Promise.resolve(JSON.stringify({ id: request.id, ...respond(request) }));
    },
    dispose(): void {
      backend.disposed = true;
    },
  };
  return backend;
}

test("resolves the Result on success", async () => {
  const backend = fakeBackend(() => ({ result: okResult }));
  const analyzer = createAnalyzerWithBackend(backend);
  const result = await analyzer.analyzeBytes({ path: "main.go", language: "go", content: "package main\n" });
  assert.deepEqual(result, okResult);
});

test("syntax errors reject with SemanticsSyntaxError carrying the partial result", async () => {
  const backend = fakeBackend(() => ({
    result: syntaxResult,
    error: { kind: "syntax", message: "semantics: 1 syntax issue(s)" },
  }));
  const analyzer = createAnalyzerWithBackend(backend);
  const err = await analyzer
    .analyzeBytes({ language: "go", content: "func oops( {" })
    .then(() => assert.fail("expected rejection"), (e: unknown) => e);
  assert.ok(err instanceof SemanticsSyntaxError);
  assert.equal(err.kind, "syntax");
  assert.equal(err.message, "semantics: 1 syntax issue(s)");
  assert.deepEqual(err.partialResult, syntaxResult);
  assert.deepEqual(err.issues, syntaxResult.syntax_errors);
});

test("error-only responses reject with SemanticsError of the wire kind", async () => {
  for (const kind of ["empty_content", "binary_content", "file_too_large", "canceled"]) {
    const backend = fakeBackend(() => ({ error: { kind, message: `boom: ${kind}` } }));
    const analyzer = createAnalyzerWithBackend(backend);
    const err = await analyzer
      .analyzeBytes({ language: "go", content: "x" })
      .then(() => assert.fail("expected rejection"), (e: unknown) => e);
    assert.ok(err instanceof SemanticsError);
    assert.ok(!(err instanceof SemanticsSyntaxError));
    assert.equal(err.kind, kind);
    assert.equal(err.message, `boom: ${kind}`);
  }
});

test("unknown wire kinds coerce to internal", async () => {
  const backend = fakeBackend(() => ({ error: { kind: "quantum_flux", message: "??" } }));
  const analyzer = createAnalyzerWithBackend(backend);
  const err = await analyzer
    .analyzeBytes({ language: "go", content: "x" })
    .then(() => assert.fail("expected rejection"), (e: unknown) => e);
  assert.ok(err instanceof SemanticsError);
  assert.equal(err.kind, "internal");
});

test("responses with neither result nor error reject as internal", async () => {
  const backend = fakeBackend(() => ({}));
  const analyzer = createAnalyzerWithBackend(backend);
  const err = await analyzer
    .analyzeBytes({ language: "go", content: "x" })
    .then(() => assert.fail("expected rejection"), (e: unknown) => e);
  assert.ok(err instanceof SemanticsError);
  assert.equal(err.kind, "internal");
});

test("requests are encoded per the wire protocol", async () => {
  const backend = fakeBackend(() => ({ result: okResult }));
  const analyzer = createAnalyzerWithBackend(backend, { languages: ["go", "tsx"], maxFileBytes: 1024 });
  await analyzer.analyzeBytes({
    path: "a/b.go",
    language: "go",
    content: "package main\n",
    timeoutMs: 5000,
  });
  const request = backend.requests[0]!;
  assert.equal(request.op, "analyze");
  assert.equal(request.path, "a/b.go");
  assert.equal(request.language, "go");
  assert.equal(request.timeout_ms, 5000);
  assert.deepEqual(request.options, { languages: ["go", "tsx"], max_file_bytes: 1024 });
  assert.equal(
    Buffer.from(request.content_b64 as string, "base64").toString("utf-8"),
    "package main\n",
  );
});

test("string content is UTF-8 encoded; byte content passes through exactly", async () => {
  const backend = fakeBackend(() => ({ result: okResult }));
  const analyzer = createAnalyzerWithBackend(backend);

  await analyzer.analyzeBytes({ language: "go", content: "héllo → 世界" });
  const stringRequest = backend.requests[0]!;
  assert.deepEqual(
    Buffer.from(stringRequest.content_b64 as string, "base64"),
    Buffer.from("héllo → 世界", "utf-8"),
  );

  const raw = new Uint8Array([0x00, 0xff, 0xfe, 0x61]);
  await analyzer.analyzeBytes({ language: "go", content: raw });
  const byteRequest = backend.requests[1]!;
  assert.deepEqual(new Uint8Array(Buffer.from(byteRequest.content_b64 as string, "base64")), raw);
});

test("request ids are unique and increasing", async () => {
  const backend = fakeBackend(() => ({ result: okResult }));
  const analyzer = createAnalyzerWithBackend(backend);
  await analyzer.analyzeBytes({ language: "go", content: "a" });
  await analyzer.analyzeBytes({ language: "go", content: "b" });
  const [first, second] = backend.requests;
  assert.ok(typeof first!.id === "number" && typeof second!.id === "number");
  assert.ok((second!.id as number) > (first!.id as number));
});

test("createAnalyzerWithBackend validates options eagerly", () => {
  const backend = fakeBackend(() => ({ result: okResult }));
  assert.throws(
    () => createAnalyzerWithBackend(backend, { maxFileBytes: -1 }),
    (err: unknown) => err instanceof SemanticsError && err.kind === "invalid_options",
  );
  assert.throws(
    () => createAnalyzerWithBackend(backend, { languages: ["python" as never] }),
    (err: unknown) => err instanceof SemanticsError && err.kind === "unsupported_language",
  );
  // maxFileBytes: 0 means "use Go default" — only negatives are invalid.
  assert.doesNotThrow(() => createAnalyzerWithBackend(backend, { maxFileBytes: 0 }));
});

test("zero and empty option values are omitted from the wire request", async () => {
  const backend = fakeBackend(() => ({ result: okResult }));
  const analyzer = createAnalyzerWithBackend(backend, { languages: [], maxFileBytes: 0 });
  await analyzer.analyzeBytes({ language: "go", content: "package main\n", timeoutMs: 0 });
  const request = backend.requests[0]!;
  assert.deepEqual(request.options, {});
  assert.equal(Object.hasOwn(request, "timeout_ms"), false);
});

test("dispose runs onDispose once and blocks further calls", async () => {
  const backend = fakeBackend(() => ({ result: okResult }));
  let released = 0;
  const analyzer = createAnalyzerWithBackend(backend, {}, () => {
    released += 1;
  });
  analyzer.dispose();
  analyzer.dispose();
  assert.equal(released, 1);
  const err = await analyzer
    .analyzeBytes({ language: "go", content: "x" })
    .then(() => assert.fail("expected rejection"), (e: unknown) => e);
  assert.ok(err instanceof SemanticsError);
  assert.equal(err.kind, "internal");
});

test("dispose falls through to backend.dispose() when no onDispose is given", async () => {
  const backend = fakeBackend(() => ({ result: okResult }));
  const analyzer = createAnalyzerWithBackend(backend);
  assert.equal(backend.disposed, false);
  analyzer.dispose();
  analyzer.dispose();
  assert.equal(backend.disposed, true);
});

test("languageForExtension mirrors semantics.LanguageForExtension", () => {
  assert.equal(languageForExtension(".go"), "go");
  assert.equal(languageForExtension(".TS"), "typescript");
  assert.equal(languageForExtension(".tsx"), "tsx");
  assert.equal(languageForExtension(".py"), undefined);
  assert.equal(languageForExtension("go"), undefined);
});
