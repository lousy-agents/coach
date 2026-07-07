/**
 * Lifecycle tests for the stdio CLI backend: missing-binary reporting,
 * pipelined correlation, crash recovery, timeouts, and dispose semantics.
 */
import assert from "node:assert/strict";
import { once } from "node:events";
import { test } from "node:test";

import { CliBackend } from "../dist/backend-cli.js";
import { createAnalyzerWithBackend, SemanticsError } from "../dist/index.js";

test("missing binary rejects with backend_unavailable naming the build command", async () => {
  const backend = new CliBackend(new URL("file:///nonexistent/coach-semantics-json"));
  const analyzer = createAnalyzerWithBackend(backend);
  const err = await analyzer
    .analyzeBytes({ language: "go", content: "package main\n" })
    .then(() => assert.fail("expected rejection"), (e: unknown) => e);
  assert.ok(err instanceof SemanticsError);
  assert.equal(err.kind, "backend_unavailable");
  assert.match(err.message, /build:backend|backend-build/);
});

test("pipelined concurrent calls stay correlated by id", async () => {
  const backend = new CliBackend();
  try {
    const analyzer = createAnalyzerWithBackend(backend);
    const inputs = Array.from({ length: 10 }, (_, i) => `package pkg${i}\n`);
    const results = await Promise.all(
      inputs.map((content, i) =>
        analyzer.analyzeBytes({ path: `file${i}.go`, language: "go", content }),
      ),
    );
    results.forEach((result, i) => {
      assert.equal(result.path, `file${i}.go`);
      assert.equal(result.parse_status, "ok");
    });
  } finally {
    backend.dispose();
  }
});

test("backend respawns its child after a crash", async () => {
  const backend = new CliBackend();
  try {
    const analyzer = createAnalyzerWithBackend(backend);
    const first = await analyzer.analyzeBytes({ language: "go", content: "package main\n" });
    assert.equal(first.parse_status, "ok");

    // Reach into the backend to kill the live child out from under it.
    const child = (backend as unknown as { child: { kill(signal: string): boolean } | undefined }).child;
    assert.ok(child, "backend has no live child after a successful call");
    const gone = once(child as unknown as NodeJS.EventEmitter, "exit");
    child.kill("SIGKILL");
    await gone;

    const second = await analyzer.analyzeBytes({ language: "go", content: "package other\n" });
    assert.equal(second.parse_status, "ok");
  } finally {
    backend.dispose();
  }
});

test("go-side timeout surfaces as canceled", async () => {
  const backend = new CliBackend();
  try {
    const analyzer = createAnalyzerWithBackend(backend);
    // ~1.5 MB of source with a 1ms budget: the deadline is already exceeded
    // by the first context check inside AnalyzeBytes.
    const big = "package main\n" + "// filler comment line to inflate the file\n".repeat(35000);
    const err = await analyzer
      .analyzeBytes({ language: "go", content: big, timeoutMs: 1 })
      .then(() => assert.fail("expected rejection"), (e: unknown) => e);
    assert.ok(err instanceof SemanticsError);
    assert.equal(err.kind, "canceled");
  } finally {
    backend.dispose();
  }
});

test("dispose ends the child and blocks further calls", async () => {
  const backend = new CliBackend();
  const analyzer = createAnalyzerWithBackend(backend);
  const result = await analyzer.analyzeBytes({ language: "go", content: "package main\n" });
  assert.equal(result.parse_status, "ok");
  backend.dispose();
  backend.dispose();
  const err = await backend
    .analyze(JSON.stringify({ id: 999, op: "analyze", language: "go", content_b64: "", options: {} }))
    .then(() => assert.fail("expected rejection"), (e: unknown) => e);
  assert.ok(err instanceof SemanticsError);
  assert.equal(err.kind, "internal");
});
