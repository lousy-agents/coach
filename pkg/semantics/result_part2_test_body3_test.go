package semantics

import (
	"encoding/json"

	"os"
	"testing"
)

func body_resultPart2Test_100(t *testing.T, tt struct {
	name              string
	result            Result
	goldenFile        string
	checkRoundTripped func(t *testing.T, r Result)
}) {
	got, err := json.MarshalIndent(tt.result, "", "  ")
	if err != nil {
		t.Fatalf("AC-4.4: marshaling the %s Result must not fail: %v", tt.name, err)
	}
	got = append(got, '\n')

	want, err := os.ReadFile(tt.goldenFile)
	if err != nil {
		t.Fatalf("AC-4.4: reading %s must not fail: %v", tt.goldenFile, err)
	}

	if string(got) != string(want) {
		t.Errorf("AC-4.4: %s Result JSON must match golden file byte-for-byte.\ngot:\n%s\nwant:\n%s", tt.name, got, want)
	}

	var roundTripped Result
	if err := json.Unmarshal(want, &roundTripped); err != nil {
		t.Fatalf("AC-4.4: %s golden file must unmarshal back into a Result: %v", tt.name, err)
	}
	tt.checkRoundTripped(t, roundTripped)
}
