import type { Readable, Writable } from "node:stream";
/**
 * Reads exactly one line (up to and excluding the first `\n`) from stdin.
 * The Go client (internal/projectbridge, pkg/projectmodel's ts_sidecar.go)
 * writes its Request as a single fixed-length buffer terminated by one
 * `\n` and then lets stdin hit EOF, so accumulating chunks until either
 * the first newline or EOF is sufficient; any bytes after the first
 * newline are never read.
 */
export declare function readRequestLine(stdin: Readable): Promise<string>;
/** Writes exactly one NDJSON-terminated Response line to stdout. */
export declare function writeResponseLine(stdout: Writable, response: unknown): void;
//# sourceMappingURL=stdio.d.ts.map