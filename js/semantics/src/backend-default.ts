import type { Backend } from "./backend.js";
import { CliBackend } from "./backend-cli.js";

interface SharedEntry {
  backend: Backend;
  refs: number;
}

let shared: SharedEntry | undefined;

/**
 * Acquire a handle on the process-wide default backend, spinning it up on
 * first use. Every createAnalyzer call acquires once and releases via
 * analyzer.dispose(); the backend is shared because the protocol is
 * stateless, and the last release disposes it (ending the child process).
 *
 * The default transport is the cmd/semantics-json stdio child. A WASM
 * transport was the preferred design but is blocked today: pkg/semantics
 * requires CGO for Tree-sitter, which standard GOOS=js cannot compile, and
 * the TinyGo route is unproven — swapping it in later only means changing
 * this function.
 */
export async function acquireDefaultBackend(): Promise<{ backend: Backend; release: () => void }> {
  if (shared === undefined) {
    shared = { backend: new CliBackend(), refs: 0 };
  }
  const entry = shared;
  entry.refs += 1;
  let released = false;
  return {
    backend: entry.backend,
    release: () => {
      if (released) {
        return;
      }
      released = true;
      entry.refs -= 1;
      if (entry.refs <= 0 && shared === entry) {
        shared = undefined;
        entry.backend.dispose();
      }
    },
  };
}
