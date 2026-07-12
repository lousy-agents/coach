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
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

// maxLineBytes bounds one stdin line. Mirrors cmd/semantics-json's
// rationale: the analyzer's default content cap is 2 MiB, which base64
// inflates by 4/3; 8 MiB leaves generous slack for the JSON envelope and
// larger caller-configured max_file_bytes.
const maxLineBytes = 8 * 1024 * 1024

func main() {
	if err := run(context.Background(), os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "codesignal-report: %v\n", err)
		os.Exit(1)
	}
}

// scopeHeader is the JSON shape of the required first non-blank stdin line.
type scopeHeader struct {
	Repository string `json:"repository"`
	Revision   string `json:"revision"`
	Base       string `json:"base"`
}

// lineRangeRequest is the JSON shape of one changed_ranges entry.
type lineRangeRequest struct {
	StartRow uint `json:"start_row"`
	EndRow   uint `json:"end_row"`
}

// fileRequest is the JSON shape of one file-request stdin line.
// HeadContent/BaseContent use *string so a present-but-empty key is
// distinguishable from an absent key, since ChangeStatus derivation depends
// on presence, not decoded-content truthiness.
type fileRequest struct {
	Path          string             `json:"path"`
	Language      string             `json:"language"`
	HeadContent   *string            `json:"head_content"`
	BaseContent   *string            `json:"base_content"`
	ChangedRanges []lineRangeRequest `json:"changed_ranges"`
}

// run reads the whole in stream, accumulates one codesignal.Input, calls
// Build exactly once at EOF, and writes exactly one JSON Report to out. The
// only errors run returns are adapter-level I/O failures or ctx
// cancellation; malformed input produces diagnostics in the final Report
// instead.
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

	var files []codesignal.FileChange
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return err
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
		return fmt.Errorf("read stdin: %w", err)
	}

	report, err := builder.Build(ctx, codesignal.Input{Scope: scope, Files: files, Diagnostics: diagnostics})
	if err != nil {
		return err
	}

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

// readScopeHeader scans forward to the first non-blank line (if any) and
// parses it as a scopeHeader. A missing or malformed header line yields a
// zero-valued Scope plus a "malformed_scope_header" diagnostic; the
// offending line is consumed either way and never reprocessed as a
// file-request line.
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

// processFileRequestLine parses one file-request line and, if well-formed,
// derives its ChangeStatus and decodes/analyzes any present content. A line
// that fails to unmarshal or is missing path/language yields a single
// "malformed_file_request" diagnostic and a nil FileChange (the line is
// dropped, not appended to Input.Files). A well-formed line always yields a
// non-nil FileChange, even if content decoding or analysis failed for one
// or both of head/base -- those failures are reported as their own
// diagnostics instead.
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

// deriveChangeStatus maps presence of head/base content pointers to a
// ChangeStatus, independent of whether decoding or analysis later succeeds.
func deriveChangeStatus(head, base *string) codesignal.ChangeStatus {
	switch {
	case head != nil && base != nil:
		return "modified"
	case head != nil:
		return "added"
	case base != nil:
		return "removed"
	default:
		return "unknown"
	}
}

// convertChangedRanges converts the wire lineRangeRequest shape to
// codesignal.LineRange without validating StartRow <= EndRow --
// codesignal.Build already emits "invalid_changed_range" diagnostics for
// that.
func convertChangedRanges(ranges []lineRangeRequest) []codesignal.LineRange {
	if len(ranges) == 0 {
		return nil
	}
	converted := make([]codesignal.LineRange, len(ranges))
	for i, r := range ranges {
		converted[i] = codesignal.LineRange{StartRow: r.StartRow, EndRow: r.EndRow}
	}
	return converted
}

// decodeAndAnalyze base64-decodes encoded and, on success, runs it through
// analyzer. A decode failure yields an "invalid_content_encoding"
// diagnostic and a nil Result. An analysis failure that is not a syntax
// error yields an "analysis_failed" diagnostic and a nil Result. A syntax
// error is not itself a failure: AnalyzeBytes's partial Result is returned
// with no diagnostic, matching the codesignal.Build contract that a
// "syntax_errors" ParseStatus is handled downstream.
func decodeAndAnalyze(ctx context.Context, analyzer *semantics.Analyzer, path, language, encoded string) (*codesignal.Diagnostic, *semantics.Result) {
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return &codesignal.Diagnostic{
			Path:    path,
			Kind:    "invalid_content_encoding",
			Message: fmt.Sprintf("invalid base64 content: %v", err),
		}, nil
	}

	result, err := analyzer.AnalyzeBytes(ctx, semantics.FileInput{
		Path:     path,
		Language: semantics.Language(language),
		Content:  decoded,
	})
	if err != nil && !errors.Is(err, semantics.ErrSyntax) {
		return &codesignal.Diagnostic{
			Path:    path,
			Kind:    "analysis_failed",
			Message: err.Error(),
		}, nil
	}
	return nil, result
}
