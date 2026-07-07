// Package jsbridge defines the JSON protocol that exposes pkg/semantics to
// JavaScript/TypeScript consumers, plus the single handler both transport
// backends (WASM and stdio CLI) call. The protocol is stateless: every
// request carries its own analyzer options, and Handle builds a fresh
// Analyzer per call, so the boundary needs no create/destroy lifecycle.
//
// The package lives under internal/ deliberately: the wire format is an
// implementation detail of the JS boundary, not public Go API.
package jsbridge

import "github.com/lousy-agents/coach/pkg/semantics"

// OpAnalyze is the only operation in protocol version 1.
const OpAnalyze = "analyze"

// Request is one JSON request from the JS side.
type Request struct {
	// ID correlates a Response with its Request; it is echoed verbatim.
	ID int64 `json:"id"`
	// Op selects the operation; only OpAnalyze exists in v1.
	Op string `json:"op"`
	// Path is opaque metadata forwarded to semantics.FileInput.Path.
	Path string `json:"path,omitempty"`
	// Language is the semantics.Language value ("go", "typescript", "tsx").
	Language string `json:"language"`
	// ContentB64 is the exact source bytes, standard base64-encoded. Base64
	// (rather than a JSON string) preserves non-UTF-8 and NUL bytes so the
	// analyzer's binary-content and size checks see the true input.
	ContentB64 string `json:"content_b64"`
	// Options carries the semantics.AnalyzerOptions for this request.
	Options Options `json:"options"`
	// TimeoutMS bounds the analysis via context.WithTimeout; 0 means none.
	TimeoutMS int64 `json:"timeout_ms,omitempty"`
}

// Options mirrors semantics.AnalyzerOptions on the wire.
type Options struct {
	Languages    []string `json:"languages,omitempty"`
	MaxFileBytes int      `json:"max_file_bytes,omitempty"`
}

// Response is one JSON response to the JS side. On a syntax-error analysis
// both Result and Error are set, mirroring AnalyzeBytes returning a partial
// *Result alongside a *SyntaxError; the partial Result's syntax_errors field
// is the single source of truth for the issues.
type Response struct {
	ID int64 `json:"id"`
	// Result reuses semantics.Result's frozen snake_case JSON tags directly;
	// the shape is never re-declared here.
	Result *semantics.Result `json:"result,omitempty"`
	Error  *ErrorPayload     `json:"error,omitempty"`
}

// ErrorPayload is the wire form of an analysis error.
type ErrorPayload struct {
	// Kind is one of the Kind* constants; JS switches on it in place of
	// errors.Is, which cannot cross the boundary.
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

// Error kinds, mapping the pkg/semantics sentinel errors (plus bridge-level
// conditions) to strings the JS side can switch on.
const (
	KindSyntax              = "syntax"
	KindEmptyContent        = "empty_content"
	KindUnsupportedLanguage = "unsupported_language"
	KindFileTooLarge        = "file_too_large"
	KindBinaryContent       = "binary_content"
	KindParseFailure        = "parse_failure"
	KindInvalidOptions      = "invalid_options"
	KindCanceled            = "canceled"
	KindInternal            = "internal"
)
