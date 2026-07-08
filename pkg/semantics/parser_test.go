package semantics

import (
	"context"
	"errors"
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// parseAndDetectSyntax is test-only scaffolding retained from Task 3's
// original pipeline design. It has no production caller: AnalyzeBytes (in
// analyzer.go) is the sole production implementation of the
// parse-then-detect-syntax-errors contract it exercises below, and the two
// have since drifted (AnalyzeBytes's Result also carries Path). It is kept
// here, rather than deleted, because it isolates that contract at the
// syntaxParser level -- parses content as lang, walks the resulting tree for
// ERROR/MISSING nodes (not via S-expression queries -- that mode is out of
// scope for v1) -- one level below AnalyzeBytes's own inline copy, which the
// tests below exercise directly.
func (sp *syntaxParser) parseAndDetectSyntax(ctx context.Context, content []byte, lang Language) (*Result, error) {
	tree, err := sp.parse(ctx, content, lang)
	if err != nil {
		return nil, err
	}
	defer tree.Close()

	root := tree.RootNode()
	if !root.HasError() {
		return &Result{Language: lang, ParseStatus: ParseStatus("ok")}, nil
	}

	issues := collectSyntaxIssues(root)
	result := &Result{
		Language:     lang,
		ParseStatus:  ParseStatus("syntax_errors"),
		SyntaxErrors: issues,
	}
	return result, &SyntaxError{Issues: issues}
}

// AC-1.8: if the supplied context is already cancelled, validate must report
// (nil, ctx.Err()) before doing any parsing work.
func TestValidate_RejectsAlreadyCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := validate(ctx, []byte("package main\n"), LanguageGo, 0, nil)

	if result != nil {
		t.Errorf("AC-1.8: validate with a cancelled context: got result %+v, want nil", result)
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("AC-1.8: validate with a cancelled context: got err %v, want errors.Is(err, context.Canceled)", err)
	}
}

// AC-1.4: empty content must be rejected with an error matching
// ErrEmptyContent.
func TestValidate_RejectsEmptyContent(t *testing.T) {
	result, err := validate(context.Background(), []byte{}, LanguageGo, 0, nil)

	if result != nil {
		t.Errorf("AC-1.4: validate with empty content: got result %+v, want nil", result)
	}
	if !errors.Is(err, ErrEmptyContent) {
		t.Errorf("AC-1.4: validate with empty content: got err %v, want errors.Is(err, ErrEmptyContent)", err)
	}
}

// AC-1.5: any language other than LanguageGo must be rejected with an error
// matching ErrUnsupportedLanguage.
func TestValidate_RejectsUnsupportedLanguage(t *testing.T) {
	result, err := validate(context.Background(), []byte("package main\n"), Language("python"), 0, nil)

	if result != nil {
		t.Errorf("AC-1.5: validate with unsupported language: got result %+v, want nil", result)
	}
	if !errors.Is(err, ErrUnsupportedLanguage) {
		t.Errorf("AC-1.5: validate with unsupported language: got err %v, want errors.Is(err, ErrUnsupportedLanguage)", err)
	}
}

// Regression guard raised by review: language-support rejection (both "not
// registered at all" and "registered but outside this Analyzer's configured
// subset") must take precedence over the size check. Previously, the
// configured-subset check ran in AnalyzeBytes only after validate had
// already returned success, so an oversized file in a registered-but-
// unconfigured language misreported ErrFileTooLarge instead of
// ErrUnsupportedLanguage. allowed here excludes LanguageGo even though it is
// registered, so this locks in that validate itself -- not a caller running
// a second check afterward -- is the single place deciding both conditions,
// and does so before the size check.
func TestValidate_UnconfiguredLanguageTakesPrecedenceOverOversizedContent(t *testing.T) {
	const maxFileBytes = 10
	content := []byte("this is more than ten bytes")
	allowed := map[Language]bool{"other-lang": true}

	result, err := validate(context.Background(), content, LanguageGo, maxFileBytes, allowed)

	if result != nil {
		t.Errorf("validate with oversized content in an unconfigured language: got result %+v, want nil", result)
	}
	if !errors.Is(err, ErrUnsupportedLanguage) {
		t.Errorf("validate with oversized content in an unconfigured language: got err %v, want errors.Is(err, ErrUnsupportedLanguage)", err)
	}
	if errors.Is(err, ErrFileTooLarge) {
		t.Errorf("validate with oversized content in an unconfigured language: got err %v, want it NOT to match ErrFileTooLarge", err)
	}
}

// AC-1.6: content exceeding the configured max size must be rejected with an
// error matching ErrFileTooLarge. Uses a small explicit max (10 bytes)
// rather than the real 2 MiB default so the test fixture stays tiny.
func TestValidate_RejectsContentOverMaxFileBytes(t *testing.T) {
	const maxFileBytes = 10
	content := []byte("this is more than ten bytes")

	result, err := validate(context.Background(), content, LanguageGo, maxFileBytes, nil)

	if result != nil {
		t.Errorf("AC-1.6: validate with content over max size: got result %+v, want nil", result)
	}
	if !errors.Is(err, ErrFileTooLarge) {
		t.Errorf("AC-1.6: validate with %d-byte content and max %d: got err %v, want errors.Is(err, ErrFileTooLarge)", len(content), maxFileBytes, err)
	}
}

// AC-1.7: content containing a NUL byte must be rejected with an error
// matching ErrBinaryContent.
func TestValidate_RejectsContentContainingNULByte(t *testing.T) {
	content := []byte("package main\x00\n")

	result, err := validate(context.Background(), content, LanguageGo, 0, nil)

	if result != nil {
		t.Errorf("AC-1.7: validate with a NUL byte in content: got result %+v, want nil", result)
	}
	if !errors.Is(err, ErrBinaryContent) {
		t.Errorf("AC-1.7: validate with a NUL byte in content: got err %v, want errors.Is(err, ErrBinaryContent)", err)
	}
}

// Regression guard raised by review: parse's doc comment claimed a
// cancelled context returns an error matching ErrParseFailure, but the
// implementation returns ctx.Err() directly, matching validate's own
// cancellation check. This pins down the actual behavior with a direct
// test rather than leaving it only indirectly covered through validate and
// AnalyzeBytes.
func TestParse_ReturnsContextErrDirectlyOnAlreadyCancelledContext(t *testing.T) {
	sp := newSyntaxParser()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	tree, err := sp.parse(ctx, []byte("package main\n"), LanguageGo)

	if tree != nil {
		t.Errorf("parse with an already-cancelled context: got non-nil tree, want nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("parse with an already-cancelled context: got err %v, want errors.Is(err, context.Canceled)", err)
	}
	if errors.Is(err, ErrParseFailure) {
		t.Errorf("parse with an already-cancelled context: got errors.Is(err, ErrParseFailure) = true, want false (cancellation is not a parse failure)")
	}
}

// Precondition for the syntax-detection tests below: parsing valid Go
// source through our own wrapper must produce a clean tree (no ERROR or
// MISSING nodes), the same as the raw Tree-sitter smoke test already shows.
func TestParse_ReturnsCleanTreeForValidSource(t *testing.T) {
	sp := newSyntaxParser()
	source := []byte("package main\nfunc main() {}\n")

	tree, err := sp.parse(context.Background(), source, LanguageGo)
	if err != nil {
		t.Fatalf("parse of valid source %q: got err %v, want nil", source, err)
	}
	if tree == nil {
		t.Fatalf("parse of valid source %q: got nil tree, want a non-nil tree", source)
	}
	defer tree.Close()

	if tree.RootNode().HasError() {
		t.Errorf("parse of valid source %q: RootNode().HasError() = true, want false", source)
	}
}

// AC-6.2: if the underlying Tree-sitter Parse call returns a nil tree, parse
// must report an error matching ErrParseFailure and must not dereference or
// Close the nil tree. A real nil tree isn't reachable through normal
// Parser.Parse calls with valid parser/content, so this test injects a
// forced-nil parseFunc via the syntaxParser seam built for this purpose.
func TestParse_ReturnsParseFailureErrorOnNilTreeWithoutDereferencing(t *testing.T) {
	sp := newSyntaxParser()
	sp.parseFunc = func(p engine.Parser, content []byte) (engine.Tree, error) {
		return nil, nil
	}

	tree, err := sp.parse(context.Background(), []byte("package main\n"), LanguageGo)

	if tree != nil {
		t.Fatalf("parse when the underlying Parse call returns nil: got non-nil tree %+v, want nil", tree)
	}
	if !errors.Is(err, ErrParseFailure) {
		t.Errorf("parse when the underlying Parse call returns nil: got err %v, want errors.Is(err, ErrParseFailure)", err)
	}
}

// AC-2.1, AC-2.2, AC-2.4: when the parse tree has any ERROR/MISSING node,
// parseAndDetectSyntax must skip import extraction/metrics/findings (not
// implemented until Tasks 4/5) and return a partial *Result with
// ParseStatus "syntax_errors", SyntaxErrors populated via tree traversal
// (not S-expression queries), and Imports/Findings/Metrics left
// zero-valued.
func TestSyntaxDetection_ErrorNodeYieldsPartialResultWithSyntaxErrorsStatus(t *testing.T) {
	sp := newSyntaxParser()
	source := []byte("package main\nfunc {")

	result, _ := sp.parseAndDetectSyntax(context.Background(), source, LanguageGo)

	if result == nil {
		t.Fatalf("parseAndDetectSyntax for source with a syntax error %q: got nil result, want a partial *Result", source)
	}
	if result.ParseStatus != ParseStatus("syntax_errors") {
		t.Errorf("parseAndDetectSyntax for source with a syntax error %q: ParseStatus = %q, want %q", source, result.ParseStatus, "syntax_errors")
	}
	if len(result.SyntaxErrors) == 0 {
		t.Fatalf("parseAndDetectSyntax for source with a syntax error %q: SyntaxErrors is empty, want at least one issue", source)
	}
	foundErrorKind := false
	for _, issue := range result.SyntaxErrors {
		if issue.Kind == "error" {
			foundErrorKind = true
		}
	}
	if !foundErrorKind {
		t.Errorf("parseAndDetectSyntax for source with a syntax error %q: SyntaxErrors = %+v, want at least one issue with Kind == %q", source, result.SyntaxErrors, "error")
	}
	if len(result.Imports) != 0 {
		t.Errorf("parseAndDetectSyntax for source with a syntax error %q: Imports = %+v, want empty (extraction is out of scope for this task)", source, result.Imports)
	}
	if len(result.Findings) != 0 {
		t.Errorf("parseAndDetectSyntax for source with a syntax error %q: Findings = %+v, want empty (extraction is out of scope for this task)", source, result.Findings)
	}
	if result.Metrics != (StructuralMetrics{}) {
		t.Errorf("parseAndDetectSyntax for source with a syntax error %q: Metrics = %+v, want the zero value (extraction is out of scope for this task)", source, result.Metrics)
	}
}

// Supporting test (no AC number): parseAndDetectSyntax must report
// ParseStatus "ok" and a nil error for source with no syntax errors. This
// justifies the "clean tree" branch alongside the "syntax_errors" branch
// exercised by AC-2.1/2.2/2.4, so the function is a coherent pipeline stage
// rather than one that only handles the error path.
func TestParseAndDetectSyntax_ReturnsOkStatusForCleanSource(t *testing.T) {
	sp := newSyntaxParser()
	source := []byte("package main\nfunc main() {}\n")

	result, err := sp.parseAndDetectSyntax(context.Background(), source, LanguageGo)

	if err != nil {
		t.Fatalf("parseAndDetectSyntax for clean source %q: got err %v, want nil", source, err)
	}
	if result == nil {
		t.Fatalf("parseAndDetectSyntax for clean source %q: got nil result, want a non-nil *Result", source)
	}
	if result.ParseStatus != ParseStatus("ok") {
		t.Errorf("parseAndDetectSyntax for clean source %q: ParseStatus = %q, want %q", source, result.ParseStatus, "ok")
	}
	if len(result.SyntaxErrors) != 0 {
		t.Errorf("parseAndDetectSyntax for clean source %q: SyntaxErrors = %+v, want empty", source, result.SyntaxErrors)
	}
}

// AC-2.3: when syntax errors are detected, parseAndDetectSyntax must also
// return a non-nil error for which errors.Is(err, ErrSyntax) is true and
// errors.As(err, &target) succeeds for target *SyntaxError, carrying the
// same issues as the result's SyntaxErrors.
func TestSyntaxDetection_ReturnsErrorMatchingIsErrSyntaxAndAsSyntaxError(t *testing.T) {
	sp := newSyntaxParser()
	source := []byte("package main\nfunc {")

	result, err := sp.parseAndDetectSyntax(context.Background(), source, LanguageGo)
	if result == nil {
		t.Fatalf("parseAndDetectSyntax for source with a syntax error %q: got nil result, want a partial *Result", source)
	}

	if !errors.Is(err, ErrSyntax) {
		t.Errorf("parseAndDetectSyntax for source with a syntax error %q: errors.Is(err, ErrSyntax) = false, want true (err = %v)", source, err)
	}

	var syntaxErr *SyntaxError
	if !errors.As(err, &syntaxErr) {
		t.Fatalf("parseAndDetectSyntax for source with a syntax error %q: errors.As(err, &SyntaxError{}) = false, want true (err = %v)", source, err)
	}

	if len(syntaxErr.Issues) != len(result.SyntaxErrors) {
		t.Fatalf("parseAndDetectSyntax for source with a syntax error %q: SyntaxError.Issues length = %d, want %d to match result.SyntaxErrors", source, len(syntaxErr.Issues), len(result.SyntaxErrors))
	}
	for i, want := range result.SyntaxErrors {
		if syntaxErr.Issues[i] != want {
			t.Errorf("parseAndDetectSyntax for source with a syntax error %q: SyntaxError.Issues[%d] = %+v, want %+v (must match result.SyntaxErrors)", source, i, syntaxErr.Issues[i], want)
		}
	}
}

// AC-2.5: a zero-width MISSING node (Tree-sitter's error recovery inserting
// a virtual token, e.g. the missing ")" that closes an unterminated call
// argument list) must report Location.StartByte == Location.EndByte, with
// no error. This exercises real Tree-sitter output -- confirmed by
// experiment (see task notes) that "g(1, 2" with no closing paren produces
// exactly one MISSING ")" node at a single byte offset, no ERROR node.
func TestSyntaxDetection_MissingNodeReportsZeroWidthLocation(t *testing.T) {
	sp := newSyntaxParser()
	source := []byte("package main\nfunc f() {\n\tg(1, 2\n}\n")

	result, err := sp.parseAndDetectSyntax(context.Background(), source, LanguageGo)
	if result == nil {
		t.Fatalf("parseAndDetectSyntax for source with an unclosed call %q: got nil result, want a partial *Result", source)
	}
	if err == nil {
		t.Fatalf("parseAndDetectSyntax for source with an unclosed call %q: got nil err, want a non-nil syntax error", source)
	}

	var missing *SyntaxIssue
	for i := range result.SyntaxErrors {
		if result.SyntaxErrors[i].Kind == "missing" {
			missing = &result.SyntaxErrors[i]
			break
		}
	}
	if missing == nil {
		t.Fatalf("parseAndDetectSyntax for source with an unclosed call %q: SyntaxErrors = %+v, want at least one issue with Kind == %q", source, result.SyntaxErrors, "missing")
	}

	if missing.Location.StartByte != missing.Location.EndByte {
		t.Errorf("parseAndDetectSyntax for source with an unclosed call %q: MISSING node Location = %+v, want StartByte == EndByte (zero-width)", source, missing.Location)
	}
}
