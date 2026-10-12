package semantics

import (
	"bytes"
	"context"
	"fmt"
)

// defaultMaxFileBytes is the content size limit applied when the caller
// specifies maxFileBytes <= 0 (e.g. the zero value).
const defaultMaxFileBytes = 2 * 1024 * 1024 // 2 MiB

// validate applies precondition checks to content before parsing, in the
// order: context cancellation, emptiness, language support, size limit, then
// binary (NUL byte) detection. The *Result return is always nil; it exists
// so callers (e.g. the AnalyzeBytes facade) can propagate (result, err)
// uniformly without a separate branch for validation failures.
//
// Language support is decided in exactly one place and one step: lang must
// both be registered in languageRegistry (the same registry NewAnalyzer
// validates AnalyzerOptions.Languages against) and, when allowed is
// non-empty, be a member of allowed (an Analyzer's configured language
// subset). allowed being empty means "any registered language is allowed."
// Deciding both conditions before the size check -- rather than checking the
// configured subset separately after validate returns -- prevents an
// oversized file in an unconfigured language from misreporting
// ErrFileTooLarge instead of ErrUnsupportedLanguage.
func validate(ctx context.Context, content []byte, lang Language, maxFileBytes int, allowed map[Language]bool) (*Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(content) == 0 {
		return nil, ErrEmptyContent
	}
	if _, ok := languageRegistry[lang]; !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedLanguage, lang)
	}
	if len(allowed) > 0 && !allowed[lang] {
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedLanguage, lang)
	}
	max := maxFileBytes
	if max <= 0 {
		max = defaultMaxFileBytes
	}
	if len(content) > max {
		return nil, fmt.Errorf("%w: content is %d bytes, exceeds max %d bytes", ErrFileTooLarge, len(content), max)
	}
	if bytes.IndexByte(content, 0x00) != -1 {
		return nil, ErrBinaryContent
	}
	return nil, nil
}
