package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

type scopeHeader struct {
	Repository string `json:"repository"`
	Revision   string `json:"revision"`
	Base       string `json:"base"`
}

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

// readFileChanges drains scanner's remaining file-request lines. A per-line
// decode or analysis failure is reported as a diagnostic rather than an
// error; only a context cancellation or a stdin read failure stops the scan.
func readFileChanges(ctx context.Context, analyzer *semantics.Analyzer, scanner *bufio.Scanner) ([]codesignal.FileChange, []codesignal.Diagnostic, error) {
	var files []codesignal.FileChange
	var diagnostics []codesignal.Diagnostic
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}

		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}

		fileDiagnostics, fc := processFileRequestLine(ctx, analyzer, line)
		diagnostics = append(diagnostics, fileDiagnostics...)
		if fc != nil {
			files = append(files, *fc)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("read stdin: %w", err)
	}
	return files, diagnostics, nil
}
