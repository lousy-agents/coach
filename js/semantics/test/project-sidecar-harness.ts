/**
 * Shared spawn harness for project-sidecar acceptance tests. Kept out of
 * project-sidecar.test.ts so the suite file stays below cognitive-complexity
 * thresholds while this helper owns the process I/O state machine.
 */
import { spawn } from "node:child_process";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

const PACKAGE_ROOT = fileURLToPath(new URL("..", import.meta.url));
export const BIN_PATH = join(PACKAGE_ROOT, "bin", "coach-ts-project-sidecar");
export const DEFAULT_COMPILER_MODULE = join(PACKAGE_ROOT, "node_modules", "typescript");

export interface WireFile {
  path: string;
  content_b64: string;
}

export interface WireRequest {
  version: number;
  op: string;
  id: number;
  files: WireFile[];
  roots?: string[];
  timeout_ms?: number;
}

export interface WireEdge {
  from: string;
  to: string;
  kind: string;
  site?: string;
  resolution?: string;
}

export interface WireDiagnostic {
  code: string;
  message: string;
  path?: string;
}

export interface WireCallGraphEdge {
  from: string;
  to: string;
}

export interface WireReachabilityStep {
  node_id: string;
}

export interface WireReachabilityFact {
  id: string;
  kind: string;
  confidence: string;
  source: string;
  sink: string;
  path: WireReachabilityStep[];
  algorithm_version: string;
  backend?: string;
}

export interface WireResponse {
  version: number;
  id: number;
  import_edges?: WireEdge[];
  call_graph?: WireCallGraphEdge[];
  reachability_facts?: WireReachabilityFact[];
  coverage: {
    phase: string;
    complete: boolean;
    counts?: Record<string, number>;
    budgets?: Record<string, number>;
    diagnostics?: WireDiagnostic[];
  };
  error?: { kind: string; message: string };
}

export function file(path: string, content: string): WireFile {
  return { path, content_b64: Buffer.from(content, "utf8").toString("base64") };
}

let nextId = 1;

export function runSidecar(
  request: Omit<WireRequest, "version" | "op" | "id"> & Partial<Pick<WireRequest, "version" | "op" | "id">>,
  env?: Record<string, string>,
  args?: readonly string[],
): Promise<{ response: WireResponse; rawLine: string; exitCode: number | null }> {
  const fullRequest: WireRequest = {
    version: 1,
    op: "analyze_project",
    id: nextId++,
    ...request,
  };
  const argv = args === undefined ? [`--compiler-module=${DEFAULT_COMPILER_MODULE}`] : [...args];
  return spawnAndRead(fullRequest, env, argv);
}

function spawnAndRead(
  fullRequest: WireRequest,
  env?: Record<string, string>,
  args?: readonly string[],
): Promise<{ response: WireResponse; rawLine: string; exitCode: number | null }> {
  return new Promise((resolve, reject) => {
    const child = spawn(BIN_PATH, args ? [...args] : [], {
      stdio: ["pipe", "pipe", "pipe"],
      env: { ...process.env, ...env },
    });
    const buckets = { stdout: "", stderr: "" };
    let settled = false;

    const finish = (fn: () => void) => {
      if (settled) return;
      settled = true;
      clearTimeout(budget);
      fn();
    };

    const budget = setTimeout(() => {
      finish(() => {
        child.kill("SIGKILL");
        reject(new Error(`sidecar did not respond within the test budget; stderr so far: ${buckets.stderr}`));
      });
    }, 30000);
    budget.unref();

    child.stdout.setEncoding("utf8");
    child.stdout.on("data", (chunk: string) => {
      buckets.stdout += chunk;
    });
    child.stderr.setEncoding("utf8");
    child.stderr.on("data", (chunk: string) => {
      buckets.stderr += chunk;
    });
    child.on("error", (err) => finish(() => reject(err)));
    child.on("exit", (code) => finish(() => settleExit(code, buckets, resolve, reject)));

    child.stdin.write(`${JSON.stringify(fullRequest)}\n`);
    child.stdin.end();
  });
}

function settleExit(
  code: number | null,
  buckets: { stdout: string; stderr: string },
  resolve: (v: { response: WireResponse; rawLine: string; exitCode: number | null }) => void,
  reject: (e: Error) => void,
): void {
  const newline = buckets.stdout.indexOf("\n");
  const rawLine = newline === -1 ? buckets.stdout : buckets.stdout.slice(0, newline);
  if (rawLine.trim() === "") {
    reject(new Error(`sidecar exited (code ${code}) without a response line; stderr: ${buckets.stderr}`));
    return;
  }
  try {
    resolve({ response: JSON.parse(rawLine) as WireResponse, rawLine, exitCode: code });
  } catch (err) {
    reject(new Error(`malformed response JSON: ${String(err)}; line=${rawLine}`));
  }
}

export function edgesTo(response: WireResponse, to: string): WireEdge[] {
  return (response.import_edges ?? []).filter((e) => e.to === to);
}
