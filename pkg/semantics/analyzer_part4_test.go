package semantics

import (
	"context"

	"sync"
	"testing"
)

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
			body_analyzerPart4Test_43(t, a, tt)
		})
	}

	t.Run("AC-1.6: content over MaxFileBytes", func(t *testing.T) {
		body_analyzerPart4Test_AC16ContentOverMaxFileBytes_55(t)
	})
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
