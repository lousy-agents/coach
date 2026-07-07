import { Buffer } from "node:buffer";

import { SemanticsError, SemanticsSyntaxError, toErrorKind } from "./errors.js";
import type { Result } from "./types.js";

/**
 * Wire types mirroring internal/jsbridge/protocol.go. snake_case fields are
 * the protocol, not a style accident.
 */
export interface WireOptions {
  languages?: string[];
  max_file_bytes?: number;
}

export interface WireRequest {
  id: number;
  op: "analyze";
  path?: string;
  language: string;
  content_b64: string;
  options: WireOptions;
  timeout_ms?: number;
}

export interface WireError {
  kind: string;
  message: string;
}

export interface WireResponse {
  id: number;
  result?: Result;
  error?: WireError;
}

let nextID = 0;

/** Allocate a fresh request id (monotonic per process). */
export function allocateRequestID(): number {
  nextID += 1;
  return nextID;
}

/**
 * Encode source content for the content_b64 field. Strings are encoded as
 * UTF-8; byte inputs pass through exactly (base64 exists so binary and
 * non-UTF-8 content survive the JSON transport unaltered).
 */
export function encodeContent(content: Uint8Array | string): string {
  const bytes = typeof content === "string" ? Buffer.from(content, "utf-8") : content;
  return Buffer.from(bytes.buffer, bytes.byteOffset, bytes.byteLength).toString("base64");
}

/**
 * Decode one Response JSON string into a Result, translating protocol errors
 * into the exception types callers switch on. A response carrying both
 * result and error is the wire form of Go's syntax double return and becomes
 * a SemanticsSyntaxError holding the partial Result.
 */
export function decodeResponse(responseJson: string): Result {
  let response: WireResponse;
  try {
    response = JSON.parse(responseJson) as WireResponse;
  } catch (err) {
    throw new SemanticsError("internal", `malformed response from backend: ${String(err)}`);
  }

  const { result, error } = response;
  if (error) {
    if (result) {
      throw new SemanticsSyntaxError(error.message, result);
    }
    throw new SemanticsError(toErrorKind(error.kind), error.message);
  }
  if (!result) {
    throw new SemanticsError("internal", "backend response carries neither result nor error");
  }
  return result;
}
