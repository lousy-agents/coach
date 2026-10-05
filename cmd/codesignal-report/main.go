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
	"bytes"
	"context"

	"encoding/json"

	"fmt"

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

type scopeHeader struct {
	Repository string `json:"repository"`
	Revision   string `json:"revision"`
	Base       string `json:"base"`
}

type lineRangeRequest struct {
	StartRow uint `json:"start_row"`
	EndRow   uint `json:"end_row"`
}

type fileRequest struct {
	Path          string             `json:"path"`
	Language      string             `json:"language"`
	HeadContent   *string            `json:"head_content"`
	BaseContent   *string            `json:"base_content"`
	ChangedRanges []lineRangeRequest `json:"changed_ranges"`
}

// readFileChanges drains scanner's remaining file-request lines. A per-line
// decode or analysis failure is reported as a diagnostic rather than an
// error; only a context cancellation or a stdin read failure stops the scan.

func readScopeHeader(scanner *bufio.Scanner) (codesignal.Scope, []codesignal.Diagnostic) {
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}

		var header scopeHeader
		if err := json.Unmarshal(line, &header); err != nil {
			return codesignal.Scope{}, []codesignal.Diagnostic{{
				Kind:    "malformed_scope_header",
				Message: fmt.Sprintf("malformed scope header: %v", err),
			}}
		}
		return codesignal.Scope{
			Repository: header.Repository,
			Revision:   header.Revision,
			Base:       header.Base,
		}, nil
	}

	return codesignal.Scope{}, []codesignal.Diagnostic{{
		Kind:    "malformed_scope_header",
		Message: "stdin ended before a scope header line was found",
	}}
}

func processFileRequestLine(ctx context.Context, analyzer *semantics.Analyzer, line []byte) ([]codesignal.Diagnostic, *codesignal.FileChange) {
	var req fileRequest
	if err := json.Unmarshal(line, &req); err != nil {
		return []codesignal.Diagnostic{{
			Kind:    "malformed_file_request",
			Message: fmt.Sprintf("malformed file request line: %v", err),
		}}, nil
	}
	if req.Path == "" || req.Language == "" {
		return []codesignal.Diagnostic{{
			Kind:    "malformed_file_request",
			Message: "file request line missing required \"path\" or \"language\"",
		}}, nil
	}

	fc := codesignal.FileChange{
		Path:          req.Path,
		Status:        deriveChangeStatus(req.HeadContent, req.BaseContent),
		ChangedRanges: convertChangedRanges(req.ChangedRanges),
	}

	var diagnostics []codesignal.Diagnostic
	if req.HeadContent != nil {
		diag, result := decodeAndAnalyze(ctx, analyzer, req.Path, req.Language, *req.HeadContent)
		if diag != nil {
			diagnostics = append(diagnostics, *diag)
		}
		fc.Head = result
	}
	if req.BaseContent != nil {
		diag, result := decodeAndAnalyze(ctx, analyzer, req.Path, req.Language, *req.BaseContent)
		if diag != nil {
			diagnostics = append(diagnostics, *diag)
		}
		fc.Base = result
	}

	return diagnostics, &fc
}
