package codesignalcli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func coachRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found from test working directory")
		}
		dir = parent
	}
}

func packageLockRootEnginesNode(t *testing.T, path string) string {
	t.Helper()
	var doc struct {
		Packages map[string]struct {
			Engines struct {
				Node string `json:"node"`
			} `json:"engines"`
		} `json:"packages"`
	}
	if err := json.Unmarshal([]byte(readFileT(t, path)), &doc); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	root, ok := doc.Packages[""]
	if !ok {
		t.Fatalf("%s missing packages[\"\"]", path)
	}
	if root.Engines.Node == "" {
		t.Fatalf("%s packages[\"\"].engines.node is empty", path)
	}
	return root.Engines.Node
}
