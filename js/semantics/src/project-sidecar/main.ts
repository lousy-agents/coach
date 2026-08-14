#!/usr/bin/env node
import { analyzeProject, SidecarBackendError } from "./analyze.js";
import { KIND_INTERNAL, OP_ANALYZE_PROJECT, PROTOCOL_VERSION, SIDECAR_PHASE, type Request, type Response } from "./protocol.js";
import { readRequestLine, writeResponseLine } from "./stdio.js";

/**
 * Entry point for the pinned Node/TypeScript project sidecar (issue #214
 * Task 2): reads exactly one internal/projectbridge.Request line from
 * stdin, analyzes the snapshot it carries, writes exactly one Response
 * line to stdout, and exits 0. Genuine internal bugs (anything not one of
 * the handled operational conditions below) are deliberately left to
 * propagate to the top-level catch, which fails loudly -- non-zero exit,
 * stderr -- rather than emitting a response that might misrepresent a
 * broken analysis as a clean one.
 */
async function main(): Promise<void> {
  const line = await readRequestLine(process.stdin);

  let req: Request;
  try {
    req = JSON.parse(line) as Request;
  } catch (err) {
    process.stderr.write(`coach-ts-project-sidecar: malformed request JSON: ${String(err)}\n`);
    process.exitCode = 1;
    return;
  }

  if (req.op !== OP_ANALYZE_PROJECT) {
    writeErrorResponse(req, `unsupported op ${JSON.stringify(req.op)}`);
    return;
  }

  try {
    const { edges, coverage } = analyzeProject({
      files: req.files ?? [],
      roots: req.roots,
      timeoutMs: req.timeout_ms,
      testDelayMsPerProject: readTestDelayHook(),
    });
    const response: Response = {
      version: PROTOCOL_VERSION,
      id: req.id,
      import_edges: edges.length > 0 ? edges : undefined,
      coverage,
    };
    writeResponseLine(process.stdout, response);
  } catch (err) {
    if (err instanceof SidecarBackendError) {
      writeErrorResponse(req, err.message);
      return;
    }
    throw err;
  }
}

function writeErrorResponse(req: Request, message: string): void {
  const response: Response = {
    version: PROTOCOL_VERSION,
    id: req.id,
    coverage: { phase: SIDECAR_PHASE, complete: false },
    error: { kind: KIND_INTERNAL, message },
  };
  writeResponseLine(process.stdout, response);
}

/**
 * Test-only hook: COACH_TS_SIDECAR_TEST_DELAY_MS injects a fixed
 * synchronous per-project delay into analysis (see analyze.ts's
 * AnalyzeOptions.testDelayMsPerProject) so timeout_ms self-enforcement can
 * be exercised deterministically. Unset in production; forwarding an
 * environment variable here (rather than a request field) keeps the test
 * hook entirely out of the wire protocol.
 */
function readTestDelayHook(): number | undefined {
  const raw = process.env.COACH_TS_SIDECAR_TEST_DELAY_MS;
  if (!raw) return undefined;
  const n = Number(raw);
  return Number.isFinite(n) && n > 0 ? n : undefined;
}

main().catch((err: unknown) => {
  process.stderr.write(`coach-ts-project-sidecar: fatal: ${err instanceof Error ? (err.stack ?? err.message) : String(err)}\n`);
  process.exitCode = 1;
});
