package semantics

import (
	"context"

	"errors"

	"testing"
)

// AC-1.10: every slice in a Result (SyntaxErrors, Imports, Findings) must be
// ordered by Location.StartByte ascending. This fixture produces two
// imports (out of alphabetical order in the source) and two findings (a
// constructor func and a pointer-returning func, also out of name order),
// so an unordered assembly step would be caught.
func TestAnalyzeBytes_OrdersEverySliceByStartByteAscending(t *testing.T) {
	a := mustNewAnalyzer(t)
	source := []byte(`package main

import (
	"os"
	"fmt"
)

func NewZeta() *int {
	fmt.Println(os.Args)
	return nil
}

func NewAlpha() *int {
	return nil
}
`)

	result, err := a.AnalyzeBytes(context.Background(), FileInput{
		Path:     "main.go",
		Language: LanguageGo,
		Content:  source,
	})
	if err != nil {
		t.Fatalf("AnalyzeBytes for %q: got err %v, want nil", source, err)
	}

	if len(result.Imports) < 2 {
		t.Fatalf("AnalyzeBytes for %q: Imports = %+v, want at least 2 to exercise ordering", source, result.Imports)
	}
	if len(result.Findings) < 2 {
		t.Fatalf("AnalyzeBytes for %q: Findings = %+v, want at least 2 to exercise ordering", source, result.Findings)
	}

	assertAscendingByStartByte(t, "SyntaxErrors", len(result.SyntaxErrors), func(i int) uint { return result.SyntaxErrors[i].Location.StartByte })
	assertAscendingByStartByte(t, "Imports", len(result.Imports), func(i int) uint { return result.Imports[i].Location.StartByte })
	assertAscendingByStartByte(t, "Findings", len(result.Findings), func(i int) uint { return result.Findings[i].Location.StartByte })
}

// assertAscendingByStartByte fails the test if startByte(i) is ever less
// than startByte(i-1) for i in [1, n).
func assertAscendingByStartByte(t *testing.T, sliceName string, n int, startByte func(i int) uint) {
	t.Helper()

	for i := 1; i < n; i++ {
		if startByte(i) < startByte(i-1) {
			t.Errorf("AC-1.10: %s must be ordered by Location.StartByte ascending: element %d (StartByte=%d) precedes element %d (StartByte=%d)", sliceName, i, startByte(i), i-1, startByte(i-1))
		}
	}
}

// AC-R2.4: a Language not in languageRegistry (e.g. "javascript", which is
// explicitly out of scope) must return ErrUnsupportedLanguage and a nil
// *Result, unchanged from the pre-existing contract.
func TestAnalyzeBytes_RejectsJavaScriptAsUnsupported(t *testing.T) {
	a := mustNewAnalyzer(t)

	result, err := a.AnalyzeBytes(context.Background(), FileInput{
		Language: "javascript",
		Content:  []byte(`const x = 1;`),
	})

	if result != nil {
		t.Errorf("AnalyzeBytes with Language \"javascript\": got non-nil result %+v, want nil", result)
	}
	if !errors.Is(err, ErrUnsupportedLanguage) {
		t.Errorf("AnalyzeBytes with Language \"javascript\": got err %v, want errors.Is(err, ErrUnsupportedLanguage)", err)
	}
}

// Regression guard raised by review: TestAnalyzeBytes_OrdersEverySliceByStartByteAscending
// only exercises a clean parse, so result.SyntaxErrors is always empty and
// its ordering assertion is vacuously true (the loop body never runs for an
// empty slice). This drives a fixture with multiple real syntax issues so
// AC-1.10 is actually exercised for SyntaxErrors.
func TestAnalyzeBytes_OrdersSyntaxErrorsByStartByteAscendingWithMultipleIssues(t *testing.T) {
	a := mustNewAnalyzer(t)

	source := []byte("package main\nfunc f() {\nfunc g() {\n")

	result, err := a.AnalyzeBytes(context.Background(), FileInput{
		Path:     "main.go",
		Language: LanguageGo,
		Content:  source,
	})
	if !errors.Is(err, ErrSyntax) {
		t.Fatalf("AnalyzeBytes for %q: got err %v, want errors.Is(err, ErrSyntax) to hold", source, err)
	}
	if len(result.SyntaxErrors) < 2 {
		t.Fatalf("AnalyzeBytes for %q: SyntaxErrors = %+v, want at least 2 to exercise ordering", source, result.SyntaxErrors)
	}

	assertAscendingByStartByte(t, "SyntaxErrors", len(result.SyntaxErrors), func(i int) uint { return result.SyntaxErrors[i].Location.StartByte })
}

// mustNewAnalyzer builds an Analyzer with default options for tests that
// don't care about AnalyzerOptions specifics.
func mustNewAnalyzer(t *testing.T) *Analyzer {
	t.Helper()

	a, err := NewAnalyzer(AnalyzerOptions{})
	if err != nil {
		t.Fatalf("NewAnalyzer(AnalyzerOptions{}): got err %v, want nil", err)
	}
	return a
}

// Constructor validation (supports AC-1.5 at the boundary): NewAnalyzer must
// reject any Languages entry that isn't a recognized Language, since
// AnalyzeBytes has no other opportunity to validate the configured set up
// front.
func TestNewAnalyzer_RejectsUnknownLanguageNames(t *testing.T) {
	_, err := NewAnalyzer(AnalyzerOptions{Languages: []Language{"python"}})

	if err == nil {
		t.Fatalf("NewAnalyzer with Languages containing an unrecognized language %q: got nil error, want non-nil", "python")
	}
}
