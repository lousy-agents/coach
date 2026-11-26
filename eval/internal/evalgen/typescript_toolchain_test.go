package evalgen_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/eval/internal/evalgen"
)

func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func writeScript(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func expectRefusal(t *testing.T, err error, words ...string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected a refusal naming %v", words)
	}
	for _, word := range words {
		if !strings.Contains(err.Error(), word) {
			t.Fatalf("refusal %q does not name %q", err, word)
		}
	}
}

func TestANodeBelowTheSupportedMajorIsRefused(t *testing.T) {
	for _, version := range []string{"v22.1.0", "v23.9.0"} {
		t.Run(version, func(t *testing.T) {
			stub := t.TempDir()
			writeScript(t, stub, "node", "echo "+version)
			t.Setenv("PATH", stub+string(os.PathListSeparator)+os.Getenv("PATH"))

			_, err := evalgen.ResolveTypeScriptTools(repositoryRoot(t))

			expectRefusal(t, err, "node", "24", version)
		})
	}
}
