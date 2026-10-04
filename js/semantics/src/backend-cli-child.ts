import { spawn, type ChildProcessByStdio } from "node:child_process";
import { existsSync } from "node:fs";
import type { Readable, Writable } from "node:stream";
import { fileURLToPath } from "node:url";

import { SemanticsError } from "./errors.js";

export interface PendingCall {
  resolve: (responseJson: string) => void;
  reject: (err: Error) => void;
  timer?: NodeJS.Timeout;
}

/**
 * Owns the long-lived semantics-json child: lazy spawn, stdout framing,
 * pending-call correlation, and crash/kill recovery. CliBackend keeps the
 * request/dispose policy and delegates process lifetime here.
 */
export class CliChildSession {
  child: ChildProcessByStdio<Writable, Readable, null> | undefined;
  private readonly calls = new Map<number, PendingCall>();
  private stdoutBuffer = "";

  /** Pending calls, keyed by request id, so a caller can see whether a backstop timer is armed. */
  get pending(): Map<number, PendingCall> {
    return this.calls;
  }

  constructor(private readonly binaryUrl: URL) {}

  ensure(): ChildProcessByStdio<Writable, Readable, null> {
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
      this.failAll(new SemanticsError("internal", `semantics backend ${cause}`));
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

  track(id: number, call: PendingCall): void {
    this.calls.set(id, call);
  }

  rejectSettled(id: number, err: SemanticsError): void {
    this.settle(id)?.reject(err);
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
    this.failAll(
      new SemanticsError("internal", `backend sent an uncorrelated response (id ${id}); child restarted`),
    );
    this.kill();
  }

  /** Remove and return one pending call, clearing its backstop timer. */
  private settle(id: number): PendingCall | undefined {
    const call = this.calls.get(id);
    if (!call) {
      return undefined;
    }
    this.calls.delete(id);
    if (call.timer !== undefined) {
      clearTimeout(call.timer);
    }
    return call;
  }

  failAll(err: SemanticsError): void {
    for (const id of [...this.calls.keys()]) {
      this.settle(id)?.reject(err);
    }
  }

  closeStdin(): void {
    this.child?.stdin.end();
    this.child = undefined;
  }

  kill(): void {
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
