package semantics

import (
	"context"
	"encoding/json"
	"os"
	"testing"
)

func body_tsResultPart2Test_76(t *testing.T, a *Analyzer, tt struct {
	name              string
	in                FileInput
	goldenFile        string
	wantErr           bool
	checkRoundTripped func(t *testing.T, r Result)
}) {
	result, err := a.AnalyzeBytes(context.Background(), tt.in)
	if tt.wantErr && err == nil {
		t.Fatalf("AnalyzeBytes(%+v): got nil err, want non-nil", tt.in)
	}
	if !tt.wantErr && err != nil {
		t.Fatalf("AnalyzeBytes(%+v): got err %v, want nil", tt.in, err)
	}
	if result == nil {
		t.Fatalf("AnalyzeBytes(%+v): got nil result, want non-nil", tt.in)
	}

	got, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatalf("AC-R6.1: marshaling the %s Result must not fail: %v", tt.name, err)
	}
	got = append(got, '\n')

	want, err := os.ReadFile(tt.goldenFile)
	if err != nil {
		t.Fatalf("AC-R6.1: reading %s must not fail: %v", tt.goldenFile, err)
	}

	if string(got) != string(want) {
		t.Errorf("AC-R6.1: %s Result JSON must match golden file byte-for-byte.\ngot:\n%s\nwant:\n%s", tt.name, got, want)
	}

	var roundTripped Result
	if err := json.Unmarshal(want, &roundTripped); err != nil {
		t.Fatalf("AC-R6.1: %s golden file must unmarshal back into a Result: %v", tt.name, err)
	}
	tt.checkRoundTripped(t, roundTripped)
}
