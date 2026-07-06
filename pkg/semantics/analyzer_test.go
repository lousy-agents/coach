package semantics

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sync"
	"testing"
)

// errAfterNCallsContext wraps a context.Context, overriding only Err() so it
// reports nil for the first n calls and context.Canceled thereafter. This
// lets a test isolate a specific ctx.Err() check in a multi-stage pipeline
// (here: the check between parse and feature extraction in AnalyzeBytes)
// from earlier checks (inside validate, then inside syntaxParser.parse)
// without those earlier stages seeing a cancelled context first.
type errAfterNCallsContext struct {
	context.Context
	n     int
	calls int
}

func (c *errAfterNCallsContext) Err() error {
	c.calls++
	if c.calls > c.n {
		return context.Canceled
	}
	return nil
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

// Constructor validation: NewAnalyzer must reject a negative MaxFileBytes,
// since a negative size limit is nonsensical (per the frozen API doc: "0 =
// default 2 MiB; negative = NewAnalyzer returns an error").
func TestNewAnalyzer_RejectsNegativeMaxFileBytes(t *testing.T) {
	_, err := NewAnalyzer(AnalyzerOptions{MaxFileBytes: -1})

	if err == nil {
		t.Fatalf("NewAnalyzer with MaxFileBytes = -1: got nil error, want non-nil")
	}
}

// AC-1.2: AnalyzeBytes on valid Go source must return a Result with
// ParseStatus "ok" and a nil error.
func TestAnalyzeBytes_ReturnsOkResultForValidSource(t *testing.T) {
	a := mustNewAnalyzer(t)
	source := []byte("package main\nfunc main() {}\n")

	result, err := a.AnalyzeBytes(context.Background(), FileInput{
		Path:     "main.go",
		Language: LanguageGo,
		Content:  source,
	})

	if err != nil {
		t.Fatalf("AnalyzeBytes for valid source %q: got err %v, want nil", source, err)
	}
	if result == nil {
		t.Fatalf("AnalyzeBytes for valid source %q: got nil result, want non-nil", source)
	}
	if result.ParseStatus != ParseStatus("ok") {
		t.Errorf("AnalyzeBytes for valid source %q: ParseStatus = %q, want %q", source, result.ParseStatus, "ok")
	}
}

// AC-1.3: calling AnalyzeBytes twice with identical input must produce
// byte-identical JSON output, since the pipeline has no hidden
// non-deterministic state (map iteration, timestamps, pointers, etc).
func TestAnalyzeBytes_IsByteIdenticalAcrossRepeatedCalls(t *testing.T) {
	a := mustNewAnalyzer(t)
	in := FileInput{
		Path:     "main.go",
		Language: LanguageGo,
		Content: []byte(`package main

import (
	"fmt"
	"os"
)

func NewFoo() *int {
	if true {
		fmt.Println(os.Args)
	}
	return nil
}
`),
	}

	first, err := a.AnalyzeBytes(context.Background(), in)
	if err != nil {
		t.Fatalf("first AnalyzeBytes call: got err %v, want nil", err)
	}
	second, err := a.AnalyzeBytes(context.Background(), in)
	if err != nil {
		t.Fatalf("second AnalyzeBytes call: got err %v, want nil", err)
	}

	firstJSON, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("marshaling first result: got err %v, want nil", err)
	}
	secondJSON, err := json.Marshal(second)
	if err != nil {
		t.Fatalf("marshaling second result: got err %v, want nil", err)
	}

	if !bytes.Equal(firstJSON, secondJSON) {
		t.Errorf("AC-1.3: repeated AnalyzeBytes calls on identical input must be byte-identical:\nfirst:  %s\nsecond: %s", firstJSON, secondJSON)
	}
}

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

// AC-1.9: a single *Analyzer must be safe for concurrent callers, since it
// holds no C-backed resources between calls (each AnalyzeBytes call creates
// and closes its own Parser/Tree/Query/QueryCursor). Two goroutines call
// AnalyzeBytes on the same *Analyzer concurrently and both results are
// collected via a sync.WaitGroup (no time.Sleep). Run with -race to confirm
// no data race, and assert both calls succeed with ParseStatus "ok".
func TestAnalyzeBytes_SafeForConcurrentCallers(t *testing.T) {
	a := mustNewAnalyzer(t)
	source := []byte("package main\nfunc main() {}\n")

	const goroutines = 2
	results := make([]*Result, goroutines)
	errs := make([]error, goroutines)

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = a.AnalyzeBytes(context.Background(), FileInput{
				Path:     "main.go",
				Language: LanguageGo,
				Content:  source,
			})
		}(i)
	}
	wg.Wait()

	for i := 0; i < goroutines; i++ {
		if errs[i] != nil {
			t.Errorf("AC-1.9: concurrent AnalyzeBytes call %d: got err %v, want nil", i, errs[i])
		}
		if results[i] == nil {
			t.Fatalf("AC-1.9: concurrent AnalyzeBytes call %d: got nil result, want non-nil", i)
		}
		if results[i].ParseStatus != ParseStatus("ok") {
			t.Errorf("AC-1.9: concurrent AnalyzeBytes call %d: ParseStatus = %q, want %q", i, results[i].ParseStatus, "ok")
		}
	}
}

// AC-1.8 (mid-pipeline): AnalyzeBytes must re-check ctx.Err() between
// parsing and running import/feature extraction, not just at validate's
// entry check. The wrapped context here reports nil for its first two Err()
// calls (one inside validate, one inside syntaxParser.parse -- both on a
// clean tree for this valid source) and context.Canceled from the third
// call onward, so only AnalyzeBytes's own mid-pipeline check can observe
// the cancellation.
func TestAnalyzeBytes_ChecksCancellationBetweenParseAndFeatureExtraction(t *testing.T) {
	a := mustNewAnalyzer(t)
	ctx := &errAfterNCallsContext{Context: context.Background(), n: 2}
	source := []byte("package main\nfunc main() {}\n")

	result, err := a.AnalyzeBytes(ctx, FileInput{
		Path:     "main.go",
		Language: LanguageGo,
		Content:  source,
	})

	if result != nil {
		t.Errorf("AC-1.8: AnalyzeBytes cancelled between parse and feature extraction: got result %+v, want nil", result)
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("AC-1.8: AnalyzeBytes cancelled between parse and feature extraction: got err %v, want errors.Is(err, context.Canceled)", err)
	}
}

// AC-2.2 / AC-2.3 end-to-end: through the full public AnalyzeBytes call, a
// source with a syntax error must produce a partial Result (ParseStatus
// "syntax_errors", Imports/Findings empty, Metrics zero) alongside an error
// that satisfies errors.Is(err, ErrSyntax) and errors.As(err, &SyntaxError)
// with Issues matching result.SyntaxErrors exactly -- the same contract
// parseAndDetectSyntax already proved at the syntaxParser level (Task 3),
// now surfaced through the public facade.
func TestAnalyzeBytes_EndToEndSyntaxErrorContract(t *testing.T) {
	a := mustNewAnalyzer(t)
	source := []byte("package main\nfunc {")

	result, err := a.AnalyzeBytes(context.Background(), FileInput{
		Path:     "broken.go",
		Language: LanguageGo,
		Content:  source,
	})

	if result == nil {
		t.Fatalf("AnalyzeBytes for source with a syntax error %q: got nil result, want a partial *Result", source)
	}
	if result.ParseStatus != ParseStatus("syntax_errors") {
		t.Errorf("AnalyzeBytes for source with a syntax error %q: ParseStatus = %q, want %q", source, result.ParseStatus, "syntax_errors")
	}
	if len(result.Imports) != 0 {
		t.Errorf("AnalyzeBytes for source with a syntax error %q: Imports = %+v, want empty", source, result.Imports)
	}
	if len(result.Findings) != 0 {
		t.Errorf("AnalyzeBytes for source with a syntax error %q: Findings = %+v, want empty", source, result.Findings)
	}
	if result.Metrics != (StructuralMetrics{}) {
		t.Errorf("AnalyzeBytes for source with a syntax error %q: Metrics = %+v, want the zero value", source, result.Metrics)
	}

	if !errors.Is(err, ErrSyntax) {
		t.Errorf("AnalyzeBytes for source with a syntax error %q: errors.Is(err, ErrSyntax) = false, want true (err = %v)", source, err)
	}

	var syntaxErr *SyntaxError
	if !errors.As(err, &syntaxErr) {
		t.Fatalf("AnalyzeBytes for source with a syntax error %q: errors.As(err, &SyntaxError{}) = false, want true (err = %v)", source, err)
	}
	if len(syntaxErr.Issues) != len(result.SyntaxErrors) {
		t.Fatalf("AnalyzeBytes for source with a syntax error %q: SyntaxError.Issues length = %d, want %d to match result.SyntaxErrors", source, len(syntaxErr.Issues), len(result.SyntaxErrors))
	}
	for i, want := range result.SyntaxErrors {
		if syntaxErr.Issues[i] != want {
			t.Errorf("AnalyzeBytes for source with a syntax error %q: SyntaxError.Issues[%d] = %+v, want %+v (must match result.SyntaxErrors)", source, i, syntaxErr.Issues[i], want)
		}
	}
}

// AC-6.3: Analyzer must hold no C-backed resources between calls (Parser,
// Tree, Query, QueryCursor are all created fresh inside AnalyzeBytes), so it
// must expose no Close method for callers to forget to call.
func TestAnalyzer_HasNoExportedCloseMethod(t *testing.T) {
	_, ok := reflect.TypeOf(&Analyzer{}).MethodByName("Close")

	if ok {
		t.Errorf("AC-6.3: (*Analyzer).Close must not exist, want ok == false, got ok == true")
	}
}

// Regression guard raised by review: AC-1.4 through AC-1.7 were previously
// exercised only against the unexported validate function (parser_test.go),
// never through the public AnalyzeBytes facade that wires validate in as
// its first pipeline step. A regression that stopped calling validate, or
// called it with the wrong arguments, would not have been caught by any
// test. This drives each precondition through AnalyzeBytes itself.
func TestAnalyzeBytes_RejectsInvalidInputThroughPublicFacade(t *testing.T) {
	tests := []struct {
		name    string
		in      FileInput
		wantErr error
	}{
		{
			name:    "AC-1.4: empty content",
			in:      FileInput{Language: LanguageGo, Content: []byte{}},
			wantErr: ErrEmptyContent,
		},
		{
			name:    "AC-1.5: unsupported language",
			in:      FileInput{Language: "python", Content: []byte("package main\n")},
			wantErr: ErrUnsupportedLanguage,
		},
		{
			name:    "AC-1.7: content containing a NUL byte",
			in:      FileInput{Language: LanguageGo, Content: []byte("package main\x00\n")},
			wantErr: ErrBinaryContent,
		},
	}

	a := mustNewAnalyzer(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := a.AnalyzeBytes(context.Background(), tt.in)

			if result != nil {
				t.Errorf("AnalyzeBytes(%+v): got non-nil result %+v, want nil", tt.in, result)
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("AnalyzeBytes(%+v): got err %v, want errors.Is(err, %v) to hold", tt.in, err, tt.wantErr)
			}
		})
	}

	t.Run("AC-1.6: content over MaxFileBytes", func(t *testing.T) {
		small, err := NewAnalyzer(AnalyzerOptions{MaxFileBytes: 4})
		if err != nil {
			t.Fatalf("NewAnalyzer(MaxFileBytes: 4): got err %v, want nil", err)
		}

		in := FileInput{Language: LanguageGo, Content: []byte("package main\n")}
		result, err := small.AnalyzeBytes(context.Background(), in)

		if result != nil {
			t.Errorf("AnalyzeBytes(%+v) with MaxFileBytes=4: got non-nil result %+v, want nil", in, result)
		}
		if !errors.Is(err, ErrFileTooLarge) {
			t.Errorf("AnalyzeBytes(%+v) with MaxFileBytes=4: got err %v, want errors.Is(err, ErrFileTooLarge) to hold", in, err)
		}
	})
}

// Regression guard raised by review: TestAnalyzeBytes_OrdersEverySliceByStartByteAscending
// only exercises a clean parse, so result.SyntaxErrors is always empty and
// its ordering assertion is vacuously true (the loop body never runs for an
// empty slice). This drives a fixture with multiple real syntax issues so
// AC-1.10 is actually exercised for SyntaxErrors.
func TestAnalyzeBytes_OrdersSyntaxErrorsByStartByteAscendingWithMultipleIssues(t *testing.T) {
	a := mustNewAnalyzer(t)
	// Two separate unclosed-brace functions, each producing at least one
	// ERROR/MISSING node, so SyntaxErrors has more than one entry to order.
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
