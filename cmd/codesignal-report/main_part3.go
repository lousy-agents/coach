package main

import (
	"bufio"

	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

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
