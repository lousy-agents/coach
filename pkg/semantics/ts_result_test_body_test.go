package semantics

import (
	"encoding/json"
	"os"
	"testing"
)

func body_tsResultTest_44(t *testing.T, tt struct {
	name       string
	result     Result
	goldenFile string
}) {
	got, err := json.MarshalIndent(tt.result, "", "  ")
	if err != nil {
		t.Fatalf("marshaling the Go %s fixture must not fail: %v", tt.name, err)
	}
	got = append(got, '\n')

	want, err := os.ReadFile(tt.goldenFile)
	if err != nil {
		t.Fatalf("reading %s must not fail: %v", tt.goldenFile, err)
	}
	if string(got) != string(want) {
		t.Errorf("AC-R6.2: %s must remain byte-identical after adding TS support.\ngot:\n%s\nwant:\n%s", tt.goldenFile, got, want)
	}
}
