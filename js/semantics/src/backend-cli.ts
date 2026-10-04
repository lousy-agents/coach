import type { ChildProcessByStdio } from "node:child_process";
import type { Readable, Writable } from "node:stream";

import type { Backend } from "./backend.js";
import { CliChildSession, type PendingCall } from "./backend-cli-child.js";
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

/**
 * Node clamps setTimeout delays to a 32-bit signed int and fires
 * immediately on overflow; cap the backstop delay here so a large
 * caller-supplied timeoutMs can't trigger a premature kill.
 */
const MAX_TIMER_MS = 2 ** 31 - 1;

/**
 * Backend that talks newline-delimited protocol JSON to a single long-lived
 * cmd/semantics-json child process. One child is shared per backend; calls
 * may pipeline and are correlated by request id. The child is spawned
 * lazily, restarted lazily after a crash or a backstop kill, and terminated
 * by dispose() (graceful stdin close, since the server exits 0 on EOF).
 */
export class CliBackend implements Backend {
  private readonly session: CliChildSession;
  private disposed = false;

  constructor(binaryUrl: URL = BINARY_URL, session: CliChildSession = new CliChildSession(binaryUrl)) {
    this.session = session;
  }

  /** Live child process, so a caller can observe lifetime across a crash and the next call's respawn. */
  get child(): CliChildSession["child"] {
    return this.session.child;
  }

  /** In-flight calls, so a caller can see whether a backstop timer was armed. */
  get pending(): CliChildSession["pending"] {
    return this.session.pending;
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
      child = this.session.ensure();
    } catch (err) {
      return Promise.reject(err instanceof Error ? err : new Error(String(err)));
    }

    return new Promise<string>((resolve, reject) => {
      const call: PendingCall = { resolve, reject };
      if (timeoutMs !== undefined && timeoutMs > 0) {
        // Backstop for the Go-side context timeout: a C parse cannot be
        // interrupted mid-flight, so a child stuck past the deadline gets
        // killed and lazily respawned. Killing rejects every pending call.
        const delayMs = Math.min(timeoutMs + BACKSTOP_SLACK_MS, MAX_TIMER_MS);
        call.timer = setTimeout(() => {
          this.session.failAll(
            new SemanticsError("canceled", `backend did not respond within ${timeoutMs}ms; child killed`),
          );
          this.session.kill();
        }, delayMs);
        call.timer.unref?.();
      }
      this.session.track(id, call);
      child.stdin.write(requestJson + "\n", (err) => {
        if (err) {
          this.session.rejectSettled(id, new SemanticsError("internal", `write to backend failed: ${err.message}`));
        }
      });
    });
  }

  dispose(): void {
    if (this.disposed) {
      return;
    }
    this.disposed = true;
    this.session.failAll(new SemanticsError("internal", "backend disposed with calls in flight"));
    if (this.session.child) {
      this.session.closeStdin();
    }
  }
}
