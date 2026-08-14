import type { Readable, Writable } from "node:stream";

/**
 * Reads exactly one line (up to and excluding the first `\n`) from stdin.
 * The Go client (internal/projectbridge, pkg/projectmodel's ts_sidecar.go)
 * writes its Request as a single fixed-length buffer terminated by one
 * `\n` and then lets stdin hit EOF, so accumulating chunks until either
 * the first newline or EOF is sufficient; any bytes after the first
 * newline are never read.
 */
export function readRequestLine(stdin: Readable): Promise<string> {
  return new Promise((resolve, reject) => {
    let buf = "";
    let settled = false;

    const cleanup = () => {
      stdin.off("data", onData);
      stdin.off("end", onEnd);
      stdin.off("error", onError);
    };
    const finish = (value: string) => {
      if (settled) return;
      settled = true;
      cleanup();
      resolve(value);
    };
    const onData = (chunk: Buffer | string) => {
      buf += typeof chunk === "string" ? chunk : chunk.toString("utf8");
      const newline = buf.indexOf("\n");
      if (newline !== -1) {
        finish(buf.slice(0, newline));
      }
    };
    const onEnd = () => finish(buf);
    const onError = (err: Error) => {
      if (settled) return;
      settled = true;
      cleanup();
      reject(err);
    };

    stdin.on("data", onData);
    stdin.on("end", onEnd);
    stdin.on("error", onError);
    stdin.resume();
  });
}

/** Writes exactly one NDJSON-terminated Response line to stdout. */
export function writeResponseLine(stdout: Writable, response: unknown): void {
  stdout.write(`${JSON.stringify(response)}\n`);
}
