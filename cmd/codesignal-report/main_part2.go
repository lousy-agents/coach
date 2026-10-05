package main

import (
	"bufio"
	"bytes"
	"context"

	"fmt"
	"io"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

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
