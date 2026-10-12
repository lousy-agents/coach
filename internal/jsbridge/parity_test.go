package jsbridge

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "regenerate testdata/parity expected files")

// parityCase is one entry of testdata/parity/manifest.json. The manifest is
// shared with js/semantics/test/parity.test.ts, so both sides of the bridge
// replay exactly the same requests.
type parityCase struct {
	Name     string  `json:"name"`
	Src      string  `json:"src"`
	Path     string  `json:"path"`
	Language string  `json:"language"`
	Options  Options `json:"options"`
	Expected string  `json:"expected"`
}

func loadManifest(t *testing.T) []parityCase {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "parity", "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var cases []parityCase
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	return cases
}

// TestParityFixtures locks the exact Response JSON for every manifest case.
// The expected files are authoritative-by-construction: `go test -run
// TestParityFixtures ./internal/jsbridge -update` regenerates them from
// whatever Handle actually emits, and this test fails on any drift, so the
// JS parity suite always compares against Go-canonical bytes.
//
// Exception: testdata/parity/ts_syntax_errors.expected.json's exact
// syntax_errors byte positions/count are backend-implementation-defined and
// not frozen across the CGO-vs-gotreesitter transition (issue #33) -- only
// parse_status == "syntax_errors" with len(syntax_errors) >= 1 is the
// frozen contract for that one case; the checked-in bytes just pin
// gotreesitter's current actual output so drift is visible, not a spec.
func TestParityFixtures(t *testing.T) {
	for _, tc := range loadManifest(t) {
		t.Run(tc.Name, func(t *testing.T) {
			expectParityFixture(t, tc)
		})
	}
}

func expectParityFixture(t *testing.T, tc parityCase) {
	content, err := os.ReadFile(filepath.Join("testdata", "parity", tc.Src))
	if err != nil {
		t.Fatalf("read src: %v", err)
	}
	resp := Handle(context.Background(), Request{
		Op:         OpAnalyze,
		Path:       tc.Path,
		Language:   tc.Language,
		ContentB64: base64.StdEncoding.EncodeToString(content),
		Options:    tc.Options,
	})
	got, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	got = append(got, '\n')

	expectedPath := filepath.Join("testdata", "parity", tc.Expected)
	if *update {
		if err := os.WriteFile(expectedPath, got, 0o644); err != nil {
			t.Fatalf("write expected: %v", err)
		}
		return
	}
	want, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("read expected (run with -update to generate): %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("response drifted from %s\ngot:\n%s\nwant:\n%s", tc.Expected, got, want)
	}
}
