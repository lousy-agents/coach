package semantics

import (
	"errors"
	"fmt"
)

// Sentinel errors returned by AnalyzeBytes and related entry points.
var (
	// ErrEmptyContent indicates the input source had no bytes.
	ErrEmptyContent = errors.New("semantics: empty content")
	// ErrUnsupportedLanguage indicates the requested Language is not
	// supported in v1.
	ErrUnsupportedLanguage = errors.New("semantics: unsupported language")
	// ErrSyntax indicates the parse tree contained one or more syntax
	// errors. Wrapped by *SyntaxError, which carries the specific issues.
	ErrSyntax = errors.New("semantics: syntax error")
	// ErrFileTooLarge indicates the input source exceeded the configured
	// size limit.
	ErrFileTooLarge = errors.New("semantics: file too large")
	// ErrBinaryContent indicates the input source appears to be binary
	// rather than text.
	ErrBinaryContent = errors.New("semantics: binary content")
	// ErrParseFailure indicates Tree-sitter failed to produce a parse tree
	// at all.
	ErrParseFailure = errors.New("semantics: parse failure")
)

// SyntaxError reports the syntax issues found while parsing a file. It
// wraps ErrSyntax so callers can use errors.Is(err, ErrSyntax) without
// needing to know the concrete type, or errors.As to recover the specific
// Issues.
type SyntaxError struct {
	Issues []SyntaxIssue
}

// Error summarizes the number of syntax issues found.
func (e *SyntaxError) Error() string {
	return fmt.Sprintf("semantics: %d syntax issue(s)", len(e.Issues))
}

// Is reports whether target is ErrSyntax, so errors.Is(err, ErrSyntax)
// matches any *SyntaxError.
func (e *SyntaxError) Is(target error) bool {
	return target == ErrSyntax
}
