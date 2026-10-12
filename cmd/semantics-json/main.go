// Command semantics-json exposes internal/jsbridge as a long-running
// newline-delimited-JSON server on stdin/stdout, so non-Go hosts (the
// js/semantics package) can call pkg/semantics without linking CGO
// themselves. One protocol Request JSON object per input line; one Response
// JSON object per output line, matched by id. stdout carries protocol JSON
// only; diagnostics go to stderr. The process exits 0 on stdin EOF.
//
// The --once flag reads exactly one request, responds, and exits, which
// makes manual debugging trivial:
//
//	echo '{"id":1,"op":"analyze","language":"go","content_b64":"cGFja2FnZSBtYWluCg=="}' | semantics-json --once
package main

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
)

func main() {
	once := flag.Bool("once", false, "read exactly one request, respond, and exit")
	flag.Parse()

	if err := serve(context.Background(), os.Stdin, os.Stdout, *once); err != nil {
		fmt.Fprintf(os.Stderr, "semantics-json: %v\n", err)
		os.Exit(1)
	}
}

// maxLineBytes bounds one request line. The analyzer's default content cap
// is 2 MiB, which base64 inflates by 4/3; 8 MiB leaves generous slack for
// the JSON envelope and larger caller-configured max_file_bytes.
const maxLineBytes = 8 * 1024 * 1024

// serve processes requests sequentially until EOF (or after one request in
// once mode). Responses come back in request order; callers may still
// pipeline writes and correlate by id.
func serve(ctx context.Context, in io.Reader, out io.Writer, once bool) error {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineBytes)
	writer := bufio.NewWriter(out)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		if err := writeResponse(writer, handleLine(ctx, line)); err != nil {
			return err
		}
		if once {
			return nil
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read stdin: %w", err)
	}
	if once {
		return fmt.Errorf("stdin closed before a request arrived")
	}
	return nil
}
