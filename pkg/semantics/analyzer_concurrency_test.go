package semantics

import (
	"context"
	"sync"
	"testing"
)

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

// AC-R7.2: a single *Analyzer must be safe for concurrent callers analyzing
// different languages (Go and TypeScript) at once, since each AnalyzeBytes
// call creates and closes its own per-language Parser/Tree/Query/
// QueryCursor. Run with -race to confirm no data race.
func TestAnalyzeBytes_SafeForConcurrentGoAndTSCallers(t *testing.T) {
	a := mustNewAnalyzer(t)
	inputs := []FileInput{
		{Path: "main.go", Language: LanguageGo, Content: []byte("package main\nfunc main() {}\n")},
		{Path: "main.ts", Language: LanguageTypeScript, Content: []byte("function f(x: number) { if (x) {} }\n")},
	}

	const rounds = 4
	total := rounds * len(inputs)
	results := make([]*Result, total)
	errs := make([]error, total)

	var wg sync.WaitGroup
	wg.Add(total)
	for i := 0; i < total; i++ {
		go func(i int) {
			defer wg.Done()
			in := inputs[i%len(inputs)]
			results[i], errs[i] = a.AnalyzeBytes(context.Background(), in)
		}(i)
	}
	wg.Wait()

	for i := 0; i < total; i++ {
		in := inputs[i%len(inputs)]
		if errs[i] != nil {
			t.Errorf("AC-R7.2: concurrent AnalyzeBytes call %d (%s): got err %v, want nil", i, in.Language, errs[i])
		}
		if results[i] == nil {
			t.Fatalf("AC-R7.2: concurrent AnalyzeBytes call %d (%s): got nil result, want non-nil", i, in.Language)
		}
		if results[i].Language != in.Language {
			t.Errorf("AC-R7.2: concurrent AnalyzeBytes call %d: Language = %q, want %q", i, results[i].Language, in.Language)
		}
		if results[i].ParseStatus != ParseStatus("ok") {
			t.Errorf("AC-R7.2: concurrent AnalyzeBytes call %d (%s): ParseStatus = %q, want %q", i, in.Language, results[i].ParseStatus, "ok")
		}
	}
}
