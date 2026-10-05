package jsbridge

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type sigTestParityFixtures12322523 struct {
	tc parityCase
}

func (sigRecv *sigTestParityFixtures12322523) call(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("testdata", "parity", sigRecv.tc.Src))
	if err != nil {
		t.Fatalf("read src: %v", err)
	}
	resp := Handle(context.Background(), Request{
		Op:         OpAnalyze,
		Path:       sigRecv.tc.Path,
		Language:   sigRecv.tc.Language,
		ContentB64: base64.StdEncoding.EncodeToString(content),
		Options:    sigRecv.tc.Options,
	})
	got, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	got = append(got, '\n')

	expectedPath := filepath.Join("testdata", "parity", sigRecv.tc.Expected)
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
		t.Errorf("response drifted from %s\ngot:\n%s\nwant:\n%s", sigRecv.tc.Expected, got, want)
	}
}
