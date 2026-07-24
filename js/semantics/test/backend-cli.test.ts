/**
 * Lifecycle tests for the stdio CLI backend: missing-binary reporting,
 * pipelined correlation, crash recovery, timeouts, and dispose semantics.
 */
import assert from "node:assert/strict";
import { once } from "node:events";
import { chmodSync, mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { pathToFileURL } from "node:url";
import { test } from "node:test";

import { CliBackend } from "../dist/backend-cli.js";
import { createAnalyzerWithBackend, SemanticsError } from "../dist/index.js";

/** A child that accepts stdin forever and never writes a response line. */
function hangForeverBinaryUrl(): URL {
  const dir = mkdtempSync(join(tmpdir(), "coach-semantics-hang-"));
  const path = join(dir, "hang-forever");
  writeFileSync(path, "#!/bin/sh\nexec cat >/dev/null\n");
  chmodSync(path, 0o755);
  return pathToFileURL(path);
}

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

// Node clamps setTimeout delays to a 32-bit signed int and fires overflowing
// ones almost immediately. A timeoutMs past that threshold must not make the
// backstop kill the child before a fast, valid call can complete.
test("backstop timer does not fire early for a timeoutMs exceeding the 32-bit setTimeout limit", async () => {
  const backend = new CliBackend();
  try {
    const analyzer = createAnalyzerWithBackend(backend);
    const result = await analyzer.analyzeBytes({
      language: "go",
      content: "package main\n",
      timeoutMs: 2 ** 31, // one past Node's signed 32-bit setTimeout cap
    });
    assert.equal(result.parse_status, "ok");
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

// timeout_ms: 0 means "no deadline" — the JS backstop must not arm (BACKSTOP_SLACK_MS
// is 500ms; a mutant that treats 0 as a positive budget would kill the child by then).
test("timeout_ms of zero does not arm the backstop timer", async () => {
  const backend = new CliBackend(hangForeverBinaryUrl());
  try {
    const pending = backend.analyze(
      JSON.stringify({
        id: 1,
        op: "analyze",
        language: "go",
        content_b64: "",
        options: {},
        timeout_ms: 0,
      }),
    );
    let settled: unknown;
    void pending.then(
      (v) => {
        settled = { ok: v };
      },
      (e: unknown) => {
        settled = { err: e };
      },
    );
    await new Promise((r) => setTimeout(r, 700));
    assert.equal(settled, undefined, `call settled early: ${String((settled as { err?: unknown })?.err ?? settled)}`);
  } finally {
    backend.dispose();
  }
});
