package semantics

import (
	"context"
	"testing"
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
