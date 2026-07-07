import type { Backend } from "./backend.js";
import { SemanticsError } from "./errors.js";

/**
 * Acquire a handle on the process-wide default backend, spinning it up on
 * first use. Every createAnalyzer call acquires once and releases via
 * analyzer.dispose(); the backend itself is shared because the protocol is
 * stateless.
 *
 * Placeholder until the transport decision (TinyGo WASM spike vs stdio CLI
 * fallback) lands; the real implementation replaces this function only.
 */
export async function acquireDefaultBackend(): Promise<{ backend: Backend; release: () => void }> {
  throw new SemanticsError(
    "backend_unavailable",
    "no semantics backend is built yet; the transport backend lands with the TinyGo-spike decision",
  );
}
