import { spawn, type ChildProcessByStdio } from "node:child_process";
import { existsSync } from "node:fs";
import type { Readable, Writable } from "node:stream";
import { fileURLToPath } from "node:url";

import type { Backend } from "./backend.js";
import { SemanticsError } from "./errors.js";

/**
 * Built by `npm run build:backend` (or `mise run backend-build`), which runs
 * `go build ./cmd/semantics-json` at the repo root. The binary requires a
 * CGO-capable Go toolchain — the price of Tree-sitter, and the reason this
 * transport exists: standard GOOS=js WASM cannot compile CGO at all.
 */
const BINARY_URL = new URL("../bin/coach-semantics-json", import.meta.url);

/** Grace period past the Go-side timeout before we assume the child is stuck. */
const BACKSTOP_SLACK_MS = 500;

interface PendingCall {
  resolve: (responseJson: string) => void;
  reject: (err: Error) => void;
  timer?: NodeJS.Timeout;
}

/**
 * Backend that talks newline-delimited protocol JSON to a single long-lived
 * cmd/semantics-json child process. One child is shared per backend; calls
 * may pipeline and are correlated by request id. The child is spawned
 * lazily, restarted lazily after a crash or a backstop kill, and terminated
 * by dispose() (graceful stdin close, since the server exits 0 on EOF).
 */
export class CliBackend implements Backend {
  private readonly binaryUrl: URL;
  private child: ChildProcessByStdio<Writable, Readable, null> | undefined;
  private readonly pending = new Map<number, PendingCall>();
  private stdoutBuffer = "";
  private disposed = false;

  constructor(binaryUrl: URL = BINARY_URL) {
    this.binaryUrl = binaryUrl;
  }

  analyze(requestJson: string): Promise<string> {
    if (this.disposed) {
      return Promise.reject(new SemanticsError("internal", "backend has been disposed"));
    }

    let id: number;
    let timeoutMs: number | undefined;
    try {
      const request = JSON.parse(requestJson) as { id: number; timeout_ms?: number };
      id = request.id;
      timeoutMs = request.timeout_ms;
    } catch (err) {
      return Promise.reject(new SemanticsError("internal", `malformed request JSON: ${String(err)}`));
    }

    let child: ChildProcessByStdio<Writable, Readable, null>;
    try {
      child = this.ensureChild();
    } catch (err) {
      return Promise.reject(err instanceof Error ? err : new Error(String(err)));
    }

    return new Promise<string>((resolve, reject) => {
      const call: PendingCall = { resolve, reject };
      if (timeoutMs !== undefined && timeoutMs > 0) {
        // Backstop for the Go-side context timeout: a C parse cannot be
        // interrupted mid-flight, so a child stuck past the deadline gets
        // killed and lazily respawned. Killing rejects every pending call.
        call.timer = setTimeout(() => {
          this.failAllPending(
            new SemanticsError("canceled", `backend did not respond within ${timeoutMs}ms; child killed`),
          );
          this.killChild();
        }, timeoutMs + BACKSTOP_SLACK_MS);
        call.timer.unref?.();
      }
      this.pending.set(id, call);
      child.stdin.write(requestJson + "\n", (err) => {
        if (err) {
          this.settle(id)?.reject(new SemanticsError("internal", `write to backend failed: ${err.message}`));
        }
      });
    });
  }

  dispose(): void {
    if (this.disposed) {
      return;
    }
    this.disposed = true;
    this.failAllPending(new SemanticsError("internal", "backend disposed with calls in flight"));
    if (this.child) {
      this.child.stdin.end();
      this.child = undefined;
    }
  }

  private ensureChild(): ChildProcessByStdio<Writable, Readable, null> {
    if (this.child) {
      return this.child;
    }
    if (!existsSync(this.binaryUrl)) {
      throw new SemanticsError(
        "backend_unavailable",
        `semantics backend binary not found at ${fileURLToPath(this.binaryUrl)}; ` +
          "build it with `npm run build:backend` in js/semantics (or `mise run backend-build` at the repo root)",
      );
    }

    const child = spawn(fileURLToPath(this.binaryUrl), [], {
      stdio: ["pipe", "pipe", "inherit"],
    });
    child.stdout.setEncoding("utf-8");
    child.stdout.on("data", (chunk: string) => {
      this.onStdout(chunk);
    });
    const onGone = (cause: string) => {
      if (this.child === child) {
        this.child = undefined;
        this.stdoutBuffer = "";
      }
      this.failAllPending(new SemanticsError("internal", `semantics backend ${cause}`));
    };
    child.on("error", (err) => {
      onGone(`failed: ${err.message}`);
    });
    child.on("exit", (code, signal) => {
      onGone(`exited (code ${code ?? "null"}, signal ${signal ?? "null"})`);
    });
    // Deliberately not unref()ed: the child must keep the event loop alive
    // while calls are in flight (as of Node 22.23, unref() detaches the
    // stdio pipes from the loop too, letting the process exit mid-call).
    // dispose() is the documented way to let the process exit.
    this.child = child;
    return child;
  }

  private onStdout(chunk: string): void {
    this.stdoutBuffer += chunk;
    for (;;) {
      const newline = this.stdoutBuffer.indexOf("\n");
      if (newline === -1) {
        return;
      }
      const line = this.stdoutBuffer.slice(0, newline);
      this.stdoutBuffer = this.stdoutBuffer.slice(newline + 1);
      if (line.trim() === "") {
        continue;
      }
      this.onResponseLine(line);
    }
  }

  private onResponseLine(line: string): void {
    let id: number;
    try {
      id = (JSON.parse(line) as { id: number }).id;
    } catch {
      id = 0;
    }
    const call = this.settle(id);
    if (call) {
      call.resolve(line);
      return;
    }
    // id 0 (unattributable server-side failure) or an id we no longer track:
    // the stream can't be trusted to stay correlated, so drop the child and
    // fail everything; the next call respawns.
    this.failAllPending(
      new SemanticsError("internal", `backend sent an uncorrelated response (id ${id}); child restarted`),
    );
    this.killChild();
  }

  /** Remove and return one pending call, clearing its backstop timer. */
  private settle(id: number): PendingCall | undefined {
    const call = this.pending.get(id);
    if (!call) {
      return undefined;
    }
    this.pending.delete(id);
    if (call.timer !== undefined) {
      clearTimeout(call.timer);
    }
    return call;
  }

  private failAllPending(err: SemanticsError): void {
    for (const id of [...this.pending.keys()]) {
      this.settle(id)?.reject(err);
    }
  }

  private killChild(): void {
    if (this.child) {
      const child = this.child;
      this.child = undefined;
      this.stdoutBuffer = "";
      child.removeAllListeners("exit");
      child.removeAllListeners("error");
      child.kill("SIGKILL");
    }
  }
}
