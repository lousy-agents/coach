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
