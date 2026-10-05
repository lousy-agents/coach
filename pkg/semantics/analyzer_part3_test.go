package semantics

import (
	"bytes"
	"context"
	"encoding/json"

	"sync"
	"testing"
)

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
