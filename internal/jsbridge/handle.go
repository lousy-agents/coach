package jsbridge

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/lousy-agents/coach/pkg/semantics"
)

// Handle runs one Request through pkg/semantics and returns its Response.
// It is the single code path shared by every transport backend. It never
// returns an error: every failure is encoded as Response.Error so the wire
// contract stays uniform.
func Handle(ctx context.Context, req Request) Response {
	resp := Response{ID: req.ID}

	if req.Op != OpAnalyze {
		resp.Error = &ErrorPayload{
			Kind:    KindInternal,
			Message: fmt.Sprintf("jsbridge: unknown op %q (protocol v1 supports only %q)", req.Op, OpAnalyze),
		}
		return resp
	}

	content, err := base64.StdEncoding.DecodeString(req.ContentB64)
	if err != nil {
		resp.Error = &ErrorPayload{
			Kind:    KindInternal,
			Message: fmt.Sprintf("jsbridge: content_b64 is not valid base64: %v", err),
		}
		return resp
	}

	if req.TimeoutMS > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(req.TimeoutMS)*time.Millisecond)
		defer cancel()
	}

	languages := make([]semantics.Language, 0, len(req.Options.Languages))
	for _, lang := range req.Options.Languages {
		languages = append(languages, semantics.Language(lang))
	}
	analyzer, err := semantics.NewAnalyzer(semantics.AnalyzerOptions{
		Languages:    languages,
		MaxFileBytes: req.Options.MaxFileBytes,
	})
	if err != nil {
		kind := errorKind(err)
		if kind == KindInternal {
			// NewAnalyzer's only non-sentinel failure is option validation
			// (negative MaxFileBytes).
			kind = KindInvalidOptions
		}
		resp.Error = &ErrorPayload{Kind: kind, Message: err.Error()}
		return resp
	}

	result, err := analyzer.AnalyzeBytes(ctx, semantics.FileInput{
		Path:     req.Path,
		Language: semantics.Language(req.Language),
		Content:  content,
	})
	// On syntax errors AnalyzeBytes returns both a partial *Result and a
	// *SyntaxError; carry both so JS sees the same double return.
	resp.Result = result
	if err != nil {
		resp.Error = &ErrorPayload{Kind: errorKind(err), Message: err.Error()}
	}
	return resp
}

// errorKind maps an error from pkg/semantics (or the context package) to its
// wire kind, most specific first.
func errorKind(err error) string {
	switch {
	case errors.Is(err, semantics.ErrSyntax):
		return KindSyntax
	case errors.Is(err, semantics.ErrEmptyContent):
		return KindEmptyContent
	case errors.Is(err, semantics.ErrUnsupportedLanguage):
		return KindUnsupportedLanguage
	case errors.Is(err, semantics.ErrFileTooLarge):
		return KindFileTooLarge
	case errors.Is(err, semantics.ErrBinaryContent):
		return KindBinaryContent
	case errors.Is(err, semantics.ErrParseFailure):
		return KindParseFailure
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return KindCanceled
	default:
		return KindInternal
	}
}
