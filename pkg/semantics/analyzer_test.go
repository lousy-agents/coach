package semantics

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
)

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

// thenResultIsNil fails the test if result is non-nil.
func thenResultIsNil(t *testing.T, result *Result, why string) {
	t.Helper()
	if result != nil {
		t.Errorf("%s: got result %+v, want nil", why, result)
	}
}

// thenErrorIs fails the test unless errors.Is(err, target) holds.
func thenErrorIs(t *testing.T, err, target error, why string) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Errorf("%s: got err %v, want errors.Is(err, %v)", why, err, target)
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
