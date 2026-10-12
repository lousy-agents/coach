// Command codesignal-report reads a batch of NDJSON lines from stdin,
// accumulates a single codesignal.Input, and writes exactly one JSON
// codesignal.Report to stdout. Unlike cmd/semantics-json's per-line
// request/response protocol, this is a batch adapter: it reads the whole
// stream, calls codesignal.Builder.Build once at EOF, and exits.
//
// The first non-blank line is a scope header object with optional
// "repository", "revision", and "base" string fields. A missing or
// malformed header line still succeeds, but is reported as a
// "malformed_scope_header" diagnostic in the final Report and leaves Scope
// zero-valued. Every later non-blank line is a file-request object:
//
//	{"path": string, "language": string, "head_content": base64?, "base_content": base64?, "changed_ranges": [{"start_row": uint, "end_row": uint}]?}
//
// head_content/base_content are base64-encoded source bytes; their presence
// (not their decoded content) determines the derived ChangeStatus
// ("added"/"modified"/"removed"/"unknown"). Malformed request lines,
// invalid base64, and analysis failures are all reported as diagnostics in
// the one final Report rather than aborting the stream. codesignal-report
// never touches the filesystem, network, GitHub, or an LLM -- all file
// content arrives inline as base64.
//
// Example:
//
//	printf '%s\n%s\n' \
//	  '{"repository":"example/repo","revision":"abc123"}' \
//	  '{"path":"main.go","language":"go","head_content":"cGFja2FnZSBtYWluCg=="}' \
//	  | go run ./cmd/codesignal-report
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

const maxLineBytes = 8 * 1024 * 1024

func main() {
	if err := run(context.Background(), os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "codesignal-report: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, in io.Reader, out io.Writer) error {
	analyzer, err := semantics.NewAnalyzer(semantics.AnalyzerOptions{})
	if err != nil {
		return err
	}
	builder, err := codesignal.New(codesignal.Options{})
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineBytes)

	scope, diagnostics := readScopeHeader(scanner)
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read stdin: %w", err)
	}

	files, fileDiagnostics, err := readFileChanges(ctx, analyzer, scanner)
	if err != nil {
		return err
	}
	diagnostics = append(diagnostics, fileDiagnostics...)

	report, err := builder.Build(ctx, codesignal.Input{Scope: scope, Files: files, Diagnostics: diagnostics})
	if err != nil {
		return err
	}
	return writeJSONLine(out, report)
}

func writeJSONLine(out io.Writer, report any) error {
	encoded, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}

	writer := bufio.NewWriter(out)
	if _, err := writer.Write(encoded); err != nil {
		return fmt.Errorf("write report: %w", err)
	}
	if err := writer.WriteByte('\n'); err != nil {
		return fmt.Errorf("write report: %w", err)
	}
	return writer.Flush()
}
