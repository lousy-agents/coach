package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

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
