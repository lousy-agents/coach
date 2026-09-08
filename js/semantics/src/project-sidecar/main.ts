#!/usr/bin/env node
import { analyzeProject, SidecarBackendError, type CompilerBundle } from "./analyze.js";
import { resolveCompilerRootURL, resolveNativeTsserverPath } from "./compiler-argv.js";
import { CompilerLoadError, loadCompiler } from "./compiler-load.js";
import { describeErrorWithoutPaths } from "./describe-error.js";
import { KIND_INTERNAL, OP_ANALYZE_PROJECT, PROTOCOL_VERSION, SIDECAR_PHASE, type Request, type Response } from "./protocol.js";
import { readRequestLine, writeResponseLine } from "./stdio.js";
import { applyTestHook, readTestDelayHook } from "./test-hook.js";

/**
 * Genuine internal bugs propagate to the top-level catch, which fails
 * loudly, rather than emitting a response that could misrepresent a broken
 * analysis as a clean one.
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

  let compiler: CompilerBundle;
  let tsserverPath: string;
  try {
    const argv = process.argv.slice(2);
    compiler = await loadCompiler(await resolveCompilerRootURL(argv));
    tsserverPath = await resolveNativeTsserverPath(argv);
  } catch (err) {
    // A CompilerLoadError's own .message is already constructed to be
    // path-free (its throw sites already run any wrapped raw fs/import
    // error through describeErrorWithoutPaths below), so it is reported
    // verbatim; anything else reaching here is an unanticipated failure
    // whose .message is not trusted to be path-free.
    const detail = err instanceof CompilerLoadError ? err.message : describeErrorWithoutPaths(err);
    writeErrorResponse(req, `failed to load resolved TypeScript compiler module: ${detail}`);
    return;
  }

  await applyTestHook(process.argv.slice(2));

  try {
    const { edges, callGraph, reachabilityFacts, coverage } = analyzeProject({
      files: req.files ?? [],
      roots: req.roots,
      timeoutMs: req.timeout_ms,
      testDelayMsPerProject: readTestDelayHook(),
      compiler,
      tsserverPath,
    });
    const response: Response = {
      version: PROTOCOL_VERSION,
      id: req.id,
      import_edges: edges.length > 0 ? edges : undefined,
      call_graph: callGraph.length > 0 ? callGraph : undefined,
      reachability_facts: reachabilityFacts.length > 0 ? reachabilityFacts : undefined,
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

main().catch((err: unknown) => {
  process.stderr.write(`coach-ts-project-sidecar: fatal: ${describeErrorWithoutPaths(err)}\n`);
  process.exitCode = 1;
});
