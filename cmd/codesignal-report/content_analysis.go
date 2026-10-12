package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

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
