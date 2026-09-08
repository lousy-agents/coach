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
export const DEFAULT_NATIVE_PACKAGE = join(
  PACKAGE_ROOT,
  "node_modules",
  "@typescript",
  `typescript-${process.platform}-${process.arch}`,
);

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

type PartialRequest = Omit<WireRequest, "version" | "op" | "id"> & Partial<Pick<WireRequest, "version" | "op" | "id">>;

interface SidecarRun {
  exitCode: number | null;
  stdout: string;
  stderr: string;
}

export function runSidecar(
  request: PartialRequest,
  env?: Record<string, string>,
  args?: readonly string[],
): Promise<{ response: WireResponse; rawLine: string; exitCode: number | null }> {
  const argv =
    args === undefined
      ? [`--compiler-module=${DEFAULT_COMPILER_MODULE}`, `--native-package=${DEFAULT_NATIVE_PACKAGE}`]
      : [...args];
  return spawnAndCollect(fullRequest(request), env, argv).then(readResponse);
}

export function spawnSidecarWithoutResponse(request: PartialRequest, args: readonly string[]): Promise<SidecarRun> {
  return spawnAndCollect(fullRequest(request), undefined, args);
}

function fullRequest(request: PartialRequest): WireRequest {
  return { version: 1, op: "analyze_project", id: nextId++, ...request };
}

function spawnAndCollect(request: WireRequest, env?: Record<string, string>, args?: readonly string[]): Promise<SidecarRun> {
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
        reject(new Error(`sidecar did not exit within the test budget; stderr so far: ${buckets.stderr}`));
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
    child.on("exit", (code) => finish(() => resolve({ exitCode: code, stdout: buckets.stdout, stderr: buckets.stderr })));

    child.stdin.write(`${JSON.stringify(request)}\n`);
    child.stdin.end();
  });
}

function readResponse(run: SidecarRun): { response: WireResponse; rawLine: string; exitCode: number | null } {
  const newline = run.stdout.indexOf("\n");
  const rawLine = newline === -1 ? run.stdout : run.stdout.slice(0, newline);
  if (rawLine.trim() === "") {
    throw new Error(`sidecar exited (code ${run.exitCode}) without a response line; stderr: ${run.stderr}`);
  }
  try {
    return { response: JSON.parse(rawLine) as WireResponse, rawLine, exitCode: run.exitCode };
  } catch (err) {
    throw new Error(`malformed response JSON: ${String(err)}; line=${rawLine}`);
  }
}

export function edgesTo(response: WireResponse, to: string): WireEdge[] {
  return (response.import_edges ?? []).filter((e) => e.to === to);
}
