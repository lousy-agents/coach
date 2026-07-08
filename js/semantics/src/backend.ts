/**
 * A transport backend carries protocol JSON strings to internal/jsbridge's
 * Handle and back. Keeping the boundary at "JSON string in, JSON string out"
 * makes the wire format byte-identical across backends (WASM instance,
 * stdio child process, or an in-memory fake in tests).
 */
export interface Backend {
  /** Send one encoded Request; resolve with the encoded Response. */
  analyze(requestJson: string): Promise<string>;
  /** Release backend resources. Idempotent. */
  dispose(): void;
}
